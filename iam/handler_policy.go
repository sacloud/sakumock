package iam

import (
	"net/http"
)

type iamRoleJSON struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Description             string `json:"description"`
	Category                string `json:"category"`
	LowestGrantableResource string `json:"lowest_grantable_resource"`
}

type idRoleJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type idPolicyBindingJSON struct {
	Role       policyRoleJSON        `json:"role"`
	Principals []policyPrincipalJSON `json:"principals"`
}

type idPolicyResponse struct {
	Bindings []idPolicyBindingJSON `json:"bindings"`
}

func (s *Server) handleListIAMRoles(w http.ResponseWriter, _ *http.Request) {
	items := make([]iamRoleJSON, 0, len(s.store.iamRoles))
	for _, r := range s.store.iamRoles {
		items = append(items, iamRoleJSON{
			ID:                      r.ID,
			Name:                    r.Name,
			Description:             r.Description,
			Category:                r.Category,
			LowestGrantableResource: r.LowestGrantableResource,
		})
	}
	writePage(w, items)
}

func (s *Server) handleReadIAMRole(w http.ResponseWriter, r *http.Request) {
	roleID := r.PathValue("iam_role_id")
	for _, role := range s.store.iamRoles {
		if role.ID == roleID {
			writeJSON(w, http.StatusOK, iamRoleJSON{
				ID:                      role.ID,
				Name:                    role.Name,
				Description:             role.Description,
				Category:                role.Category,
				LowestGrantableResource: role.LowestGrantableResource,
			})
			return
		}
	}
	writeError(w, http.StatusNotFound, "IAM role not found")
}

func (s *Server) handleListIDRoles(w http.ResponseWriter, _ *http.Request) {
	items := make([]idRoleJSON, 0, len(s.store.idRoles))
	for _, r := range s.store.idRoles {
		items = append(items, idRoleJSON{
			ID:          r.ID,
			Name:        r.Name,
			Description: r.Description,
		})
	}
	writePage(w, items)
}

func (s *Server) handleReadIDRole(w http.ResponseWriter, r *http.Request) {
	roleID := r.PathValue("id_role_id")
	for _, role := range s.store.idRoles {
		if role.ID == roleID {
			writeJSON(w, http.StatusOK, idRoleJSON{
				ID:          role.ID,
				Name:        role.Name,
				Description: role.Description,
			})
			return
		}
	}
	writeError(w, http.StatusNotFound, "ID role not found")
}

func (s *Server) handleReadOrgIAMPolicy(w http.ResponseWriter, _ *http.Request) {
	s.store.mu.RLock()
	bindings := s.store.orgIAMPolicy
	s.store.mu.RUnlock()
	if bindings == nil {
		bindings = []PolicyBinding{}
	}
	writeJSON(w, http.StatusOK, iamPolicyResponse{Bindings: bindingsToJSON(bindings)})
}

func (s *Server) handleUpdateOrgIAMPolicy(w http.ResponseWriter, r *http.Request) {
	var req iamPolicyResponse
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	bindings := bindingsFromJSON(req.Bindings)
	s.store.mu.Lock()
	s.store.orgIAMPolicy = bindings
	s.store.mu.Unlock()
	s.logger.Debug("organization IAM policy updated")
	writeJSON(w, http.StatusOK, iamPolicyResponse{Bindings: bindingsToJSON(bindings)})
}

func (s *Server) handleReadOrgIDPolicy(w http.ResponseWriter, _ *http.Request) {
	s.store.mu.RLock()
	bindings := s.store.orgIDPolicy
	s.store.mu.RUnlock()
	out := make([]idPolicyBindingJSON, 0, len(bindings))
	for _, b := range bindings {
		principals := make([]policyPrincipalJSON, 0, len(b.Principals))
		for _, p := range b.Principals {
			principals = append(principals, policyPrincipalJSON{Type: p.Type, ID: p.ID})
		}
		out = append(out, idPolicyBindingJSON{
			Role:       policyRoleJSON{Type: b.Role.Type, ID: b.Role.ID},
			Principals: principals,
		})
	}
	writeJSON(w, http.StatusOK, idPolicyResponse{Bindings: out})
}

func (s *Server) handleUpdateOrgIDPolicy(w http.ResponseWriter, r *http.Request) {
	var req idPolicyResponse
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	bindings := make([]PolicyBinding, 0, len(req.Bindings))
	for _, b := range req.Bindings {
		principals := make([]PolicyPrincipal, 0, len(b.Principals))
		for _, p := range b.Principals {
			principals = append(principals, PolicyPrincipal{Type: p.Type, ID: p.ID})
		}
		bindings = append(bindings, PolicyBinding{
			Role:       PolicyRole{Type: b.Role.Type, ID: b.Role.ID},
			Principals: principals,
		})
	}
	s.store.mu.Lock()
	s.store.orgIDPolicy = bindings
	s.store.mu.Unlock()
	s.logger.Debug("organization ID policy updated")
	out := make([]idPolicyBindingJSON, 0, len(bindings))
	for _, b := range bindings {
		principals := make([]policyPrincipalJSON, 0, len(b.Principals))
		for _, p := range b.Principals {
			principals = append(principals, policyPrincipalJSON{Type: p.Type, ID: p.ID})
		}
		out = append(out, idPolicyBindingJSON{
			Role:       policyRoleJSON{Type: b.Role.Type, ID: b.Role.ID},
			Principals: principals,
		})
	}
	writeJSON(w, http.StatusOK, idPolicyResponse{Bindings: out})
}
