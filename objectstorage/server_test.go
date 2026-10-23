package objectstorage_test

import (
	"context"
	"net/http"
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

// closeAndCheck closes srv, failing the test if any handler response
// diverged from the OpenAPI spec.
func closeAndCheck(t *testing.T, srv *objectstorage.Server) {
	t.Helper()
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations recorded: %+v", v)
	}
	srv.Close()
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

type dummySecuritySource struct{}

func (dummySecuritySource) BasicAuth(context.Context, v2.OperationName) (v2.BasicAuth, error) {
	return v2.BasicAuth{}, nil
}

// newRawClient returns the SDK's generated client rooted at basePath (e.g.
// "/fed/v1"), for operations the SDK's high-level ops do not wrap.
func newRawClient(t *testing.T, serverURL, basePath string) *v2.Client {
	t.Helper()
	c, err := v2.NewClient(serverURL+basePath, dummySecuritySource{}, v2.WithClient(newTestSAClient(t, serverURL)))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSiteAPI(t *testing.T) {
	srv := objectstorage.NewTestServer(objectstorage.Config{})
	defer closeAndCheck(t, srv)
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
	defer closeAndCheck(t, srv)
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

	// The real API allows only one root access key per account (num_root_keys
	// quota); a second create is a conflict.
	if _, err := accountOp.CreateAccessKey(ctx); !saclient.IsConflictError(err) {
		t.Fatalf("expected conflict creating a second account key, got %v", err)
	}

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

	if err := accountOp.DeleteAccessKey(ctx, string(created.ID.Value)); err != nil {
		t.Fatal(err)
	}
	if err := accountOp.Delete(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionCRUD(t *testing.T) {
	srv := objectstorage.NewTestServer(objectstorage.Config{})
	defer closeAndCheck(t, srv)
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

	// The real API allows only one access key per permission
	// (num_keys_per_permission quota); a second create is a conflict. Checked
	// via a raw request: the SDK's Permissions.CreateAccessKey has no case
	// for a 409 response, so it can't be observed through saclient.IsConflictError.
	resp, err := http.Post(srv.TestURL()+"/isk01/v2/permissions/"+id+"/keys", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 creating a second permission key, got %d", resp.StatusCode)
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
	defer closeAndCheck(t, srv)
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

	// Replication to a bucket in another site.
	if _, err := bucketOp.Create(ctx, &sdk.BucketCreateParams{Bucket: "enc-bucket-replica", SiteId: "tky01"}); err != nil {
		t.Fatal(err)
	}
	fedRaw := newRawClient(t, srv.TestURL(), "/fed/v1")
	res, err := fedRaw.GetBucketReplicableTargets(ctx, v2.GetBucketReplicableTargetsParams{Name: "enc-bucket"})
	if err != nil {
		t.Fatal(err)
	}
	targets, ok := res.(*v2.HandlerGetReplicableTargetsRes)
	if !ok {
		t.Fatalf("unexpected replicable-targets response: %T", res)
	}
	if len(targets.Data) != 1 || targets.Data[0].Name.Value != "enc-bucket-replica" {
		t.Fatalf("unexpected replicable targets: %+v", targets.Data)
	}

	repl, err := extra.EnableReplication(ctx, "enc-bucket-replica")
	if err != nil {
		t.Fatal(err)
	}
	if repl.DestBucket.ClusterID.Value != "tky01" {
		t.Errorf("unexpected replication destination cluster: %s", repl.DestBucket.ClusterID.Value)
	}
	if _, err := extra.ReadReplication(ctx); err != nil {
		t.Fatal(err)
	}
	if err := extra.DisableReplication(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := extra.ReadReplication(ctx); !saclient.IsNotFoundError(err) {
		t.Fatalf("expected not-found reading disabled replication, got %v", err)
	}
}

func TestBucketPlan(t *testing.T) {
	srv := objectstorage.NewTestServer(objectstorage.Config{})
	defer closeAndCheck(t, srv)
	fed, site := newClients(t, srv.TestURL())
	ctx := t.Context()

	if _, err := sdk.NewBucketOp(fed, site).Create(ctx, &sdk.BucketCreateParams{Bucket: "plan-bucket", SiteId: testSiteID}); err != nil {
		t.Fatal(err)
	}
	siteRaw := newRawClient(t, srv.TestURL(), "/"+testSiteID+"/v2")

	res, err := siteRaw.GetBucketPlan(ctx, v2.GetBucketPlanParams{Name: "plan-bucket"})
	if err != nil {
		t.Fatal(err)
	}
	current, ok := res.(*v2.GetBucketPlanOK)
	if !ok {
		t.Fatalf("unexpected get-plan response: %T", res)
	}
	contract := current.Data.Contract.Value
	if contract.Status.Value != v2.ContractSummaryStatusActive || !contract.DeletedAt.Null {
		t.Errorf("unexpected current contract: %+v", contract)
	}
	if plan := current.Data.Plan.Value; plan.ClusterID.Value != testSiteID || !plan.CapacityGib.Null {
		t.Errorf("unexpected current plan: %+v", plan)
	}

	pres, err := siteRaw.PutBucketPlan(ctx, &v2.PlanChangeReqBody{
		PreviousContract: v2.PlanChangeReqBodyPreviousContract{ResourceID: v2.NewOptResourceID(v2.ResourceID(contract.ResourceID.Value))},
		NewPlan:          v2.PlanChangeReqBodyNewPlan{Type: v2.ModelPlanTypeStandard, ServiceClassPath: "objectstorage/isk01/bucket"},
	}, v2.PutBucketPlanParams{Name: "plan-bucket"})
	if err != nil {
		t.Fatal(err)
	}
	changed, ok := pres.(*v2.PutBucketPlanOK)
	if !ok {
		t.Fatalf("unexpected put-plan response: %T", pres)
	}
	prev, next := changed.Data.PreviousContract.Value, changed.Data.NewContract.Value
	if prev.Status.Value != v2.ContractSummaryStatusTerminated || prev.DeletedAt.Null || !prev.DeletedAt.Set {
		t.Errorf("unexpected previous contract: %+v", prev)
	}
	if next.ResourceID.Value == contract.ResourceID.Value || !next.DeletedAt.Null {
		t.Errorf("unexpected new contract: %+v", next)
	}
}

func TestSiteStatusQuotaPenaltyMetering(t *testing.T) {
	srv := objectstorage.NewTestServer(objectstorage.Config{})
	defer closeAndCheck(t, srv)
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
	defer closeAndCheck(t, srv)
	if len(srv.Routes()) == 0 {
		t.Fatal("expected routes")
	}
}
