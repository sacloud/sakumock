package objectstorage

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// ---- JSON request/response types (field names match the OpenAPI spec) ----

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

type clusterInfo struct {
	ID              string
	Region          string
	DisplayName     string
	DisplayNameJa   string
	DisplayNameEnUS string
	ControlPanelURL string
	EndpointBase    string
	S3Endpoint      string
	IAMEndpoint     string
	APIZone         []string
	StorageZone     []string
	PlanFamily      string
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

func (c clusterInfo) toJSON() map[string]any {
	return map[string]any{
		"id":                 c.ID,
		"region":             c.Region,
		"display_name":       c.DisplayName,
		"display_name_ja":    c.DisplayNameJa,
		"display_name_en_us": c.DisplayNameEnUS,
		"control_panel_url":  c.ControlPanelURL,
		"endpoint_base":      c.EndpointBase,
		"s3_endpoint":        c.S3Endpoint,
		"iam_endpoint":       c.IAMEndpoint,
		"api_zone":           c.APIZone,
		"storage_zone":       c.StorageZone,
		"plan_family":        c.PlanFamily,
	}
}

func findCluster(id string) (clusterInfo, bool) {
	for _, c := range clusters {
		if c.ID == id {
			return c, true
		}
	}
	return clusterInfo{}, false
}

// rfc3339 formats t the way the API renders date-time fields.
func rfc3339(t time.Time) string { return t.Format(time.RFC3339) }

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

func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

// writeError writes the object storage error shape: {"error":{"code","message"}}.
// The SDK reads error.code to classify the failure (e.g. saclient.IsNotFoundError
// checks for 404), so the body's code must mirror the HTTP status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    status,
			"message": msg,
		},
	})
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
	data := make([]map[string]any, len(clusters))
	for i, c := range clusters {
		data[i] = c.toJSON()
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	c, ok := findCluster(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": c.toJSON()})
}

type createBucketRequest struct {
	ClusterID string    `json:"cluster_id"`
	Plan      *planJSON `json:"plan"`
}

func (s *Server) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req createBucketRequest
	if err := readJSON(r, &req); err != nil {
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
	writeJSON(w, http.StatusCreated, map[string]any{"data": bucketToJSON(b)})
}

func (s *Server) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	// The spec defines no 404 for bucket deletion, so treat it as idempotent:
	// deleting a missing bucket still returns 204 (matching Terraform's expectation
	// that a delete of an already-gone resource is not an error).
	s.store.DeleteBucket(r.PathValue("name"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) replicationJSON(b Bucket) map[string]any {
	dest := map[string]any{"name": b.Replication.DestBucket, "cluster_id": "", "plan": planJSON{}}
	if db, ok := s.store.GetBucket(b.Replication.DestBucket); ok {
		dest = map[string]any{"name": db.Name, "cluster_id": db.ClusterID, "plan": planJSON{Type: db.PlanType, ServiceClassPath: db.ServiceClassPath}}
	}
	return map[string]any{
		"source_bucket": map[string]any{"name": b.Name, "cluster_id": b.ClusterID, "plan": planJSON{Type: b.PlanType, ServiceClassPath: b.ServiceClassPath}},
		"dest_bucket":   dest,
		"config_status": b.Replication.ConfigStatus,
		"created_at":    rfc3339(b.Replication.CreatedAt),
	}
}

func (s *Server) handleGetReplication(w http.ResponseWriter, r *http.Request) {
	b, ok := s.store.GetBucket(r.PathValue("name"))
	if !ok || b.Replication == nil {
		writeError(w, http.StatusNotFound, "replication configuration not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": s.replicationJSON(b)})
}

func (s *Server) handlePostReplication(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		DestBucket string `json:"dest_bucket"`
	}
	if err := readJSON(r, &req); err != nil {
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
	writeJSON(w, http.StatusOK, map[string]any{"data": s.replicationJSON(b)})
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
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
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
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// ---- Account handlers ----

func accountJSON(a Account) map[string]any {
	return map[string]any{
		"resource_id": a.ResourceID,
		"code":        a.Code,
		"created_at":  rfc3339(a.CreatedAt),
	}
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	a, ok := s.store.GetAccount(r.PathValue("site"))
	if !ok {
		writeError(w, http.StatusNotFound, "account does not exist")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": accountJSON(a)})
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	a, ok := s.store.CreateAccount(r.PathValue("site"))
	if !ok {
		writeError(w, http.StatusConflict, "account already exists")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": accountJSON(a)})
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if !s.store.DeleteAccount(r.PathValue("site")) {
		writeError(w, http.StatusNotFound, "account does not exist")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// accountKeyJSON renders an access key. showSecret controls whether the secret
// is included: the real API returns it only when the key is created. On reads
// the secret field is omitted entirely (rather than sent empty) because the
// spec's SecretAccessKey pattern rejects an empty string.
func accountKeyJSON(k AccountKey, showSecret bool) map[string]any {
	m := map[string]any{
		"id":         k.ID,
		"created_at": rfc3339(k.CreatedAt),
	}
	if showSecret {
		m["secret"] = k.Secret
	}
	return m
}

func (s *Server) handleListAccountKeys(w http.ResponseWriter, r *http.Request) {
	keys, ok := s.store.ListAccountKeys(r.PathValue("site"))
	if !ok {
		writeError(w, http.StatusNotFound, "account does not exist")
		return
	}
	data := make([]map[string]any, len(keys))
	for i, k := range keys {
		data[i] = accountKeyJSON(k, false)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) handleCreateAccountKey(w http.ResponseWriter, r *http.Request) {
	k, ok := s.store.CreateAccountKey(r.PathValue("site"))
	if !ok {
		writeError(w, http.StatusNotFound, "account does not exist")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": accountKeyJSON(k, true)})
}

func (s *Server) handleGetAccountKey(w http.ResponseWriter, r *http.Request) {
	k, ok := s.store.GetAccountKey(r.PathValue("site"), r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "access key not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": accountKeyJSON(k, false)})
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

func permissionJSON(p Permission) map[string]any {
	controls := make([]map[string]any, len(p.BucketControls))
	for i, c := range p.BucketControls {
		controls[i] = map[string]any{
			"bucket_name": c.BucketName,
			"can_read":    c.CanRead,
			"can_write":   c.CanWrite,
			"created_at":  rfc3339(c.CreatedAt),
		}
	}
	return map[string]any{
		"id":              p.ID,
		"display_name":    p.DisplayName,
		"bucket_controls": controls,
		"created_at":      rfc3339(p.CreatedAt),
	}
}

func (s *Server) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	perms := s.store.ListPermissions(r.PathValue("site"))
	data := make([]map[string]any, len(perms))
	for i, p := range perms {
		data[i] = permissionJSON(p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (s *Server) handleCreatePermission(w http.ResponseWriter, r *http.Request) {
	var req permissionBody
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "display_name is required")
		return
	}
	p := s.store.CreatePermission(r.PathValue("site"), req.DisplayName, req.toControls())
	writeJSON(w, http.StatusCreated, map[string]any{"data": permissionJSON(p)})
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
	writeJSON(w, http.StatusOK, map[string]any{"data": permissionJSON(p)})
}

func (s *Server) handleUpdatePermission(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePermissionID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	var req permissionBody
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, ok := s.store.UpdatePermission(r.PathValue("site"), id, req.DisplayName, req.toControls())
	if !ok {
		writeError(w, http.StatusNotFound, "permission not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": permissionJSON(p)})
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

func permissionKeyJSON(k PermissionKey, showSecret bool) map[string]any {
	m := map[string]any{
		"id":         k.ID,
		"created_at": rfc3339(k.CreatedAt),
	}
	if showSecret {
		m["secret"] = k.Secret
	}
	return m
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
	data := make([]map[string]any, len(keys))
	for i, k := range keys {
		data[i] = permissionKeyJSON(k, false)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
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
	writeJSON(w, http.StatusCreated, map[string]any{"data": permissionKeyJSON(k, true)})
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
	writeJSON(w, http.StatusOK, map[string]any{"data": permissionKeyJSON(k, false)})
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

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"accept_new":  true,
			"message":     "",
			"started_at":  rfc3339(time.Now()),
			"status_code": map[string]any{"id": 1, "status": "available"},
		},
	})
}

func (s *Server) handlePlans(w http.ResponseWriter, r *http.Request) {
	site := r.PathValue("site")
	planType := "standard"
	if c, ok := findCluster(site); ok && c.PlanFamily == "archive" {
		planType = "archive"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": []map[string]any{
			{
				"service_class_path": "objectstorage/" + site + "/bucket",
				"type":               planType,
				"cluster_id":         site,
				"capacity_gib":       20000,
				"fee":                map[string]any{"for_month": 1980, "monthly": 7200, "daily": 360, "hourly": 36},
			},
		},
	})
}

func (s *Server) handleQuota(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"num_root_keys":              1,
			"num_buckets":                1000,
			"num_permissions":            1000,
			"num_keys_per_permission":    1,
			"num_buckets_per_permission": 1000,
			"num_objects_per_bucket":     10000000,
			"amount_gib_per_bucket":      10240,
		},
	})
}

func (s *Server) handleBucketMetering(w http.ResponseWriter, _ *http.Request) {
	// The mock keeps no usage history, so it reports no billing items.
	writeJSON(w, http.StatusOK, map[string]any{"data": []map[string]any{}})
}

// ---- Bucket sub-resources (encryption / penalty / usage / quota / plan) ----

func encryptionJSON(e *Encryption) map[string]any {
	return map[string]any{
		"kms_key_id":    e.KMSKeyID,
		"configured_at": rfc3339(e.ConfiguredAt),
	}
}

func (s *Server) handleGetEncryption(w http.ResponseWriter, r *http.Request) {
	b, ok := s.store.GetBucket(r.PathValue("name"))
	if !ok || b.Encryption == nil {
		writeError(w, http.StatusNotFound, "encryption configuration not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": encryptionJSON(b.Encryption)})
}

func (s *Server) handlePutEncryption(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		KMSKeyID string `json:"kms_key_id"`
	}
	if err := readJSON(r, &req); err != nil {
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
	writeJSON(w, http.StatusOK, map[string]any{"data": encryptionJSON(b.Encryption)})
}

func (s *Server) handleDeleteEncryption(w http.ResponseWriter, r *http.Request) {
	if !s.store.DeleteBucketEncryption(r.PathValue("name")) {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleBucketPenalty(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.store.GetBucket(r.PathValue("name")); !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"num_objects_per_bucket": map[string]any{"val": 0, "quota": 10000000, "is_applied": false},
			"amount_gib_per_bucket":  map[string]any{"val": 0, "quota": 10240, "is_applied": false},
		},
	})
}

func (s *Server) handleBucketUsage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.store.GetBucket(r.PathValue("name")); !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{"num_objects_per_bucket": 0, "amount_gib_per_bucket": 0},
	})
}

