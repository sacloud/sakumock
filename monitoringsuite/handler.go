package monitoringsuite

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sacloud/sakumock/core"
)

// buildMux registers every route from the single-source-of-truth route table.
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

// errorResponse mirrors the monitoring-suite error envelope:
// {"is_ok": false, "status": <code>, "error_code": <text>, "error_msg": <msg>}.
type errorResponse struct {
	IsOk      bool   `json:"is_ok"`
	Status    int    `json:"status"`
	ErrorCode string `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	core.WriteJSON(w, status, errorResponse{
		IsOk:      false,
		Status:    status,
		ErrorCode: http.StatusText(status),
		ErrorMsg:  msg,
	})
}

// paginated is the DRF-style list envelope shared by every collection.
type paginated[T any] struct {
	Count   int  `json:"count"`
	From    int  `json:"from"`
	Total   int  `json:"total"`
	IsOk    bool `json:"is_ok"`
	Results []T  `json:"results"`
}

func writePage[T any](w http.ResponseWriter, items []T) {
	if items == nil {
		items = []T{}
	}
	core.WriteJSON(w, http.StatusOK, paginated[T]{
		Count:   len(items),
		From:    0,
		Total:   len(items),
		IsOk:    true,
		Results: items,
	})
}

func boolPtr(b bool) *bool { return &b }

// idKey renders a numeric ID as the string key used in the in-memory tables.
func idKey(id int64) string { return strconv.FormatInt(id, 10) }

// --- Project (alert / dashboard) JSON ---

type projectJSON struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Icon        any      `json:"icon"`
	AccountID   string   `json:"account_id"`
	ResourceID  int64    `json:"resource_id"`
	CreatedAt   string   `json:"created_at"`
	IsOk        *bool    `json:"is_ok,omitempty"`
}

func projectToJSON(p *Project, wrapped bool) projectJSON {
	j := projectJSON{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Tags:        p.Tags,
		Icon:        nil,
		AccountID:   p.AccountID,
		ResourceID:  p.ResourceID,
		CreatedAt:   core.FormatRFC3339Nano(p.CreatedAt),
	}
	if wrapped {
		j.IsOk = boolPtr(true)
	}
	return j
}

type projectCreateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type projectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// --- Project handlers, shared by the alert and dashboard collections ---

func (s *Server) listProjects(w http.ResponseWriter, tbl *table[Project]) {
	items := tbl.all()
	out := make([]projectJSON, len(items))
	for i, p := range items {
		out[i] = projectToJSON(p, false)
	}
	writePage(w, out)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request, tbl *table[Project]) {
	var req projectCreateRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	now := time.Now()
	rid := s.store.nextResourceID()
	p := &Project{
		ID:          rid,
		ResourceID:  rid,
		Name:        req.Name,
		Description: derefString(req.Description),
		Tags:        []string{},
		AccountID:   dummyAccountID,
		CreatedAt:   now,
	}
	tbl.set(idKey(rid), p)
	core.WriteJSON(w, http.StatusCreated, projectToJSON(p, false))
}

func (s *Server) readProject(w http.ResponseWriter, r *http.Request, tbl *table[Project]) {
	p, ok := tbl.get(r.PathValue("resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No project matches the given query.")
		return
	}
	core.WriteJSON(w, http.StatusOK, projectToJSON(p, true))
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request, tbl *table[Project]) {
	p, ok := tbl.get(r.PathValue("resource_id"))
	if !ok {
		writeError(w, http.StatusNotFound, "No project matches the given query.")
		return
	}
	var req projectRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	core.WriteJSON(w, http.StatusOK, projectToJSON(p, true))
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request, tbl *table[Project]) {
	if !tbl.delete(r.PathValue("resource_id")) {
		writeError(w, http.StatusNotFound, "No project matches the given query.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Alert project handlers.
func (s *Server) handleListAlertProjects(w http.ResponseWriter, r *http.Request) {
	s.listProjects(w, s.store.alertProjects)
}
func (s *Server) handleCreateAlertProject(w http.ResponseWriter, r *http.Request) {
	s.createProject(w, r, s.store.alertProjects)
}
func (s *Server) handleReadAlertProject(w http.ResponseWriter, r *http.Request) {
	s.readProject(w, r, s.store.alertProjects)
}
func (s *Server) handleUpdateAlertProject(w http.ResponseWriter, r *http.Request) {
	s.updateProject(w, r, s.store.alertProjects)
}
func (s *Server) handleDeleteAlertProject(w http.ResponseWriter, r *http.Request) {
	s.deleteProject(w, r, s.store.alertProjects)
}

// Dashboard project handlers.
func (s *Server) handleListDashboardProjects(w http.ResponseWriter, r *http.Request) {
	s.listProjects(w, s.store.dashboardProjects)
}
func (s *Server) handleCreateDashboardProject(w http.ResponseWriter, r *http.Request) {
	s.createProject(w, r, s.store.dashboardProjects)
}
func (s *Server) handleReadDashboardProject(w http.ResponseWriter, r *http.Request) {
	s.readProject(w, r, s.store.dashboardProjects)
}
func (s *Server) handleUpdateDashboardProject(w http.ResponseWriter, r *http.Request) {
	s.updateProject(w, r, s.store.dashboardProjects)
}
func (s *Server) handleDeleteDashboardProject(w http.ResponseWriter, r *http.Request) {
	s.deleteProject(w, r, s.store.dashboardProjects)
}
