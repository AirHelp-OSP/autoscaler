package sqs

import (
	"context"
	"errors"

	"github.com/AirHelp/autoscaler/probe/sqs/mocks"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Probe", func() {
	Describe("New()", func() {
		var config Config
		var ctx = context.Background()

		It("Returns error when no queue provided", func() {
			probe, err := New(ctx, &config)

			Expect(probe).To(Equal(&Probe{}))
			Expect(err).To(Equal(ErrNoQueueSpecified))
		})
	})

	Describe("Probe receiver", func() {
		var (
			mockCtrl *gomock.Controller
			mockSqs  *sqsMock.MockSqsClient

			queueURLs = []string{"q1", "q2", "q3"}
			probe     Probe
		)

		ctx := context.Background()

		BeforeEach(func() {
			mockCtrl = gomock.NewController(GinkgoT())
			mockSqs = sqsMock.NewMockSqsClient(mockCtrl)

			probe = Probe{
				queueURLs: queueURLs,
				client:    mockSqs,
			}
		})

		AfterEach(func() {
			mockCtrl.Finish()
		})

		Describe("Kind()", func() {
			It("Return sqs string", func() {
				Expect(probe.Kind()).To(Equal("sqs"))
			})
		})

		Describe("Check()", func() {
			It("Returns accumulated count of messages in queues", func() {
				noOfMessages := []string{"1", "0", "1001"}
				noOfMessagesNotVisible := []string{"10", "0", "0"}

				q1Result := sqs.GetQueueAttributesOutput{
					Attributes: map[string]string{
						"ApproximateNumberOfMessages":           noOfMessages[0],
						"ApproximateNumberOfMessagesNotVisible": noOfMessagesNotVisible[0],
					},
				}
				q2Result := sqs.GetQueueAttributesOutput{
					Attributes: map[string]string{
						"ApproximateNumberOfMessages":           noOfMessages[1],
						"ApproximateNumberOfMessagesNotVisible": noOfMessagesNotVisible[1],
					},
				}
				q3Result := sqs.GetQueueAttributesOutput{
					Attributes: map[string]string{
						"ApproximateNumberOfMessages":           noOfMessages[2],
						"ApproximateNumberOfMessagesNotVisible": noOfMessagesNotVisible[2],
					},
				}

				mockSqs.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
					QueueUrl:       &queueURLs[0],
					AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameApproximateNumberOfMessages, types.QueueAttributeNameApproximateNumberOfMessagesNotVisible},
				}).Return(&q1Result, nil)

				mockSqs.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
					QueueUrl:       &queueURLs[1],
					AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameApproximateNumberOfMessages, types.QueueAttributeNameApproximateNumberOfMessagesNotVisible},
				}).Return(&q2Result, nil)
				mockSqs.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
					QueueUrl:       &queueURLs[2],
					AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameApproximateNumberOfMessages, types.QueueAttributeNameApproximateNumberOfMessagesNotVisible},
				}).Return(&q3Result, nil)

				res, err := probe.Check(ctx)

				Expect(res).To(Equal(1012))
				Expect(err).ToNot(HaveOccurred())
			})

			It("When error happens it proxies error", func() {
				mockSqs.EXPECT().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
					QueueUrl:       &queueURLs[0],
					AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameApproximateNumberOfMessages, types.QueueAttributeNameApproximateNumberOfMessagesNotVisible},
				}).Return(&sqs.GetQueueAttributesOutput{}, errors.New("access denied"))

				res, err := probe.Check(ctx)

				Expect(res).To(Equal(0))
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
