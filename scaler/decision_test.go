package scaler

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Decision", func() {
	Describe("Decision receiver", func() {
		DescribeTable("Properly builds decision string",
			func(d decision, expectation string) { Expect(d.toText()).To(Equal(expectation)) },
			Entry("When scale up", decision{
				value:   scaleUp,
				current: 0,
				target:  1,
			}, "scale up deployment from 0 to 1 replicas"),
			Entry("When scale down", decision{
				value:   scaleDown,
				current: 10,
				target:  9,
			}, "scale down deployment from 10 to 9 replicas"),
			Entry("When remain", decision{
				value:   remain,
				current: 5,
				target:  5,
			}, "remain at 5 replicas"),
		)
	})
})
