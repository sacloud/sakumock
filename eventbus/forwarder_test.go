package eventbus_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	sdk "github.com/sacloud/sacloud-sdk-go/api/eventbus"
	v1 "github.com/sacloud/sacloud-sdk-go/api/eventbus/apis/v1"
	simplenotificationsdk "github.com/sacloud/sacloud-sdk-go/api/simple-notification"
	snv1 "github.com/sacloud/sacloud-sdk-go/api/simple-notification/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/api/simplemq/apis/v1/message"
	"github.com/sacloud/sacloud-sdk-go/api/simplemq/apis/v1/queue"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/eventbus"
	"github.com/sacloud/sakumock/simplemq"
	"github.com/sacloud/sakumock/simplenotification"
)

// serviceLinkEnv builds a []core.EnvVar for testing by taking each service's
// ClientEnv() and replacing the address with the test server's URL. This
// mirrors what AllCmd.serviceLinkEnv() does for the real binary, keeping the
// env var key names in the owning service package.
func serviceLinkEnv(services map[core.ServiceConfig]string) []core.EnvVar {
	var env []core.EnvVar
	for cfg, testURL := range services {
		for _, e := range cfg.ClientEnv() {
			// ClientEnv values are "http://<configured-addr>..." — replace with the test URL.
			if i := strings.Index(e.Value, "://"); i >= 0 {
				e.Value = testURL
			}
			env = append(env, e)
		}
	}
	env = append(env, core.DummyCredentialEnv()...)
	return env
}

