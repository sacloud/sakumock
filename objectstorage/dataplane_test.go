package objectstorage_test

import (
	"io"
	"net"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	sdk "github.com/sacloud/sacloud-sdk-go/api/object-storage"

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
// (path-style addressing, static root credentials, custom endpoint).
func newS3Client(addr string) *s3.Client {
	cfg := aws.Config{
		Region:      dataPlaneRegion,
		Credentials: credentials.NewStaticCredentialsProvider(dataPlaneAccessKey, dataPlaneSecretKey, ""),
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
	s3c := newS3Client(addr)
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
