package scaler

import (
	"context"
	"errors"
	"math"
	"time"

	"gopkg.in/yaml.v2"

	"go.uber.org/zap"
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
}

type NewScalerInput struct {
	Ctx context.Context

	DeploymentName string
	RawYamlConfig  string

	K8sService K8SClient
	Notifiers  []notification.Notifier

	GlobalConfig config.Config
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
	}

	zap.S().With("deployment", i.DeploymentName).Debug("starting prefetch of deployment")
	deployment, err := s.k8sService.GetDeployment(i.Ctx, s.deploymentName)
	if err != nil {
		zap.S().With("deployment", i.DeploymentName, "error", err).Errorf("failed to fetch deployment")
		return &s, err
	}
	s.deployment = deployment
	zap.S().With("deployment", i.DeploymentName).Debug("finished prefetch of deployment")

	scalerConfig := NewScalerConfigWithDefaults()
	if err := yaml.Unmarshal([]byte(i.RawYamlConfig), &scalerConfig); err != nil {
		zap.S().With("deployment", i.DeploymentName, "error", err).Warn("failed to parse config")
		zap.S().With("deployment", i.DeploymentName, "error", err).Debugf("raw config: %+v", i.RawYamlConfig)
		return &s, err
	}
	s.scalerConfig = scalerConfig
	zap.S().With("deployment", i.DeploymentName).Debugf("parsed autoscaler config: %+v", scalerConfig)

	var requestedProbe probe.Probe
	zap.S().With("deployment", i.DeploymentName).Debug("initializing probe")

	switch {
	case s.scalerConfig.Sqs != nil:
		requestedProbe, err = sqs.New(s.scalerConfig.Sqs)
	case s.scalerConfig.Redis != nil:
		requestedProbe, err = redis.New(s.scalerConfig.Redis)
	case s.scalerConfig.Nginx != nil:
		requestedProbe, err = nginx.New(s.scalerConfig.Nginx, i.K8sService, s.deployment)
	default:
		return &s, ErrProbeNotSpecified
	}

	if err != nil {
		return &s, err
	}

	s.probe = requestedProbe
	zap.S().With("deployment", i.DeploymentName).Debugf("initialized probe: %s", s.probe.Kind())

	return &s, nil
}

func (s *Scaler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.scalerConfig.CheckInterval)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			zap.S().With("deployment", s.deploymentName).Debug("shutting down scaler")

			return
		case <-ticker.C:
			zap.S().With("deployment", s.deploymentName).Debug("interval tick")
			timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			s.perform(timeoutCtx)
			cancel()
		}
	}
}

func (s *Scaler) perform(ctx context.Context) {
	zap.S().With("deployment", s.deploymentName).Debug("starting to evaluate autoscaling needs")

	currentTime := time.Now()

	probeResult, err := s.probe.Check(ctx)
	if err != nil {
		zap.S().With("deployment", s.deploymentName, "error", err).Warnf("skipping autoscaling, probe failed: %v", err)
		return
	}

	zap.S().With("deployment", s.deploymentName).Debugf("probe %s returned %d", s.probe.Kind(), probeResult)
	s.lastTenResults = append(s.lastTenResults, probeResult)
	s.lastTenResults = helper.Last(s.lastTenResults, resultsToStore)
	zap.S().With("deployment", s.deploymentName).Debugf("last 10 probe runs %+v", s.lastTenResults)

	if err = s.refreshDeployment(ctx); err != nil {
		zap.S().With("deployment", s.deploymentName, "error", err).Warnf("failed to refresh deployment: %v", err)
		return
	}

	if s.isDeploymentNotAtTargetReplicas() {
		zap.S().With("deployment", s.deploymentName).Warn("deployment available replicas not at target. won't adjust")
		return
	}

	if s.isAutoscalerInCooldown(currentTime) {
		zap.S().With("deployment", s.deploymentName).Debug("autoscaler in cooldown, not making decision")
		return
	}

	decision := s.calculateDecision(probeResult)
	zap.S().With("deployment", s.deploymentName).Infof("decision: %s", decision.toText())

	if decision.value != remain {
		_, err = s.k8sService.ScaleDeployment(ctx, s.deployment, decision.target)

		if err != nil {
			zap.S().With("deployment", s.deploymentName, "error", err).Warn("updating replication failed")
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
					zap.S().With("deployment", s.deploymentName, "error", err).Warnf("failed to notify %v", notifier.Kind())
				}
			}
		}
	}

	zap.S().With("deployment", s.deploymentName).Debug("finished evaluating autoscaling needs")
}

