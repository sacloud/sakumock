package objectstorage

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sacloud/sakumock/core"
)

// ---- JSON request/response types (field names match the OpenAPI spec) ----

// dataResponse is the { "data": ... } envelope the object storage API wraps most
// of its responses in.
type dataResponse struct {
	Data any `json:"data"`
}

// errorResponse is the API's error shape: {"error":{"code","message"}}.
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type planJSON struct {
	Type             string `json:"type"`
	ServiceClassPath string `json:"service_class_path"`
}

type bucketJSON struct {
	ClusterID string   `json:"cluster_id"`
	Name      string   `json:"name"`
	Plan      planJSON `json:"plan"`
}

type bucketListItemJSON struct {
	Name       string   `json:"name"`
	ResourceID string   `json:"resource_id"`
	Plan       planJSON `json:"plan"`
}

// accountData is the per-site root account representation.
type accountData struct {
	ResourceID string `json:"resource_id"`
	Code       string `json:"code"`
	CreatedAt  string `json:"created_at"`
}

// accessKeyData is an account or permission access key (both share this shape).
// Secret is only present when the key was just created — the real API never
// returns it on reads — and omitempty keeps it absent rather than empty, which
// the spec's secret pattern would reject.
type accessKeyData struct {
	ID        string `json:"id"`
	Secret    string `json:"secret,omitempty"`
	CreatedAt string `json:"created_at"`
}

// bucketControlData grants a permission read/write access to a bucket.
type bucketControlData struct {
	BucketName string `json:"bucket_name"`
	CanRead    bool   `json:"can_read"`
	CanWrite   bool   `json:"can_write"`
	CreatedAt  string `json:"created_at"`
}

// permissionData is a permission with its bucket controls.
type permissionData struct {
	ID             int64               `json:"id"`
	DisplayName    string              `json:"display_name"`
	BucketControls []bucketControlData `json:"bucket_controls"`
	CreatedAt      string              `json:"created_at"`
}

// encryptionData is a bucket's server-side encryption configuration.
type encryptionData struct {
	KMSKeyID     string `json:"kms_key_id"`
	ConfiguredAt string `json:"configured_at"`
}

// replicationData is a bucket's replication configuration; source and dest reuse
// the bucket representation.
type replicationData struct {
	SourceBucket bucketJSON `json:"source_bucket"`
	DestBucket   bucketJSON `json:"dest_bucket"`
	ConfigStatus string     `json:"config_status"`
	CreatedAt    string     `json:"created_at"`
}

// bucketMetrics is the {num_objects_per_bucket, amount_gib_per_bucket} shape
// shared by the bucket usage and quota responses.
type bucketMetrics struct {
	NumObjects int     `json:"num_objects_per_bucket"`
	AmountGiB  float64 `json:"amount_gib_per_bucket"`
}

// bucketPlanSummary and contractData are the plan/contract shapes shared by the
// get-plan and change-plan responses.
type bucketPlanSummary struct {
	Type             string `json:"type"`
	ServiceClassPath string `json:"service_class_path"`
	ClusterID        string `json:"cluster_id"`
}

