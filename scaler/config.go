package scaler

import (
	"fmt"
	"time"

	"github.com/AirHelp/autoscaler/probe/nginx"
	"github.com/AirHelp/autoscaler/probe/redis"
	"github.com/AirHelp/autoscaler/probe/sqs"
	log "github.com/sirupsen/logrus"
)

type MinMaxConfig struct {
	MinimumNumberOfPods int `yaml:"minimum_number_of_pods"`
	MaximumNumberOfPods int `yaml:"maximum_number_of_pods"`
}

type HourlyConfig struct {
	MinMaxConfig `yaml:",inline"`

	Name      string `yaml:"name"`
	StartHour int    `yaml:"start_hour"`
	EndHour   int    `yaml:"end_hour"`
}

type Config struct {
	MinMaxConfig `yaml:",inline"`

	CheckInterval  time.Duration `yaml:"check_interval"`
	CooldownPeriod time.Duration `yaml:"cooldown_period"`
	Threshold      int           `yaml:"threshold"`

	HourlyConfig []*HourlyConfig `yaml:"hourly_config"`

	Sqs   *sqs.Config   `yaml:"sqs"`
	Redis *redis.Config `yaml:"redis"`
	Nginx *nginx.Config `yaml:"nginx"`
}

func NewScalerConfigWithDefaults() Config {
	return Config{
		MinMaxConfig: MinMaxConfig{
			MinimumNumberOfPods: 0,
			MaximumNumberOfPods: 3,
		},
		CheckInterval:  time.Minute,
		CooldownPeriod: time.Minute * 5,
	}
}

// Export `now` function to variable - make it available for stubbing in tests while not having massive hacks on code level
var now = time.Now

func (sc Config) ApplicableLimits(l *log.Entry) MinMaxConfig {
	if len(sc.HourlyConfig) == 0 {
		l.Debug("No hourly configs defined, applying default")
		return sc.MinMaxConfig
	}

	hours, _, _ := now().Clock()

	for _, hc := range sc.HourlyConfig {
		if isHourWithinBoundaries(hours, hc.StartHour, hc.EndHour) {
			l.Debug(fmt.Sprintf("Applying `%v` hourly config", hc.Name))
			return hc.MinMaxConfig
		}
	}

	l.Debug("None hourly config is applicable, fallback to default")
	return sc.MinMaxConfig
}

func isHourWithinBoundaries(hour, min, max int) bool {
	return hour >= min && hour < max
}