func (s *Server) handleBucketQuota(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.store.GetBucket(r.PathValue("name")); !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{"num_objects_per_bucket": 10000000, "amount_gib_per_bucket": 10240},
	})
}

func (s *Server) handleGetBucketPlan(w http.ResponseWriter, r *http.Request) {
	b, ok := s.store.GetBucket(r.PathValue("name"))
	if !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"plan": map[string]any{
				"type":               b.PlanType,
				"service_class_path": b.ServiceClassPath,
				"cluster_id":         b.ClusterID,
			},
			"contract": map[string]any{
				"resource_id": b.ResourceID,
				"status":      "active",
				"created_at":  rfc3339(b.CreatedAt),
			},
		},
	})
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
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	b, ok := s.store.GetBucket(name)
	if !ok {
		writeError(w, http.StatusNotFound, "bucket not found")
		return
	}
	prevResourceID := b.ResourceID
	// Re-create the bucket entry with the new plan to assign a fresh contract ID.
	s.store.DeleteBucket(name)
	nb, _ := s.store.CreateBucket(name, b.ClusterID, req.NewPlan.Type, req.NewPlan.ServiceClassPath)
	now := rfc3339(time.Now())
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"previous_contract": map[string]any{"resource_id": prevResourceID, "status": "terminated", "created_at": rfc3339(b.CreatedAt)},
			"new_contract":      map[string]any{"resource_id": nb.ResourceID, "status": "active", "created_at": now},
			"plan": map[string]any{
				"type":               nb.PlanType,
				"service_class_path": nb.ServiceClassPath,
				"cluster_id":         nb.ClusterID,
			},
		},
	})
}
