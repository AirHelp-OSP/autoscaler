package k8s

import (
	"context"

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
})
