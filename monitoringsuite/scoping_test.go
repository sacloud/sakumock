package monitoringsuite_test

import (
	"testing"

	mssdk "github.com/sacloud/sacloud-sdk-go/api/monitoring-suite"
)

// TestAlertRuleProjectScoping ensures a rule created under one project cannot be
// read, updated, or deleted through a different project's path.
func TestAlertRuleProjectScoping(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()

	storageOp := mssdk.NewMetricsStorageOp(client)
	storage, err := storageOp.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "metrics"})
	if err != nil {
		t.Fatal(err)
	}
	sid := ridOf(t, storage.GetResourceID())

	projOp := mssdk.NewAlertProjectOp(client)
	projA, err := projOp.Create(ctx, mssdk.AlertProjectCreateParams{Name: "proj-a"})
	if err != nil {
		t.Fatal(err)
	}
	pidA := ridOf(t, projA.GetResourceID())
	projB, err := projOp.Create(ctx, mssdk.AlertProjectCreateParams{Name: "proj-b"})
	if err != nil {
		t.Fatal(err)
	}
	pidB := ridOf(t, projB.GetResourceID())

	ruleOp := mssdk.NewAlertRuleOp(client)
	rule, err := ruleOp.Create(ctx, pidA, mssdk.AlertRuleCreateParams{
		MetricsStorageID: sid,
		Query:            "up == 0",
		Name:             ref("rule"),
	})
	if err != nil {
		t.Fatal(err)
	}
	uid := rule.GetUID()

	if _, err := ruleOp.Read(ctx, pidB, uid); err == nil {
		t.Fatal("expected error reading rule through another project")
	}
	if _, err := ruleOp.Update(ctx, pidB, uid, mssdk.AlertRuleUpdateParams{Name: ref("hijacked")}); err == nil {
		t.Fatal("expected error updating rule through another project")
	}
	if err := ruleOp.Delete(ctx, pidB, uid); err == nil {
		t.Fatal("expected error deleting rule through another project")
	}

	// The rule is still intact under its own project.
	if _, err := ruleOp.Read(ctx, pidA, uid); err != nil {
		t.Fatalf("rule should still exist in its own project: %v", err)
	}
}

// TestMetricsStorageKeyScoping ensures an access key created under one storage
// cannot be read or deleted through a different storage's path.
func TestMetricsStorageKeyScoping(t *testing.T) {
	client, closeFn := newServer(t)
	defer closeFn()
	ctx := t.Context()

	op := mssdk.NewMetricsStorageOp(client)
	storageA, err := op.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "metrics-a"})
	if err != nil {
		t.Fatal(err)
	}
	sidA := ridOf(t, storageA.GetResourceID())
	storageB, err := op.Create(ctx, mssdk.MetricsStorageCreateParams{Name: "metrics-b"})
	if err != nil {
		t.Fatal(err)
	}
	sidB := ridOf(t, storageB.GetResourceID())

	key, err := op.CreateKey(ctx, sidA, ref("key"))
	if err != nil {
		t.Fatal(err)
	}
	uid := key.GetUID()

	if _, err := op.ReadKey(ctx, sidB, uid); err == nil {
		t.Fatal("expected error reading key through another storage")
	}
	if _, err := op.UpdateKey(ctx, sidB, uid, ref("hijacked")); err == nil {
		t.Fatal("expected error updating key through another storage")
	}
	if err := op.DeleteKey(ctx, sidB, uid); err == nil {
		t.Fatal("expected error deleting key through another storage")
	}

	// The key is still intact under its own storage.
	if _, err := op.ReadKey(ctx, sidA, uid); err != nil {
		t.Fatalf("key should still exist in its own storage: %v", err)
	}
}
