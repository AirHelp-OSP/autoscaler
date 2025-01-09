package nginx

import (
	"context"
	"errors"
	"time"

	nginxMock "github.com/AirHelp/autoscaler/probe/nginx/mock"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Probe", func() {
	var (
		probe Probe

		deployment *appsv1.Deployment

		mockCtrl        *gomock.Controller
		k8sServiceMock  *nginxMock.MockK8SClient
		nginxClientMock *nginxMock.MockNginxClient

		pods *v1.PodList

		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		mockCtrl = gomock.NewController(GinkgoT())
		k8sServiceMock = nginxMock.NewMockK8SClient(mockCtrl)
		nginxClientMock = nginxMock.NewMockNginxClient(mockCtrl)

		deployment = &appsv1.Deployment{}

		pods = &v1.PodList{
			Items: []v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1",
					},
					Status: v1.PodStatus{
						PodIP: "0.0.0.0",
						Phase: v1.PodRunning,
						Conditions: []v1.PodCondition{
							{
								Status: v1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod2",
					},
					Status: v1.PodStatus{
						PodIP: "0.0.0.1",
						Phase: v1.PodRunning,
						Conditions: []v1.PodCondition{
							{
								Status: v1.ConditionTrue,
							},
						},
					},
				},
			},
		}

		probe = Probe{
			k8sService:  k8sServiceMock,
			nginxClient: nginxClientMock,

			deployment: deployment,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
		probe = Probe{}
	})

	Context("All good", func() {
		BeforeEach(func() {
			probe.consecutiveReads = 3
			probe.timeout = 100 * time.Millisecond

			k8sServiceMock.EXPECT().GetPodsFromDeployment(ctx, deployment, additionalExpectedWebPodLabels).Return(pods, nil)

			gomock.InOrder(
				nginxClientMock.EXPECT().GetActiveConnections(gomock.Any(), "0.0.0.0").Return(25, nil),
				nginxClientMock.EXPECT().GetActiveConnections(gomock.Any(), "0.0.0.0").Return(8, nil),
				nginxClientMock.EXPECT().GetActiveConnections(gomock.Any(), "0.0.0.0").Return(40, nil),
			)

			gomock.InOrder(
				nginxClientMock.EXPECT().GetActiveConnections(gomock.Any(), "0.0.0.1").Return(2, nil),
				nginxClientMock.EXPECT().GetActiveConnections(gomock.Any(), "0.0.0.1").Return(16, nil),
				nginxClientMock.EXPECT().GetActiveConnections(gomock.Any(), "0.0.0.1").Return(60, nil),
			)
		})
		It("When statistic=average the end value is calculated properly", func() {
			probe.statistic = "average"

			res, err := probe.Check(ctx)

			Expect(res).To(Equal(51))
			Expect(err).ToNot(HaveOccurred())
		})

		It("When statistic=median the end value is calculated properly", func() {
			probe.statistic = "median"

			res, err := probe.Check(ctx)

			Expect(res).To(Equal(27))
			Expect(err).ToNot(HaveOccurred())
		})

		It("When statistic=maximum the end value is calculated properly", func() {
			probe.statistic = "maximum"

			res, err := probe.Check(ctx)

			Expect(res).To(Equal(100))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("When fetching pods fails", func() {
		var err error

		BeforeEach(func() {
			err = errors.New("Failed to fetch pods")
			k8sServiceMock.EXPECT().GetPodsFromDeployment(ctx, deployment, additionalExpectedWebPodLabels).Return(&v1.PodList{}, err)
		})

		It("Returns 0 and error", func() {
			res, resultErr := probe.Check(ctx)

			Expect(res).To(Equal(0))
			Expect(resultErr).To(Equal(err))
		})
	})

	Context("When one of pods isn't ready", func() {
		BeforeEach(func() {
			pods.Items[0].Status.Phase = v1.PodFailed

			k8sServiceMock.EXPECT().GetPodsFromDeployment(ctx, deployment, additionalExpectedWebPodLabels).Return(pods, nil)
		})

		It("Returns 0 and error", func() {
			res, resultErr := probe.Check(ctx)

			Expect(res).To(Equal(0))
			Expect(resultErr).To(Equal(errors.New("deployment not fully operational")))
		})
	})

	Context("When one of pods conditions are failing", func() {
		BeforeEach(func() {
			pods.Items[1].Status.Conditions[0].Status = v1.ConditionFalse

			k8sServiceMock.EXPECT().GetPodsFromDeployment(ctx, deployment, additionalExpectedWebPodLabels).Return(pods, nil)
		})

		It("Returns 0 and error", func() {
			res, resultErr := probe.Check(ctx)

			Expect(res).To(Equal(0))
			Expect(resultErr).To(Equal(errors.New("deployment not fully operational")))
		})
	})

	Context("When one of pods stats are unreachable", func() {
		BeforeEach(func() {
			probe.consecutiveReads = 2
			probe.timeout = 100 * time.Millisecond

			k8sServiceMock.EXPECT().GetPodsFromDeployment(ctx, deployment, additionalExpectedWebPodLabels).Return(pods, nil)

			gomock.InOrder(
				nginxClientMock.EXPECT().GetActiveConnections(gomock.Any(), "0.0.0.0").Return(25, nil),
				nginxClientMock.EXPECT().GetActiveConnections(gomock.Any(), "0.0.0.0").Return(0, errors.New("Expected response: 200, got: 502")),
			)

			gomock.InOrder(
				nginxClientMock.EXPECT().GetActiveConnections(gomock.Any(), "0.0.0.1").Return(2, nil),
				nginxClientMock.EXPECT().GetActiveConnections(gomock.Any(), "0.0.0.1").Return(16, nil),
			)
		})

		It("Returns 0 and error", func() {
			res, resultErr := probe.Check(ctx)

			Expect(res).To(Equal(0))
			Expect(resultErr).To(Equal(errors.New("Expected response: 200, got: 502")))
		})
	})
})
