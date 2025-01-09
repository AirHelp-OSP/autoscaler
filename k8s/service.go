package k8s

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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

func (s *Service) GetPodsFromDeployment(ctx context.Context, deployment *appsv1.Deployment, additionalLabels map[string]string) (*v1.PodList, error) {
	selectorLabels := labels.Set(deployment.Spec.Selector.MatchLabels)

	for label, value := range additionalLabels {
		selectorLabels[label] = value
	}

	options := metav1.ListOptions{LabelSelector: selectorLabels.String()}

	return s.Client.CoreV1().Pods(s.Namespace).List(ctx, options)
}

func generateKubeConfig() (*rest.Config, error) {
	var config *rest.Config

	config, err := rest.InClusterConfig()

	if !errors.Is(err, rest.ErrNotInCluster) {
		return config, err
	}

	return clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
}
