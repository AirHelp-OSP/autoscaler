package redis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/alicebob/miniredis/v2"
)

var _ = Describe("Probe", func() {
	Describe("New", func() {
		var config Config

		AfterEach(func() {
			config = Config{}
		})

		Context("When hosts list empty", func() {
			It("Returns error", func() {
				config.ListKeys = []string{"asdf", "qwerty"}

				probe, err := New(&config)

				Expect(*probe).To(BeZero())
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(fmt.Errorf("hosts list cannot be empty")))
			})
		})

		Context("When list keys empty", func() {
			It("Returns error", func() {
				config.Hosts = []Host{{Address: "host:6379"}}

				probe, err := New(&config)

				Expect(*probe).To(BeZero())
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(fmt.Errorf("list keys cannot be empty")))
			})
		})

		Context("When host is not reachable", func() {
			It("Returns error", func() {
				config.Hosts = []Host{{Address: "host:6379"}}
				config.ListKeys = []string{"asdf"}

				probe, err := New(&config)

				Expect(*probe).To(BeZero())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dial tcp"))
			})
		})

		Context("When all good", func() {
			var server *miniredis.Miniredis
			var err error

			BeforeEach(func() {
				server, err = miniredis.Run()

				if err != nil {
					Fail("miniredis failed to start")
				}

				config = Config{
					Hosts:    []Host{{Address: server.Addr()}},
					ListKeys: []string{"asdf"},
				}
			})

			AfterEach(func() {
				server.Close()
			})

			It("Properly setup redis probe", func() {

				probe, err := New(&config)

				Expect(err).ToNot(HaveOccurred())
				Expect(probe.listKeys).To(Equal(config.ListKeys))

				pingRes, err := probe.client.Ping(context.Background()).Result()
				Expect(err).ToNot(HaveOccurred())
				Expect(pingRes).To(Equal("PONG"))
			})
		})

		Context("When auth is required", func() {
			var server *miniredis.Miniredis
			var err error

			BeforeEach(func() {
				server, err = miniredis.Run()

				if err != nil {
					Fail("miniredis failed to start")
				}

				server.RequireAuth("secret")
				GinkgoT().Setenv("REDIS_PASSWORD", "secret")
				GinkgoT().Setenv("REDIS_URL", fmt.Sprintf("redis://:secret@%s", server.Addr()))
				config.ListKeys = []string{"asdf"}
			})

			AfterEach(func() {
				server.Close()
			})

			It("Connects with password from env", func() {
				config.Hosts = []Host{{Address: server.Addr(), Password: "${REDIS_PASSWORD}"}}

				_, err := New(&config)

				Expect(err).ToNot(HaveOccurred())
			})

			It("Connects with url from env", func() {
				config.Hosts = []Host{{URL: "${REDIS_URL}"}}

				_, err := New(&config)

				Expect(err).ToNot(HaveOccurred())
			})

			It("Returns error without password", func() {
				config.Hosts = []Host{{Address: server.Addr()}}

				_, err := New(&config)

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Host", func() {
		It("Unmarshals string, object and url entries", func() {
			var config Config

			err := yaml.Unmarshal([]byte(`
hosts:
  - legacy:6379
  - address: tls-host:6379
    password: ${REDIS_PASSWORD}
    tls: true
  - url: ${REDIS_URL}
`), &config)

			Expect(err).ToNot(HaveOccurred())
			Expect(config.Hosts).To(Equal([]Host{
				{Address: "legacy:6379"},
				{Address: "tls-host:6379", Password: "${REDIS_PASSWORD}", TLS: true},
				{URL: "${REDIS_URL}"},
			}))
		})

		It("Sets TLS server name", func() {
			opts, err := Host{Address: "tls-host:6379", TLS: true}.options()

			Expect(err).ToNot(HaveOccurred())
			Expect(opts.TLSConfig.ServerName).To(Equal("tls-host"))
		})

		It("Enables TLS for rediss url", func() {
			opts, err := Host{URL: "rediss://:pass@tls-host:6379"}.options()

			Expect(err).ToNot(HaveOccurred())
			Expect(opts.Password).To(Equal("pass"))
			Expect(opts.TLSConfig.ServerName).To(Equal("tls-host"))
		})
	})

	Describe("Probe receiver", func() {
		var (
			ctx context.Context

			listKeys = []string{"k1", "k2", "k3"}
			probe    Probe

			ringClient *redis.Ring
			server     *miniredis.Miniredis
			err        error
		)

		BeforeEach(func() {
			server, err = miniredis.Run()

			ctx = context.Background()

			if err != nil {
				Fail("miniredis failed to start")
			}

			ringClient = redis.NewRing(&redis.RingOptions{
				Addrs: map[string]string{
					"h1": server.Addr(),
				},
			})

			probe = Probe{
				listKeys: listKeys,
				client:   ringClient,
			}
		})

		AfterEach(func() {
			server.Close()
			probe = Probe{}
		})

		Describe("Kind()", func() {
			It("Return redis string", func() {
				Expect(probe.Kind()).To(Equal("redis"))
			})
		})

		Describe("Check()", func() {
			It("Returns proper count", func() {
				var err error

				for _, keyValue := range [][]string{
					{"k1", "1"},
					{"k1", "2"},
					{"k1", "3"},
					{"k2", "q"},
					{"k2", "w"},
					{"k3", "asdf"},
				} {
					_, err := server.Lpush(keyValue[0], keyValue[1])

					if err != nil {
						Fail("can't push entity to Redis")
					}
				}

				res, err := probe.Check(ctx)

				Expect(res).To(Equal(6))
				Expect(err).ToNot(HaveOccurred())
			})

			It("When keys are not existing returns zero", func() {
				res, err := probe.Check(ctx)

				Expect(res).To(Equal(0))
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
