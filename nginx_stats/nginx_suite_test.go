package nginx_stats_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNginx(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nginx Stats Suite")
}