func TestForwardToSimpleMQ(t *testing.T) {
	mqSrv := simplemq.NewTestServer(simplemq.Config{})
	defer mqSrv.Close()

	createQueue(t, mqSrv.TestURL(), "test-queue-00001")

	env := serviceLinkEnv(map[core.ServiceConfig]string{
		simplemq.Config{}: mqSrv.TestURL(),
	})
	ebSrv := eventbus.NewTestServerWithServiceLink(eventbus.Config{}, env)
	defer ebSrv.Close()

	client := newTestClient(t, ebSrv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	triggerOp := sdk.NewTriggerOp(client)

	pc, err := pcOp.Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name:     "pc-simplemq",
			Settings: sdk.CreateSimpleMqSettings("test-queue-00001", "dGVzdA=="),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = triggerOp.Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "trigger-simplemq",
			Settings: v1.NewTriggerSettingsSettings(v1.TriggerSettings{
				Source:                 "test",
				ProcessConfigurationID: pc.ID,
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := postJSON(t, ebSrv.TestURL()+"/_sakumock/events", map[string]any{
		"Source": "test",
	})
	if got.Count != 1 {
		t.Fatalf("expected 1 delivery, got %d", got.Count)
	}
	if got.Deliveries[0].Error != "" {
		t.Fatalf("delivery error: %s", got.Deliveries[0].Error)
	}
	if got.Deliveries[0].Destination != "simplemq" {
		t.Errorf("expected destination simplemq, got %s", got.Deliveries[0].Destination)
	}

	msgs := receiveFromQueue(t, mqSrv.TestURL(), "test-queue-00001")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in simplemq queue, got %d", len(msgs))
	}
	if msgs[0].Content != "dGVzdA==" {
		t.Errorf("unexpected message content: %s", msgs[0].Content)
	}
}

func TestForwardToSimpleMQNoEndpoint(t *testing.T) {
	ebSrv := eventbus.NewTestServer(eventbus.Config{})
	defer ebSrv.Close()

	client := newTestClient(t, ebSrv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	triggerOp := sdk.NewTriggerOp(client)

	pc, err := pcOp.Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name:     "pc-simplemq",
			Settings: sdk.CreateSimpleMqSettings("test-queue-00001", "dGVzdA=="),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = triggerOp.Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "trigger-simplemq",
			Settings: v1.NewTriggerSettingsSettings(v1.TriggerSettings{
				Source:                 "test",
				ProcessConfigurationID: pc.ID,
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := postJSON(t, ebSrv.TestURL()+"/_sakumock/events", map[string]any{
		"Source": "test",
	})
	if got.Count != 1 {
		t.Fatalf("expected 1 delivery, got %d", got.Count)
	}
	if got.Deliveries[0].Error != "" {
		t.Errorf("expected no error without service link, got: %s", got.Deliveries[0].Error)
	}
}

func TestForwardToSimpleNotification(t *testing.T) {
	snSrv := simplenotification.NewTestServer(simplenotification.Config{})
	defer snSrv.Close()

	groupID := createNotificationGroup(t, snSrv.TestURL(), "test-group")

	env := serviceLinkEnv(map[core.ServiceConfig]string{
		simplenotification.Config{}: snSrv.TestURL(),
	})
	ebSrv := eventbus.NewTestServerWithServiceLink(eventbus.Config{}, env)
	defer ebSrv.Close()

	client := newTestClient(t, ebSrv.TestURL())
	pcOp := sdk.NewProcessConfigurationOp(client)
	triggerOp := sdk.NewTriggerOp(client)

	pc, err := pcOp.Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name:     "pc-simplenotification",
			Settings: sdk.CreateSimpleNotificationSettings(groupID, "hello from eventbus"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = triggerOp.Create(t.Context(), v1.CreateCommonServiceItemRequest{
		CommonServiceItem: v1.CreateCommonServiceItemRequestCommonServiceItem{
			Name: "trigger-simplenotification",
			Settings: v1.NewTriggerSettingsSettings(v1.TriggerSettings{
				Source:                 "test",
				ProcessConfigurationID: pc.ID,
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := postJSON(t, ebSrv.TestURL()+"/_sakumock/events", map[string]any{
		"Source": "test",
	})
	if got.Count != 1 {
		t.Fatalf("expected 1 delivery, got %d", got.Count)
	}
	if got.Deliveries[0].Error != "" {
		t.Fatalf("delivery error: %s", got.Deliveries[0].Error)
	}
	if got.Deliveries[0].Destination != "simplenotification" {
		t.Errorf("expected destination simplenotification, got %s", got.Deliveries[0].Destination)
	}

	msgs := inspectNotifications(t, snSrv.TestURL())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 notification message, got %d", len(msgs))
	}
	if msgs[0].Message != "hello from eventbus" {
		t.Errorf("unexpected notification message: %s", msgs[0].Message)
	}
	if msgs[0].GroupID != groupID {
		t.Errorf("unexpected group_id: %s", msgs[0].GroupID)
	}
}

func createNotificationGroup(t *testing.T, baseURL, name string) string {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_SIMPLE_NOTIFICATION=" + baseURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := simplenotificationsdk.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	group, err := simplenotificationsdk.NewGroupOp(client).Create(t.Context(), snv1.PostCommonServiceItemRequest{
		CommonServiceItem: snv1.PostCommonServiceItemRequestCommonServiceItem{
			Name: name,
			Icon: snv1.NilCommonServiceItemIcon{Null: true},
			Settings: snv1.CommonServiceItemSettings{
				GroupSettings: snv1.GroupSettings{
					Destinations: []string{},
				},
			},
			Tags: []string{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return group.CommonServiceItem.ID
}

type snMessage struct {
	GroupID string `json:"group_id"`
	Message string `json:"message"`
}

func inspectNotifications(t *testing.T, baseURL string) []snMessage {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/_sakumock/messages")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("inspect notifications: status %d", resp.StatusCode)
	}
	var result struct {
		Messages []snMessage `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result.Messages
}

type mqQueueSecurity struct{}

func (s *mqQueueSecurity) ApiKeyAuth(_ context.Context, _ queue.OperationName) (queue.ApiKeyAuth, error) {
	return queue.ApiKeyAuth{Username: "dummy", Password: "dummy"}, nil
}

func createQueue(t *testing.T, baseURL, name string) {
	t.Helper()
	client, err := queue.NewClient(baseURL, &mqQueueSecurity{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.CreateQueue(t.Context(), &queue.CreateQueueRequest{
		CommonServiceItem: queue.CreateQueueRequestCommonServiceItem{
			Name:     queue.QueueName(name),
			Provider: queue.CreateQueueRequestCommonServiceItemProvider{Class: queue.CreateQueueRequestCommonServiceItemProviderClassSimplemq},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.(*queue.CreateQueueCreated); !ok {
		t.Fatalf("expected CreateQueueCreated, got %T", res)
	}
}

type mqMessageSecurity struct{}

func (s *mqMessageSecurity) ApiKeyAuth(_ context.Context, _ message.OperationName) (message.ApiKeyAuth, error) {
	return message.ApiKeyAuth{Token: "dummy"}, nil
}

type mqMessage struct {
	Content string `json:"content"`
}

func receiveFromQueue(t *testing.T, baseURL, queueName string) []mqMessage {
	t.Helper()
	client, err := message.NewClient(baseURL, &mqMessageSecurity{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.ReceiveMessage(t.Context(), message.ReceiveMessageParams{QueueName: message.QueueName(queueName)})
	if err != nil {
		t.Fatal(err)
	}
	recvOK, ok := res.(*message.ReceiveMessageOK)
	if !ok {
		t.Fatalf("expected ReceiveMessageOK, got %T", res)
	}
	msgs := make([]mqMessage, len(recvOK.Messages))
	for i, m := range recvOK.Messages {
		msgs[i] = mqMessage{Content: string(m.Content)}
	}
	return msgs
}
