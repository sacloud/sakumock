package objectstorage_test

import (
	"io"
	"net"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	sdk "github.com/sacloud/sacloud-sdk-go/api/object-storage"
	v2 "github.com/sacloud/sacloud-sdk-go/api/object-storage/apis/v2"

	"github.com/sacloud/sakumock/objectstorage"
)

const (
	dataPlaneAccessKey = "sakumock"
	dataPlaneSecretKey = "sakumocksecret"
	dataPlaneRegion    = "us-east-1"
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

// newS3Client builds an aws-sdk-go-v2 S3 client pointed at the data plane
// (path-style addressing, static credentials, custom endpoint).
func newS3Client(addr, access, secret string) *s3.Client {
	cfg := aws.Config{
		Region:      dataPlaneRegion,
		Credentials: credentials.NewStaticCredentialsProvider(access, secret, ""),
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://" + addr)
		o.UsePathStyle = true
	})
}

func bucketListed(t *testing.T, c *s3.Client, name string) bool {
	t.Helper()
	out, err := c.ListBuckets(t.Context(), &s3.ListBucketsInput{})
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	for _, b := range out.Buckets {
		if aws.ToString(b.Name) == name {
			return true
		}
	}
	return false
}

func putObject(t *testing.T, c *s3.Client, bucket, key, body string) {
	t.Helper()
	if _, err := c.PutObject(t.Context(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(body),
	}); err != nil {
		t.Fatalf("PutObject %s/%s: %v", bucket, key, err)
	}
}

