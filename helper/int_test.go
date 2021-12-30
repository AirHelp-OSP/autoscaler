package helper

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Int", func() {
	Describe("Last()", func() {
		DescribeTable("Properly return last elements",
			func(input, expectation []int, count int) {
				res := Last(input, count)
				Expect(len(res)).To(Equal(Min(count, len(input))))
				Expect(res).To(Equal(expectation))
			},
			Entry("Empty slice", []int{}, []int{}, 5),
			Entry("Smaller slice than requested", []int{1, 2, 3}, []int{1, 2, 3}, 5),
			Entry("Larger slice than requested", []int{1, 2, 3, 10, 20, 60, 70}, []int{3, 10, 20, 60, 70}, 5),
			Entry("When 0 to return", []int{1, 2, 3}, []int{}, 0),
		)
	})

	Describe("OnlyZeros()", func() {
		DescribeTable("Properly identifies zero slices",
			func(input []int, expectation bool) { Expect(OnlyZeros(input)).To(Equal(expectation)) },
			Entry("Empty slice", []int{}, false),
			Entry("One element 0", []int{0}, true),
			Entry("One element non 0", []int{9}, false),
			Entry("Multiple elements 0", []int{0, 0, 0}, true),
			Entry("Multiple elements with non 0", []int{0, 0, 1}, false),
			Entry("Multiple elements #1", []int{-99, 0, 0}, false),
			Entry("Multiple elements #2", []int{-99, 0, 99, 0, 0, 0}, false),
		)
	})

	Describe("IntSliceToString()", func() {
		DescribeTable("Properly builds string from slice of ints",
			func(input []int, expectation string) { Expect(IntSliceToString(input)).To(Equal(expectation)) },
			Entry("Empty slice", []int{}, ""),
			Entry("One element", []int{1}, "1"),
			Entry("Multiple elements", []int{1, 5, 9}, "1, 5, 9"),
		)
	})
})
