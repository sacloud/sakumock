package monitoringsuite

import (
	"net/http"
	"time"
)

// ptrOf returns a pointer to a copy of v, for building optional JSON fields.
func ptrOf[T any](v T) *T { return &v }

// routingJSON is the shared shape of a log or metrics routing. Exactly one of
// LogStorage / MetricsStorage is populated, matching the respective schema.
type routingJSON struct {
	ID             int64               `json:"id"`
	UID            string              `json:"uid"`
	ResourceID     *int64              `json:"resource_id"`
	Publisher      publisherJSON       `json:"publisher"`
	Variant        string              `json:"variant"`
	LogStorage     *logStorageJSON     `json:"log_storage,omitempty"`
	MetricsStorage *metricsStorageJSON `json:"metrics_storage,omitempty"`
	CreatedAt      string              `json:"created_at"`
	UpdatedAt      string              `json:"updated_at"`
	IsOk           *bool               `json:"is_ok,omitempty"`
}

type routingRequest struct {
	ResourceID       *int64  `json:"resource_id"`
	PublisherCode    *string `json:"publisher_code"`
	Variant          *string `json:"variant"`
	LogStorageID     *int64  `json:"log_storage_id"`
	MetricsStorageID *int64  `json:"metrics_storage_id"`
}

// ===== Log routings =====

func (s *Server) logRoutingToJSON(rt *Routing, wrapped bool) (routingJSON, bool) {
	pub, ok := s.store.publisher(rt.PublisherCode)
	if !ok {
		return routingJSON{}, false
	}
	st, ok := s.store.findLogStorage(rt.StorageID)
	if !ok {
		return routingJSON{}, false
	}
	j := routingJSON{
		ID:         rt.ID,
		UID:        rt.UID,
		ResourceID: rt.ResourceID,
		Publisher:  publisherToJSON(pub, false),
		Variant:    rt.Variant,
		LogStorage: ptrOf(s.logStorageToJSON(st, false)),
		CreatedAt:  formatTime(rt.CreatedAt),
		UpdatedAt:  formatTime(rt.UpdatedAt),
	}
	if wrapped {
		j.IsOk = boolPtr(true)
	}
	return j, true
}

func (s *Server) handleListLogRoutings(w http.ResponseWriter, r *http.Request) {
	out := []routingJSON{}
	for _, rt := range s.store.logRoutings.all() {
		if j, ok := s.logRoutingToJSON(rt, false); ok {
			out = append(out, j)
		}
	}
	writePage(w, out)
}