func getObjectBody(t *testing.T, c *s3.Client, bucket, key string) string {
	t.Helper()
	out, err := c.GetObject(t.Context(), &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("GetObject %s/%s: %v", bucket, key, err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatalf("read %s/%s: %v", bucket, key, err)
	}
	return string(data)
}

func listObjectKeys(t *testing.T, c *s3.Client, bucket string) []string {
	t.Helper()
	out, err := c.ListObjectsV2(t.Context(), &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("ListObjectsV2 %s: %v", bucket, err)
	}
	keys := make([]string, 0, len(out.Contents))
	for _, o := range out.Contents {
		keys = append(keys, aws.ToString(o.Key))
	}
	sort.Strings(keys)
	return keys
}

// TestDataPlaneErrorsWhenVersitygwAbsent verifies that requesting the data plane
// without versitygw on PATH is a hard error rather than a silent no-op: the user
// opted in explicitly, so NewHandler must fail. Emptying PATH makes the lookup
// fail regardless of whether versitygw is installed, so this runs everywhere.
func TestDataPlaneErrorsWhenVersitygwAbsent(t *testing.T) {
	t.Setenv("PATH", "")

	if _, err := objectstorage.NewHandler(objectstorage.Config{EnableDataPlane: true}); err == nil {
		t.Fatal("expected NewHandler to error when --enable-data-plane is set but versitygw is absent")
	}
}

// TestDataPlaneEndToEnd drives the full path: a bucket created through the
// control plane is usable over the S3 data plane (list/put/get), and deleting it
// through the control plane removes it from the data plane. It needs versitygw
// installed (CI installs the release binary).
func TestDataPlaneEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("versitygw"); err != nil {
		t.Skip("versitygw not found in PATH; skipping data plane test")
	}

	addr := freeLoopbackAddr(t)
	srv := objectstorage.NewTestServer(objectstorage.Config{
		EnableDataPlane: true,
		DataPlaneAddr:   addr,
		DataPlaneDir:    t.TempDir(),
		DataPlaneRegion: dataPlaneRegion,
	})
	defer srv.Close()

	ctx := t.Context()
	fed, site := newClients(t, srv.TestURL())
	bucketOp := sdk.NewBucketOp(fed, site)
	const bucketA, bucketB = "tf-test-bucket-a", "tf-test-bucket-b"

	// Create two buckets via the control plane; both must appear as S3 buckets.
	for _, name := range []string{bucketA, bucketB} {
		if _, err := bucketOp.Create(ctx, &sdk.BucketCreateParams{Bucket: name, SiteId: testSiteID}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	s3c := newS3Client(addr, dataPlaneAccessKey, dataPlaneSecretKey)
	if !bucketListed(t, s3c, bucketA) || !bucketListed(t, s3c, bucketB) {
		t.Fatalf("control-plane buckets not both visible over the S3 data plane")
	}

	// The same key holds different content in each bucket, and one object exists
	// only in bucket A — verifying objects are stored per bucket, not shared.
	const sharedKey = "shared.txt"
	putObject(t, s3c, bucketA, sharedKey, "content-a")
	putObject(t, s3c, bucketB, sharedKey, "content-b")
	putObject(t, s3c, bucketA, "only-a.txt", "only-in-a")

	if got := getObjectBody(t, s3c, bucketA, sharedKey); got != "content-a" {
		t.Errorf("bucket A %s = %q, want %q", sharedKey, got, "content-a")
	}
	if got := getObjectBody(t, s3c, bucketB, sharedKey); got != "content-b" {
		t.Errorf("bucket B %s = %q, want %q", sharedKey, got, "content-b")
	}

	// Each bucket lists only its own objects.
	if got, want := listObjectKeys(t, s3c, bucketA), []string{"only-a.txt", sharedKey}; !slices.Equal(got, want) {
		t.Errorf("bucket A keys = %v, want %v", got, want)
	}
	if got, want := listObjectKeys(t, s3c, bucketB), []string{sharedKey}; !slices.Equal(got, want) {
		t.Errorf("bucket B keys = %v, want %v", got, want)
	}

	// Deleting bucket A via the control plane removes only A from the data plane;
	// bucket B and its object are untouched.
	if err := bucketOp.Delete(ctx, bucketA); err != nil {
		t.Fatal(err)
	}
	if bucketListed(t, s3c, bucketA) {
		t.Errorf("bucket A still visible over the S3 data plane after control-plane delete")
	}
	if !bucketListed(t, s3c, bucketB) {
		t.Errorf("bucket B should remain after deleting bucket A")
	}
	if got := getObjectBody(t, s3c, bucketB, sharedKey); got != "content-b" {
		t.Errorf("bucket B %s after deleting A = %q, want %q", sharedKey, got, "content-b")
	}
}

// TestDataPlaneControlPlaneKeys drives the "issue key via the control plane,
// use it against S3" flow (issue #152): account keys and permission keys
// issued through the control plane authenticate S3 requests on the data plane
// alongside the fixed root credential, and deleting a key (or the permission
// owning it) revokes access immediately.
func TestDataPlaneControlPlaneKeys(t *testing.T) {
	if _, err := exec.LookPath("versitygw"); err != nil {
		t.Skip("versitygw not found in PATH; skipping data plane test")
	}

	addr := freeLoopbackAddr(t)
	srv := objectstorage.NewTestServer(objectstorage.Config{
		EnableDataPlane: true,
		DataPlaneAddr:   addr,
		DataPlaneDir:    t.TempDir(),
		DataPlaneRegion: dataPlaneRegion,
	})
	defer srv.Close()

	ctx := t.Context()
	fed, site := newClients(t, srv.TestURL())
	const bucket, object = "key-test-bucket", "hello.txt"
	if _, err := sdk.NewBucketOp(fed, site).Create(ctx, &sdk.BucketCreateParams{Bucket: bucket, SiteId: testSiteID}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	accountOp := sdk.NewAccountOp(site)
	if _, err := accountOp.Create(ctx); err != nil {
		t.Fatalf("create account: %v", err)
	}
	key, err := accountOp.CreateAccessKey(ctx)
	if err != nil {
		t.Fatalf("create account access key: %v", err)
	}

	// The issued key authenticates S3 requests...
	issued := newS3Client(addr, string(key.ID.Value), string(key.Secret.Value))
	putObject(t, issued, bucket, object, "issued-key")
	if got := getObjectBody(t, issued, bucket, object); got != "issued-key" {
		t.Errorf("object via issued key = %q, want %q", got, "issued-key")
	}
	if !bucketListed(t, issued, bucket) {
		t.Error("bucket not listed via issued key")
	}

	// ...alongside the fixed root credential, and only for keys actually
	// issued.
	root := newS3Client(addr, dataPlaneAccessKey, dataPlaneSecretKey)
	if got := getObjectBody(t, root, bucket, object); got != "issued-key" {
		t.Errorf("object via root credential = %q, want %q", got, "issued-key")
	}
	unissued := newS3Client(addr, "UNISSUEDKEY00000", "unissued-secret-0000000000000000")
	if _, err := unissued.ListBuckets(ctx, &s3.ListBucketsInput{}); err == nil {
		t.Error("unissued key must not authenticate")
	}

	// Deleting the key revokes it immediately (versitygw runs with its IAM
	// cache disabled).
	if err := accountOp.DeleteAccessKey(ctx, string(key.ID.Value)); err != nil {
		t.Fatalf("delete account access key: %v", err)
	}
	if _, err := issued.ListBuckets(ctx, &s3.ListBucketsInput{}); err == nil {
		t.Error("deleted account key must not authenticate")
	}

	// Permission keys follow the same flow, revoked when their permission is
	// deleted.
	permOp := sdk.NewPermissionOp(site)
	perm, err := permOp.Create(ctx, "dataplane-key-test", v2.BucketControls{{
		BucketName: v2.NewOptBucketName(bucket),
		CanRead:    v2.NewOptCanRead(true),
		CanWrite:   v2.NewOptCanWrite(true),
	}})
	if err != nil {
		t.Fatalf("create permission: %v", err)
	}
	permID := strconv.FormatInt(int64(perm.ID.Value), 10)
	pkey, err := permOp.CreateAccessKey(ctx, permID)
	if err != nil {
		t.Fatalf("create permission access key: %v", err)
	}
	permClient := newS3Client(addr, string(pkey.ID.Value), string(pkey.Secret.Value))
	if got := getObjectBody(t, permClient, bucket, object); got != "issued-key" {
		t.Errorf("object via permission key = %q, want %q", got, "issued-key")
	}
	if err := permOp.Delete(ctx, permID); err != nil {
		t.Fatalf("delete permission: %v", err)
	}
	if _, err := permClient.ListBuckets(ctx, &s3.ListBucketsInput{}); err == nil {
		t.Error("key of a deleted permission must not authenticate")
	}
}
