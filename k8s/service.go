package k8s

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AirHelp/autoscaler/events"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Service struct {
	Client    kubernetes.Interface
	Namespace string
}

func New(namespace string) (*Service, error) {
	svc := &Service{
		Namespace: namespace,
	}

	config, err := generateKubeConfig()
	if err != nil {
		return svc, err
	}

	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		return svc, err
	}

	svc.Client = c

	return svc, nil
}

func (s *Service) GetDeployments(ctx context.Context) (*appsv1.DeploymentList, error) {
	return s.Client.AppsV1().Deployments(s.Namespace).List(ctx, metav1.ListOptions{})
}

func (s *Service) GetDeployment(ctx context.Context, name string) (*appsv1.Deployment, error) {
	return s.Client.AppsV1().Deployments(s.Namespace).Get(ctx, name, metav1.GetOptions{})
}

func (s *Service) GetConfigMap(ctx context.Context, name string) (*corev1.ConfigMap, error) {
	return s.Client.CoreV1().ConfigMaps(s.Namespace).Get(ctx, name, metav1.GetOptions{})
}

func (s *Service) ScaleDeployment(ctx context.Context, deployment *appsv1.Deployment, newReplicasCount int) (*appsv1.Deployment, error) {
	replicas := int32(newReplicasCount)
	deployment.Spec.Replicas = &replicas

	return s.Client.AppsV1().Deployments(s.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
}

func (s *Service) GetPodsFromDeployment(ctx context.Context, deployment *appsv1.Deployment, additionalLabels map[string]string) (*corev1.PodList, error) {
	selectorLabels := labels.Set(deployment.Spec.Selector.MatchLabels)

	for label, value := range additionalLabels {
		selectorLabels[label] = value
	}

	options := metav1.ListOptions{LabelSelector: selectorLabels.String()}

	return s.Client.CoreV1().Pods(s.Namespace).List(ctx, options)
}

func (s *Service) CreateScalingEvent(ctx context.Context, deployment *appsv1.Deployment, eventData *events.ScalingEventData) error {
	now := metav1.NewTime(time.Now())

	eventType := "Normal"
	if eventData.ScalingReason == "at_max_limit" || eventData.ScalingReason == "at_min_limit" {
		eventType = "Warning"
	}

	reason := "Scaled" + capitalizeFirst(eventData.ScalingDirection)

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s.%d", deployment.Name, now.UnixNano()),
			Namespace: s.Namespace,

			Labels: map[string]string{
				"probe-type":           eventData.ProbeType,
				"scaling-direction":    eventData.ScalingDirection,
				"scaling-reason":       eventData.ScalingReason,
				"scaled-deployment":    eventData.DeploymentName,
				"scaling-environment":  eventData.Environment,
				"scaling-namespace":    eventData.Namespace,
			},

			Annotations: map[string]string{
				"current-replicas":  strconv.Itoa(eventData.CurrentReplicas),
				"target-replicas":   strconv.Itoa(eventData.TargetReplicas),
				"probe-value":       strconv.Itoa(eventData.ProbeValue),
				"threshold":         strconv.Itoa(eventData.Threshold),
				"load-percentage":   fmt.Sprintf("%.2f", eventData.LoadPercentage),
				"min-pods":          strconv.Itoa(eventData.MinPods),
				"max-pods":          strconv.Itoa(eventData.MaxPods),
				"scaling-timestamp": strconv.FormatInt(eventData.Timestamp, 10),
			},
		},

		InvolvedObject: corev1.ObjectReference{
			Kind:            "Deployment",
			APIVersion:      "apps/v1",
			Name:            deployment.Name,
			Namespace:       deployment.Namespace,
			UID:             deployment.UID,
			ResourceVersion: deployment.ResourceVersion,
		},

		Reason:  reason,
		Message: eventData.HumanMessage,
		Source: corev1.EventSource{
			Component: "autoscaler",
			Host:      fmt.Sprintf("autoscaler-%s", eventData.Environment),
		},

		FirstTimestamp: now,
		LastTimestamp:  now,
		Count:          1,
		Type:           eventType,
	}

	_, err := s.Client.CoreV1().Events(s.Namespace).Create(ctx, event, metav1.CreateOptions{})
	return err
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return fmt.Sprintf("%c%s", s[0]-32, s[1:])
}

func generateKubeConfig() (*rest.Config, error) {
	var config *rest.Config

	config, err := rest.InClusterConfig()

	if !errors.Is(err, rest.ErrNotInCluster) {
		return config, err
	}

	return clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
}
