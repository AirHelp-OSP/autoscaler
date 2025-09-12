package k8s

import (
	"context"

	"github.com/AirHelp/autoscaler/events"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Service with fake client", func() {
	var (
		namespace string
		client    *fake.Clientset
		ctx       context.Context
	)

	BeforeEach(func() {
		namespace = "ugabuga"
		ctx = context.TODO()
	})

	Describe("GetDeployments()", func() {
		var firstDeployment *appsv1.Deployment
		var secondDeployment *appsv1.Deployment
		var otherNamespaceDeployment *appsv1.Deployment

		It("returns deployments", func() {
			r := int32(1)

			firstDeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "first-deployment",
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{Replicas: &r},
			}

			secondDeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "second-deployment",
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{Replicas: &r},
			}

			otherNamespaceDeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "third-deployment",
					Namespace: "some-other-namespace",
				},
				Spec: appsv1.DeploymentSpec{Replicas: &r},
			}

			client = fake.NewSimpleClientset(firstDeployment, secondDeployment, otherNamespaceDeployment)

			svc := Service{
				Client:    client,
				Namespace: namespace,
			}

			res, err := svc.GetDeployments(ctx)

			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(&appsv1.DeploymentList{Items: []appsv1.Deployment{
				*firstDeployment,
				*secondDeployment,
			}}))
		})
	})

	Describe("GetDeployment()", func() {
		var deployment *appsv1.Deployment

		It("when deployment in given namespace is present it properly returns it", func() {
			r := int32(2)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-deployment",
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{Replicas: &r},
			}

			client = fake.NewSimpleClientset(deployment)

			svc := Service{
				Client:    client,
				Namespace: namespace,
			}

			res, err := svc.GetDeployment(ctx, "some-deployment")

			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(deployment))
		})

		It("when deployment is not found", func() {
			emptyDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{},
			}
			svc := Service{
				Client:    fake.NewSimpleClientset(emptyDeployment),
				Namespace: namespace,
			}

			res, err := svc.GetDeployment(ctx, "some-deployment")

			Expect(err.Error()).To(Equal("deployments.apps \"some-deployment\" not found"))
			Expect(res).To(Equal(emptyDeployment))
		})
	})

	Describe("GetPodsFromDeployment()", func() {
		var (
			deployment *appsv1.Deployment

			firstPodOfDeployment  *corev1.Pod
			secondPodOfDeployment *corev1.Pod
			otherPod              *corev1.Pod
		)

		BeforeEach(func() {
			r := int32(2)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-deployment",
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &r,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"role": "some-role",
							"app":  "some-app",
						},
					},
				},
			}

			firstPodOfDeployment = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-deployment-1231-sxada",
					Namespace: namespace,
					Labels: map[string]string{
						"role":  "some-role",
						"app":   "some-app",
						"chart": "some-chart",
						"team":  "test",
						"type":  "web",
					},
				},
			}

			secondPodOfDeployment = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-deployment-1231-dyerty",
					Namespace: namespace,
					Labels: map[string]string{
						"role":       "some-role",
						"app":        "some-app",
						"chart":      "some-chart",
						"team":       "test",
						"type":       "web",
						"additional": "1",
					},
				},
			}

			otherPod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-deployment-aaaa-bbbb",
					Namespace: namespace,
					Labels: map[string]string{
						"role":  "some-other-role",
						"app":   "some-other-app",
						"chart": "some-chart",
						"team":  "test",
						"type":  "web",
					},
				},
			}
		})

		It("Properly returns only matched pods", func() {
			client = fake.NewSimpleClientset(
				deployment,
				firstPodOfDeployment,
				secondPodOfDeployment,
				otherPod,
			)

			svc := Service{
				Client:    client,
				Namespace: namespace,
			}

			res, err := svc.GetPodsFromDeployment(ctx, deployment, map[string]string{})

			Expect(err).ToNot(HaveOccurred())

			Expect(res.Items).To(ContainElement(*firstPodOfDeployment))
			Expect(res.Items).To(ContainElement(*secondPodOfDeployment))
			Expect(res.Items).ToNot(ContainElement(*otherPod))
		})

		Context("when additional labels requested", func() {
			It("Properly builds kubernetes request using dedicated APIs", func() {
				additionalLabels := map[string]string{
					"type":       "web",
					"additional": "1",
				}
				client = fake.NewSimpleClientset(
					deployment,
					firstPodOfDeployment,
					secondPodOfDeployment,
					otherPod,
				)

				svc := Service{
					Client:    client,
					Namespace: namespace,
				}

				res, err := svc.GetPodsFromDeployment(ctx, deployment, additionalLabels)

				Expect(err).To(Not(HaveOccurred()))
				Expect(res).To(Equal(&corev1.PodList{Items: []corev1.Pod{
					*secondPodOfDeployment,
				}}))
			})
		})
	})

	Describe("GetConfigMap()", func() {
		var configMap *corev1.ConfigMap

		It("when config map in given namespace is present it properly returns it", func() {
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-config-map",
					Namespace: namespace,
				},
				Data: map[string]string{"test": "1"},
			}

			client = fake.NewSimpleClientset(configMap)

			svc := Service{
				Client:    client,
				Namespace: namespace,
			}

			res, err := svc.GetConfigMap(ctx, "some-config-map")

			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(configMap))
		})

		It("when config map is not found", func() {
			emptyConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{},
			}
			svc := Service{
				Client:    fake.NewSimpleClientset(emptyConfigMap),
				Namespace: namespace,
			}
			res, err := svc.GetConfigMap(ctx, "some-config-map")

			Expect(err.Error()).To(Equal("configmaps \"some-config-map\" not found"))
			Expect(res).To(Equal(emptyConfigMap))
		})
	})

	Describe("GetDeployment()", func() {
		var deployment *appsv1.Deployment

		BeforeEach(func() {
			r := int32(2)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-deployment",
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{Replicas: &r},
			}

			client = fake.NewSimpleClientset(deployment)
		})

		It("scales up deployment when requested", func() {
			svc := Service{
				Client:    client,
				Namespace: namespace,
			}

			res, err := svc.ScaleDeployment(ctx, deployment, 3)

			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(deployment))
			Expect(*deployment.Spec.Replicas).To(Equal(int32(3)))

			// Check if change is applied on k8s side too
			deploymentFromApi, _ := client.AppsV1().Deployments(namespace).Get(ctx, deployment.ObjectMeta.Name, metav1.GetOptions{})
			Expect(*deploymentFromApi.Spec.Replicas).To(Equal(int32(3)))
		})

		It("scales down deployment when requested", func() {
			svc := Service{
				Client:    client,
				Namespace: namespace,
			}

			res, err := svc.ScaleDeployment(ctx, deployment, 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(deployment))
			Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))

			// Check if change is applied on k8s side too
			deploymentFromApi, _ := client.AppsV1().Deployments(namespace).Get(ctx, deployment.ObjectMeta.Name, metav1.GetOptions{})
			Expect(*deploymentFromApi.Spec.Replicas).To(Equal(int32(1)))
		})
	})

	Describe("CreateScalingEvent()", func() {
		var deployment *appsv1.Deployment

		BeforeEach(func() {
			r := int32(2)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-deployment",
					Namespace:       namespace,
					UID:             "test-uid-123",
					ResourceVersion: "12345",
				},
				Spec: appsv1.DeploymentSpec{Replicas: &r},
			}

			client = fake.NewSimpleClientset(deployment)
		})

		It("successfully creates a rich scaling event for scale up", func() {
			svc := Service{
				Client:    client,
				Namespace: namespace,
			}

			eventData := &events.ScalingEventData{
				CurrentReplicas:  2,
				TargetReplicas:   3,
				ProbeValue:       150,
				Threshold:        100,
				LoadPercentage:   150.0,
				MinPods:          1,
				MaxPods:          10,
				ScalingDirection: "up",
				ProbeType:        "sqs",
				ScalingReason:    "high_load",
				DeploymentName:   "test-deployment",
				Namespace:        namespace,
				Environment:      "test",
				Timestamp:        1736432512,
				HumanMessage:     "Scaled up from 2 to 3 replicas | sqs: 150/100 (150.0%) | Reason: high_load",
			}

			err := svc.CreateScalingEvent(ctx, deployment, eventData)

			Expect(err).ToNot(HaveOccurred())

			kubeEvents, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(kubeEvents.Items).To(HaveLen(1))

			event := kubeEvents.Items[0]
			Expect(event.InvolvedObject.Kind).To(Equal("Deployment"))
			Expect(event.InvolvedObject.APIVersion).To(Equal("apps/v1"))
			Expect(event.InvolvedObject.Name).To(Equal("test-deployment"))
			Expect(event.InvolvedObject.Namespace).To(Equal(namespace))
			Expect(event.InvolvedObject.UID).To(Equal(deployment.UID))
			Expect(event.InvolvedObject.ResourceVersion).To(Equal(deployment.ResourceVersion))
			Expect(event.Reason).To(Equal("ScaledUp"))
			Expect(event.Message).To(Equal("Scaled up from 2 to 3 replicas | sqs: 150/100 (150.0%) | Reason: high_load"))
			Expect(event.Type).To(Equal("Normal"))
			Expect(event.Source.Component).To(Equal("autoscaler"))
			Expect(event.Count).To(Equal(int32(1)))

			Expect(event.Labels["probe-type"]).To(Equal("sqs"))
			Expect(event.Labels["scaling-direction"]).To(Equal("up"))
			Expect(event.Labels["scaling-environment"]).To(Equal("test"))
			Expect(event.Annotations["current-replicas"]).To(Equal("2"))
			Expect(event.Annotations["target-replicas"]).To(Equal("3"))
			Expect(event.Annotations["probe-value"]).To(Equal("150"))
			Expect(event.Annotations["load-percentage"]).To(Equal("150.00"))
		})

		It("successfully creates a scaling event for scale down", func() {
			svc := Service{
				Client:    client,
				Namespace: namespace,
			}

			eventData := &events.ScalingEventData{
				CurrentReplicas:  2,
				TargetReplicas:   1,
				ProbeValue:       25,
				Threshold:        100,
				LoadPercentage:   25.0,
				MinPods:          1,
				MaxPods:          10,
				ScalingDirection: "down",
				ProbeType:        "sqs",
				ScalingReason:    "low_load",
				DeploymentName:   "test-deployment",
				Namespace:        namespace,
				Environment:      "test",
				Timestamp:        1736432512,
				HumanMessage:     "Scaled down from 2 to 1 replicas | sqs: 25/100 (25.0%) | Reason: low_load",
			}

			err := svc.CreateScalingEvent(ctx, deployment, eventData)

			Expect(err).ToNot(HaveOccurred())

			kubeEvents, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(kubeEvents.Items).To(HaveLen(1))

			event := kubeEvents.Items[0]
			Expect(event.Reason).To(Equal("ScaledDown"))
			Expect(event.Message).To(Equal("Scaled down from 2 to 1 replicas | sqs: 25/100 (25.0%) | Reason: low_load"))
			Expect(event.Type).To(Equal("Normal"))
		})

		It("creates events with unique names based on timestamp", func() {
			svc := Service{
				Client:    client,
				Namespace: namespace,
			}

			eventData1 := &events.ScalingEventData{
				CurrentReplicas:  1,
				TargetReplicas:   2,
				ProbeValue:       120,
				Threshold:        100,
				LoadPercentage:   120.0,
				ScalingDirection: "up",
				ProbeType:        "sqs",
				ScalingReason:    "high_load",
				DeploymentName:   "test-deployment",
				Namespace:        namespace,
				Environment:      "test",
				HumanMessage:     "first scaling event",
			}
			err1 := svc.CreateScalingEvent(ctx, deployment, eventData1)
			Expect(err1).ToNot(HaveOccurred())

			eventData2 := &events.ScalingEventData{
				CurrentReplicas:  2,
				TargetReplicas:   1,
				ProbeValue:       50,
				Threshold:        100,
				LoadPercentage:   50.0,
				ScalingDirection: "down",
				ProbeType:        "sqs",
				ScalingReason:    "low_load",
				DeploymentName:   "test-deployment",
				Namespace:        namespace,
				Environment:      "test",
				HumanMessage:     "second scaling event",
			}
			err2 := svc.CreateScalingEvent(ctx, deployment, eventData2)
			Expect(err2).ToNot(HaveOccurred())

			kubeEvents, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(kubeEvents.Items).To(HaveLen(2))
			Expect(kubeEvents.Items[0].Name).To(ContainSubstring("test-deployment."))
			Expect(kubeEvents.Items[1].Name).To(ContainSubstring("test-deployment."))
			Expect(kubeEvents.Items[0].Name).ToNot(Equal(kubeEvents.Items[1].Name))
		})
	})
})
