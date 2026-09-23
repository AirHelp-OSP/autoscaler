package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type Config struct {
	Hosts    []Host   `yaml:"hosts"`
	ListKeys []string `yaml:"list_keys"`
}

type Host struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	TLS      bool   `yaml:"tls"`
	URL      string `yaml:"url"`
}

// Accepts the legacy plain "host:port" string as well as the object form.
func (h *Host) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&h.Address); err == nil {
		return nil
	}

	type plain Host
	return unmarshal((*plain)(h))
}

func (h Host) options() (*redis.Options, error) {
	if h.URL != "" {
		return redis.ParseURL(os.ExpandEnv(h.URL))
	}

	opts := &redis.Options{
		Addr:     os.ExpandEnv(h.Address),
		Password: os.ExpandEnv(h.Password),
	}

	if h.TLS {
		host, _, err := net.SplitHostPort(opts.Addr)
		if err != nil {
			return nil, err
		}
		opts.TLSConfig = &tls.Config{ServerName: host}
	}

	return opts, nil
}

type Probe struct {
	client   *redis.Ring
	listKeys []string
}

func New(config *Config) (*Probe, error) {
	if len(config.Hosts) == 0 {
		return &Probe{}, fmt.Errorf("hosts list cannot be empty")
	}

	if len(config.ListKeys) == 0 {
		return &Probe{}, fmt.Errorf("list keys cannot be empty")
	}

	ringOpts := make(map[string]string)
	hostOpts := make(map[string]*redis.Options)

	for i, host := range config.Hosts {
		opts, err := host.options()
		if err != nil {
			return &Probe{}, err
		}

		key := fmt.Sprintf("host%d", i+1)
		ringOpts[key] = opts.Addr
		hostOpts[key] = opts
	}

	c := redis.NewRing(&redis.RingOptions{
		Addrs: ringOpts,
		// Ring-level Password/TLSConfig are shared by all shards, so per-host settings are applied here.
		NewClient: func(name string, _ *redis.Options) *redis.Client {
			return redis.NewClient(hostOpts[name])
		},
	})

	err := c.ForEachShard(context.Background(), func(ctx context.Context, shard *redis.Client) error {
		res := shard.Ping(ctx)
		err := res.Err()

		if err != nil {
			zap.S().Errorf("failed to connect to Redis instance: %v", shard.Options().Addr)
			return err
		}

		zap.S().Debugf("successfully connected to Redis instance: %v, result: %v", shard.Options().Addr, res.Val())

		return nil
	})

	if err != nil {
		return &Probe{}, err
	}

	return &Probe{
		client:   c,
		listKeys: config.ListKeys,
	}, nil
}

func (p *Probe) Kind() string {
	return "redis"
}

func (p *Probe) Check(ctx context.Context) (int, error) {
	var acc int

	for _, key := range p.listKeys {
		err := p.client.ForEachShard(ctx, func(ctx context.Context, shard *redis.Client) error {
			cmdRes := shard.LLen(ctx, key)

			acc += int(cmdRes.Val())

			return cmdRes.Err()
		})

		if err != nil {
			return 0, err
		}
	}

	return acc, nil
}