type contractData struct {
	ResourceID string `json:"resource_id"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

type clusterInfo struct {
	ID              string   `json:"id"`
	Region          string   `json:"region"`
	DisplayName     string   `json:"display_name"`
	DisplayNameJa   string   `json:"display_name_ja"`
	DisplayNameEnUS string   `json:"display_name_en_us"`
	ControlPanelURL string   `json:"control_panel_url"`
	EndpointBase    string   `json:"endpoint_base"`
	S3Endpoint      string   `json:"s3_endpoint"`
	IAMEndpoint     string   `json:"iam_endpoint"`
	APIZone         []string `json:"api_zone"`
	StorageZone     []string `json:"storage_zone"`
	PlanFamily      string   `json:"plan_family"`
}

// clusters are the static object storage sites the real API exposes.
var clusters = []clusterInfo{
	{
		ID: "isk01", Region: "jp-north-1",
		DisplayName: "石狩第1サイト", DisplayNameJa: "石狩第1サイト", DisplayNameEnUS: "Ishikari Cluster #1",
		ControlPanelURL: "secure.sakura.ad.jp/objectstorage/isk01",
		EndpointBase:    "isk01.objectstorage.sakurastorage.jp",
		S3Endpoint:      "s3.isk01.objectstorage.sakurastorage.jp",
		IAMEndpoint:     "iam.isk01.objectstorage.sakurastorage.jp",
		APIZone:         []string{"is1a", "is1b"}, StorageZone: []string{"is1a", "is1b"},
		PlanFamily: "standard",
	},
	{
		ID: "tky01", Region: "jp-east-1",
		DisplayName: "東京第1サイト", DisplayNameJa: "東京第1サイト", DisplayNameEnUS: "Tokyo Cluster #1",
		ControlPanelURL: "secure.sakura.ad.jp/objectstorage/tky01",
		EndpointBase:    "tky01.objectstorage.sakurastorage.jp",
		S3Endpoint:      "s3.tky01.objectstorage.sakurastorage.jp",
		IAMEndpoint:     "iam.tky01.objectstorage.sakurastorage.jp",
		APIZone:         []string{"tk1a", "tk1b"}, StorageZone: []string{"tk1a", "tk1b"},
		PlanFamily: "standard",
	},
	{
		ID: "arc02", Region: "jp-north-1",
		DisplayName: "石狩アーカイブサイト", DisplayNameJa: "石狩アーカイブサイト", DisplayNameEnUS: "Ishikari Archive Cluster #2",
		ControlPanelURL: "secure.sakura.ad.jp/objectstorage/arc02",
		EndpointBase:    "arc02.objectstorage.sakurastorage.jp",
		S3Endpoint:      "s3.arc02.objectstorage.sakurastorage.jp",
		IAMEndpoint:     "iam.arc02.objectstorage.sakurastorage.jp",
		APIZone:         []string{"is1a", "is1b"}, StorageZone: []string{"is1a", "is1b"},
		PlanFamily: "archive",
	},
}

func findCluster(id string) (clusterInfo, bool) {
	for _, c := range clusters {
		if c.ID == id {
			return c, true
		}
	}
	return clusterInfo{}, false
}

func bucketToJSON(b Bucket) bucketJSON {
	return bucketJSON{
		ClusterID: b.ClusterID,
		Name:      b.Name,
		Plan:      planJSON{Type: b.PlanType, ServiceClassPath: b.ServiceClassPath},
	}
}

// ---- HTTP plumbing ----

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, r := range s.routeTable() {
		mux.HandleFunc(r.Method+" "+r.Path, r.Handler)
	}
	return mux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	s.mux.ServeHTTP(rw, r)
	s.logger.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"status", rw.statusCode,
	)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// writeError writes the object storage error shape: {"error":{"code","message"}}.
// The SDK reads error.code to classify the failure (e.g. saclient.IsNotFoundError
// checks for 404), so the body's code must mirror the HTTP status.
func writeError(w http.ResponseWriter, status int, msg string) {
	core.WriteJSON(w, status, errorResponse{Error: errorBody{Code: status, Message: msg}})
}

func parsePermissionID(s string) (int64, bool) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// ---- Federation handlers (/fed/v1) ----

func (s *Server) handleListClusters(w http.ResponseWriter, _ *http.Request) {
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: clusters})
}

func (s *Server) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	c, ok := findCluster(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: c})
}

type createBucketRequest struct {
	ClusterID string    `json:"cluster_id"`
	Plan      *planJSON `json:"plan"`
}

func (s *Server) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req createBucketRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ClusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster_id is required")
		return
	}
	planType := "standard"
	if c, ok := findCluster(req.ClusterID); ok && c.PlanFamily == "archive" {
		planType = "archive"
	}
	serviceClassPath := "objectstorage/" + req.ClusterID + "/bucket"
	if req.Plan != nil {
		if req.Plan.Type != "" {
			planType = req.Plan.Type
		}
		if req.Plan.ServiceClassPath != "" {
			serviceClassPath = req.Plan.ServiceClassPath
		}
	}
	b, ok := s.store.CreateBucket(name, req.ClusterID, planType, serviceClassPath)
	if !ok {
		writeError(w, http.StatusConflict, fmt.Sprintf("bucket %q already exists", name))
		return
	}
	s.dataPlane.createBucket(name)
	core.WriteJSON(w, http.StatusCreated, dataResponse{Data: bucketToJSON(b)})
}

func (s *Server) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	// The spec defines no 404 for bucket deletion, so treat it as idempotent:
	// deleting a missing bucket still returns 204 (matching Terraform's expectation
	// that a delete of an already-gone resource is not an error).
	name := r.PathValue("name")
	s.store.DeleteBucket(name)
	s.dataPlane.deleteBucket(name)
	w.WriteHeader(http.StatusNoContent)
}

// buildReplicationData builds the replication response for a bucket with a
// replication config set. The destination's cluster/plan are filled in when the
// destination bucket is known to the mock; otherwise only its name is set.
func (s *Server) buildReplicationData(b Bucket) replicationData {
	dest := bucketJSON{Name: b.Replication.DestBucket}
	if db, ok := s.store.GetBucket(b.Replication.DestBucket); ok {
		dest = bucketToJSON(db)
	}
	return replicationData{
		SourceBucket: bucketToJSON(b),
		DestBucket:   dest,
		ConfigStatus: b.Replication.ConfigStatus,
		CreatedAt:    core.FormatRFC3339(b.Replication.CreatedAt),
	}
}

func (s *Server) handleGetReplication(w http.ResponseWriter, r *http.Request) {
	b, ok := s.store.GetBucket(r.PathValue("name"))
	if !ok || b.Replication == nil {
		writeError(w, http.StatusNotFound, "replication configuration not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: s.buildReplicationData(b)})
}

func (s *Server) handlePostReplication(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		DestBucket string `json:"dest_bucket"`
	}
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.DestBucket == "" {
		writeError(w, http.StatusBadRequest, "dest_bucket is required")
		return
	}
	b, ok := s.store.SetBucketReplication(name, req.DestBucket)
	if !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	// The spec declares 201 for replication creation.
	core.WriteJSON(w, http.StatusCreated, dataResponse{Data: s.buildReplicationData(b)})
}

func (s *Server) handleDeleteReplication(w http.ResponseWriter, r *http.Request) {
	if !s.store.DeleteBucketReplication(r.PathValue("name")) {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReplicableTargets(w http.ResponseWriter, r *http.Request) {
	src, ok := s.store.GetBucket(r.PathValue("name"))
	if !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	// Replicable targets are buckets in other clusters.
	var data []bucketJSON
	for _, b := range s.store.ListBuckets("") {
		if b.Name == src.Name || b.ClusterID == src.ClusterID {
			continue
		}
		data = append(data, bucketToJSON(b))
	}
	if data == nil {
		data = []bucketJSON{}
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: data})
}

// ---- Site bucket handlers (/{site}/v2) ----

func (s *Server) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets := s.store.ListBuckets(r.PathValue("site"))
	data := make([]bucketListItemJSON, len(buckets))
	for i, b := range buckets {
		data[i] = bucketListItemJSON{
			Name:       b.Name,
			ResourceID: b.ResourceID,
			Plan:       planJSON{Type: b.PlanType, ServiceClassPath: b.ServiceClassPath},
		}
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: data})
}

// ---- Account handlers ----

func toAccountData(a Account) accountData {
	return accountData{
		ResourceID: a.ResourceID,
		Code:       a.Code,
		CreatedAt:  core.FormatRFC3339(a.CreatedAt),
	}
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	a, ok := s.store.GetAccount(r.PathValue("site"))
	if !ok {
		writeError(w, http.StatusNotFound, "account does not exist")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: toAccountData(a)})
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	a, ok := s.store.CreateAccount(r.PathValue("site"))
	if !ok {
		writeError(w, http.StatusConflict, "account already exists")
		return
	}
	core.WriteJSON(w, http.StatusCreated, dataResponse{Data: toAccountData(a)})
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if !s.store.DeleteAccount(r.PathValue("site")) {
		writeError(w, http.StatusNotFound, "account does not exist")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// toAccountKeyData renders an access key. showSecret controls whether the secret
// is included: the real API returns it only when the key is created, so reads
// omit it (accessKeyData.Secret is omitempty, which the spec's pattern requires
// — an empty secret string would fail validation).
func toAccountKeyData(k AccountKey, showSecret bool) accessKeyData {
	d := accessKeyData{ID: k.ID, CreatedAt: core.FormatRFC3339(k.CreatedAt)}
	if showSecret {
		d.Secret = k.Secret
	}
	return d
}

func (s *Server) handleListAccountKeys(w http.ResponseWriter, r *http.Request) {
	keys, ok := s.store.ListAccountKeys(r.PathValue("site"))
	if !ok {
		writeError(w, http.StatusNotFound, "account does not exist")
		return
	}
	data := make([]accessKeyData, len(keys))
	for i, k := range keys {
		data[i] = toAccountKeyData(k, false)
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: data})
}

func (s *Server) handleCreateAccountKey(w http.ResponseWriter, r *http.Request) {
	k, ok := s.store.CreateAccountKey(r.PathValue("site"))
	if !ok {
		writeError(w, http.StatusNotFound, "account does not exist")
		return
	}
	core.WriteJSON(w, http.StatusCreated, dataResponse{Data: toAccountKeyData(k, true)})
}

func (s *Server) handleGetAccountKey(w http.ResponseWriter, r *http.Request) {
	k, ok := s.store.GetAccountKey(r.PathValue("site"), r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "access key not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: toAccountKeyData(k, false)})
}

func (s *Server) handleDeleteAccountKey(w http.ResponseWriter, r *http.Request) {
	if !s.store.DeleteAccountKey(r.PathValue("site"), r.PathValue("id")) {
		writeError(w, http.StatusNotFound, "access key not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Permission handlers ----

type permissionBody struct {
	DisplayName    string `json:"display_name"`
	BucketControls []struct {
		BucketName string `json:"bucket_name"`
		CanRead    bool   `json:"can_read"`
		CanWrite   bool   `json:"can_write"`
	} `json:"bucket_controls"`
}

func (b permissionBody) toControls() []BucketControl {
	controls := make([]BucketControl, len(b.BucketControls))
	for i, c := range b.BucketControls {
		controls[i] = BucketControl{BucketName: c.BucketName, CanRead: c.CanRead, CanWrite: c.CanWrite}
	}
	return controls
}

func toPermissionData(p Permission) permissionData {
	controls := make([]bucketControlData, len(p.BucketControls))
	for i, c := range p.BucketControls {
		controls[i] = bucketControlData{
			BucketName: c.BucketName,
			CanRead:    c.CanRead,
			CanWrite:   c.CanWrite,
			CreatedAt:  core.FormatRFC3339(c.CreatedAt),
		}
	}
	return permissionData{
		ID:             p.ID,
		DisplayName:    p.DisplayName,
		BucketControls: controls,
		CreatedAt:      core.FormatRFC3339(p.CreatedAt),
	}
}

func (s *Server) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	perms := s.store.ListPermissions(r.PathValue("site"))
	data := make([]permissionData, len(perms))
	for i, p := range perms {
		data[i] = toPermissionData(p)
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: data})
}

func (s *Server) handleCreatePermission(w http.ResponseWriter, r *http.Request) {
	var req permissionBody
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "display_name is required")
		return
	}
	p := s.store.CreatePermission(r.PathValue("site"), req.DisplayName, req.toControls())
	core.WriteJSON(w, http.StatusCreated, dataResponse{Data: toPermissionData(p)})
}

func (s *Server) handleGetPermission(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePermissionID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	p, ok := s.store.GetPermission(r.PathValue("site"), id)
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: toPermissionData(p)})
}

func (s *Server) handleUpdatePermission(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePermissionID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	var req permissionBody
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, ok := s.store.UpdatePermission(r.PathValue("site"), id, req.DisplayName, req.toControls())
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: toPermissionData(p)})
}

func (s *Server) handleDeletePermission(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePermissionID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	if !s.store.DeletePermission(r.PathValue("site"), id) {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toPermissionKeyData(k PermissionKey, showSecret bool) accessKeyData {
	d := accessKeyData{ID: k.ID, CreatedAt: core.FormatRFC3339(k.CreatedAt)}
	if showSecret {
		d.Secret = k.Secret
	}
	return d
}

func (s *Server) handleListPermissionKeys(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePermissionID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	keys, ok := s.store.ListPermissionKeys(r.PathValue("site"), id)
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	data := make([]accessKeyData, len(keys))
	for i, k := range keys {
		data[i] = toPermissionKeyData(k, false)
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: data})
}

func (s *Server) handleCreatePermissionKey(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePermissionID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	k, ok := s.store.CreatePermissionKey(r.PathValue("site"), id)
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	core.WriteJSON(w, http.StatusCreated, dataResponse{Data: toPermissionKeyData(k, true)})
}

func (s *Server) handleGetPermissionKey(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePermissionID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	k, ok := s.store.GetPermissionKey(r.PathValue("site"), id, r.PathValue("key_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "access key not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: toPermissionKeyData(k, false)})
}

func (s *Server) handleDeletePermissionKey(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePermissionID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	if !s.store.DeletePermissionKey(r.PathValue("site"), id, r.PathValue("key_id")) {
		writeError(w, http.StatusNotFound, "access key not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Site status / plans / quota / metering ----

type statusData struct {
	AcceptNew  bool           `json:"accept_new"`
	Message    string         `json:"message"`
	StartedAt  string         `json:"started_at"`
	StatusCode statusCodeData `json:"status_code"`
}

type statusCodeData struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: statusData{
		AcceptNew:  true,
		StartedAt:  core.FormatRFC3339(time.Now()),
		StatusCode: statusCodeData{ID: 1, Status: "available"},
	}})
}

type planItemData struct {
	ServiceClassPath string      `json:"service_class_path"`
	Type             string      `json:"type"`
	ClusterID        string      `json:"cluster_id"`
	CapacityGiB      int         `json:"capacity_gib"`
	Fee              planFeeData `json:"fee"`
}

type planFeeData struct {
	ForMonth int `json:"for_month"`
	Monthly  int `json:"monthly"`
	Daily    int `json:"daily"`
	Hourly   int `json:"hourly"`
}

func (s *Server) handlePlans(w http.ResponseWriter, r *http.Request) {
	site := r.PathValue("site")
	planType := "standard"
	if c, ok := findCluster(site); ok && c.PlanFamily == "archive" {
		planType = "archive"
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: []planItemData{
		{
			ServiceClassPath: "objectstorage/" + site + "/bucket",
			Type:             planType,
			ClusterID:        site,
			CapacityGiB:      20000,
			Fee:              planFeeData{ForMonth: 1980, Monthly: 7200, Daily: 360, Hourly: 36},
		},
	}})
}

type siteQuotaData struct {
	NumRootKeys             int     `json:"num_root_keys"`
	NumBuckets              int     `json:"num_buckets"`
	NumPermissions          int     `json:"num_permissions"`
	NumKeysPerPermission    int     `json:"num_keys_per_permission"`
	NumBucketsPerPermission int     `json:"num_buckets_per_permission"`
	NumObjectsPerBucket     int     `json:"num_objects_per_bucket"`
	AmountGiBPerBucket      float64 `json:"amount_gib_per_bucket"`
}

func (s *Server) handleQuota(w http.ResponseWriter, _ *http.Request) {
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: siteQuotaData{
		NumRootKeys:             1,
		NumBuckets:              1000,
		NumPermissions:          1000,
		NumKeysPerPermission:    1,
		NumBucketsPerPermission: 1000,
		NumObjectsPerBucket:     10000000,
		AmountGiBPerBucket:      10240,
	}})
}

// bucketBillingItem is the per-bucket metering record. The mock keeps no usage
// history, so the metering endpoint returns an empty slice of these; the type
// documents the shape the real API would return.
type bucketBillingItem struct {
	YearMonth       string        `json:"year_month"`
	Basic           billingDetail `json:"basic"`
	StorageUsage    billingDetail `json:"storage_usage"`
	NumObjects      billingDetail `json:"num_objects"`
	NumTransactions billingDetail `json:"num_transactions"`
	TransferAmount  billingDetail `json:"transfer_amount"`
}

type billingDetail struct {
	Found     bool    `json:"found"`
	Usage     float64 `json:"usage"`
	Charge    int     `json:"charge"`
	IsClosing bool    `json:"is_closing"`
	From      string  `json:"from"`
	To        string  `json:"to"`
}

func (s *Server) handleBucketMetering(w http.ResponseWriter, _ *http.Request) {
	// The mock keeps no usage history, so it reports no billing items.
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: []bucketBillingItem{}})
}

// ---- Bucket sub-resources (encryption / penalty / usage / quota / plan) ----

func toEncryptionData(e *Encryption) encryptionData {
	return encryptionData{KMSKeyID: e.KMSKeyID, ConfiguredAt: core.FormatRFC3339(e.ConfiguredAt)}
}

func (s *Server) handleGetEncryption(w http.ResponseWriter, r *http.Request) {
	b, ok := s.store.GetBucket(r.PathValue("name"))
	if !ok || b.Encryption == nil {
		writeError(w, http.StatusNotFound, "encryption configuration not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: toEncryptionData(b.Encryption)})
}

func (s *Server) handlePutEncryption(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		KMSKeyID string `json:"kms_key_id"`
	}
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.KMSKeyID == "" {
		writeError(w, http.StatusBadRequest, "kms_key_id is required")
		return
	}
	b, ok := s.store.SetBucketEncryption(name, req.KMSKeyID)
	if !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: toEncryptionData(b.Encryption)})
}

func (s *Server) handleDeleteEncryption(w http.ResponseWriter, r *http.Request) {
	if !s.store.DeleteBucketEncryption(r.PathValue("name")) {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type bucketPenaltyData struct {
	NumObjectsPerBucket penaltyMetric `json:"num_objects_per_bucket"`
	AmountGiBPerBucket  penaltyMetric `json:"amount_gib_per_bucket"`
}

type penaltyMetric struct {
	Val       float64 `json:"val"`
	Quota     float64 `json:"quota"`
	IsApplied bool    `json:"is_applied"`
}

func (s *Server) handleBucketPenalty(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.store.GetBucket(r.PathValue("name")); !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: bucketPenaltyData{
		NumObjectsPerBucket: penaltyMetric{Val: 0, Quota: 10000000, IsApplied: false},
		AmountGiBPerBucket:  penaltyMetric{Val: 0, Quota: 10240, IsApplied: false},
	}})
}

func (s *Server) handleBucketUsage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.store.GetBucket(r.PathValue("name")); !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: bucketMetrics{NumObjects: 0, AmountGiB: 0}})
}

func (s *Server) handleBucketQuota(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.store.GetBucket(r.PathValue("name")); !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: bucketMetrics{NumObjects: 10000000, AmountGiB: 10240}})
}

func (s *Server) handleGetBucketPlan(w http.ResponseWriter, r *http.Request) {
	b, ok := s.store.GetBucket(r.PathValue("name"))
	if !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: struct {
		Plan     bucketPlanSummary `json:"plan"`
		Contract contractData      `json:"contract"`
	}{
		Plan:     bucketPlanSummary{Type: b.PlanType, ServiceClassPath: b.ServiceClassPath, ClusterID: b.ClusterID},
		Contract: contractData{ResourceID: b.ResourceID, Status: "active", CreatedAt: core.FormatRFC3339(b.CreatedAt)},
	}})
}

func (s *Server) handlePutBucketPlan(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		PreviousContract struct {
			ResourceID string `json:"resource_id"`
		} `json:"previous_contract"`
		NewPlan struct {
			Type             string `json:"type"`
			ServiceClassPath string `json:"service_class_path"`
		} `json:"new_plan"`
	}
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.NewPlan.Type == "" || req.NewPlan.ServiceClassPath == "" {
		writeError(w, http.StatusBadRequest, "new_plan.type and new_plan.service_class_path are required")
		return
	}
	b, ok := s.store.GetBucket(name)
	if !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	// previous_contract.resource_id guards against recontracting from an
	// unexpected state, so it must match the bucket's current contract.
	if req.PreviousContract.ResourceID != b.ResourceID {
		writeError(w, http.StatusConflict, "previous_contract.resource_id does not match the current contract")
		return
	}
	prevResourceID := b.ResourceID
	// Re-create the bucket entry with the new plan to assign a fresh contract ID.
	s.store.DeleteBucket(name)
	nb, _ := s.store.CreateBucket(name, b.ClusterID, req.NewPlan.Type, req.NewPlan.ServiceClassPath)
	now := core.FormatRFC3339(time.Now())
	core.WriteJSON(w, http.StatusOK, dataResponse{Data: struct {
		PreviousContract contractData      `json:"previous_contract"`
		NewContract      contractData      `json:"new_contract"`
		Plan             bucketPlanSummary `json:"plan"`
	}{
		PreviousContract: contractData{ResourceID: prevResourceID, Status: "terminated", CreatedAt: core.FormatRFC3339(b.CreatedAt)},
		NewContract:      contractData{ResourceID: nb.ResourceID, Status: "active", CreatedAt: now},
		Plan:             bucketPlanSummary{Type: nb.PlanType, ServiceClassPath: nb.ServiceClassPath, ClusterID: nb.ClusterID},
	}})
}
