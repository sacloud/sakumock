package simplemq_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/sacloud/sakumock/simplemq"
	simplemqsdk "github.com/sacloud/simplemq-api-go"
	"github.com/sacloud/simplemq-api-go/apis/v1/message"
	"github.com/sacloud/simplemq-api-go/apis/v1/queue"
)

type testQueueSecuritySource struct {
	username string
	password string
}

func (s *testQueueSecuritySource) ApiKeyAuth(_ context.Context, _ queue.OperationName) (queue.ApiKeyAuth, error) {
	return queue.ApiKeyAuth{Username: s.username, Password: s.password}, nil
}

func newTestQueueClient(t *testing.T, serverURL string) *queue.Client {
	t.Helper()
	client, err := queue.NewClient(serverURL, &testQueueSecuritySource{username: "token", password: "secret"})
	if err != nil {
		t.Fatalf("failed to create queue client: %v", err)
	}
	return client
}

// controlPlaneBackends enumerates the storage backends every control plane test
// runs against, so the SQLite-backed implementation is exercised alongside the
// in-memory one.
var controlPlaneBackends = []struct {
	name   string
	config func(t *testing.T) simplemq.Config
}{
	{"memory", func(t *testing.T) simplemq.Config { return simplemq.Config{} }},
	{"sqlite", func(t *testing.T) simplemq.Config {
		return simplemq.Config{Database: filepath.Join(t.TempDir(), "test.db")}
	}},
}

// eachBackend runs fn as a subtest against every storage backend.
func eachBackend(t *testing.T, fn func(t *testing.T, srv *simplemq.Server)) {
	t.Helper()
	for _, b := range controlPlaneBackends {
		t.Run(b.name, func(t *testing.T) {
			srv := simplemq.NewTestServer(b.config(t))
			defer srv.Close()
			fn(t, srv)
		})
	}
}

// createQueue is a helper that creates a queue and returns its resource ID.
func createQueue(t *testing.T, ctx context.Context, client *queue.Client, name string) string {
	t.Helper()
	res, err := client.CreateQueue(ctx, &queue.CreateQueueRequest{
		CommonServiceItem: queue.CreateQueueRequestCommonServiceItem{
			Name:     queue.QueueName(name),
			Provider: queue.CreateQueueRequestCommonServiceItemProvider{Class: queue.CreateQueueRequestCommonServiceItemProviderClassSimplemq},
		},
	})
	if err != nil {
		t.Fatalf("CreateQueue failed: %v", err)
	}
	created, ok := res.(*queue.CreateQueueCreated)
	if !ok {
		t.Fatalf("expected CreateQueueCreated, got %T", res)
	}
	return simplemqsdk.GetQueueID(&created.CommonServiceItem)
}

func TestCreateAndGetQueue(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		res, err := client.CreateQueue(ctx, &queue.CreateQueueRequest{
			CommonServiceItem: queue.CreateQueueRequestCommonServiceItem{
				Name:     "test-queue-1",
				Provider: queue.CreateQueueRequestCommonServiceItemProvider{Class: queue.CreateQueueRequestCommonServiceItemProviderClassSimplemq},
			},
		})
		if err != nil {
			t.Fatalf("CreateQueue failed: %v", err)
		}
		created, ok := res.(*queue.CreateQueueCreated)
		if !ok {
			t.Fatalf("expected CreateQueueCreated, got %T", res)
		}
		csi := created.CommonServiceItem
		if csi.Name != "test-queue-1" {
			t.Errorf("expected Name=test-queue-1, got %s", csi.Name)
		}
		if csi.Status.GetQueueName() != "test-queue-1" {
			t.Errorf("expected Status.QueueName=test-queue-1, got %s", csi.Status.GetQueueName())
		}
		if csi.ServiceClass != "cloud/simplemq/1" {
			t.Errorf("expected ServiceClass=cloud/simplemq/1, got %s", csi.ServiceClass)
		}
		if csi.Availability != "available" {
			t.Errorf("expected Availability=available, got %s", csi.Availability)
		}

		id := simplemqsdk.GetQueueID(&csi)

		// GetQueue
		getRes, err := client.GetQueue(ctx, queue.GetQueueParams{ID: id})
		if err != nil {
			t.Fatalf("GetQueue failed: %v", err)
		}
		got, ok := getRes.(*queue.GetQueueOK)
		if !ok {
			t.Fatalf("expected GetQueueOK, got %T", getRes)
		}
		if simplemqsdk.GetQueueID(&got.CommonServiceItem) != id {
			t.Errorf("expected ID=%s, got %s", id, simplemqsdk.GetQueueID(&got.CommonServiceItem))
		}
	})
}

