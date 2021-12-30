package stat_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStat(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stat Suite")
}
