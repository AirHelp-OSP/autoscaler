package scaler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"gopkg.in/yaml.v2"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/AirHelp/autoscaler/config"
	"github.com/AirHelp/autoscaler/helper"
	"github.com/AirHelp/autoscaler/notification"
	"github.com/AirHelp/autoscaler/probe"
	"github.com/AirHelp/autoscaler/probe/nginx"
	"github.com/AirHelp/autoscaler/probe/redis"
	"github.com/AirHelp/autoscaler/probe/sqs"
)

const (
	resultsToStore                   = 10
	consecutiveZerosToZeroDeployment = 5
)

type Scaler struct {
	deploymentName string
	deployment     *appsv1.Deployment
	scalerConfig   Config

	probe          probe.Probe
	lastTenResults []int
	lastActionAt   time.Time

	k8sService K8SClient
	notifiers  []notification.Notifier

	globalConfig config.Config
	logger       *log.Entry
}

type NewScalerInput struct {
	Ctx context.Context

	DeploymentName string
	RawYamlConfig  string

	K8sService K8SClient
	Notifiers  []notification.Notifier

	GlobalConfig config.Config
	Logger       *log.Entry
}

//go:generate mockgen -destination=mock/k8s_client_mock.go -package scalerMock github.com/AirHelp/autoscaler/scaler K8SClient
type K8SClient interface {
	GetDeployment(context.Context, string) (*appsv1.Deployment, error)
	ScaleDeployment(context.Context, *appsv1.Deployment, int) (*appsv1.Deployment, error)

	nginx.K8SClient
}

var ErrProbeNotSpecified = errors.New("no probe specified for autoscaler")

func New(i NewScalerInput) (*Scaler, error) {
	s := Scaler{
		deploymentName: i.DeploymentName,
		notifiers:      i.Notifiers,
		k8sService:     i.K8sService,
		globalConfig:   i.GlobalConfig,
		logger:         i.Logger.WithFields(log.Fields{"deployment": i.DeploymentName}),
	}

	s.logger.Debug("Starting prefetch of deployment")
	deployment, err := s.k8sService.GetDeployment(i.Ctx, s.deploymentName)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to fetch deployment")
		return &s, err
	}
	s.deployment = deployment
	s.logger.Debug("Finished fetching deployment")

	scalerConfig := NewScalerConfigWithDefaults()
	err = yaml.Unmarshal([]byte(i.RawYamlConfig), &scalerConfig)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to parse config")
		s.logger.WithError(err).Debugf("Raw config: %+v", i.RawYamlConfig)
		return &s, err
	}
	s.scalerConfig = scalerConfig
	s.logger.Debug(fmt.Sprintf("Parsed autoscaler config: %+v", scalerConfig))

	var requestedProbe probe.Probe
	s.logger.Debug("Initializing probe")

	switch {
	case s.scalerConfig.Sqs != nil:
		requestedProbe, err = sqs.New(s.scalerConfig.Sqs, s.logger)
	case s.scalerConfig.Redis != nil:
		requestedProbe, err = redis.New(s.scalerConfig.Redis, s.logger)
	case s.scalerConfig.Nginx != nil:
		requestedProbe, err = nginx.New(s.scalerConfig.Nginx, i.K8sService, s.deployment, s.logger)
	default:
		return &s, ErrProbeNotSpecified
	}

	if err != nil {
		return &s, err
	}

	s.probe = requestedProbe
	s.logger.Debug(fmt.Sprintf("Initialized probe: %v", s.probe.Kind()))

	return &s, nil
}

func (s *Scaler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.scalerConfig.CheckInterval)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			s.logger.Debug("Shutting down scaler")

			return
		case <-ticker.C:
			s.logger.Debug("Interval tick")
			timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			s.perform(timeoutCtx)
			cancel()
		}
	}
}

