package probe

import "context"

//go:generate mockgen -destination=mock/probe_mock.go -package probeMock github.com/AirHelp/autoscaler/probe Probe
type Probe interface {
	Kind() string
	Check(context.Context) (int, error)
}
