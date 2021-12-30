package stat_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/AirHelp/autoscaler/stat"
)

var _ = Describe("Stat", func() {
	Describe("Average", func() {
		DescribeTable("calculating Average of int64 slices",
			func(input []int, output float64) {
				Expect(stat.Average(input)).To(Equal(output))
			},
			Entry("empty slice", []int{}, float64(0)),
			Entry("normal slice 1", []int{1, 3, 5}, float64(3)),
			Entry("normal slice 2", []int{1, 1, 4, 3}, float64(2.25)),
			Entry("normal slice 3", []int{-5, 0, -25, 9}, float64(-5.25)),
		)
	})

	Describe("Median", func() {
		DescribeTable("calculating median of int64 slices",
			func(input []int, output float64) {
				Expect(stat.Median(input)).To(Equal(output))
			},
			Entry("empty slice", []int{}, float64(0)),
			Entry("normal slice 1", []int{1, 3, 5}, float64(3)),
			Entry("normal slice 2", []int{1, 1, 4, 3}, float64(2)),
			Entry("normal slice 3", []int{5, 22, 33, 41, 45, 64, 98}, float64(41)),
			Entry("normal slice 4", []int{-11, -9, -5, 1, 6, 21, 41, 999}, float64(3.5)),
		)
	})

	Describe("Maximum", func() {
		DescribeTable("calculating Maximum of int64 slices",
			func(input []int, output int) {
				Expect(stat.Maximum(input)).To(Equal(output))
			},
			Entry("empty slice", []int{}, 0),
			Entry("normal slice 1", []int{1, 3, 5}, 5),
			Entry("normal slice 2", []int{1, 1, 4, 3}, 4),
			Entry("normal slice 3", []int{-5, 0, -25, 9}, 9),
		)
	})
})
