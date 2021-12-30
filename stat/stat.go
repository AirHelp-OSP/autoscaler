package stat

import "sort"

func Average(numbers []int) float64 {
	if len(numbers) == 0 {
		return float64(0)
	}

	var sum int

	for _, n := range numbers {
		sum += n
	}

	return float64(sum) / float64(len(numbers))
}

func Median(numbers []int) float64 {
	if len(numbers) == 0 {
		return 0
	}

	sort.Ints(numbers)
	l := len(numbers)

	if l%2 == 0 {
		return float64(numbers[l/2-1]+numbers[l/2]) / float64(2)
	} else {
		return float64(numbers[(l-1)/2])
	}
}

func Maximum(numbers []int) int {
	max := 0

	for _, n := range numbers {
		if n > max {
			max = n
		}
	}

	return max
}
