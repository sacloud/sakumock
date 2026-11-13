package simplemq_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sacloud/sacloud-sdk-go/api/simplemq/apis/v1/message"
	"github.com/sacloud/sacloud-sdk-go/api/simplemq/apis/v1/queue"

	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/simplemq"
)

// runWithState serves cfg on opts, as the unified binary builds it, for the
// duration of fn.
func runWithState(t *testing.T, cfg simplemq.Config, opts core.ServerOptions, fn func(url string)) {
	t.Helper()
	srv, err := cfg.NewServer(opts)
	if err != nil {
		t.Fatalf("start simplemq: %v", err)
	}
	ts := httptest.NewServer(srv)
	fn(ts.URL)
	ts.Close()
	if violations := srv.(*simplemq.Server).SpecViolations(); len(violations) != 0 {
		t.Errorf("spec violations: %+v", violations)
	}
	srv.Close()
}

// The state directory holds a SQLite database rather than a JSON snapshot; a
// queue, its settings and an unreceived message survive a restart, and a queue
// created afterwards gets a new ID even though the shared ID generator of the
// second run starts over.
func TestStateSurvivesRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	ctx := t.Context()
	const queueName = "persisted"
	var queueID, messageID string

	runWithState(t, simplemq.Config{}, core.ServerOptions{StateDir: dir, IDGen: core.NewIDGenerator(900_000_000_100)}, func(url string) {
		queues := newTestQueueClient(t, url)
		queueID = createQueue(t, ctx, queues, queueName)
		if _, err := queues.UpdateQueue(ctx, &queue.ConfigQueueRequest{
			CommonServiceItem: queue.ConfigQueueRequestCommonServiceItem{
				Settings: queue.Settings{VisibilityTimeoutSeconds: 45, ExpireSeconds: 3600},
			},
		}, queue.UpdateQueueParams{ID: queueID}); err != nil {
			t.Fatalf("UpdateQueue failed: %v", err)
		}
		res, err := newTestClient(t, url, "token").SendMessage(ctx,
			&message.SendRequest{Content: message.MessageContent(b64("hello"))},
			message.SendMessageParams{QueueName: queueName})
		if err != nil {
			t.Fatalf("send failed: %v", err)
		}
		sent, ok := res.(*message.SendMessageOK)
		if !ok {
			t.Fatalf("expected SendMessageOK, got %T", res)
		}
		messageID = string(sent.Message.ID)
	})
	if t.Failed() {
		return
	}
	if _, err := os.Stat(filepath.Join(dir, "simplemq.db")); err != nil {
		t.Fatalf("state database not written: %v", err)
	}

	// The second run's shared generator starts below the persisted queue ID.
	ids := core.NewIDGenerator(900_000_000_000)
	runWithState(t, simplemq.Config{}, core.ServerOptions{StateDir: dir, IDGen: ids}, func(url string) {
		queues := newTestQueueClient(t, url)
		res, err := queues.ReadQueue(ctx, queue.ReadQueueParams{ID: queueID})
		if err != nil {
			t.Fatalf("ReadQueue failed: %v", err)
		}
		got, ok := res.(*queue.ReadQueueOK)
		if !ok {
			t.Fatalf("expected ReadQueueOK, got %T", res)
		}
		csi := got.CommonServiceItem
		if string(csi.Name) != queueName || csi.Settings.GetVisibilityTimeoutSeconds() != 45 || csi.Settings.GetExpireSeconds() != 3600 {
			t.Errorf("restored queue = %+v", csi)
		}

		recvRes, err := newTestClient(t, url, "token").ReceiveMessage(ctx, message.ReceiveMessageParams{QueueName: queueName})
		if err != nil {
			t.Fatalf("receive failed: %v", err)
		}
		recv, ok := recvRes.(*message.ReceiveMessageOK)
		if !ok {
			t.Fatalf("expected ReceiveMessageOK, got %T", recvRes)
		}
		if len(recv.Messages) != 1 || string(recv.Messages[0].ID) != messageID || string(recv.Messages[0].Content) != b64("hello") {
			t.Errorf("restored messages = %+v, want the one sent before the restart", recv.Messages)
		}

		id := createQueue(t, ctx, queues, "created-after-restart")
		if id <= queueID {
			t.Errorf("queue created after the restart got ID %s, want one past the restored %s", id, queueID)
		}
		if next := ids.Next(); next <= id {
			t.Errorf("shared generator is at %s, behind the new queue ID %s: the store did not allocate from it", next, id)
		}
	})
}

// An explicit --database wins over --state-dir.
func TestStateDirYieldsToDatabase(t *testing.T) {
	dir := t.TempDir()
	database := filepath.Join(t.TempDir(), "explicit.db")
	runWithState(t, simplemq.Config{Database: database}, core.ServerOptions{StateDir: dir}, func(url string) {
		createQueue(t, t.Context(), newTestQueueClient(t, url), "explicit-database")
	})
	if _, err := os.Stat(database); err != nil {
		t.Errorf("explicit database not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "simplemq.db")); !os.IsNotExist(err) {
		t.Errorf("state dir database exists although --database was set (stat error: %v)", err)
	}
}