func (s *Scaler) perform(ctx context.Context) {
	s.logger.Debug("Starting to evaluate autoscaling needs")

	currentTime := time.Now()

	probeResult, err := s.probe.Check(ctx)
	if err != nil {
		s.logger.WithError(err).Warn("Probe failed, skipping autoscaling")
		return
	}

	s.logger.Debugf("Probe %v returned %d", s.probe.Kind(), probeResult)
	s.lastTenResults = append(s.lastTenResults, probeResult)
	s.lastTenResults = helper.Last(s.lastTenResults, resultsToStore)
	s.logger.Debugf("Last 10 probe runs %+v", s.lastTenResults)

	if err = s.refreshDeployment(ctx); err != nil {
		s.logger.WithError(err).Warn("Failed to refresh deployment")
		return
	}

	if s.isDeploymentNotAtTargetReplicas() {
		s.logger.Warn("Deployment available replicas not at target. won't adjust")
		return
	}

	if s.isAutoscalerInCooldown(currentTime) {
		s.logger.Debug("Autoscaler in cooldown, not making decision")
		return
	}

	decision := s.calculateDecision(probeResult)
	s.logger.Info(decision.toText())

	if decision.value != remain {
		_, err = s.k8sService.ScaleDeployment(ctx, s.deployment, decision.target)

		if err != nil {
			s.logger.WithError(err).Warn("Updating replication failed")
		}

		s.lastActionAt = currentTime

		if len(s.notifiers) > 0 {
			notificationPayload := notification.NotificationPayload{
				Decision:         decision.toText(),
				LastProbeResults: s.lastTenResults,
				DeploymentName:   s.deployment.GetName(),
				ChangedAt:        currentTime,
				Source:           s.probe.Kind(),
				Namespace:        s.globalConfig.Namespace,
				Environment:      s.globalConfig.Environment,
			}

			for _, notifier := range s.notifiers {
				if err := notifier.Notify(ctx, notificationPayload); err != nil {
					s.logger.WithError(err).Warnf("Failed to notify %v", notifier.Kind())
				}
			}
		}
	}

	s.logger.Debug("Finished evaluating autoscaling needs")
}

func (s *Scaler) calculateDecision(probeResult int) decision {
	currentReplicasCount := int(*s.deployment.Spec.Replicas)

	d := decision{
		current: currentReplicasCount,
		value:   remain,
		target:  currentReplicasCount,
	}

	desiredReplicasCount := int(math.Ceil(float64(probeResult) / float64(s.scalerConfig.Threshold)))

	s.logger.Debugf("Current replicas count: %d, desired replicas count: %d", currentReplicasCount, desiredReplicasCount)

	minMaxConfig := s.scalerConfig.ApplicableLimits(s.logger)

	if currentReplicasCount == desiredReplicasCount {
		s.logger.Debug("Current replicas same as desired, deployment remain the same")
	} else if currentReplicasCount < desiredReplicasCount {
		s.logger.Debug("Current replicas lower than desired")
		if currentReplicasCount+1 <= minMaxConfig.MaximumNumberOfPods {
			s.logger.Debug("Scale up available, decided to scale up")
			d.value = scaleUp
			d.target = currentReplicasCount + 1
		} else {
			s.logger.Debug("Scale up unavailable, reached maximum number of pods")
		}
	} else if currentReplicasCount > desiredReplicasCount {
		s.logger.Debug("Current replicas higher than desired")
		if currentReplicasCount-1 >= minMaxConfig.MinimumNumberOfPods {
			if currentReplicasCount-1 == 0 {
				// Check if last `consecutiveZerosToZeroDeployment` are zero read outs
				if helper.OnlyZeros(helper.Last(s.lastTenResults, consecutiveZerosToZeroDeployment)) {
					s.logger.Debug("Scalling down to zero, consecutive zero reads")
					d.value = scaleDown
					d.target = currentReplicasCount - 1
				} else {
					s.logger.Debug("Scaling down to zero unavailable, no consecutive zero reads")
				}
			} else {
				s.logger.Debug("Scale down available, decided to scale down")
				d.value = scaleDown
				d.target = currentReplicasCount - 1
			}
		} else {
			s.logger.Debug("Scale down unavailable, reached minimum number of pods")
		}
	}

	return d
}

func (s *Scaler) refreshDeployment(ctx context.Context) error {
	s.logger.Debug("Starting refreshing of deployment")
	deployment, err := s.k8sService.GetDeployment(ctx, s.deploymentName)
	if err != nil {
		return err
	}
	s.deployment = deployment
	s.logger.Debug("Done refreshing of deployment")
	return nil
}

func (s *Scaler) isDeploymentNotAtTargetReplicas() bool {
	return s.deployment.Status.Replicas != s.deployment.Status.AvailableReplicas
}

func (s *Scaler) isAutoscalerInCooldown(currentTime time.Time) bool {
	return !s.lastActionAt.IsZero() && s.deployment.Status.Replicas != int32(0) && s.lastActionAt.After(currentTime.Add(-s.scalerConfig.CooldownPeriod))
}
