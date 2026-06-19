package iam

import (
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

type groupJSON struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type createGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type groupMembershipItem struct {
	ID int `json:"id"`
}

type groupMembershipsResponse struct {
	CompatUsers []groupMembershipItem `json:"compat_users"`
}

type updateMembershipsRequest struct {
	CompatUsers []groupMembershipItem `json:"compat_users"`
}

func groupToJSON(r *GroupRecord) groupJSON {
	return groupJSON{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   core.FormatRFC3339(r.CreatedAt),
		UpdatedAt:   core.FormatRFC3339(r.UpdatedAt),
	}
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	records := s.store.groups.all()
	items := make([]groupJSON, 0, len(records))
	for _, rec := range records {
		items = append(items, groupToJSON(rec))
	}
	writePage(w, items)
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var req createGroupRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	now := time.Now()
	rec := &GroupRecord{
		ID:          s.store.nextID(),
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.store.groups.set(idKey(rec.ID), rec)
	s.logger.Debug("group created", "id", rec.ID, "name", rec.Name)
	core.WriteJSON(w, http.StatusCreated, groupToJSON(rec))
}

func (s *Server) handleReadGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("group_id")
	rec, ok := s.store.groups.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, groupToJSON(rec))
}

func (s *Server) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("group_id")
	rec, ok := s.store.groups.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	var req createGroupRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rec.Name = req.Name
	rec.Description = req.Description
	rec.UpdatedAt = time.Now()
	s.store.groups.set(id, rec)
	s.logger.Debug("group updated", "id", id)
	core.WriteJSON(w, http.StatusOK, groupToJSON(rec))
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("group_id")
	if !s.store.groups.delete(id) {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	s.logger.Debug("group deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReadMemberships(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("group_id")
	rec, ok := s.store.groups.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	items := make([]groupMembershipItem, 0, len(rec.Members))
	for _, uid := range rec.Members {
		items = append(items, groupMembershipItem{ID: uid})
	}
	core.WriteJSON(w, http.StatusOK, groupMembershipsResponse{CompatUsers: items})
}

func (s *Server) handleUpdateMemberships(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("group_id")
	rec, ok := s.store.groups.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}
	var req updateMembershipsRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	members := make([]int, 0, len(req.CompatUsers))
	for _, u := range req.CompatUsers {
		members = append(members, u.ID)
	}
	rec.Members = members
	rec.UpdatedAt = time.Now()
	s.store.groups.set(id, rec)
	s.logger.Debug("memberships updated", "group_id", id, "count", len(members))
	items := make([]groupMembershipItem, 0, len(members))
	for _, uid := range members {
		items = append(items, groupMembershipItem{ID: uid})
	}
	core.WriteJSON(w, http.StatusOK, groupMembershipsResponse{CompatUsers: items})
}
