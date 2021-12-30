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
	"github.com/AirHelp/autoscaler/notification"
	"github.com/AirHelp/autoscaler/notification/slack"
	"github.com/AirHelp/autoscaler/scaler"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

const configMapName = "autoscaler-config"

type ScalerEntity interface {
	Start(context.Context)
}

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

func main() {
	cfg := parseStartingFlags()

	if cfg.Version {
		fmt.Println(versionString())
		os.Exit(0)
	}

	log.Infof("Autoscaler starting, version: %v", strings.TrimSpace(version))

	if cfg.Verbose {
		log.Info("Starting autoscaler with verbose logging mode")
		log.SetLevel(log.DebugLevel)
	}

	logger := log.WithFields(log.Fields{
		"namespace":   cfg.Namespace,
		"environment": cfg.Environment,
	})

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	logger.Debug("Initializing k8s client and config")
	k8sSvc, err := k8s.New(cfg.Namespace, logger)

	if err != nil {
		logger.WithError(err).Error("Failed to initialize k8s client")
		panic(err)
	}
	logger.Debug("Successfully initialized k8s client and config")

	configMap, err := k8sSvc.GetConfigMap(ctx, configMapName)

	if err != nil {
		logger.WithError(err).Error("Error getting autoscaler configmap")
		panic(err)
	}

	var notifiers []notification.Notifier

	if cfg.SlackWebhookUrl != "" {
		logger.Debug("Initializing Slack client")
		notifiers = append(notifiers, slack.NewClient(cfg.SlackWebhookUrl, cfg.SlackChannel, cfg.ClusterName, "autoscaler"))
		logger.Debug("Done initializing Slack client")
	}

	logger.Debug("Initializing scalers on all enabled deployments")

	waitGroup := sync.WaitGroup{}

	for deployment, rawYamlConfig := range configMap.Data {
		var scalerInstance ScalerEntity

		scalerInstance, err := scaler.New(scaler.NewScalerInput{
			Ctx:            ctx,
			DeploymentName: deployment,
			RawYamlConfig:  rawYamlConfig,
			Notifiers:      notifiers,
			K8sService:     k8sSvc,
			GlobalConfig:   cfg,
			Logger:         logger,
		})

		if err != nil {
			logger.WithError(err).Error(fmt.Sprintf("Failed to initialize autoscaler for %v, skipping.", deployment))
			continue
		}

		go func() {
			scalerInstance.Start(ctx)
			waitGroup.Done()
		}()

		waitGroup.Add(1)
	}

	logger.Debug("Done initializing scalers on all enabled deployments")

	<-interruptChan
	cancel()

	waitGroup.Wait()

	logger.Info("Received shutdown, shutting down")
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
