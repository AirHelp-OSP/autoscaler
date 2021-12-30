package notification

import (
	"context"
	"time"
)

//go:generate mockgen -destination=mock/notification_mock.go -package notificationMock github.com/AirHelp/autoscaler/notification Notifier
type Notifier interface {
	Notify(context.Context, NotificationPayload) error
	Kind() string
}

type NotificationPayload struct {
	Decision         string
	Environment      string
	DeploymentName   string
	Namespace        string
	ChangedAt        time.Time
	Source           string
	LastProbeResults []int
}
