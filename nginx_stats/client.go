package nginx_stats

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

type NginxClient struct {
	endpoint string
}

func NewClient(endpoint string) (*NginxClient, error) {
	return &NginxClient{
		endpoint: endpoint,
	}, nil
}

func (c *NginxClient) GetActiveConnections(ctx context.Context, ip string) (int, error) {
	url := fmt.Sprintf("http://%v/%v", ip, c.endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to get %v: %v", url, err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			zap.S().Error("failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("expected %v response, got %v", http.StatusOK, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read the response body: %v", err)
	}

	result, err := strconv.Atoi(strings.TrimSpace(string(body)))

	if err != nil {
		return 0, fmt.Errorf("returned active connections is not number: %v", string(body))
	}

	return result, nil
}
