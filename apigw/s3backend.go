package apigw

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// serveObjectStorage answers a request for a service backed by an
// S3-compatible bucket (objectStorageConfig): instead of proxying to an HTTP
// upstream, the object addressed by the (stripPath-applied) request path is
// fetched from the configured endpoint with SigV4 credentials. Any
// S3-compatible endpoint works — sakumock's own objectstorage data plane,
// MinIO, or real object storage — since the configuration is self-contained.
func (dp *dataPlane) serveObjectStorage(w http.ResponseWriter, r *http.Request, m *matchResult) {
	osc := m.service.ObjectStorage
	applyCORSResponseHeaders(w.Header(), m.service.CorsConfig, r.Header.Get("Origin"))

	switch r.Method {
	case http.MethodGet, http.MethodHead:
	case http.MethodOptions:
		// CORS preflights are answered before this point; a plain OPTIONS
		// gets an empty success.
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		writeError(w, http.StatusMethodNotAllowed, "object storage services allow only GET, HEAD, and OPTIONS")
		return
	}

	key := objectKey(osc, m.upstreamPath)
	client := dp.s3ClientFor(m.service)

	if r.Method == http.MethodHead {
		out, err := client.HeadObject(r.Context(), &s3.HeadObjectInput{
			Bucket: aws.String(osc.BucketName),
			Key:    aws.String(key),
		})
		if err != nil {
			dp.writeS3Error(w, key, err)
			return
		}
		writeObjectHeaders(w, aws.ToString(out.ContentType), out.ContentLength)
		w.WriteHeader(http.StatusOK)
		return
	}

	out, err := client.GetObject(r.Context(), &s3.GetObjectInput{
		Bucket: aws.String(osc.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		dp.writeS3Error(w, key, err)
		return
	}
	defer out.Body.Close()
	writeObjectHeaders(w, aws.ToString(out.ContentType), out.ContentLength)
	w.WriteHeader(http.StatusOK)
	io.Copy(w, out.Body)
}

// objectKey maps the request path to the object key: the folder prefix is
// prepended and, with useDocumentIndex (the default), directory paths get
// index.html appended.
func objectKey(osc *ObjectStore, upstreamPath string) string {
	key := strings.TrimPrefix(upstreamPath, "/")
	if osc.UseDocumentIndex == nil || *osc.UseDocumentIndex {
		if key == "" || strings.HasSuffix(key, "/") {
			key += "index.html"
		}
	}
	if folder := strings.Trim(osc.FolderName, "/"); folder != "" {
		key = folder + "/" + key
	}
	return key
}

// writeObjectHeaders keeps nil (unknown length) distinct from an explicit 0
// (empty object): only a known length becomes a Content-Length header.
func writeObjectHeaders(w http.ResponseWriter, contentType string, contentLength *int64) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if contentLength != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(*contentLength, 10))
	}
}

// writeS3Error maps a backend failure: a missing object is the client's 404,
// anything else is a debuggable 502.
func (dp *dataPlane) writeS3Error(w http.ResponseWriter, key string, err error) {
	var noKey *types.NoSuchKey
	var apiErr smithy.APIError
	if errors.As(err, &noKey) ||
		(errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound")) {
		writeError(w, http.StatusNotFound, "object not found: "+key)
		return
	}
	dp.logger.Debug("object storage backend error", "key", key, "error", err)
	writeError(w, http.StatusBadGateway, "object storage backend error: "+err.Error())
}

// s3ClientFor returns the cached per-service S3 client, rebuilt when the
// object storage settings change.
func (dp *dataPlane) s3ClientFor(svc Service) *s3.Client {
	osc := svc.ObjectStorage
	fp := fmt.Sprintf("%s|%s|%s|%s", osc.Endpoint, osc.Region, osc.AccessKeyID, osc.SecretAccessKey)

	dp.mu.Lock()
	defer dp.mu.Unlock()
	if c, ok := dp.s3Clients[svc.ID]; ok && c.fingerprint == fp {
		return c.client
	}
	client := s3.NewFromConfig(aws.Config{
		Region:      osc.Region,
		Credentials: credentials.NewStaticCredentialsProvider(osc.AccessKeyID, osc.SecretAccessKey, ""),
	}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(osc.Endpoint)
		// Path-style addressing works with every S3-compatible endpoint and
		// avoids requiring wildcard DNS for virtual-hosted buckets.
		o.UsePathStyle = true
	})
	dp.s3Clients[svc.ID] = &cachedS3Client{fingerprint: fp, client: client}
	return client
}

type cachedS3Client struct {
	fingerprint string
	client      *s3.Client
}
