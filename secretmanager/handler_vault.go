package secretmanager

import (
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

// JSON request/response types for the vault control plane, matching the
// SecretManager OpenAPI spec (the SDK's CreateVault/Vault/PaginatedVaultList).
// Timestamps are RFC3339 strings (the SDK's DateTime is a string type).

type vaultJSON struct {
	ID          string   `json:"ID"`
	CreatedAt   string   `json:"CreatedAt"`
	ModifiedAt  string   `json:"ModifiedAt"`
	Name        string   `json:"Name"`
	Description string   `json:"Description"`
	KmsKeyID    string   `json:"KmsKeyID"`
	Tags        []string `json:"Tags"`
}

type wrappedVault struct {
	Vault vaultJSON `json:"Vault"`
}

type paginatedVaultList struct {
	Count  int         `json:"Count"`
	From   int         `json:"From"`
	Total  int         `json:"Total"`
	Vaults []vaultJSON `json:"Vaults"`
}

type vaultRequestBody struct {
	Name        string   `json:"Name"`
	Description string   `json:"Description"`
	KmsKeyID    string   `json:"KmsKeyID"`
	Tags        []string `json:"Tags"`
}

type wrappedVaultRequest struct {
	Vault vaultRequestBody `json:"Vault"`
}

func toVaultJSON(v *Vault) vaultJSON {
	tags := v.Tags
	if tags == nil {
		tags = []string{}
	}
	return vaultJSON{
		ID:          v.ID,
		CreatedAt:   v.CreatedAt.Format(time.RFC3339),
		ModifiedAt:  v.ModifiedAt.Format(time.RFC3339),
		Name:        v.Name,
		Description: v.Description,
		KmsKeyID:    v.KmsKeyID,
		Tags:        tags,
	}
}

func (s *Server) handleListVaults(w http.ResponseWriter, r *http.Request) {
	vaults := s.store.ListVaults()
	items := make([]vaultJSON, len(vaults))
	for i, v := range vaults {
		items[i] = toVaultJSON(v)
	}
	core.WriteJSON(w, http.StatusOK, paginatedVaultList{
		Count:  len(items),
		From:   0,
		Total:  len(items),
		Vaults: items,
	})
}

func (s *Server) handleCreateVault(w http.ResponseWriter, r *http.Request) {
	var req wrappedVaultRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Vault.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.Vault.KmsKeyID == "" {
		writeError(w, http.StatusBadRequest, "KmsKeyID is required")
		return
	}
	v := s.store.CreateVault(req.Vault.Name, req.Vault.KmsKeyID, req.Vault.Description, req.Vault.Tags)
	s.logger.Debug("vault created", "vault_id", v.ID, "name", v.Name)
	core.WriteJSON(w, http.StatusCreated, wrappedVault{Vault: toVaultJSON(v)})
}

func (s *Server) handleGetVault(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("vault_resource_id")
	v, ok := s.store.GetVault(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vault not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedVault{Vault: toVaultJSON(v)})
}

func (s *Server) handleUpdateVault(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("vault_resource_id")
	var req wrappedVaultRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	v, ok := s.store.UpdateVault(id, req.Vault.Name, req.Vault.Description, req.Vault.Tags)
	if !ok {
		writeError(w, http.StatusNotFound, "vault not found")
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedVault{Vault: toVaultJSON(v)})
}

func (s *Server) handleDeleteVault(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("vault_resource_id")
	if !s.store.DeleteVault(id) {
		writeError(w, http.StatusNotFound, "vault not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
