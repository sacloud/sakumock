package objectstorage_test

import (
	"io"
	"net"
	"os/exec"
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
		EnableDataPlane:    true,
		DataPlaneAddr:      addr,
		DataPlaneDir:       t.TempDir(),
		DataPlaneAccessKey: dataPlaneAccessKey,
		DataPlaneSecretKey: dataPlaneSecretKey,
		DataPlaneRegion:    dataPlaneRegion,
	})
	defer srv.Close()

	ctx := t.Context()
	fed, site := newClients(t, srv.TestURL())
	bucketOp := sdk.NewBucketOp(fed, site)
	const bucketName = "tf-test-bucket"

	// Create via the control plane; it must appear as an S3 bucket.
	if _, err := bucketOp.Create(ctx, &sdk.BucketCreateParams{Bucket: bucketName, SiteId: testSiteID}); err != nil {
		t.Fatal(err)
	}
	s3c := newS3Client(addr)
	if !bucketListed(t, s3c, bucketName) {
		t.Fatalf("control-plane bucket %q not visible over the S3 data plane", bucketName)
	}

	// Put and get an object through the S3 data plane.
	const key, body = "hello.txt", "hello sakumock"
	if _, err := s3c.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   strings.NewReader(body),
	}); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	got, err := s3c.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucketName), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer got.Body.Close()
	data, err := io.ReadAll(got.Body)
	if err != nil {
		t.Fatalf("read object body: %v", err)
	}
	if string(data) != body {
		t.Errorf("object body = %q, want %q", data, body)
	}

	// Delete via the control plane; the bucket must disappear from the data plane.
	if err := bucketOp.Delete(ctx, bucketName); err != nil {
		t.Fatal(err)
	}
	if bucketListed(t, s3c, bucketName) {
		t.Errorf("bucket %q still visible over the S3 data plane after control-plane delete", bucketName)
	}
}
