package objectstorage_test

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	sdk "github.com/sacloud/sacloud-sdk-go/api/object-storage"
	v2 "github.com/sacloud/sacloud-sdk-go/api/object-storage/apis/v2"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/core/coretest"
	"github.com/sacloud/sakumock/objectstorage"
)

// readOnly is the bucket controls of a permission that may only read bucket.
func readOnly(bucket string) v2.BucketControls {
	return v2.BucketControls{{
		BucketName: v2.NewOptBucketName(v2.BucketName(bucket)),
		CanRead:    v2.NewOptCanRead(true),
		CanWrite:   v2.NewOptCanWrite(false),
	}}
}

func TestStateSurvivesRestart(t *testing.T) {
	const kmsKeyID = "990000099999"
	var accountID, accountKeyID, permID, permKeyID string

	coretest.StateRestart(t, objectstorage.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			fed, site := newClients(t, url)
			bucketOp := sdk.NewBucketOp(fed, site)
			for name, siteID := range map[string]string{"persisted": testSiteID, "replica": "tky01", "deleted": testSiteID} {
				if _, err := bucketOp.Create(ctx, &sdk.BucketCreateParams{Bucket: name, SiteId: siteID}); err != nil {
					t.Fatalf("create bucket %s: %v", name, err)
				}
			}
			if err := bucketOp.Delete(ctx, "deleted"); err != nil {
				t.Fatal(err)
			}
			extra := sdk.NewBucketExtraOp(site, fed, "persisted")
			if err := extra.EnableEncryption(ctx, kmsKeyID); err != nil {
				t.Fatal(err)
			}
			if _, err := extra.EnableReplication(ctx, "replica"); err != nil {
				t.Fatal(err)
			}

			accountOp := sdk.NewAccountOp(site)
			account, err := accountOp.Create(ctx)
			if err != nil {
				t.Fatal(err)
			}
			accountID = string(account.ResourceID.Value)
			accountKey, err := accountOp.CreateAccessKey(ctx)
			if err != nil {
				t.Fatal(err)
			}
			accountKeyID = string(accountKey.ID.Value)

			permOp := sdk.NewPermissionOp(site)
			perm, err := permOp.Create(ctx, "persisted-permission", readOnly("persisted"))
			if err != nil {
				t.Fatal(err)
			}
			permID = strconv.FormatInt(int64(perm.ID.Value), 10)
			permKey, err := permOp.CreateAccessKey(ctx, permID)
			if err != nil {
				t.Fatal(err)
			}
			permKeyID = string(permKey.ID.Value)
			// A permission whose key was deleted is restored without keys.
			keyless, err := permOp.Create(ctx, "keyless", readOnly("replica"))
			if err != nil {
				t.Fatal(err)
			}
			keylessID := strconv.FormatInt(int64(keyless.ID.Value), 10)
			key, err := permOp.CreateAccessKey(ctx, keylessID)
			if err != nil {
				t.Fatal(err)
			}
			if err := permOp.DeleteAccessKey(ctx, keylessID, string(key.ID.Value)); err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			fed, site := newClients(t, url)
			buckets, err := sdk.NewBucketOp(fed, site).List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(buckets) != 1 || string(buckets[0].Name) != "persisted" {
				t.Errorf("restored buckets of %s = %+v, want only persisted", testSiteID, buckets)
			}
			extra := sdk.NewBucketExtraOp(site, fed, "persisted")
			enc, err := extra.ReadEncryption(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if string(enc.KmsKeyID.Value) != kmsKeyID {
				t.Errorf("restored encryption key = %s, want %s", enc.KmsKeyID.Value, kmsKeyID)
			}
			repl, err := extra.ReadReplication(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if string(repl.DestBucket.Name.Value) != "replica" || repl.DestBucket.ClusterID.Value != "tky01" {
				t.Errorf("restored replication = %+v, want replica in tky01", repl.DestBucket)
			}

			accountOp := sdk.NewAccountOp(site)
			account, err := accountOp.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if string(account.ResourceID.Value) != accountID {
				t.Errorf("restored account = %s, want %s", account.ResourceID.Value, accountID)
			}
			if _, err := accountOp.ReadAccessKey(ctx, accountKeyID); err != nil {
				t.Errorf("restored account key: %v", err)
			}

			permOp := sdk.NewPermissionOp(site)
			perms, err := permOp.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(perms) != 2 {
				t.Fatalf("restored permissions = %+v, want 2", perms)
			}
			perm, err := permOp.Read(ctx, permID)
			if err != nil {
				t.Fatal(err)
			}
			if perm.DisplayName.Value != "persisted-permission" || len(perm.BucketControls) != 1 {
				t.Fatalf("restored permission = %+v", perm)
			}
			if c := perm.BucketControls[0]; string(c.BucketName.Value) != "persisted" || !bool(c.CanRead.Value) || bool(c.CanWrite.Value) {
				t.Errorf("restored bucket control = %+v", c)
			}
			if _, err := permOp.ReadAccessKey(ctx, permID, permKeyID); err != nil {
				t.Errorf("restored permission key: %v", err)
			}
		})
}

// Resources created after a restart never reuse a restored ID: buckets,
// accounts and permissions all draw from the one generator.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	restored := map[string]bool{}
	create := func(t *testing.T, url, bucket, siteID string) []string {
		t.Helper()
		ctx := t.Context()
		sa := newTestSAClient(t, url)
		fed, err := sdk.NewFedClient(sa)
		if err != nil {
			t.Fatal(err)
		}
		site, err := sdk.NewSiteClient(sa, siteID)
		if err != nil {
			t.Fatal(err)
		}
		bucketOp := sdk.NewBucketOp(fed, site)
		if _, err := bucketOp.Create(ctx, &sdk.BucketCreateParams{Bucket: bucket, SiteId: siteID}); err != nil {
			t.Fatal(err)
		}
		// The create response carries no resource ID; the site listing does.
		buckets, err := bucketOp.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(buckets) != 1 {
			t.Fatalf("buckets of %s = %+v, want 1", siteID, buckets)
		}
		account, err := sdk.NewAccountOp(site).Create(ctx)
		if err != nil {
			t.Fatal(err)
		}
		perm, err := sdk.NewPermissionOp(site).Create(ctx, "p", readOnly(bucket))
		if err != nil {
			t.Fatal(err)
		}
		return []string{
			string(buckets[0].ResourceID.Value),
			string(account.ResourceID.Value),
			strconv.FormatInt(int64(perm.ID.Value), 10),
		}
	}
	coretest.StateRestart(t, objectstorage.Config{},
		func(t *testing.T, url string) {
			for _, id := range create(t, url, "first", testSiteID) {
				restored[id] = true
			}
		},
		func(t *testing.T, url string) {
			for _, id := range create(t, url, "second", "tky01") {
				if restored[id] {
					t.Errorf("new resource reused the restored ID %s", id)
				}
			}
		}, true)
}

