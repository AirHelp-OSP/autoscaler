package scaler_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestScaler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scaler Suite")
}
