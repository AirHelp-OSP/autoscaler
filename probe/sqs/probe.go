package sqs

import (
	"context"
	"errors"
	"strconv"

	awsCfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

//go:generate mockgen -destination=mocks/sqsClientInterface.go -package sqsMock github.com/AirHelp/autoscaler/probe/sqs SqsClient
type SQSClient interface {
	GetQueueUrl(context.Context, *sqs.GetQueueUrlInput, ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)
	GetQueueAttributes(context.Context, *sqs.GetQueueAttributesInput, ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
}

type Config struct {
	Queues []string `yaml:"queues"`
}

type Probe struct {
	queueURLs []string
	client    SQSClient
}

type SQSService struct {
	Client SQSClient
}

var ErrNoQueueSpecified = errors.New("no queues provided")

func NewSQSService(ctx context.Context) (*SQSService, error) {
	cfg, err := awsCfg.LoadDefaultConfig(ctx)
	if err != nil {
		return &SQSService{}, err
	}

	return &SQSService{
		Client: sqs.NewFromConfig(cfg),
	}, nil
}

func New(ctx context.Context, config *Config, s *SQSService) (*Probe, error) {
	var queueURLs []string
	if len(config.Queues) == 0 {
		return &Probe{}, ErrNoQueueSpecified
	}
	queueURLInput := &sqs.GetQueueUrlInput{}
	for _, queue := range config.Queues {
		queueURLInput.QueueName = &queue
		res, err := s.Client.GetQueueUrl(ctx, queueURLInput)
		if err != nil {
			return &Probe{}, err
		}

		queueURLs = append(queueURLs, *res.QueueUrl)
	}
	return &Probe{
		queueURLs: queueURLs,
		client:    s.Client,
	}, nil
}

func (p *Probe) Kind() string {
	return "sqs"
}

func (p *Probe) Check(ctx context.Context) (int, error) {
	var acc int

	for _, queueURL := range p.queueURLs {
		output, err := p.client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       &queueURL,
			AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameApproximateNumberOfMessages, types.QueueAttributeNameApproximateNumberOfMessagesNotVisible},
		})

		if err != nil {
			return 0, err
		}

		for _, num := range output.Attributes {
			size, err := strconv.Atoi(num)

			if err != nil {
				return 0, err
			}

			acc += size
		}
	}

	return acc, nil
}