func (s *Scaler) calculateDecision(probeResult int) decision {
	currentReplicasCount := int(*s.deployment.Spec.Replicas)

	d := decision{
		current: currentReplicasCount,
		value:   remain,
		target:  currentReplicasCount,
	}

	desiredReplicasCount := int(math.Ceil(float64(probeResult) / float64(s.scalerConfig.Threshold)))

	zap.S().With("deployment", s.deploymentName).Debugf("current replicas count: %d, desired replicas count: %d", probeResult, desiredReplicasCount)

	minMaxConfig := s.scalerConfig.ApplicableLimits()

	if currentReplicasCount == desiredReplicasCount {
		zap.S().With("deployment", s.deploymentName).Debug("current replicas same as desired, deployment remain the same")
	} else if currentReplicasCount < desiredReplicasCount {
		zap.S().With("deployment", s.deploymentName).Debug("current replicas lower than desired")
		if currentReplicasCount+1 <= minMaxConfig.MaximumNumberOfPods {
			zap.S().With("deployment", s.deploymentName).Debug("scale up available, decided to scale up")
			d.value = scaleUp
			d.target = currentReplicasCount + 1
		} else {
			zap.S().With("deployment", s.deploymentName).Debug("scale up unavailable, reached maximum number of pods")
		}
	} else if currentReplicasCount > desiredReplicasCount {
		zap.S().With("deployment", s.deploymentName).Debug("current replicas higher than desired")
		if currentReplicasCount-1 >= minMaxConfig.MinimumNumberOfPods {
			if currentReplicasCount-1 == 0 {
				// Check if last `consecutiveZerosToZeroDeployment` are zero read outs
				if helper.OnlyZeros(helper.Last(s.lastTenResults, consecutiveZerosToZeroDeployment)) {
					zap.S().With("deployment", s.deploymentName).Debug("scalling down to zero, consecutive zero reads")
					d.value = scaleDown
					d.target = currentReplicasCount - 1
				} else {
					zap.S().With("deployment", s.deploymentName).Debug("scaling down to zero unavailable, no consecutive zero reads")
				}
			} else {
				zap.S().With("deployment", s.deploymentName).Debug("scale down available, decided to scale down")
				d.value = scaleDown
				d.target = currentReplicasCount - 1
			}
		} else {
			zap.S().With("deployment", s.deploymentName).Debug("scale down unavailable, reached minimum number of pods")
		}
	}

	return d
}

func (s *Scaler) refreshDeployment(ctx context.Context) error {
	zap.S().With("deployment", s.deploymentName).Debug("starting refreshing of deployment")
	deployment, err := s.k8sService.GetDeployment(ctx, s.deploymentName)
	if err != nil {
		return err
	}
	s.deployment = deployment
	zap.S().With("deployment", s.deploymentName).Debug("finished refreshing of deployment")
	return nil
}

func (s *Scaler) isDeploymentNotAtTargetReplicas() bool {
	return s.deployment.Status.Replicas != s.deployment.Status.AvailableReplicas
}

func (s *Scaler) isAutoscalerInCooldown(currentTime time.Time) bool {
	return !s.lastActionAt.IsZero() && s.deployment.Status.Replicas != int32(0) && s.lastActionAt.After(currentTime.Add(-s.scalerConfig.CooldownPeriod))
}