func TestQueueIDFormat(t *testing.T) {
	// Generated IDs must be realistic 12-digit values with no leading zeros so
	// they round-trip through clients that parse the oneOf string|int ID as an
	// integer (re-formatting an int must yield the same string).
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		id := createQueue(t, ctx, client, "id-format-queue")

		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			t.Fatalf("queue ID %q is not numeric: %v", id, err)
		}
		if got := strconv.FormatInt(n, 10); got != id {
			t.Errorf("queue ID %q does not round-trip as integer (got %q)", id, got)
		}
		if n < 100000000000 {
			t.Errorf("expected a 12-digit queue ID, got %q", id)
		}

		// The integer form must still resolve via GetQueue.
		getRes, err := client.GetQueue(ctx, queue.GetQueueParams{ID: strconv.FormatInt(n, 10)})
		if err != nil {
			t.Fatalf("GetQueue failed: %v", err)
		}
		if _, ok := getRes.(*queue.GetQueueOK); !ok {
			t.Fatalf("expected GetQueueOK for integer-form ID, got %T", getRes)
		}
	})
}

func TestGetQueueNotFound(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		res, err := client.GetQueue(ctx, queue.GetQueueParams{ID: "999999999999"})
		if err != nil {
			t.Fatalf("GetQueue request failed: %v", err)
		}
		if _, ok := res.(*queue.GetQueueNotFound); !ok {
			t.Fatalf("expected GetQueueNotFound, got %T", res)
		}
	})
}

func TestCreateQueueConflict(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		req := &queue.CreateQueueRequest{
			CommonServiceItem: queue.CreateQueueRequestCommonServiceItem{
				Name:     "conflict-queue",
				Provider: queue.CreateQueueRequestCommonServiceItemProvider{Class: queue.CreateQueueRequestCommonServiceItemProviderClassSimplemq},
			},
		}

		if _, err := client.CreateQueue(ctx, req); err != nil {
			t.Fatalf("first CreateQueue failed: %v", err)
		}

		res, err := client.CreateQueue(ctx, req)
		if err != nil {
			t.Fatalf("second CreateQueue request failed: %v", err)
		}
		if _, ok := res.(*queue.CreateQueueConflict); !ok {
			t.Fatalf("expected CreateQueueConflict, got %T", res)
		}
	})
}

func TestListQueues(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		for i := range 3 {
			createQueue(t, ctx, client, fmt.Sprintf("list-queue-%d", i))
		}

		res, err := client.GetQueues(ctx)
		if err != nil {
			t.Fatalf("GetQueues failed: %v", err)
		}
		got, ok := res.(*queue.GetQueuesOK)
		if !ok {
			t.Fatalf("expected GetQueuesOK, got %T", res)
		}
		if len(got.CommonServiceItems) != 3 {
			t.Errorf("expected 3 queues, got %d", len(got.CommonServiceItems))
		}
	})
}

