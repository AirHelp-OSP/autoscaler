package nginx

import (
	"context"
	"errors"
	"math"
	"time"

	"go.uber.org/zap"
	"github.com/AirHelp/autoscaler/nginx_stats"
	"github.com/AirHelp/autoscaler/stat"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

const (
	defaultEndpoint         = "/stats/active_connections"
	defaultConsecutiveReads = 3
	defaultStatistic        = "maximum"
	defaultTimeout          = 1 * time.Second
	defaultRequestTimeout   = 3 * time.Second
)

type Config struct {
	Endpoint         string        `yaml:"endpoint"`
	Statistic        string        `yaml:"statistic"`
	ConsecutiveReads int           `yaml:"consecutive_reads"`
	Timeout          time.Duration `yaml:"timeout"`
	RequestTimeout   time.Duration `yaml:"request_timeout"`
}

type Probe struct {
	k8sService  K8SClient
	nginxClient NginxClient

	deployment *appsv1.Deployment

	statistic        string
	consecutiveReads int
	timeout          time.Duration
	requestTimeout   time.Duration
}

//go:generate mockgen -destination=mock/k8s_client_mock.go -package nginxMock github.com/AirHelp/autoscaler/probe/nginx K8SClient
type K8SClient interface {
	GetPodsFromDeployment(context.Context, *appsv1.Deployment, map[string]string) (*v1.PodList, error)
}

//go:generate mockgen -destination=mock/nginx_client_mock.go -package nginxMock github.com/AirHelp/autoscaler/probe/nginx NginxClient
type NginxClient interface {
	GetActiveConnections(context.Context, string) (int, error)
}

func New(config *Config, k8sSvc K8SClient, deployment *appsv1.Deployment) (*Probe, error) {
	endpoint := config.Endpoint

	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	consecutiveReads := config.ConsecutiveReads

	if consecutiveReads == 0 {
		consecutiveReads = defaultConsecutiveReads
	}

	statistic := config.Statistic

	if statistic == "" {
		statistic = defaultStatistic
	}

	timeout := config.Timeout

	if timeout == time.Duration(0) {
		timeout = defaultTimeout
	}

	requestTimeout := config.RequestTimeout

	if requestTimeout == time.Duration(0) {
		requestTimeout = defaultRequestTimeout
	}

	nginxClient, err := nginx_stats.NewClient(endpoint)

	if err != nil {
		return nil, err
	}

	return &Probe{
		k8sService:  k8sSvc,
		nginxClient: nginxClient,

		deployment: deployment,

		statistic:        statistic,
		consecutiveReads: consecutiveReads,
		timeout:          timeout,
		requestTimeout:   requestTimeout,
	}, nil
}

func (p *Probe) Kind() string {
	return "nginx"
}

var additionalExpectedWebPodLabels = map[string]string{
	"type": "web",
}

func (p *Probe) Check(ctx context.Context) (int, error) {
	var acc int

	pods, err := p.k8sService.GetPodsFromDeployment(ctx, p.deployment, additionalExpectedWebPodLabels)

	if err != nil {
		zap.S().Warnf("failed to get pods for deployment: %v", err)
		return 0, err
	}

	zap.S().Debugf("found %d pods for deployment", len(pods.Items))

	allPodsReadyAndOperational := true
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			allPodsReadyAndOperational = false
			break
		}

		for _, condition := range pod.Status.Conditions {
			if condition.Status == corev1.ConditionFalse {
				allPodsReadyAndOperational = false
				break
			}
		}
	}

	if !allPodsReadyAndOperational {
		zap.S().Warn("at least one pod found not in ready state")
		return 0, errors.New("deployment not fully operational")
	}

	zap.S().Debug("all pods ready and serving traffic")

	var fullResults [][]int

	for i := 0; i < p.consecutiveReads; i++ {
		fullResults = append(fullResults, []int{})

		result, err := p.fetchActiveConnectionsFromPods(pods)

		if err != nil {
			zap.S().Warnf("failed to fetch active connections: %v", err)
			return 0, err
		}

		// groups each consecutive run in one collection with results from all pods
		for _, activeConnections := range result {
			fullResults[i] = append(fullResults[i], activeConnections)
		}

		time.Sleep(p.timeout)
	}

	connections := []int{}
	for _, results := range fullResults {
		sum := 0

		for _, result := range results {
			sum += result
		}

		connections = append(connections, sum)
	}

	zap.S().Debugf("connections slice gathered by probe: %+v", connections)

	switch p.statistic {
	case "average":
		acc = int(math.Ceil(stat.Average(connections)))
	case "median":
		acc = int(math.Ceil(stat.Median(connections)))
	case "maximum":
		acc = stat.Maximum(connections)
	}

	return acc, nil
}

type nginxStatsResult struct {
	pod               corev1.Pod
	activeConnections int
	err               error
}

func (p *Probe) fetchActiveConnectionsFromPods(pods *corev1.PodList) (map[string]int, error) {
	results := map[string]int{}

	statsChan := make(chan nginxStatsResult)

	getStatsFunc := func(pod corev1.Pod) {
		ctx, cancel := context.WithTimeout(context.Background(), p.requestTimeout)
		defer cancel()

		activeConnections, err := p.nginxClient.GetActiveConnections(ctx, pod.Status.PodIP)

		zap.S().With("pod", pod.ObjectMeta.Name).Debugf("fetched active connections from pod: %+v", activeConnections)

		statsChan <- nginxStatsResult{
			pod:               pod,
			activeConnections: activeConnections,
			err:               err,
		}
	}

	for _, pod := range pods.Items {
		go getStatsFunc(pod)
	}

	for range pods.Items {
		result := <-statsChan

		if result.err != nil {
			return map[string]int{}, result.err
		}

		results[result.pod.ObjectMeta.Name] = result.activeConnections
	}

	return results, nil
}
