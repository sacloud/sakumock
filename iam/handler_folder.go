package iam

import (
	"net/http"
	"time"
)

type folderJSON struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ParentID    *int   `json:"parent_id"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type createFolderRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ParentID    *int   `json:"parent_id,omitempty"`
}

type updateFolderRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type moveFoldersRequest struct {
	FolderIDs []int `json:"folder_ids"`
	ParentID  *int  `json:"parent_id"`
}

func folderToJSON(r *FolderRecord) folderJSON {
	return folderJSON{
		ID:          r.ID,
		Name:        r.Name,
		ParentID:    r.ParentID,
		Description: r.Description,
		CreatedAt:   formatTime(r.CreatedAt),
		UpdatedAt:   formatTime(r.UpdatedAt),
	}
}

func (s *Server) handleListFolders(w http.ResponseWriter, r *http.Request) {
	records := s.store.folders.all()
	items := make([]folderJSON, 0, len(records))
	for _, rec := range records {
		items = append(items, folderToJSON(rec))
	}
	writePage(w, items)
}

func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	var req createFolderRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	now := time.Now()
	rec := &FolderRecord{
		ID:          s.store.nextID(),
		Name:        req.Name,
		ParentID:    req.ParentID,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.store.folders.set(idKey(rec.ID), rec)
	s.logger.Debug("folder created", "id", rec.ID, "name", rec.Name)
	writeJSON(w, http.StatusCreated, folderToJSON(rec))
}

func (s *Server) handleReadFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("folder_id")
	rec, ok := s.store.folders.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	writeJSON(w, http.StatusOK, folderToJSON(rec))
}

func (s *Server) handleUpdateFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("folder_id")
	rec, ok := s.store.folders.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	var req updateFolderRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rec.Name = req.Name
	if req.Description != nil {
		rec.Description = *req.Description
	}
	rec.UpdatedAt = time.Now()
	s.store.folders.set(id, rec)
	s.logger.Debug("folder updated", "id", id)
	writeJSON(w, http.StatusOK, folderToJSON(rec))
}

func (s *Server) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("folder_id")
	if !s.store.folders.delete(id) {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	s.store.mu.Lock()
	delete(s.store.folderIAMPolicies, id)
	s.store.mu.Unlock()
	s.logger.Debug("folder deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMoveFolders(w http.ResponseWriter, r *http.Request) {
	var req moveFoldersRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, fid := range req.FolderIDs {
		rec, ok := s.store.folders.get(idKey(fid))
		if !ok {
			continue
		}
		rec.ParentID = req.ParentID
		rec.UpdatedAt = time.Now()
		s.store.folders.set(idKey(fid), rec)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReadFolderIAMPolicy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("folder_id")
	if _, ok := s.store.folders.get(id); !ok {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	s.store.mu.RLock()
	bindings := s.store.folderIAMPolicies[id]
	s.store.mu.RUnlock()
	if bindings == nil {
		bindings = []PolicyBinding{}
	}
	writeJSON(w, http.StatusOK, iamPolicyResponse{Bindings: bindingsToJSON(bindings)})
}

func (s *Server) handleUpdateFolderIAMPolicy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("folder_id")
	if _, ok := s.store.folders.get(id); !ok {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	var req iamPolicyResponse
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	bindings := bindingsFromJSON(req.Bindings)
	s.store.mu.Lock()
	s.store.folderIAMPolicies[id] = bindings
	s.store.mu.Unlock()
	s.logger.Debug("folder IAM policy updated", "folder_id", id)
	writeJSON(w, http.StatusOK, iamPolicyResponse{Bindings: bindingsToJSON(bindings)})
}
