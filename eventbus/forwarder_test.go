package eventbus_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	sdk "github.com/sacloud/sacloud-sdk-go/api/eventbus"
	v1 "github.com/sacloud/sacloud-sdk-go/api/eventbus/apis/v1"

	"github.com/sacloud/sakumock/eventbus"
	"github.com/sacloud/sakumock/simplemq"
	"github.com/sacloud/sakumock/simplenotification"
)

func TestForwardToSimpleMQ(t *testing.T) {
	mqSrv := simplemq.NewTestServer(simplemq.Config{})
	defer mqSrv.Close()

	createQueue(t, mqSrv.TestURL(), "test-queue-00001")

	ebSrv := eventbus.NewTestServerWithEndpoints(eventbus.Config{}, map[string]string{
		"simplemq": mqSrv.TestURL(),
	})
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

	ebSrv := eventbus.NewTestServerWithEndpoints(eventbus.Config{}, map[string]string{
		"simplenotification": snSrv.TestURL(),
	})
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

// createNotificationGroup creates a notification group via the SimpleNotification
// control-plane API and returns its ID.
func createNotificationGroup(t *testing.T, baseURL, name string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"CommonServiceItem": map[string]any{
			"Name": name,
			"Provider": map[string]any{
				"Class": "saknoticegroup",
			},
		},
	})
	resp, err := http.Post(baseURL+"/commonserviceitem", "application/json",
		bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create notification group: status %d", resp.StatusCode)
	}
	var result struct {
		CommonServiceItem struct {
			ID string `json:"ID"`
		} `json:"CommonServiceItem"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result.CommonServiceItem.ID
}

type snMessage struct {
	GroupID string `json:"group_id"`
	Message string `json:"message"`
}

// inspectNotifications retrieves accepted notifications from the /_sakumock/messages endpoint.
func inspectNotifications(t *testing.T, baseURL string) []snMessage {
	t.Helper()
	resp, err := http.Get(baseURL + "/_sakumock/messages")
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

func createQueue(t *testing.T, baseURL, name string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"CommonServiceItem": map[string]any{
			"Name": name,
			"Provider": map[string]any{
				"Class": "simplemq",
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/commonserviceitem", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("dummy", "dummy")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create queue: status %d", resp.StatusCode)
	}
}

type mqMessage struct {
	Content string `json:"content"`
}

func receiveFromQueue(t *testing.T, baseURL, queueName string) []mqMessage {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/queues/"+queueName+"/messages", nil)
	req.Header.Set("Authorization", "Bearer dummy")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("receive messages: status %d", resp.StatusCode)
	}
	var result struct {
		Messages []mqMessage `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result.Messages
}
