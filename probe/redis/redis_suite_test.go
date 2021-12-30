package redis_test

import (
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	log "github.com/sirupsen/logrus"
)

func TestRedis(t *testing.T) {
	RegisterFailHandler(Fail)
	log.SetOutput(io.Discard)
	RunSpecs(t, "Redis Probe Suite")
}
