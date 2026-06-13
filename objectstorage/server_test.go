package objectstorage_test

import (
	"strconv"
	"testing"
	"time"

	sdk "github.com/sacloud/sacloud-sdk-go/api/object-storage"
	v2 "github.com/sacloud/sacloud-sdk-go/api/object-storage/apis/v2"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/objectstorage"
)

const testSiteID = "isk01"

func newTestSAClient(t *testing.T, serverURL string) *saclient.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_OBJECT_STORAGE=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	return &sa
}

func newClients(t *testing.T, serverURL string) (*sdk.FedClient, *sdk.SiteClient) {
	t.Helper()
	sa := newTestSAClient(t, serverURL)
	fed, err := sdk.NewFedClient(sa)
	if err != nil {
		t.Fatal(err)
	}
	site, err := sdk.NewSiteClient(sa, testSiteID)
	if err != nil {
		t.Fatal(err)
	}
	return fed, site
}

func TestSiteAPI(t *testing.T) {
	srv := objectstorage.NewTestServer(objectstorage.Config{})
	defer srv.Close()
	fed, _ := newClients(t, srv.TestURL())
	op := sdk.NewSiteOp(fed)

	sites, err := op.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) == 0 {
		t.Fatal("no sites returned")
	}
	found := false
	for _, s := range sites {
		if s.ID.Value == testSiteID {
			found = true
		}
	}
	if !found {
		t.Errorf("site %q not in list", testSiteID)
	}

	site, err := op.Read(t.Context(), testSiteID)
	if err != nil {
		t.Fatal(err)
	}
	if site.ID.Value != testSiteID {
		t.Errorf("unexpected site id: %s", site.ID.Value)
	}

	if _, err := op.Read(t.Context(), "no-such-site"); err == nil {
		t.Error("expected error reading unknown site")
	}
}

