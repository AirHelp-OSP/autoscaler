package scaler

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/AirHelp/autoscaler/probe/sqs/mocks"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AirHelp/autoscaler/config"
	"github.com/AirHelp/autoscaler/notification"
	notificationMock "github.com/AirHelp/autoscaler/notification/mock"
	probeMock "github.com/AirHelp/autoscaler/probe/mock"
	sqsProbe "github.com/AirHelp/autoscaler/probe/sqs"
	scalerMock "github.com/AirHelp/autoscaler/scaler/mock"
	"github.com/AirHelp/autoscaler/testdata"
	"github.com/alicebob/miniredis/v2"

	uberGomock "go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Scaler", func() {
	var (
		mockCtrl       *gomock.Controller
		notifierMock   *notificationMock.MockNotifier
		k8sServiceMock *scalerMock.MockK8SClient

		ctx context.Context

		deploymentName = "test-deployment"
		globalConfig   config.Config
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockCtrl = gomock.NewController(GinkgoT())

		notifierMock = notificationMock.NewMockNotifier(mockCtrl)
		k8sServiceMock = scalerMock.NewMockK8SClient(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("New()", func() {
		var (
			rawYamlConfig = ""
			input         NewScalerInput
			deployment    appsv1.Deployment
		)

		BeforeEach(func() {
			rawYamlConfig = testdata.LoadFixture("autoscaler-config.yaml")

			r := int32(9)
			deployment = appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: deploymentName,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &r,
				},
			}

			input = NewScalerInput{
				Ctx:            ctx,
				DeploymentName: deploymentName,
				RawYamlConfig:  rawYamlConfig,
				Notifiers:      []notification.Notifier{notifierMock},
				K8sService:     k8sServiceMock,
				GlobalConfig:   globalConfig,
			}
		})

		AfterEach(func() {
			rawYamlConfig = ""
			input = NewScalerInput{}
		})

		It("When all good it properly build scaler instance", func() {
			mockCtrl := uberGomock.NewController(GinkgoT())
			mockSqs := sqsMock.NewMockSqsClient(mockCtrl)

			queInput := &sqs.GetQueueUrlInput{
				QueueName: aws.String("q1"),
			}
			queOutput := &sqs.GetQueueUrlOutput{
				QueueUrl: aws.String("https://sqs.eu-west-1.amazonaws.com/123456789012/q1"),
			}
			sqsService := &sqsProbe.SQSService{
				Client: mockSqs,
			}

			input.SQSService = sqsService

			k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)
			mockSqs.EXPECT().GetQueueUrl(ctx, queInput).Return(queOutput, nil)

			sc, err := New(input)

			Expect(err).ToNot(HaveOccurred())

			Expect(sc.deploymentName).To(Equal(deploymentName))
			Expect(sc.deployment).To(Equal(&deployment))
			Expect(sc.probe.Kind()).To(Equal("sqs"))
			Expect(sc.notifiers[0]).To(Equal(notifierMock))
			Expect(sc.k8sService).To(Equal(k8sServiceMock))
			Expect(sc.globalConfig).To(Equal(globalConfig))
			mockCtrl.Finish()
		})

		It("When Redis probe requested it properly creates Redis based scaler", func() {
			// TODO: Consider refactor to not require using of Redis here
			server, err := miniredis.Run()
			Expect(err).ToNot(HaveOccurred())
			defer server.Close()

			k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)
			input.RawYamlConfig = strings.Replace(testdata.LoadFixture("autoscaler-config-redis.yaml"), "localhost:6379", server.Addr(), 1)

			sc, err := New(input)

			Expect(err).ToNot(HaveOccurred())
			Expect(sc.deploymentName).To(Equal(deploymentName))
			Expect(sc.deployment).To(Equal(&deployment))
			Expect(sc.probe.Kind()).To(Equal("redis"))
			Expect(sc.notifiers[0]).To(Equal(notifierMock))
			Expect(sc.k8sService).To(Equal(k8sServiceMock))
			Expect(sc.globalConfig).To(Equal(globalConfig))
		})

		It("When Nginx probe requested it properly creates Nginx based scaler", func() {
			k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)
			input.RawYamlConfig = testdata.LoadFixture("autoscaler-config-nginx.yaml")

			sc, err := New(input)

			Expect(err).ToNot(HaveOccurred())
			Expect(sc.deploymentName).To(Equal(deploymentName))
			Expect(sc.deployment).To(Equal(&deployment))
			Expect(sc.probe.Kind()).To(Equal("nginx"))
			Expect(sc.notifiers[0]).To(Equal(notifierMock))
			Expect(sc.k8sService).To(Equal(k8sServiceMock))
			Expect(sc.globalConfig).To(Equal(globalConfig))
		})

		It("When fetching deployment fails it returns error", func() {
			k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&appsv1.Deployment{}, errors.New("Failed to fetch deployment \"test-deployment\""))

			res, err := New(input)

			Expect(res).ToNot(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Failed to fetch deployment \"test-deployment\""))
		})

		It("When malformed yaml config it returns error", func() {
			rawYamlConfig = `
			malformed:
			yaml config: <-
			{

			}
			`
			input.RawYamlConfig = rawYamlConfig

			k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)

			res, err := New(input)

			Expect(res).ToNot(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("yaml: line 2: found character that cannot start any token"))
		})

		It("When autoscaler config does not specify probe", func() {
			rawYamlConfig = testdata.LoadFixture("autoscaler-config-without-probe.yaml")
			input.RawYamlConfig = rawYamlConfig

			k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)

			res, err := New(input)

			Expect(res).ToNot(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(ErrProbeNotSpecified))
		})
	})

	Describe("Scaler receiver", func() {
		Describe("perform()", func() {
			var (
				probeInstanceMock *probeMock.MockProbe
				deployment        appsv1.Deployment

				expectedReplicas  int32
				availableReplicas int32

				scalerConfig Config
				sc           Scaler
			)

			BeforeEach(func() {
				probeInstanceMock = probeMock.NewMockProbe(mockCtrl)

				expectedReplicas = int32(4)
				availableReplicas = int32(4)
				deployment = appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: deploymentName,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &expectedReplicas,
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          expectedReplicas,
						AvailableReplicas: availableReplicas,
					},
				}

				scalerConfig = Config{
					MinMaxConfig: MinMaxConfig{
						MinimumNumberOfPods: 1,
						MaximumNumberOfPods: 5,
					},
					Threshold:      20,
					CooldownPeriod: 5 * time.Minute,
				}

				sc = Scaler{
					deploymentName: deploymentName,
					notifiers:      []notification.Notifier{notifierMock},
					k8sService:     k8sServiceMock,
					globalConfig:   globalConfig,
					deployment:     &deployment,
					probe:          probeInstanceMock,
					scalerConfig:   scalerConfig,
				}
			})

			AfterEach(func() {
				sc = Scaler{}
			})

			Context("When no external problems", func() {
				It("Properly makes remain decision", func() {
					probeInstanceMock.EXPECT().Check(ctx).Return(75, nil)
					probeInstanceMock.EXPECT().Kind().Return("sqs").AnyTimes()
					k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)

					sc.perform(ctx)
					Expect(sc.lastTenResults).To(Equal([]int{75}))
				})

				It("Properly makes scaleUp decision", func() {
					probeInstanceMock.EXPECT().Check(ctx).Return(500, nil)
					probeInstanceMock.EXPECT().Kind().Return("sqs").AnyTimes()
					k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)
					k8sServiceMock.EXPECT().ScaleDeployment(ctx, &deployment, 5)
					notifierMock.EXPECT().Notify(ctx, gomock.Any()).DoAndReturn(
						func(_ context.Context, payload notification.NotificationPayload) error {
							Expect(payload.Decision).To(Equal("scale up deployment from 4 to 5 replicas"))
							Expect(payload.DeploymentName).To(Equal("test-deployment"))
							return nil
						},
					)

					sc.perform(ctx)
					Expect(sc.lastTenResults).To(Equal([]int{500}))
				})

				It("Properly makes scaleDown decision", func() {
					probeInstanceMock.EXPECT().Check(ctx).Return(0, nil)
					probeInstanceMock.EXPECT().Kind().Return("sqs").AnyTimes()
					k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)
					k8sServiceMock.EXPECT().ScaleDeployment(ctx, &deployment, 3)
					notifierMock.EXPECT().Notify(ctx, gomock.Any()).DoAndReturn(
						func(_ context.Context, payload notification.NotificationPayload) error {
							Expect(payload.Decision).To(Equal("scale down deployment from 4 to 3 replicas"))
							Expect(payload.DeploymentName).To(Equal("test-deployment"))
							return nil
						},
					)

					sc.perform(ctx)
					Expect(sc.lastTenResults).To(Equal([]int{0}))

				})
			})

			Context("When deployment is not in full ready state", func() {
				It("Does not make changes", func() {
					probeInstanceMock.EXPECT().Check(ctx).Return(666, nil)
					probeInstanceMock.EXPECT().Kind().Return("sqs").AnyTimes()

					deployment.Status.AvailableReplicas = int32(1)
					k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)

					sc.perform(ctx)
				})
			})

			Context("When deployment was modified and scaler is in cooldown", func() {
				It("Does not make changes", func() {
					probeInstanceMock.EXPECT().Check(ctx).Return(666, nil)
					probeInstanceMock.EXPECT().Kind().Return("sqs").AnyTimes()

					sc.lastActionAt = time.Now().Add(-30 * time.Second)

					k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)

					sc.perform(ctx)
				})

				Context("But deployment is scaled down to 0", func() {
					It("Does not apply cooldown period and makes a scaleUp decision", func() {
						probeInstanceMock.EXPECT().Check(ctx).Return(666, nil)
						probeInstanceMock.EXPECT().Kind().Return("sqs").AnyTimes()

						sc.lastActionAt = time.Now().Add(-30 * time.Second)

						zeroReplicas := int32(0)
						deployment.Status.AvailableReplicas = zeroReplicas
						deployment.Status.Replicas = zeroReplicas
						deployment.Spec.Replicas = &zeroReplicas

						k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&deployment, nil)
						k8sServiceMock.EXPECT().ScaleDeployment(ctx, &deployment, 1)

						notifierMock.EXPECT().Notify(ctx, gomock.Any()).Return(nil)

						sc.perform(ctx)

						Expect(sc.lastTenResults).To(Equal([]int{666}))
					})
				})
			})

			Context("When probe fails", func() {
				It("Does not make changes", func() {
					probeInstanceMock.EXPECT().Check(ctx).Return(0, errors.New("random error"))
					probeInstanceMock.EXPECT().Kind().Return("sqs").AnyTimes()

					sc.perform(ctx)
				})
			})
		})

		Describe("calculateDecision()", func() {
			var (
				probeInstanceMock *probeMock.MockProbe
				deployment        appsv1.Deployment

				currentReplicas int32

				scalerConfig Config
				sc           Scaler
			)

			BeforeEach(func() {
				probeInstanceMock = probeMock.NewMockProbe(mockCtrl)

				currentReplicas = int32(4)
				deployment = appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: deploymentName,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &currentReplicas,
					},
				}

				scalerConfig = Config{
					MinMaxConfig: MinMaxConfig{
						MinimumNumberOfPods: 0,
						MaximumNumberOfPods: 5,
					},
					Threshold: 20,
				}

				sc = Scaler{
					deploymentName: deploymentName,
					notifiers:      []notification.Notifier{notifierMock},
					k8sService:     k8sServiceMock,
					globalConfig:   globalConfig,
					deployment:     &deployment,
					probe:          probeInstanceMock,
					scalerConfig:   scalerConfig,
				}
			})

			AfterEach(func() {
				sc = Scaler{}
			})

			Context("Scalling up decision", func() {
				It("Decides to scale up when maximum number of pods isn't reached", func() {
					res := sc.calculateDecision(88)

					Expect(res.value).To(Equal(scaleUp))
					Expect(res.current).To(Equal(4))
					Expect(res.target).To(Equal(5))
				})

				It("Decides to remain when maximum number of pods is reached", func() {
					sc.scalerConfig.MaximumNumberOfPods = 4

					res := sc.calculateDecision(88)

					Expect(res.value).To(Equal(remain))
					Expect(res.current).To(Equal(4))
					Expect(res.target).To(Equal(4))
				})
			})

			Context("Scalling down decision", func() {
				It("Decides to scale down when minimum number of pods isn't reached", func() {
					res := sc.calculateDecision(21)

					Expect(res.value).To(Equal(scaleDown))
					Expect(res.current).To(Equal(4))
					Expect(res.target).To(Equal(3))
				})

				It("Decides to remain when minimum number of pods is reached", func() {
					sc.scalerConfig.MinimumNumberOfPods = 4

					res := sc.calculateDecision(21)

					Expect(res.value).To(Equal(remain))
					Expect(res.current).To(Equal(4))
					Expect(res.target).To(Equal(4))
				})

				Context("When scalling down to 0", func() {
					It("Scales down to 0 when consecutive zero read outs from probe", func() {
						r := int32(1)
						sc.deployment.Spec.Replicas = &r
						sc.lastTenResults = []int{5, 0, 0, 0, 0, 0}

						res := sc.calculateDecision(0)
						Expect(res.value).To(Equal(scaleDown))
						Expect(res.current).To(Equal(1))
						Expect(res.target).To(Equal(0))

					})

					It("Decides to remain when no consecutive zero readouts from probe", func() {
						r := int32(1)
						sc.deployment.Spec.Replicas = &r
						sc.lastTenResults = []int{0, 0, 0, 5, 0, 0, 10, 0}

						res := sc.calculateDecision(0)
						Expect(res.value).To(Equal(remain))
						Expect(res.current).To(Equal(1))
						Expect(res.target).To(Equal(1))
					})
				})

			})

			Context("Remain decision", func() {
				It("Decides to remain when calculated number is same", func() {
					res := sc.calculateDecision(75)

					Expect(res.value).To(Equal(remain))
					Expect(res.current).To(Equal(4))
					Expect(res.target).To(Equal(4))
				})
			})

			Context("When scaler config is overwritten by hourly config", func() {
				BeforeEach(func() {
					scalerConfig = Config{
						MinMaxConfig: MinMaxConfig{
							MinimumNumberOfPods: 0,
							MaximumNumberOfPods: 1,
						},
						Threshold: 20,
						HourlyConfig: []*HourlyConfig{
							{
								MinMaxConfig: MinMaxConfig{
									MinimumNumberOfPods: 1,
									MaximumNumberOfPods: 3,
								},
								StartHour: 9,
								EndHour:   17,
							},
						},
					}

					sc.scalerConfig = scalerConfig

					now = func() time.Time { return time.Date(2020, 12, 14, 13, 30, 0, 0, time.UTC) }
				})

				AfterEach(func() {
					now = time.Now
				})

				Context("When deployment is still at 0 instances and no bump up is needed to fulfill needs", func() {
					It("Decides to remain at 0", func() {
						r := int32(0)
						sc.deployment.Spec.Replicas = &r

						res := sc.calculateDecision(0)

						Expect(res.value).To(Equal(remain))
						Expect(res.current).To(Equal(0))
						Expect(res.target).To(Equal(0))
					})
				})

				Context("When deployment normally would autoscale to 0 but working-hours override is active", func() {
					It("Decides to remain at 1", func() {
						r := int32(1)
						sc.deployment.Spec.Replicas = &r
						sc.lastTenResults = []int{5, 0, 0, 0, 0, 0, 0, 0, 0}

						res := sc.calculateDecision(0)

						Expect(res.value).To(Equal(remain))
						Expect(res.current).To(Equal(1))
						Expect(res.target).To(Equal(1))
					})
				})
			})

		})

		Describe("refreshDeployment", func() {
			var (
				probeInstanceMock *probeMock.MockProbe
				oldDeployment     appsv1.Deployment
				newDeployment     appsv1.Deployment

				sc Scaler
			)

			BeforeEach(func() {
				probeInstanceMock = probeMock.NewMockProbe(mockCtrl)

				r := int32(9)
				oldDeployment = appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: deploymentName,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &r,
					},
				}

				sc = Scaler{
					deploymentName: deploymentName,
					notifiers:      []notification.Notifier{notifierMock},
					k8sService:     k8sServiceMock,
					globalConfig:   globalConfig,
					deployment:     &oldDeployment,
					probe:          probeInstanceMock,
				}
			})

			AfterEach(func() {
				sc = Scaler{}
			})

			It("properly refreshes deployment", func() {
				r := int32(1)
				newDeployment = appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: deploymentName,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &r,
					},
				}

				k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&newDeployment, nil)

				Expect(sc.deployment).To(Equal(&oldDeployment))
				err := sc.refreshDeployment(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(sc.deployment).To(Equal(&newDeployment))
			})

			It("When fetching deployment fails it returns error", func() {
				k8sServiceMock.EXPECT().GetDeployment(ctx, deploymentName).Return(&appsv1.Deployment{}, errors.New("failed to fetch deployment \"test-deployment\""))

				Expect(sc.deployment).To(Equal(&oldDeployment))
				err := sc.refreshDeployment(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("failed to fetch deployment \"test-deployment\""))
				Expect(sc.deployment).To(Equal(&oldDeployment))
			})
		})
	})
})