func (s *Server) handleCreateLogRouting(w http.ResponseWriter, r *http.Request) {
	var req routingRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.PublisherCode == nil || req.Variant == nil || req.LogStorageID == nil {
		writeError(w, http.StatusBadRequest, "publisher_code, variant and log_storage_id are required")
		return
	}
	pub, ok := s.store.publisher(*req.PublisherCode)
	if !ok || !pub.hasVariant(*req.Variant) {
		writeError(w, http.StatusBadRequest, "invalid publisher_code or variant")
		return
	}
	st, ok := s.store.findLogStorage(*req.LogStorageID)
	if !ok {
		writeError(w, http.StatusBadRequest, "log_storage not found")
		return
	}
	now := time.Now()
	rt := &Routing{
		ID:            s.store.nextInternalID(),
		UID:           newUUID(),
		ResourceID:    req.ResourceID,
		PublisherCode: pub.Code,
		Variant:       *req.Variant,
		StorageID:     st.ID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.store.logRoutings.set(rt.UID, rt)
	j, _ := s.logRoutingToJSON(rt, true)
	writeJSON(w, http.StatusCreated, j)
}

func (s *Server) handleReadLogRouting(w http.ResponseWriter, r *http.Request) {
	rt, ok := s.store.logRoutings.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No LogRouting matches the given query.")
		return
	}
	j, _ := s.logRoutingToJSON(rt, true)
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleUpdateLogRouting(w http.ResponseWriter, r *http.Request) {
	rt, ok := s.store.logRoutings.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No LogRouting matches the given query.")
		return
	}
	var req routingRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.PublisherCode != nil {
		pub, ok := s.store.publisher(*req.PublisherCode)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid publisher_code")
			return
		}
		rt.PublisherCode = pub.Code
	}
	if req.Variant != nil {
		rt.Variant = *req.Variant
	}
	if req.LogStorageID != nil {
		st, ok := s.store.findLogStorage(*req.LogStorageID)
		if !ok {
			writeError(w, http.StatusBadRequest, "log_storage not found")
			return
		}
		rt.StorageID = st.ID
	}
	if req.ResourceID != nil {
		rt.ResourceID = req.ResourceID
	}
	rt.UpdatedAt = time.Now()
	j, _ := s.logRoutingToJSON(rt, true)
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleDeleteLogRouting(w http.ResponseWriter, r *http.Request) {
	if !s.store.logRoutings.delete(r.PathValue("uid")) {
		writeError(w, http.StatusNotFound, "No LogRouting matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== Metrics routings =====

func (s *Server) metricsRoutingToJSON(rt *Routing, wrapped bool) (routingJSON, bool) {
	pub, ok := s.store.publisher(rt.PublisherCode)
	if !ok {
		return routingJSON{}, false
	}
	st, ok := s.store.findMetricsStorage(rt.StorageID)
	if !ok {
		return routingJSON{}, false
	}
	j := routingJSON{
		ID:             rt.ID,
		UID:            rt.UID,
		ResourceID:     rt.ResourceID,
		Publisher:      publisherToJSON(pub, false),
		Variant:        rt.Variant,
		MetricsStorage: ptrOf(s.metricsStorageToJSON(st, false)),
		CreatedAt:      formatTime(rt.CreatedAt),
		UpdatedAt:      formatTime(rt.UpdatedAt),
	}
	if wrapped {
		j.IsOk = boolPtr(true)
	}
	return j, true
}

func (s *Server) handleListMetricsRoutings(w http.ResponseWriter, r *http.Request) {
	out := []routingJSON{}
	for _, rt := range s.store.metricsRoutings.all() {
		if j, ok := s.metricsRoutingToJSON(rt, false); ok {
			out = append(out, j)
		}
	}
	writePage(w, out)
}

func (s *Server) handleCreateMetricsRouting(w http.ResponseWriter, r *http.Request) {
	var req routingRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.PublisherCode == nil || req.Variant == nil || req.MetricsStorageID == nil {
		writeError(w, http.StatusBadRequest, "publisher_code, variant and metrics_storage_id are required")
		return
	}
	pub, ok := s.store.publisher(*req.PublisherCode)
	if !ok || !pub.hasVariant(*req.Variant) {
		writeError(w, http.StatusBadRequest, "invalid publisher_code or variant")
		return
	}
	st, ok := s.store.findMetricsStorage(*req.MetricsStorageID)
	if !ok {
		writeError(w, http.StatusBadRequest, "metrics_storage not found")
		return
	}
	now := time.Now()
	rt := &Routing{
		ID:            s.store.nextInternalID(),
		UID:           newUUID(),
		ResourceID:    req.ResourceID,
		PublisherCode: pub.Code,
		Variant:       *req.Variant,
		StorageID:     st.ID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.store.metricsRoutings.set(rt.UID, rt)
	j, _ := s.metricsRoutingToJSON(rt, true)
	writeJSON(w, http.StatusCreated, j)
}

func (s *Server) handleReadMetricsRouting(w http.ResponseWriter, r *http.Request) {
	rt, ok := s.store.metricsRoutings.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No MetricsRouting matches the given query.")
		return
	}
	j, _ := s.metricsRoutingToJSON(rt, true)
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleUpdateMetricsRouting(w http.ResponseWriter, r *http.Request) {
	rt, ok := s.store.metricsRoutings.get(r.PathValue("uid"))
	if !ok {
		writeError(w, http.StatusNotFound, "No MetricsRouting matches the given query.")
		return
	}
	var req routingRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.PublisherCode != nil {
		pub, ok := s.store.publisher(*req.PublisherCode)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid publisher_code")
			return
		}
		rt.PublisherCode = pub.Code
	}
	if req.Variant != nil {
		rt.Variant = *req.Variant
	}
	if req.MetricsStorageID != nil {
		st, ok := s.store.findMetricsStorage(*req.MetricsStorageID)
		if !ok {
			writeError(w, http.StatusBadRequest, "metrics_storage not found")
			return
		}
		rt.StorageID = st.ID
	}
	if req.ResourceID != nil {
		rt.ResourceID = req.ResourceID
	}
	rt.UpdatedAt = time.Now()
	j, _ := s.metricsRoutingToJSON(rt, true)
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleDeleteMetricsRouting(w http.ResponseWriter, r *http.Request) {
	if !s.store.metricsRoutings.delete(r.PathValue("uid")) {
		writeError(w, http.StatusNotFound, "No MetricsRouting matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
