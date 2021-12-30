package helper

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("MinMax", func() {
	Describe("Max()", func() {
		DescribeTable("Properly determines maximum",
			func(a, b, expectation int) { Expect(Max(a, b)).To(Equal(expectation)) },
			Entry("Normal ints", 0, 1, 1),
			Entry("Negative number", -99, 50, 50),
			Entry("Negative numbers", -99, -1000, -99),
		)
	})

	Describe("Min()", func() {
		DescribeTable("Properly determines maximum",
			func(a, b, expectation int) { Expect(Min(a, b)).To(Equal(expectation)) },
			Entry("Normal ints", 0, 1, 0),
			Entry("Negative number", -99, 50, -99),
			Entry("Negative numbers", -99, -1000, -1000),
		)
	})

})