func TestConfigQueue(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		id := createQueue(t, ctx, client, "config-queue")

		configRes, err := client.ConfigQueue(ctx, &queue.ConfigQueueRequest{
			CommonServiceItem: queue.ConfigQueueRequestCommonServiceItem{
				Settings: queue.Settings{
					VisibilityTimeoutSeconds: 60,
					ExpireSeconds:            86400,
				},
			},
		}, queue.ConfigQueueParams{ID: id})
		if err != nil {
			t.Fatalf("ConfigQueue failed: %v", err)
		}
		updated, ok := configRes.(*queue.ConfigQueueOK)
		if !ok {
			t.Fatalf("expected ConfigQueueOK, got %T", configRes)
		}
		if updated.CommonServiceItem.Settings.GetVisibilityTimeoutSeconds() != 60 {
			t.Errorf("expected VisibilityTimeoutSeconds=60, got %d", updated.CommonServiceItem.Settings.GetVisibilityTimeoutSeconds())
		}
		if updated.CommonServiceItem.Settings.GetExpireSeconds() != 86400 {
			t.Errorf("expected ExpireSeconds=86400, got %d", updated.CommonServiceItem.Settings.GetExpireSeconds())
		}

		// The change must survive a round-trip read (important for SQLite).
		getRes, err := client.GetQueue(ctx, queue.GetQueueParams{ID: id})
		if err != nil {
			t.Fatalf("GetQueue failed: %v", err)
		}
		gotSettings := getRes.(*queue.GetQueueOK).CommonServiceItem.Settings
		if gotSettings.GetVisibilityTimeoutSeconds() != 60 || gotSettings.GetExpireSeconds() != 86400 {
			t.Errorf("settings not persisted: got vt=%d exp=%d", gotSettings.GetVisibilityTimeoutSeconds(), gotSettings.GetExpireSeconds())
		}
	})
}

func TestConfigQueueNotFound(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		res, err := client.ConfigQueue(ctx, &queue.ConfigQueueRequest{
			CommonServiceItem: queue.ConfigQueueRequestCommonServiceItem{
				Settings: queue.Settings{VisibilityTimeoutSeconds: 30, ExpireSeconds: 3600},
			},
		}, queue.ConfigQueueParams{ID: "999999999999"})
		if err != nil {
			t.Fatalf("ConfigQueue request failed: %v", err)
		}
		if _, ok := res.(*queue.ConfigQueueNotFound); !ok {
			t.Fatalf("expected ConfigQueueNotFound, got %T", res)
		}
	})
}

func TestDeleteQueue(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		id := createQueue(t, ctx, client, "delete-queue")

		delRes, err := client.DeleteQueue(ctx, queue.DeleteQueueParams{ID: id})
		if err != nil {
			t.Fatalf("DeleteQueue failed: %v", err)
		}
		if _, ok := delRes.(*queue.DeleteQueueOK); !ok {
			t.Fatalf("expected DeleteQueueOK, got %T", delRes)
		}

		// Queue should no longer exist
		getRes, err := client.GetQueue(ctx, queue.GetQueueParams{ID: id})
		if err != nil {
			t.Fatalf("GetQueue request failed: %v", err)
		}
		if _, ok := getRes.(*queue.GetQueueNotFound); !ok {
			t.Fatalf("expected GetQueueNotFound after delete, got %T", getRes)
		}
	})
}

func TestDeleteQueueRemovesMessages(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		cpClient := newTestQueueClient(t, srv.TestURL())
		dpClient := newTestClient(t, srv.TestURL(), "test-api-key")

		const queueName = "purge-queue"
		id := createQueue(t, ctx, cpClient, queueName)

		content := message.MessageContent(base64.StdEncoding.EncodeToString([]byte("hello")))
		if _, err := dpClient.SendMessage(ctx, &message.SendRequest{Content: content}, message.SendMessageParams{QueueName: queueName}); err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		if _, err := cpClient.DeleteQueue(ctx, queue.DeleteQueueParams{ID: id}); err != nil {
			t.Fatalf("DeleteQueue failed: %v", err)
		}

		// Recreate with the same name; messages from the deleted queue must not linger.
		newID := createQueue(t, ctx, cpClient, queueName)
		countRes, err := cpClient.GetMessageCount(ctx, queue.GetMessageCountParams{ID: newID})
		if err != nil {
			t.Fatalf("GetMessageCount failed: %v", err)
		}
		if c := countRes.(*queue.GetMessageCountOK).SimpleMQ.GetCount(); c != 0 {
			t.Errorf("expected 0 messages after queue delete+recreate, got %d", c)
		}
	})
}

