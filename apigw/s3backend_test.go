package apigw_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	apigwsdk "github.com/sacloud/sacloud-sdk-go/api/apigw"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apigw/apis/v1"
)

// newFakeS3 serves a fixed object set over the S3 REST API (path-style GET
// and HEAD), rejecting unsigned requests so the SigV4 path is exercised.
func newFakeS3(t *testing.T, objects map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			// S3 answers CORS preflights (bucket CORS) without signatures.
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			w.Header().Set("Access-Control-Allow-Methods", "GET")
			w.WriteHeader(http.StatusOK)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/") // "bucket/key..."
		body, ok := objects[key]
		if !ok {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			if r.Method != http.MethodHead {
				io.WriteString(w, `<?xml version="1.0"?><Error><Code>NoSuchKey</Code><Message>no such key</Message></Error>`)
			}
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// createObjectStorageService creates a service backed by the fake S3.
func createObjectStorageService(t *testing.T, client *v1.Client, name, endpoint, bucket, folder string) *v1.ServiceDetailResponse {
	t.Helper()
	sub := createSubscription(t, client, name+"_sub")
	svcOp := apigwsdk.NewServiceOp(client)
	req := &v1.ServiceDetailRequest{
		Name:         v1.Name(name),
		Protocol:     v1.ServiceDetailRequestProtocolHTTPS,
		Host:         "storage.example.com", // unused when objectStorageConfig is set
		Subscription: v1.ServiceSubscriptionRequest{ID: sub.ID.Value},
		ObjectStorageConfig: v1.NewOptObjectStorageConfig(v1.ObjectStorageConfig{
			BucketName:       bucket,
			FolderName:       v1.NewOptString(folder),
			Endpoint:         endpoint,
			Region:           "jp-north-1",
			AccessKeyID:      "test-access-key",
			SecretAccessKey:  "test-secret-key",
			UseDocumentIndex: true,
		}),
	}
	created, err := svcOp.Create(t.Context(), req)
	if err != nil {
		t.Fatalf("create object storage service: %v", err)
	}
	read, err := svcOp.Read(t.Context(), created.ID.Value)
	if err != nil {
		t.Fatal(err)
	}
	return read
}

func TestDataPlaneObjectStorageBackend(t *testing.T) {
	cpURL, client, dpAddr := newGatewayWithControlPlane(t)
	fakeS3 := newFakeS3(t, map[string]string{
		"site-bucket/static/hello.txt":  "hello from s3",
		"site-bucket/static/index.html": "<html>index</html>",
	})

	svc := createObjectStorageService(t, client, "s3_svc", fakeS3.URL, "site-bucket", "static")
	serviceID := uuid.MustParse(svc.ID.Value.String())
	createGatewayRoute(t, client, serviceID, &v1.RouteDetail{
		Name:      v1.NewOptName("s3_route"),
		Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
		Methods:   []v1.HTTPMethod{v1.HTTPMethodGET, v1.HTTPMethodHEAD, v1.HTTPMethodOPTIONS},
	})
	host := svc.RouteHost.Value

	t.Run("serves an object", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "GET", host, "/hello.txt")
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "hello from s3" {
			t.Errorf("body = %q", body)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "text/plain" {
			t.Errorf("Content-Type = %q", ct)
		}
	})

	t.Run("document index resolves directory paths", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "GET", host, "/")
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "<html>index</html>" {
			t.Errorf("body = %q, want index.html content", body)
		}
	})

	t.Run("missing object is a 404", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "GET", host, "/nope.txt")
		if resp.StatusCode != 404 {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "object not found: static/nope.txt") {
			t.Errorf("body = %s", body)
		}
	})

	t.Run("HEAD works", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "HEAD", host, "/hello.txt")
		if resp.StatusCode != 200 {
			t.Errorf("status = %d", resp.StatusCode)
		}
	})

	t.Run("OPTIONS answers without hitting the backend", func(t *testing.T) {
		resp := gwDo(t, dpAddr, "OPTIONS", host, "/hello.txt")
		if resp.StatusCode != 204 {
			t.Errorf("status = %d, want 204", resp.StatusCode)
		}
	})

	t.Run("empty methods default to GET, HEAD, and OPTIONS", func(t *testing.T) {
		// The SDK fills methods client-side, so exercise the omission with a
		// raw request. An empty list must not expand to all methods here.
		resp, err := http.Post(cpURL+"/services/"+svc.ID.Value.String()+"/routes", "application/json",
			strings.NewReader(`{"name":"s3_defaults_route","protocols":"http","path":"/defaults","methods":[]}`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var created struct {
			Apigw struct {
				Route struct {
					Methods []string `json:"methods"`
				} `json:"route"`
			} `json:"apigw"`
		}
		if resp.StatusCode != 201 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d: %s", resp.StatusCode, body)
		}
		if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
			t.Fatal(err)
		}
		got := created.Apigw.Route.Methods
		if len(got) != 3 {
			t.Errorf("methods = %v, want [GET HEAD OPTIONS]", got)
		}
	})

	t.Run("route with a write method is refused", func(t *testing.T) {
		_, err := apigwsdk.NewRouteOp(client, serviceID).Create(t.Context(), &v1.RouteDetail{
			Name:      v1.NewOptName("s3_post_route"),
			Protocols: v1.NewOptRouteDetailProtocols(v1.RouteDetailProtocolsHTTP),
			Path:      v1.NewOptString("/post"),
			Methods:   []v1.HTTPMethod{v1.HTTPMethodPOST},
		})
		if err == nil || !strings.Contains(err.Error(), "GET, HEAD, and OPTIONS") {
			t.Errorf("err = %v, want the method restriction", err)
		}
	})
}
