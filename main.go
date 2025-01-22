package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/AirHelp/autoscaler/config"
	"github.com/AirHelp/autoscaler/k8s"
	"github.com/AirHelp/autoscaler/logger"
	"github.com/AirHelp/autoscaler/notification"
	"github.com/AirHelp/autoscaler/notification/slack"
	"github.com/AirHelp/autoscaler/probe/sqs"
	"github.com/AirHelp/autoscaler/scaler"
	flag "github.com/spf13/pflag"

	"go.uber.org/zap"
)

const (
	configMapName = "autoscaler-config"
)

var cfg config.Config

type ScalerEntity interface {
	Start(context.Context)
}

func init() {
	cfg = parseStartingFlags()
	logLevel := "info"
	if cfg.Verbose {
		logLevel = "debug"
	}
	log.InitLogger(cfg.Namespace, cfg.Environment, logLevel)
}

func main() {
	if cfg.Version {
		fmt.Println(versionString())
		os.Exit(0)
	}

	zap.S().Infof("autoscaler starting, version: %v", strings.TrimSpace(version))

	if cfg.Verbose {
		zap.S().Info("running in verbose logging mode")
	}

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	zap.S().Debug("initializing k8s client and config")
	k8sSvc, err := k8s.New(cfg.Namespace)
	if err != nil {
		zap.S().With("error", err).Error("failed to initialize K8s client")
		panic(err)
	}
	zap.S().Debug("successfully initialized k8s client and config")

	configMap, err := k8sSvc.GetConfigMap(ctx, configMapName)
	if err != nil {
		zap.S().With("error", err).Error("failed to get autoscaler configmap")
		panic(err)
	}

	var notifiers []notification.Notifier

	if cfg.SlackWebhookUrl != "" {
		zap.S().Debug("initializing Slack client")
		notifiers = append(notifiers, slack.NewClient(cfg.SlackWebhookUrl, cfg.SlackChannel, cfg.ClusterName, "autoscaler"))
		zap.S().Debug("slack client initialized successfully")
	}

	zap.S().Debug("initializing scalers on the all enabled deployments")

	waitGroup := sync.WaitGroup{}

	for deployment, rawYamlConfig := range configMap.Data {
		var sqsService *sqs.SQSService
		var scalerInstance ScalerEntity

		sqsService, err = InitializeSQSService(ctx, rawYamlConfig)
		if err != nil {
			zap.S().With("error", err).Errorf("failed to initialize autoscaler for %v: , skipping", deployment)
			continue
		}

		scalerInstance, err = scaler.New(scaler.NewScalerInput{
			Ctx:            ctx,
			DeploymentName: deployment,
			RawYamlConfig:  rawYamlConfig,
			Notifiers:      notifiers,
			K8sService:     k8sSvc,
			SQSService:     sqsService,
			GlobalConfig:   cfg,
		})

		if err != nil {
			zap.S().With("error", err).Errorf("failed to initialize autoscaler for %v: , skipping", deployment)
			continue
		}

		go func() {
			scalerInstance.Start(ctx)
			waitGroup.Done()
		}()

		waitGroup.Add(1)
	}

	zap.S().Debug("initializing scalers on the all enabled deployments")

	<-interruptChan
	cancel()

	waitGroup.Wait()

	zap.S().Info("received shutdown, shutting down")
}

func parseStartingFlags() config.Config {
	cfg := config.NewWithDefaults()
	flag.BoolVarP(&cfg.Verbose, "verbose", "v", false, "Debug mode")
	flag.BoolVar(&cfg.Version, "version", false, "Prints version number")

	flag.StringVar(&cfg.Environment, "environment", "", "Environment name")
	flag.StringVar(&cfg.Namespace, "namespace", "", "Namespace of autoscaler to run within")
	flag.StringVar(&cfg.SlackWebhookUrl, "slack_url", "", "Slack Webhook URL to use")
	flag.StringVar(&cfg.SlackChannel, "slack_channel", "", "Slack channel to send messages to")
	flag.StringVar(&cfg.ClusterName, "cluster_name", "", "Name of cluster")
	flag.Parse()

	return cfg
}

func InitializeSQSService(ctx context.Context, rawConfig string) (*sqs.SQSService, error) {
	var sqsService *sqs.SQSService
	config, err := scaler.ParseRawScalerConfig(rawConfig)
	if err != nil {
		zap.S().Debugf("failed to parse config: %v", err)
		return sqsService, err
	}

	if config.Sqs == nil {
		return sqsService, nil
	}

	sqsService, err = sqs.NewSQSService(ctx)
	if err != nil {
		zap.S().Debugf("failed to initialize SQS client: %v", err)
		return sqsService, err
	}

	return sqsService, nil
}
