package valkey_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestValkey(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Valkey Probe Suite")
}
