package sqs

import (
	"context"
	"errors"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

//go:generate mockgen -destination=aws_mocks/sqs_iface_mock.go -package awsMock github.com/aws/aws-sdk-go/service/sqs/sqsiface SQSAPI

type Config struct {
	Queues []string `yaml:"queues"`
}

type Probe struct {
	queueURLs []string
	client    sqsiface.SQSAPI
	logger    *log.Entry
}

var ErrNoQueueSpecified = errors.New("no queues provided")

func New(config *Config, logger *log.Entry) (*Probe, error) {
	sess, err := session.NewSession()
	if err != nil {
		return &Probe{}, err
	}

	svc := sqs.New(sess)

	var queueURLs []string

	queueURLInput := &sqs.GetQueueUrlInput{}

	if len(config.Queues) == 0 {
		return &Probe{}, ErrNoQueueSpecified
	}

	for _, queue := range config.Queues {
		queueURLInput.SetQueueName(queue)
		res, err := svc.GetQueueUrl(queueURLInput)

		if err != nil {
			return &Probe{}, err
		}

		queueURLs = append(queueURLs, *res.QueueUrl)
	}

	return &Probe{
		queueURLs: queueURLs,
		client:    svc,
		logger:    logger,
	}, nil
}

func (p *Probe) Kind() string {
	return "sqs"
}

func (p *Probe) Check(ctx context.Context) (int, error) {
	var acc int

	for _, queueURL := range p.queueURLs {
		output, err := p.client.GetQueueAttributesWithContext(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       &queueURL,
			AttributeNames: []*string{aws.String("ApproximateNumberOfMessages"), aws.String("ApproximateNumberOfMessagesNotVisible")},
		})

		if err != nil {
			return 0, err
		}

		for _, num := range output.Attributes {
			size, err := strconv.Atoi(*num)

			if err != nil {
				return 0, err
			}

			acc += size
		}
	}

	return acc, nil
}