func TestDeleteQueueNotFound(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		res, err := client.DeleteQueue(ctx, queue.DeleteQueueParams{ID: "999999999999"})
		if err != nil {
			t.Fatalf("DeleteQueue request failed: %v", err)
		}
		if _, ok := res.(*queue.DeleteQueueNotFound); !ok {
			t.Fatalf("expected DeleteQueueNotFound, got %T", res)
		}
	})
}

func TestGetMessageCount(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		cpClient := newTestQueueClient(t, srv.TestURL())
		dpClient := newTestClient(t, srv.TestURL(), "test-api-key")

		id := createQueue(t, ctx, cpClient, "count-queue")

		// Count before sending (should be 0)
		countRes, err := cpClient.GetMessageCount(ctx, queue.GetMessageCountParams{ID: id})
		if err != nil {
			t.Fatalf("GetMessageCount failed: %v", err)
		}
		countOK, ok := countRes.(*queue.GetMessageCountOK)
		if !ok {
			t.Fatalf("expected GetMessageCountOK, got %T", countRes)
		}
		if countOK.SimpleMQ.GetCount() != 0 {
			t.Errorf("expected count=0, got %d", countOK.SimpleMQ.GetCount())
		}

		// Send 3 messages via data plane
		content := message.MessageContent(base64.StdEncoding.EncodeToString([]byte("hello")))
		for range 3 {
			if _, err := dpClient.SendMessage(ctx, &message.SendRequest{Content: content}, message.SendMessageParams{QueueName: "count-queue"}); err != nil {
				t.Fatalf("SendMessage failed: %v", err)
			}
		}

		// Count after sending (should be 3)
		countRes2, err := cpClient.GetMessageCount(ctx, queue.GetMessageCountParams{ID: id})
		if err != nil {
			t.Fatalf("GetMessageCount failed: %v", err)
		}
		countOK2 := countRes2.(*queue.GetMessageCountOK)
		if countOK2.SimpleMQ.GetCount() != 3 {
			t.Errorf("expected count=3, got %d", countOK2.SimpleMQ.GetCount())
		}
	})
}

func TestGetMessageCountNotFound(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		res, err := client.GetMessageCount(ctx, queue.GetMessageCountParams{ID: "999999999999"})
		if err != nil {
			t.Fatalf("GetMessageCount request failed: %v", err)
		}
		if _, ok := res.(*queue.GetMessageCountNotFound); !ok {
			t.Fatalf("expected GetMessageCountNotFound, got %T", res)
		}
	})
}

func TestRotateAPIKey(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		id := createQueue(t, ctx, client, "rotate-queue")

		rotateRes, err := client.RotateAPIKey(ctx, queue.RotateAPIKeyParams{ID: id})
		if err != nil {
			t.Fatalf("RotateAPIKey failed: %v", err)
		}
		rotateOK, ok := rotateRes.(*queue.RotateAPIKeyOK)
		if !ok {
			t.Fatalf("expected RotateAPIKeyOK, got %T", rotateRes)
		}
		if rotateOK.SimpleMQ.GetApikey() == "" {
			t.Error("expected non-empty apikey")
		}

		// Rotate again should return a different key
		rotateRes2, err := client.RotateAPIKey(ctx, queue.RotateAPIKeyParams{ID: id})
		if err != nil {
			t.Fatalf("second RotateAPIKey failed: %v", err)
		}
		rotateOK2 := rotateRes2.(*queue.RotateAPIKeyOK)
		if rotateOK2.SimpleMQ.GetApikey() == rotateOK.SimpleMQ.GetApikey() {
			t.Error("expected different API key after second rotation")
		}
	})
}

