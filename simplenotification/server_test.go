package simplenotification_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sacloud/saclient-go"
	sdk "github.com/sacloud/simple-notification-api-go"
	v1 "github.com/sacloud/simple-notification-api-go/apis/v1"

	"github.com/sacloud/sakumock/simplenotification"
)

func newTestGroupOp(t *testing.T, serverURL string) sdk.GroupAPI {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_SIMPLE_NOTIFICATION=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := sdk.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return sdk.NewGroupOp(client)
}

func TestSendMessage_Success(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()
	ctx := t.Context()
	groupOp := newTestGroupOp(t, srv.TestURL())

	resp, err := groupOp.SendMessage(ctx, "123456789012", v1.SendNotificationMessageRequest{Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsOk {
		t.Fatalf("expected IsOk=true, got %v", resp.IsOk)
	}

	msgs := srv.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].GroupID != "123456789012" {
		t.Fatalf("unexpected group id: %s", msgs[0].GroupID)
	}
	if msgs[0].Message != "hello" {
		t.Fatalf("unexpected message: %s", msgs[0].Message)
	}
}

func TestSendMessage_Multiple(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()
	ctx := t.Context()
	groupOp := newTestGroupOp(t, srv.TestURL())

	for _, m := range []string{"first", "second", "third"} {
		if _, err := groupOp.SendMessage(ctx, "123456789012", v1.SendNotificationMessageRequest{Message: m}); err != nil {
			t.Fatal(err)
		}
	}

	msgs := srv.Messages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	for i, want := range []string{"first", "second", "third"} {
		if msgs[i].Message != want {
			t.Fatalf("messages[%d]: expected %q, got %q", i, want, msgs[i].Message)
		}
	}
}

// rawSend posts directly to the mock without going through the SDK so we can
// exercise validation behavior that the SDK rejects on the client side.
func rawSend(t *testing.T, baseURL, id string, body any) (*http.Response, []byte) {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/commonserviceitem/"+id+"/simplenotification/message", bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, respBody
}

func TestSendMessage_InvalidID(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()

	resp, _ := rawSend(t, srv.TestURL(), "abc", map[string]string{"Message": "hi"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if len(srv.Messages()) != 0 {
		t.Fatalf("expected 0 stored messages, got %d", len(srv.Messages()))
	}
}

func TestSendMessage_EmptyMessage(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()

	resp, _ := rawSend(t, srv.TestURL(), "123456789012", map[string]string{"Message": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSendMessage_TooLongMessage(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()

	resp, _ := rawSend(t, srv.TestURL(), "123456789012", map[string]string{"Message": strings.Repeat("a", 2049)})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSendMessage_MaxLengthMessage(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()

	resp, _ := rawSend(t, srv.TestURL(), "123456789012", map[string]string{"Message": strings.Repeat("a", 2048)})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(srv.Messages()) != 1 {
		t.Fatalf("expected 1 stored message, got %d", len(srv.Messages()))
	}
}
