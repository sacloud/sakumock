package objectstorage_test

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	sdk "github.com/sacloud/sacloud-sdk-go/api/object-storage"

	"github.com/sacloud/sakumock/objectstorage"
)

// freeLoopbackAddr returns a currently-free loopback address for versitygw to
// bind. There is a small window between closing the listener and versitygw
// binding it, which is acceptable for a test.
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// TestDataPlaneDisabledWhenVersitygwAbsent verifies that requesting the data
// plane without versitygw on PATH degrades gracefully: the control plane still
// builds and serves. Emptying PATH makes the lookup fail regardless of whether
// versitygw is installed, so this runs everywhere.
func TestDataPlaneDisabledWhenVersitygwAbsent(t *testing.T) {
	t.Setenv("PATH", "")

	srv := objectstorage.NewTestServer(objectstorage.Config{EnableDataPlane: true})
	defer srv.Close()

	fed, site := newClients(t, srv.TestURL())
	bucketOp := sdk.NewBucketOp(fed, site)
	if _, err := bucketOp.Create(t.Context(), &sdk.BucketCreateParams{Bucket: "no-data-plane", SiteId: testSiteID}); err != nil {
		t.Fatalf("control plane should work without a data plane: %v", err)
	}
}

// TestDataPlaneBucketSync verifies that a control-plane bucket create/delete is
// mirrored into the versitygw backend directory. It needs versitygw installed.
func TestDataPlaneBucketSync(t *testing.T) {
	if _, err := exec.LookPath("versitygw"); err != nil {
		t.Skip("versitygw not found in PATH; skipping data plane test")
	}

	dir := t.TempDir()
	srv := objectstorage.NewTestServer(objectstorage.Config{
		EnableDataPlane:    true,
		DataPlaneAddr:      freeLoopbackAddr(t),
		DataPlaneDir:       dir,
		DataPlaneAccessKey: "sakumock",
		DataPlaneSecretKey: "sakumocksecret",
		DataPlaneRegion:    "us-east-1",
	})
	defer srv.Close()

	fed, site := newClients(t, srv.TestURL())
	bucketOp := sdk.NewBucketOp(fed, site)
	const bucketName = "tf-test-bucket"

	if _, err := bucketOp.Create(t.Context(), &sdk.BucketCreateParams{Bucket: bucketName, SiteId: testSiteID}); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(dir, bucketName)); err != nil || !fi.IsDir() {
		t.Fatalf("expected bucket directory mirrored into data plane backend: err=%v", err)
	}

	if err := bucketOp.Delete(t.Context(), bucketName); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, bucketName)); !os.IsNotExist(err) {
		t.Fatalf("expected bucket directory removed from data plane backend, got err=%v", err)
	}
}
