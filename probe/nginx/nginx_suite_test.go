package nginx_test

import (
	"io"
	"testing"

	log "github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNginx(t *testing.T) {
	RegisterFailHandler(Fail)
	log.SetOutput(io.Discard)
	RunSpecs(t, "Nginx Probe Suite")
}
