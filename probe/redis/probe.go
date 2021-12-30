package redis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	Hosts    []string `yaml:"hosts"`
	ListKeys []string `yaml:"list_keys"`
}

type Probe struct {
	client   *redis.Ring
	listKeys []string
	logger   *log.Entry
}

func New(config *Config, logger *log.Entry) (*Probe, error) {
	if len(config.Hosts) == 0 {
		return &Probe{}, fmt.Errorf("hosts list cannot be empty")
	}

	if len(config.ListKeys) == 0 {
		return &Probe{}, fmt.Errorf("list keys cannot be empty")
	}

	ringOpts := make(map[string]string)

	for i, addr := range config.Hosts {
		key := fmt.Sprintf("host%d", i+1)
		ringOpts[key] = addr
	}

	c := redis.NewRing(&redis.RingOptions{
		Addrs: ringOpts,
	})

	err := c.ForEachShard(context.Background(), func(ctx context.Context, shard *redis.Client) error {
		res := shard.Ping(ctx)
		err := res.Err()

		if err != nil {
			logger.Errorf("failed to connect to Redis instance: %v", shard.Options().Addr)
			return err
		}

		logger.Debugf("successfully connected to Redis instance: %v, result: %v", shard.Options().Addr, res.Val())
		return nil
	})

	if err != nil {
		return &Probe{}, err
	}

	return &Probe{
		client:   c,
		listKeys: config.ListKeys,
		logger:   logger,
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
