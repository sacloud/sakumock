package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	simplenotificationsdk "github.com/sacloud/sacloud-sdk-go/api/simple-notification"
	snv1 "github.com/sacloud/sacloud-sdk-go/api/simple-notification/apis/v1"
	simplemqsdk "github.com/sacloud/sacloud-sdk-go/api/simplemq"
	"github.com/sacloud/sacloud-sdk-go/api/simplemq/apis/v1/message"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"
)

const serviceLinkTimeout = 5 * time.Second

// forwarder delivers fired jobs to their destination service over HTTP using
// the official SDK clients. It is wired in only when service linking is enabled
// (the unified binary passes service endpoints via ServerOptions); without it,
// firings are recorded but not forwarded.
type forwarder struct {
	endpoints map[string]string // service name → base URL
	logger    *slog.Logger

	mqClient *message.Client
	snClient *snv1.Client
}

func newForwarder(endpoints map[string]string, logger *slog.Logger) *forwarder {
	f := &forwarder{
		endpoints: endpoints,
		logger:    logger,
	}
	if ep, ok := endpoints["simplemq"]; ok {
		var sa saclient.Client
		if err := sa.SetEnviron([]string{
			"SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE=" + ep,
		}); err == nil {
			if client, err := simplemqsdk.NewMessageClient("servicelink", &sa); err == nil {
				f.mqClient = client
			}
		}
	}
	if ep, ok := endpoints["simplenotification"]; ok {
		var sa saclient.Client
		if err := sa.SetEnviron([]string{
			"SAKURA_ENDPOINTS_SIMPLE_NOTIFICATION=" + ep,
		}); err == nil {
			if client, err := simplenotificationsdk.NewClient(&sa); err == nil {
				f.snClient = client
			}
		}
	}
	return f
}

// forward dispatches a delivery to its destination service. It returns an
// error string for the delivery record; empty means success.
func (f *forwarder) forward(ctx context.Context, d Delivery) string {
	ctx, cancel := context.WithTimeout(ctx, serviceLinkTimeout)
	defer cancel()
	switch d.Destination {
	case "simplemq":
		return f.forwardToSimpleMQ(ctx, d)
	case "simplenotification":
		return f.forwardToSimpleNotification(ctx, d)
	default:
		return ""
	}
}

// simpleMQParams is the parsed Parameters for a simplemq destination.
type simpleMQParams struct {
	QueueName string `json:"queue_name"`
	Content   string `json:"content"`
}

func (f *forwarder) forwardToSimpleMQ(ctx context.Context, d Delivery) string {
	if f.mqClient == nil {
		return "service link: simplemq endpoint not configured"
	}

	var params simpleMQParams
	if err := json.Unmarshal([]byte(d.Parameters), &params); err != nil {
		return fmt.Sprintf("service link: invalid simplemq parameters: %v", err)
	}
	if params.QueueName == "" {
		return "service link: simplemq parameters missing queue_name"
	}

	op := simplemqsdk.NewMessageOp(f.mqClient, params.QueueName)
	if _, err := op.Send(ctx, params.Content); err != nil {
		return fmt.Sprintf("service link: simplemq send failed: %v", err)
	}

	f.logger.Info("forwarded to simplemq",
		"queue", params.QueueName,
		"process_configuration", d.ProcessConfigurationID,
	)
	return ""
}

// simpleNotificationParams is the parsed Parameters for a simplenotification destination.
type simpleNotificationParams struct {
	GroupID string `json:"group_id"`
	Message string `json:"message"`
}

func (f *forwarder) forwardToSimpleNotification(ctx context.Context, d Delivery) string {
	if f.snClient == nil {
		return "service link: simplenotification endpoint not configured"
	}

	var params simpleNotificationParams
	if err := json.Unmarshal([]byte(d.Parameters), &params); err != nil {
		return fmt.Sprintf("service link: invalid simplenotification parameters: %v", err)
	}
	if params.GroupID == "" {
		return "service link: simplenotification parameters missing group_id"
	}

	op := simplenotificationsdk.NewGroupOp(f.snClient)
	if _, err := op.SendMessage(ctx, params.GroupID, snv1.SendNotificationMessageRequest{
		Message: params.Message,
	}); err != nil {
		return fmt.Sprintf("service link: simplenotification send failed: %v", err)
	}

	f.logger.Info("forwarded to simplenotification",
		"group_id", params.GroupID,
		"process_configuration", d.ProcessConfigurationID,
	)
	return ""
}