// TestStateDataPlaneSurvivesRestart checks that both planes agree after a
// restart on one state directory: the restored bucket still holds its object
// (the backend directory defaults to a subdirectory of the state directory),
// the restored account and permission keys authenticate again, and a key
// deleted before the restart does not.
func TestStateDataPlaneSurvivesRestart(t *testing.T) {
	if _, err := exec.LookPath("versitygw"); err != nil {
		t.Skip("versitygw not found in PATH; skipping data plane test")
	}

	dir := t.TempDir()
	const bucket, object = "state-bucket", "hello.txt"
	// start runs the service on dir as the unified binary does and returns the
	// control-plane URL, the data-plane address, and the shutdown (also run at
	// the end of the test if not called).
	start := func() (string, string, func()) {
		t.Helper()
		addr := freeLoopbackAddr(t)
		srv, err := objectstorage.Config{
			EnableDataPlane: true,
			DataPlaneAddr:   addr,
			DataPlaneRegion: dataPlaneRegion,
		}.NewServer(core.ServerOptions{StateDir: dir})
		if err != nil {
			t.Fatalf("start objectstorage on %s: %v", dir, err)
		}
		ts := httptest.NewServer(srv)
		stop := sync.OnceFunc(func() {
			ts.Close()
			closeAndCheck(t, srv.(*objectstorage.Server))
		})
		t.Cleanup(stop)
		return ts.URL, addr, stop
	}

	ctx := t.Context()
	url, addr, stop := start()
	fed, site := newClients(t, url)
	if _, err := sdk.NewBucketOp(fed, site).Create(ctx, &sdk.BucketCreateParams{Bucket: bucket, SiteId: testSiteID}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	accountOp := sdk.NewAccountOp(site)
	if _, err := accountOp.Create(ctx); err != nil {
		t.Fatalf("create account: %v", err)
	}
	accountKey, err := accountOp.CreateAccessKey(ctx)
	if err != nil {
		t.Fatalf("create account access key: %v", err)
	}
	permOp := sdk.NewPermissionOp(site)
	newPermKey := func(name string) (string, *v2.PermissionKeyData) {
		t.Helper()
		perm, err := permOp.Create(ctx, name, readOnly(bucket))
		if err != nil {
			t.Fatalf("create permission %s: %v", name, err)
		}
		id := strconv.FormatInt(int64(perm.ID.Value), 10)
		key, err := permOp.CreateAccessKey(ctx, id)
		if err != nil {
			t.Fatalf("create permission access key: %v", err)
		}
		return id, key
	}
	_, permKey := newPermKey("kept")
	revokedPermID, revokedKey := newPermKey("revoked")
	if err := permOp.DeleteAccessKey(ctx, revokedPermID, string(revokedKey.ID.Value)); err != nil {
		t.Fatalf("delete permission access key: %v", err)
	}
	putObject(t, newS3Client(addr, string(accountKey.ID.Value), string(accountKey.Secret.Value)), bucket, object, "before-restart")
	stop()

	if _, err := os.Stat(filepath.Join(dir, "objectstorage-data", bucket, object)); err != nil {
		t.Fatalf("object not kept under the state directory: %v", err)
	}

	url, addr, _ = start()
	fed, site = newClients(t, url)
	if _, err := sdk.NewAccountOp(site).ReadAccessKey(ctx, string(accountKey.ID.Value)); err != nil {
		t.Errorf("restored account key on the control plane: %v", err)
	}
	for name, key := range map[string][2]string{
		"account key":    {string(accountKey.ID.Value), string(accountKey.Secret.Value)},
		"permission key": {string(permKey.ID.Value), string(permKey.Secret.Value)},
	} {
		c := newS3Client(addr, key[0], key[1])
		if !bucketListed(t, c, bucket) {
			t.Errorf("restored bucket not listed over S3 via the restored %s", name)
		}
		if got := getObjectBody(t, c, bucket, object); got != "before-restart" {
			t.Errorf("object via the restored %s = %q, want %q", name, got, "before-restart")
		}
	}
	revoked := newS3Client(addr, string(revokedKey.ID.Value), string(revokedKey.Secret.Value))
	if _, err := revoked.ListBuckets(ctx, &s3.ListBucketsInput{}); err == nil {
		t.Error("a key deleted before the restart must not authenticate after it")
	}

	// Deleting the restored bucket through the control plane still reaches the
	// data plane.
	if err := sdk.NewBucketOp(fed, site).Delete(ctx, bucket); err != nil && !saclient.IsNotFoundError(err) {
		t.Fatalf("delete restored bucket: %v", err)
	}
	if bucketListed(t, newS3Client(addr, dataPlaneAccessKey, dataPlaneSecretKey), bucket) {
		t.Error("restored bucket still visible over S3 after control-plane delete")
	}
}
