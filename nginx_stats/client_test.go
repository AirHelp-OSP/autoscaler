package nginx_stats

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/h2non/gock.v1"
)

var _ = Describe("Client", func() {
	Describe("GetActiveConnections()", func() {
		var (
			client   NginxClient
			ip       string
			endpoint string

			ctx context.Context
		)

		BeforeEach(func() {
			ctx = context.Background()

			endpoint = "stats/active_connections"
			ip = "0.0.0.0"

			client = NginxClient{
				endpoint: endpoint,
			}
		})

		AfterEach(func() {
			Expect(gock.IsDone()).To(BeTrue())
			gock.Off()
		})

		Context("Timeout test", func() {
			var srv *httptest.Server

			BeforeEach(func() {
				// Gock cannot simulate such a scenario
				// So we need to create a fake server
				gock.Off()

				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(50 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("10"))
				}))
			})

			AfterEach(func() {
				srv.Close()
			})

			It("Respects context timeout", func() {
				ctx, fn := context.WithTimeout(context.Background(), 25*time.Millisecond)
				defer fn()

				srvWithoutProtocol := strings.TrimPrefix(srv.URL, "http://")

				res, err := client.GetActiveConnections(ctx, srvWithoutProtocol)

				Expect(res).To(Equal(0))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("context deadline exceeded"))
			})
		})

		Context("When all good", func() {
			It("Returns proper number from nginx stats", func() {
				gock.New("http://" + ip).
					Get("/stats/active_connections").
					Reply(200).
					BodyString("13")

				res, err := client.GetActiveConnections(ctx, ip)

				Expect(res).To(Equal(13))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("When not ok", func() {
			Context("When failed request", func() {
				It("returns 0 and error", func() {
					respErr := errors.New("unreachable")
					gock.New("http://" + ip).
						Get("/stats/active_connections").
						ReplyError(respErr)

					res, err := client.GetActiveConnections(ctx, ip)

					Expect(res).To(Equal(0))
					Expect(err).To(Equal(errors.New("failed to get http://0.0.0.0/stats/active_connections: Get \"http://0.0.0.0/stats/active_connections\": unreachable")))
				})

			})

			Context("When not 200 response", func() {
				It("returns 0 and error", func() {
					gock.New("http://" + ip).
						Get("/stats/active_connections").
						Reply(500).
						BodyString("internal server error")

					res, err := client.GetActiveConnections(ctx, ip)

					Expect(res).To(Equal(0))
					Expect(err).To(Equal(errors.New("expected 200 response, got 500")))
				})
			})

			Context("When not number in response", func() {
				It("returns 0 and error", func() {
					gock.New("http://" + ip).
						Get("/stats/active_connections").
						Reply(200).
						BodyString("asdf")

					res, err := client.GetActiveConnections(ctx, ip)

					Expect(res).To(Equal(0))
					Expect(err).To(Equal(errors.New("returned active connections is not number: asdf")))
				})
			})

		})
	})
})
