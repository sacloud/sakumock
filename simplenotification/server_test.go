package simplenotification_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

	resp, body := rawSend(t, srv.TestURL(), "abc", map[string]string{"Message": "hi"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if len(srv.Messages()) != 0 {
		t.Fatalf("expected 0 stored messages, got %d", len(srv.Messages()))
	}

	var got struct {
		Status   string `json:"status"`
		ErrorMsg string `json:"error_msg"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to parse error response: %v (body=%s)", err, body)
	}
	if got.ErrorMsg == "" {
		t.Fatalf("expected error_msg field, got %s", body)
	}
	if !strings.HasPrefix(got.Status, "400 ") {
		t.Fatalf("expected status field starting with '400 ', got %q", got.Status)
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

func TestInspectMessages(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()

	for _, m := range []string{"first", "second"} {
		resp, _ := rawSend(t, srv.TestURL(), "123456789012", map[string]string{"Message": m})
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("send %q: expected 202, got %d", m, resp.StatusCode)
		}
	}

	resp, err := http.Get(srv.TestURL() + "/_sakumock/messages")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got struct {
		Messages []struct {
			ID        string `json:"id"`
			GroupID   string `json:"group_id"`
			Message   string `json:"message"`
			CreatedAt string `json:"created_at"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got.Messages))
	}
	if got.Messages[0].Message != "first" || got.Messages[1].Message != "second" {
		t.Fatalf("unexpected message order: %+v", got.Messages)
	}
	if got.Messages[0].GroupID != "123456789012" {
		t.Fatalf("unexpected group id: %s", got.Messages[0].GroupID)
	}
	if got.Messages[0].CreatedAt == "" {
		t.Fatalf("expected non-empty created_at")
	}
}

func TestSendMessage_Exec(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exec test requires sh")
	}
	out := filepath.Join(t.TempDir(), "out")
	script := fmt.Sprintf(`{ printf "msg="; cat; printf "\ngroup=%%s\nid=%%s\n" "$SAKUMOCK_GROUP_ID" "$SAKUMOCK_MESSAGE_ID"; } > %s`, out)
	srv := simplenotification.NewTestServer(simplenotification.Config{Exec: script})
	defer srv.Close()

	resp, _ := rawSend(t, srv.TestURL(), "123456789012", map[string]string{"Message": "exec-me"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		data, err := os.ReadFile(out)
		if err == nil && bytes.Contains(data, []byte("msg=exec-me")) && bytes.Contains(data, []byte("group=123456789012")) {
			if !bytes.Contains(data, []byte("id=1")) {
				t.Fatalf("expected message id in output, got %q", data)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for exec output (last read: %q, err=%v)", data, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSendMessage_ExecFailureStillReturns202(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exec test requires sh")
	}
	srv := simplenotification.NewTestServer(simplenotification.Config{Exec: "exit 1"})
	defer srv.Close()

	resp, _ := rawSend(t, srv.TestURL(), "123456789012", map[string]string{"Message": "doomed"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 even though exec fails, got %d", resp.StatusCode)
	}
	if len(srv.Messages()) != 1 {
		t.Fatalf("expected message to be stored even though exec failed")
	}
}

func TestResetMessages(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()

	rawSend(t, srv.TestURL(), "123456789012", map[string]string{"Message": "to be reset"})
	if len(srv.Messages()) != 1 {
		t.Fatalf("expected 1 message before reset")
	}

	req, _ := http.NewRequest(http.MethodDelete, srv.TestURL()+"/_sakumock/messages", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	if len(srv.Messages()) != 0 {
		t.Fatalf("expected 0 messages after reset, got %d", len(srv.Messages()))
	}
}
