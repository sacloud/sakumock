package iam

import (
	"net/http"
	"time"
)

type projectJSON struct {
	ID             int    `json:"id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Status         string `json:"status"`
	ParentFolderID *int   `json:"parent_folder_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type createProjectRequest struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	ParentFolderID *int   `json:"parent_folder_id,omitempty"`
}

type updateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type moveProjectsRequest struct {
	ProjectIDs     []int `json:"project_ids"`
	ParentFolderID *int  `json:"parent_folder_id"`
}

func projectToJSON(r *ProjectRecord) projectJSON {
	return projectJSON{
		ID:             r.ID,
		Code:           r.Code,
		Name:           r.Name,
		Description:    r.Description,
		Status:         r.Status,
		ParentFolderID: r.ParentFolderID,
		CreatedAt:      formatTime(r.CreatedAt),
		UpdatedAt:      formatTime(r.UpdatedAt),
	}
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	records := s.store.projects.all()
	items := make([]projectJSON, 0, len(records))
	for _, rec := range records {
		items = append(items, projectToJSON(rec))
	}
	writePage(w, items)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" || req.Code == "" {
		writeError(w, http.StatusBadRequest, "name and code are required")
		return
	}
	now := time.Now()
	rec := &ProjectRecord{
		ID:             s.store.nextID(),
		Code:           req.Code,
		Name:           req.Name,
		Description:    req.Description,
		Status:         "available",
		ParentFolderID: req.ParentFolderID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.store.projects.set(idKey(rec.ID), rec)
	s.logger.Debug("project created", "id", rec.ID, "name", rec.Name)
	writeJSON(w, http.StatusCreated, projectToJSON(rec))
}

func (s *Server) handleReadProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project_id")
	rec, ok := s.store.projects.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, projectToJSON(rec))
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project_id")
	rec, ok := s.store.projects.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var req updateProjectRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rec.Name = req.Name
	rec.Description = req.Description
	rec.UpdatedAt = time.Now()
	s.store.projects.set(id, rec)
	s.logger.Debug("project updated", "id", id)
	writeJSON(w, http.StatusOK, projectToJSON(rec))
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project_id")
	if !s.store.projects.delete(id) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	s.store.mu.Lock()
	delete(s.store.projectIAMPolicies, id)
	s.store.mu.Unlock()
	s.logger.Debug("project deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMoveProjects(w http.ResponseWriter, r *http.Request) {
	var req moveProjectsRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, pid := range req.ProjectIDs {
		rec, ok := s.store.projects.get(idKey(pid))
		if !ok {
			continue
		}
		rec.ParentFolderID = req.ParentFolderID
		rec.UpdatedAt = time.Now()
		s.store.projects.set(idKey(pid), rec)
	}
	w.WriteHeader(http.StatusNoContent)
}

type iamPolicyResponse struct {
	Bindings []policyBindingJSON `json:"bindings"`
}

type policyBindingJSON struct {
	Role       policyRoleJSON        `json:"role"`
	Principals []policyPrincipalJSON `json:"principals"`
}

type policyRoleJSON struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type policyPrincipalJSON struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

func bindingsToJSON(bindings []PolicyBinding) []policyBindingJSON {
	out := make([]policyBindingJSON, 0, len(bindings))
	for _, b := range bindings {
		principals := make([]policyPrincipalJSON, 0, len(b.Principals))
		for _, p := range b.Principals {
			principals = append(principals, policyPrincipalJSON{Type: p.Type, ID: p.ID})
		}
		out = append(out, policyBindingJSON{
			Role:       policyRoleJSON{Type: b.Role.Type, ID: b.Role.ID},
			Principals: principals,
		})
	}
	return out
}

func bindingsFromJSON(input []policyBindingJSON) []PolicyBinding {
	out := make([]PolicyBinding, 0, len(input))
	for _, b := range input {
		principals := make([]PolicyPrincipal, 0, len(b.Principals))
		for _, p := range b.Principals {
			principals = append(principals, PolicyPrincipal{Type: p.Type, ID: p.ID})
		}
		out = append(out, PolicyBinding{
			Role:       PolicyRole{Type: b.Role.Type, ID: b.Role.ID},
			Principals: principals,
		})
	}
	return out
}

func (s *Server) handleReadProjectIAMPolicy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project_id")
	if _, ok := s.store.projects.get(id); !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	s.store.mu.RLock()
	bindings := s.store.projectIAMPolicies[id]
	s.store.mu.RUnlock()
	if bindings == nil {
		bindings = []PolicyBinding{}
	}
	writeJSON(w, http.StatusOK, iamPolicyResponse{Bindings: bindingsToJSON(bindings)})
}

func (s *Server) handleUpdateProjectIAMPolicy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("project_id")
	if _, ok := s.store.projects.get(id); !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var req iamPolicyResponse
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	bindings := bindingsFromJSON(req.Bindings)
	s.store.mu.Lock()
	s.store.projectIAMPolicies[id] = bindings
	s.store.mu.Unlock()
	s.logger.Debug("project IAM policy updated", "project_id", id)
	writeJSON(w, http.StatusOK, iamPolicyResponse{Bindings: bindingsToJSON(bindings)})
}
