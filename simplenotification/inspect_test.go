package simplenotification_test

import (
	"testing"

	v1 "github.com/sacloud/sacloud-sdk-go/api/simple-notification/apis/v1"

	"github.com/sacloud/sakumock/simplenotification"
)

func TestInspectionClientMessages(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()
	ctx := t.Context()
	groupOp := newTestGroupOp(t, srv.TestURL())
	ic := simplenotification.NewInspectionClient(srv.TestURL())

	for _, m := range []string{"first", "second"} {
		if _, err := groupOp.SendMessage(ctx, "123456789012", v1.SendNotificationMessageRequest{Message: m}); err != nil {
			t.Fatal(err)
		}
	}

	msgs, err := ic.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Message != "first" || msgs[1].Message != "second" {
		t.Fatalf("unexpected message order: %+v", msgs)
	}
	if msgs[0].GroupID != "123456789012" {
		t.Fatalf("unexpected group id: %s", msgs[0].GroupID)
	}
	if msgs[0].CreatedAt.IsZero() {
		t.Fatal("expected non-zero created_at")
	}
}

func TestInspectionClientClearMessages(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()
	ctx := t.Context()
	groupOp := newTestGroupOp(t, srv.TestURL())
	ic := simplenotification.NewInspectionClient(srv.TestURL())

	if _, err := groupOp.SendMessage(ctx, "123456789012", v1.SendNotificationMessageRequest{Message: "to-clear"}); err != nil {
		t.Fatal(err)
	}

	msgs, err := ic.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	if err := ic.ClearMessages(ctx); err != nil {
		t.Fatal(err)
	}

	msgs, err = ic.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after clear, got %d", len(msgs))
	}
}