func TestRotateAPIKeyNotFound(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		res, err := client.RotateAPIKey(ctx, queue.RotateAPIKeyParams{ID: "999999999999"})
		if err != nil {
			t.Fatalf("RotateAPIKey request failed: %v", err)
		}
		if _, ok := res.(*queue.RotateAPIKeyNotFound); !ok {
			t.Fatalf("expected RotateAPIKeyNotFound, got %T", res)
		}
	})
}

func TestClearMessages(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		cpClient := newTestQueueClient(t, srv.TestURL())
		dpClient := newTestClient(t, srv.TestURL(), "test-api-key")

		id := createQueue(t, ctx, cpClient, "clear-queue")

		// Send 2 messages
		content := message.MessageContent(base64.StdEncoding.EncodeToString([]byte("hello")))
		for range 2 {
			if _, err := dpClient.SendMessage(ctx, &message.SendRequest{Content: content}, message.SendMessageParams{QueueName: "clear-queue"}); err != nil {
				t.Fatalf("SendMessage failed: %v", err)
			}
		}

		// Clear messages
		clearRes, err := cpClient.ClearQueue(ctx, queue.ClearQueueParams{ID: id})
		if err != nil {
			t.Fatalf("ClearQueue failed: %v", err)
		}
		if _, ok := clearRes.(*queue.ClearQueueOK); !ok {
			t.Fatalf("expected ClearQueueOK, got %T", clearRes)
		}

		// Count should be 0
		countRes, err := cpClient.GetMessageCount(ctx, queue.GetMessageCountParams{ID: id})
		if err != nil {
			t.Fatalf("GetMessageCount failed: %v", err)
		}
		countOK := countRes.(*queue.GetMessageCountOK)
		if countOK.SimpleMQ.GetCount() != 0 {
			t.Errorf("expected count=0 after clear, got %d", countOK.SimpleMQ.GetCount())
		}
	})
}

func TestClearMessagesNotFound(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		client := newTestQueueClient(t, srv.TestURL())

		res, err := client.ClearQueue(ctx, queue.ClearQueueParams{ID: "999999999999"})
		if err != nil {
			t.Fatalf("ClearQueue request failed: %v", err)
		}
		if _, ok := res.(*queue.ClearQueueNotFound); !ok {
			t.Fatalf("expected ClearQueueNotFound, got %T", res)
		}
	})
}

func TestControlPlaneUnauthorized(t *testing.T) {
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		// Empty credentials should fail
		noAuthClient, err := queue.NewClient(srv.TestURL(), &testQueueSecuritySource{username: "", password: ""})
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		res, err := noAuthClient.GetQueues(ctx)
		if err != nil {
			t.Fatalf("GetQueues request failed: %v", err)
		}
		if _, ok := res.(*queue.GetQueuesUnauthorized); !ok {
			t.Fatalf("expected GetQueuesUnauthorized, got %T", res)
		}
	})
}

