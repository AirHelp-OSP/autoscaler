package slack

import (
	"context"
	"fmt"
	"time"

	"github.com/AirHelp/autoscaler/helper"
	"github.com/AirHelp/autoscaler/notification"

	"github.com/slack-go/slack"
)

type Client struct {
	url         string
	icon        string
	username    string
	channel     string
	clusterName string
}

func NewClient(url, channel, clusterName, username string) Client {
	return Client{
		url:         url,
		channel:     channel,
		username:    username,
		clusterName: clusterName,
		icon:        "scales",
	}
}

func (c Client) Kind() string {
	return "slack"
}

func (c Client) Notify(ctx context.Context, payload notification.NotificationPayload) error {
	att := slack.Attachment{
		Color:      "good",
		AuthorIcon: c.icon,
		Pretext:    "Autoscaler has made a change in deployment",
		Footer:     fmt.Sprintf("autoscaler @ %v", payload.ChangedAt.Format(time.RFC3339)),
		Fields: []slack.AttachmentField{
			{
				Title: "Decision",
				Value: payload.Decision,
			},
			{
				Title: "Last probe results",
				Value: helper.IntSliceToString(payload.LastProbeResults),
			},
			{
				Title: "Cluster name",
				Value: c.clusterName,
			},
			{
				Title: "Deployment",
				Value: payload.DeploymentName,
				Short: true,
			},
			{
				Title: "Namespace",
				Value: payload.Namespace,
				Short: true,
			},
			{
				Title: "Environment",
				Value: payload.Environment,
				Short: true,
			},
			{
				Title: "Source",
				Value: payload.Source,
				Short: true,
			},
		},
	}

	msg := slack.WebhookMessage{
		Username:    c.username,
		IconEmoji:   c.icon,
		Channel:     c.channel,
		Attachments: []slack.Attachment{att},
	}

	return slack.PostWebhookContext(ctx, c.url, &msg)
}
