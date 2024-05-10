package config

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Base", func() {
	Describe("NewWithDefaults", func() {
		It("Returns new Config struct instance", func() {
			Expect(NewWithDefaults()).To(Equal(Config{}))
		})
	})
})