func TestStrictModeDataPlaneAuth(t *testing.T) {
	// In strict mode the data plane only accepts queues created via the control
	// plane, authenticated with the API key issued by rotate-apikey.
	for _, b := range controlPlaneBackends {
		t.Run(b.name, func(t *testing.T) {
			cfg := b.config(t)
			cfg.Strict = true
			srv := simplemq.NewTestServer(cfg)
			defer srv.Close()

			ctx := t.Context()
			cpClient := newTestQueueClient(t, srv.TestURL())
			const queueName = "strict-queue"
			id := createQueue(t, ctx, cpClient, queueName)

			content := message.MessageContent(base64.StdEncoding.EncodeToString([]byte("hello")))
			send := func(token, queue string) message.SendMessageRes {
				t.Helper()
				c := newTestClient(t, srv.TestURL(), token)
				res, err := c.SendMessage(ctx, &message.SendRequest{Content: content}, message.SendMessageParams{QueueName: message.QueueName(queue)})
				if err != nil {
					t.Fatalf("send request failed: %v", err)
				}
				return res
			}

			// Before a key is issued, no token is accepted.
			if _, ok := send("anything", queueName).(*message.SendMessageUnauthorized); !ok {
				t.Fatal("expected unauthorized before key issuance")
			}

			// Issue the key via rotate-apikey.
			rotateRes, err := cpClient.RotateAPIKey(ctx, queue.RotateAPIKeyParams{ID: id})
			if err != nil {
				t.Fatalf("RotateAPIKey failed: %v", err)
			}
			key := rotateRes.(*queue.RotateAPIKeyOK).SimpleMQ.GetApikey()

			// Correct key succeeds.
			if _, ok := send(key, queueName).(*message.SendMessageOK); !ok {
				t.Fatal("expected success with issued key")
			}
			// Wrong key is rejected.
			if _, ok := send("wrong-key", queueName).(*message.SendMessageUnauthorized); !ok {
				t.Fatal("expected unauthorized with wrong key")
			}
			// A queue never created via the control plane is rejected even with a valid-looking key.
			if _, ok := send(key, "ghost-queue").(*message.SendMessageUnauthorized); !ok {
				t.Fatal("expected unauthorized for queue not created via control plane")
			}

			// Rotating again invalidates the old key.
			rotateRes2, err := cpClient.RotateAPIKey(ctx, queue.RotateAPIKeyParams{ID: id})
			if err != nil {
				t.Fatalf("second RotateAPIKey failed: %v", err)
			}
			newKey := rotateRes2.(*queue.RotateAPIKeyOK).SimpleMQ.GetApikey()
			if _, ok := send(key, queueName).(*message.SendMessageUnauthorized); !ok {
				t.Fatal("expected old key to be rejected after rotation")
			}
			if _, ok := send(newKey, queueName).(*message.SendMessageOK); !ok {
				t.Fatal("expected new key to be accepted after rotation")
			}
		})
	}
}

func TestConfigQueueSettingsAffectDataPlane(t *testing.T) {
	// Verify that updating queue settings via control plane changes data plane behavior.
	eachBackend(t, func(t *testing.T, srv *simplemq.Server) {
		ctx := t.Context()
		cpClient := newTestQueueClient(t, srv.TestURL())
		dpClient := newTestClient(t, srv.TestURL(), "test-api-key")

		id := createQueue(t, ctx, cpClient, "settings-queue")

		// Set very long visibility timeout (900s)
		if _, err := cpClient.ConfigQueue(ctx, &queue.ConfigQueueRequest{
			CommonServiceItem: queue.ConfigQueueRequestCommonServiceItem{
				Settings: queue.Settings{VisibilityTimeoutSeconds: 900, ExpireSeconds: 345600},
			},
		}, queue.ConfigQueueParams{ID: id}); err != nil {
			t.Fatalf("ConfigQueue failed: %v", err)
		}

		// Send and receive — the received message should have a ~900s visibility timeout
		content := message.MessageContent(base64.StdEncoding.EncodeToString([]byte("test")))
		if _, err := dpClient.SendMessage(ctx, &message.SendRequest{Content: content}, message.SendMessageParams{QueueName: "settings-queue"}); err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		recvRes, err := dpClient.ReceiveMessage(ctx, message.ReceiveMessageParams{QueueName: "settings-queue"})
		if err != nil {
			t.Fatalf("ReceiveMessage failed: %v", err)
		}
		recvOK := recvRes.(*message.ReceiveMessageOK)
		if len(recvOK.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(recvOK.Messages))
		}
		msg := recvOK.Messages[0]
		// visibility_timeout_at should be ~900s from now (not 30s)
		now := time.Now()
		timeoutAt := time.UnixMilli(msg.VisibilityTimeoutAt)
		elapsed := timeoutAt.Sub(now)
		if elapsed < 800*time.Second {
			t.Errorf("expected visibility timeout ~900s, got ~%.0fs", elapsed.Seconds())
		}
	})
}
