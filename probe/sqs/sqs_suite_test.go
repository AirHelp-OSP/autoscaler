package sqs_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSqs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sqs Probe Suite")
}