func TestAccountAndBucketLifecycle(t *testing.T) {
	srv := objectstorage.NewTestServer(objectstorage.Config{})
	defer srv.Close()
	fed, site := newClients(t, srv.TestURL())
	ctx := t.Context()

	accountOp := sdk.NewAccountOp(site)

	// Account does not exist yet.
	if _, err := accountOp.Read(ctx); !saclient.IsNotFoundError(err) {
		t.Fatalf("expected not-found reading account, got %v", err)
	}
	if _, err := accountOp.Create(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := accountOp.Read(ctx); err != nil {
		t.Fatalf("account should exist after create: %v", err)
	}

	// Access keys: secret only on creation, empty afterwards.
	created, err := accountOp.CreateAccessKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if created.Secret.Value == "" {
		t.Error("expected secret on key creation")
	}
	got, err := accountOp.ReadAccessKey(ctx, string(created.ID.Value))
	if err != nil {
		t.Fatal(err)
	}
	if got.Secret.Value != "" {
		t.Errorf("expected empty secret on read, got %q", got.Secret.Value)
	}
	keys, err := accountOp.ListAccessKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	// Bucket create / list / delete.
	bucketOp := sdk.NewBucketOp(fed, site)
	const bucketName = "tf-test-bucket"
	bucket, err := bucketOp.Create(ctx, &sdk.BucketCreateParams{Bucket: bucketName, SiteId: testSiteID})
	if err != nil {
		t.Fatal(err)
	}
	if bucket.Name.Value != bucketName {
		t.Errorf("unexpected bucket name: %s", bucket.Name.Value)
	}
	if bucket.ClusterID.Value != testSiteID {
		t.Errorf("unexpected cluster id: %s", bucket.ClusterID.Value)
	}

	// Duplicate create is a conflict.
	if _, err := bucketOp.Create(ctx, &sdk.BucketCreateParams{Bucket: bucketName, SiteId: testSiteID}); err == nil {
		t.Error("expected conflict creating duplicate bucket")
	}

	buckets, err := bucketOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 || string(buckets[0].Name) != bucketName {
		t.Fatalf("unexpected bucket list: %+v", buckets)
	}
	if buckets[0].ResourceID.Value == "" {
		t.Error("expected a resource id in bucket list")
	}

	if err := bucketOp.Delete(ctx, bucketName); err != nil {
		t.Fatal(err)
	}
	buckets, err = bucketOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 0 {
		t.Fatalf("expected no buckets after delete, got %d", len(buckets))
	}

	// Deleting access key and account.
	if err := accountOp.DeleteAccessKey(ctx, string(created.ID.Value)); err != nil {
		t.Fatal(err)
	}
	if err := accountOp.Delete(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionCRUD(t *testing.T) {
	srv := objectstorage.NewTestServer(objectstorage.Config{})
	defer srv.Close()
	_, site := newClients(t, srv.TestURL())
	ctx := t.Context()

	op := sdk.NewPermissionOp(site)
	controls := v2.BucketControls{
		{
			BucketName: v2.NewOptBucketName("tf-test-bucket"),
			CanRead:    v2.NewOptCanRead(true),
			CanWrite:   v2.NewOptCanWrite(false),
		},
	}
	created, err := op.Create(ctx, "my-permission", controls)
	if err != nil {
		t.Fatal(err)
	}
	if created.DisplayName.Value != "my-permission" {
		t.Errorf("unexpected display name: %s", created.DisplayName.Value)
	}
	id := strconv.FormatInt(int64(created.ID.Value), 10)

	got, err := op.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.BucketControls) != 1 || string(got.BucketControls[0].BucketName.Value) != "tf-test-bucket" {
		t.Errorf("unexpected bucket controls: %+v", got.BucketControls)
	}

	updated, err := op.Update(ctx, id, "renamed", controls)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName.Value != "renamed" {
		t.Errorf("unexpected display name after update: %s", updated.DisplayName.Value)
	}

	// Permission keys.
	key, err := op.CreateAccessKey(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if key.Secret.Value == "" {
		t.Error("expected secret on permission key creation")
	}
	readKey, err := op.ReadAccessKey(ctx, id, string(key.ID.Value))
	if err != nil {
		t.Fatal(err)
	}
	if readKey.Secret.Value != "" {
		t.Errorf("expected empty secret on read, got %q", readKey.Secret.Value)
	}

	list, err := op.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(list))
	}

	if err := op.DeleteAccessKey(ctx, id, string(key.ID.Value)); err != nil {
		t.Fatal(err)
	}
	if err := op.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := op.Read(ctx, id); !saclient.IsNotFoundError(err) {
		t.Errorf("expected not-found after delete, got %v", err)
	}
}

func TestBucketEncryptionAndReplication(t *testing.T) {
	srv := objectstorage.NewTestServer(objectstorage.Config{})
	defer srv.Close()
	fed, site := newClients(t, srv.TestURL())
	ctx := t.Context()

	bucketOp := sdk.NewBucketOp(fed, site)
	if _, err := bucketOp.Create(ctx, &sdk.BucketCreateParams{Bucket: "enc-bucket", SiteId: testSiteID}); err != nil {
		t.Fatal(err)
	}

	extra := sdk.NewBucketExtraOp(site, fed, "enc-bucket")

	// Encryption is absent until enabled.
	if _, err := extra.ReadEncryption(ctx); !saclient.IsNotFoundError(err) {
		t.Fatalf("expected not-found reading encryption, got %v", err)
	}
	if err := extra.EnableEncryption(ctx, "990000099999"); err != nil {
		t.Fatal(err)
	}
	enc, err := extra.ReadEncryption(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(enc.KmsKeyID.Value) != "990000099999" {
		t.Errorf("unexpected kms key id: %s", enc.KmsKeyID.Value)
	}
	if err := extra.DisableEncryption(ctx); err != nil {
		t.Fatal(err)
	}

	// Usage / quota are readable for an existing bucket.
	if _, err := extra.ReadUsage(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := extra.ReadQuota(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestSiteStatusQuotaPenaltyMetering exercises the read-only site/bucket info
// endpoints through the SDK, confirming their typed responses decode.
func TestSiteStatusQuotaPenaltyMetering(t *testing.T) {
	srv := objectstorage.NewTestServer(objectstorage.Config{})
	defer srv.Close()
	fed, site := newClients(t, srv.TestURL())
	ctx := t.Context()

	statusOp := sdk.NewSiteStatusOp(site)
	if _, err := statusOp.Read(ctx); err != nil {
		t.Fatalf("status: %v", err)
	}
	if _, err := statusOp.ReadQuota(ctx); err != nil {
		t.Fatalf("quota: %v", err)
	}

	planOp := sdk.NewSiteWithPlansOp(fed, site)
	if plans, err := planOp.ListPlans(ctx); err != nil {
		t.Fatalf("plans: %v", err)
	} else if len(plans) == 0 {
		t.Error("expected at least one plan")
	}

	bucketOp := sdk.NewBucketOp(fed, site)
	if _, err := bucketOp.Create(ctx, &sdk.BucketCreateParams{Bucket: "metrics-bucket", SiteId: testSiteID}); err != nil {
		t.Fatal(err)
	}
	if _, err := statusOp.ReadBucketMetering(ctx, "metrics-bucket", time.Now().Add(-24*time.Hour), time.Now()); err != nil {
		t.Fatalf("metering: %v", err)
	}

	extra := sdk.NewBucketExtraOp(site, fed, "metrics-bucket")
	if _, err := extra.ReadPenalty(ctx); err != nil {
		t.Fatalf("penalty: %v", err)
	}
}

func TestRoutes(t *testing.T) {
	srv := objectstorage.NewTestServer(objectstorage.Config{})
	defer srv.Close()
	if len(srv.Routes()) == 0 {
		t.Fatal("expected routes")
	}
}
