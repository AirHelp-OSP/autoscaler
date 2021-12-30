package helper

import (
	"strconv"
)

func Last(input []int, count int) []int {
	return input[Max(0, len(input)-(count)):]
}

func OnlyZeros(input []int) bool {
	if len(input) == 0 {
		return false
	}

	for _, n := range input {
		if n != 0 {
			return false
		}
	}

	return true
}

func IntSliceToString(input []int) string {
	b := ""

	for _, v := range input {
		if len(b) > 0 {
			b += ", "
		}

		b += strconv.Itoa(v)
	}

	return b
}
