package config

type Config struct {
	Version bool
	Verbose bool

	Environment     string
	Namespace       string
	SlackWebhookUrl string
	SlackChannel    string
	ClusterName     string
}

func NewWithDefaults() Config {
	return Config{}
}
