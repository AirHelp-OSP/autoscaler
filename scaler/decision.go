package scaler

import "fmt"

const (
	scaleUp = iota
	scaleDown
	remain
)

type decision struct {
	value   int
	current int
	target  int
}

func (d decision) toText() string {
	switch d.value {
	case scaleUp:
		return fmt.Sprintf("scale up deployment from %d to %d replicas", d.current, d.target)
	case scaleDown:
		return fmt.Sprintf("scale down deployment from %d to %d replicas", d.current, d.target)
	case remain:
		return fmt.Sprintf("remain at %d replicas", d.current)
	default:
		return ""
	}
}
