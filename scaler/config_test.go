package scaler

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("Config.ApplicableLimits()", func() {
		type applicableLimitsTestInput struct {
			config                      Config
			now                         time.Time
			expectedMinimumNumberOfPods int
			expectedMaximumNumberOfPods int
		}

		DescribeTable("Properly identifies applicable limits",
			func(i applicableLimitsTestInput) {
				now = func() time.Time { return i.now }

				res := i.config.ApplicableLimits()

				Expect(res.MinimumNumberOfPods).To(Equal(i.expectedMinimumNumberOfPods))
				Expect(res.MaximumNumberOfPods).To(Equal(i.expectedMaximumNumberOfPods))

				now = time.Now
			},
			Entry("When no hourly config specified", applicableLimitsTestInput{
				config: Config{
					MinMaxConfig: MinMaxConfig{
						MinimumNumberOfPods: 0,
						MaximumNumberOfPods: 2,
					},
				},
				now:                         time.Date(2020, 12, 14, 15, 0, 0, 0, time.UTC),
				expectedMinimumNumberOfPods: 0,
				expectedMaximumNumberOfPods: 2,
			}),
			Entry("When hourly config specified, but not overlapping with now", applicableLimitsTestInput{
				config: Config{
					MinMaxConfig: MinMaxConfig{
						MinimumNumberOfPods: 0,
						MaximumNumberOfPods: 2,
					},
					HourlyConfig: []*HourlyConfig{
						{
							MinMaxConfig: MinMaxConfig{
								MinimumNumberOfPods: 1,
								MaximumNumberOfPods: 5,
							},
							Name:      "working-hours",
							StartHour: 8,
							EndHour:   17,
						},
					},
				},
				now:                         time.Date(2020, 12, 14, 4, 0, 0, 0, time.UTC),
				expectedMinimumNumberOfPods: 0,
				expectedMaximumNumberOfPods: 2,
			}),
			Entry("When hourly config specified, and overlapping with now", applicableLimitsTestInput{
				config: Config{
					MinMaxConfig: MinMaxConfig{
						MinimumNumberOfPods: 0,
						MaximumNumberOfPods: 2,
					},
					HourlyConfig: []*HourlyConfig{
						{
							MinMaxConfig: MinMaxConfig{
								MinimumNumberOfPods: 1,
								MaximumNumberOfPods: 5,
							},
							Name:      "working-hours",
							StartHour: 8,
							EndHour:   17,
						},
					},
				},
				now:                         time.Date(2020, 12, 14, 11, 45, 0, 0, time.UTC),
				expectedMinimumNumberOfPods: 1,
				expectedMaximumNumberOfPods: 5,
			}),
			Entry("When multiple hourly configs specified, and overlapping with now - it uses first match", applicableLimitsTestInput{
				config: Config{
					MinMaxConfig: MinMaxConfig{
						MinimumNumberOfPods: 0,
						MaximumNumberOfPods: 2,
					},
					HourlyConfig: []*HourlyConfig{
						{
							MinMaxConfig: MinMaxConfig{
								MinimumNumberOfPods: 1,
								MaximumNumberOfPods: 5,
							},
							Name:      "working-hours",
							StartHour: 8,
							EndHour:   17,
						},
						{
							MinMaxConfig: MinMaxConfig{
								MinimumNumberOfPods: 5,
								MaximumNumberOfPods: 15,
							},
							Name:      "noon",
							StartHour: 11,
							EndHour:   13,
						},
					},
				},
				now:                         time.Date(2020, 12, 14, 11, 45, 0, 0, time.UTC),
				expectedMinimumNumberOfPods: 1,
				expectedMaximumNumberOfPods: 5,
			}),
			Entry("When multiple hourly configs specified, one after another - it uses proper group", applicableLimitsTestInput{
				config: Config{
					MinMaxConfig: MinMaxConfig{
						MinimumNumberOfPods: 0,
						MaximumNumberOfPods: 2,
					},
					HourlyConfig: []*HourlyConfig{
						{
							MinMaxConfig: MinMaxConfig{
								MinimumNumberOfPods: 1,
								MaximumNumberOfPods: 5,
							},
							Name:      "working-hours",
							StartHour: 8,
							EndHour:   12,
						},
						{
							MinMaxConfig: MinMaxConfig{
								MinimumNumberOfPods: 5,
								MaximumNumberOfPods: 15,
							},
							Name:      "noon",
							StartHour: 12,
							EndHour:   17,
						},
					},
				},
				now:                         time.Date(2020, 12, 14, 11, 45, 0, 0, time.UTC),
				expectedMinimumNumberOfPods: 1,
				expectedMaximumNumberOfPods: 5,
			}),
		)
	})
})
