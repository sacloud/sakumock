package kms

import (
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

// JSON types matching the KMS OpenAPI spec.

type keyResponse struct {
	ID            string `json:"ID"`
	CreatedAt     string `json:"CreatedAt"`
	ModifiedAt    string `json:"ModifiedAt"`
	ServiceClass  string `json:"ServiceClass"`
	Name          string `json:"Name"`
	Description   string `json:"Description"`
	KeyOrigin     string `json:"KeyOrigin"`
	LatestVersion int    `json:"LatestVersion"`
	Status        string `json:"Status"`
	// DeletionScheduledAfter is null unless destruction is scheduled.
	DeletionScheduledAfter *string  `json:"DeletionScheduledAfter"`
	Tags                   []string `json:"Tags"`
}

// wrappedKey is both WrappedKey and WrappedCreateKeyResponse: the spec's
// CreateKeyResponse has the same shape as Key.
type wrappedKey struct {
	Key  keyResponse `json:"Key"`
	IsOK bool        `json:"is_ok"`
}

type paginatedKeyList struct {
	Count int           `json:"Count"`
	From  int           `json:"From"`
	Total int           `json:"Total"`
	Keys  []keyResponse `json:"Keys"`
	IsOK  bool          `json:"is_ok"`
}

type createKeyRequest struct {
	Key struct {
		Name        string   `json:"Name"`
		Description string   `json:"Description"`
		Tags        []string `json:"Tags"`
		PlainKey    string   `json:"PlainKey"`
	} `json:"Key"`
}

type updateKeyRequest struct {
	Key struct {
		Name        string   `json:"Name"`
		Description string   `json:"Description"`
		Tags        []string `json:"Tags"`
	} `json:"Key"`
}

type changeStatusRequest struct {
	Key struct {
		Status string `json:"Status"`
	} `json:"Key"`
}

type changeKeyStateResponse struct {
	Status string `json:"Status"`
}

type wrappedChangeKeyState struct {
	Key  changeKeyStateResponse `json:"Key"`
	IsOK bool                   `json:"is_ok"`
}

type scheduleDestructionRequest struct {
	Key struct {
		PendingDays *int `json:"PendingDays"`
	} `json:"Key"`
}

type keyScheduledDestructionResponse struct {
	Status                 string  `json:"Status"`
	DeletionScheduledAfter *string `json:"DeletionScheduledAfter"`
}

type wrappedKeyScheduledDestruction struct {
	Key  keyScheduledDestructionResponse `json:"Key"`
	IsOK bool                            `json:"is_ok"`
}

type keyCipherResponse struct {
	Cipher string `json:"Cipher"`
}

type wrappedKeyCipher struct {
	Key  keyCipherResponse `json:"Key"`
	IsOK bool              `json:"is_ok"`
}

type keyPlainResponse struct {
	Plain string `json:"Plain"`
}

type wrappedKeyPlain struct {
	Key  keyPlainResponse `json:"Key"`
	IsOK bool             `json:"is_ok"`
}

// defaultPendingDays is the spec default of ScheduleDestructionKeyRequest.PendingDays.
const defaultPendingDays = 7

type encryptRequest struct {
	Key struct {
		Plain string `json:"Plain"`
		Algo  string `json:"Algo"`
	} `json:"Key"`
}

type decryptRequest struct {
	Key struct {
		Cipher string `json:"Cipher"`
	} `json:"Key"`
}

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, r := range s.routeTable() {
		mux.HandleFunc(r.Method+" "+r.Path, r.Handler)
	}
	return mux
}

// ServeHTTP dispatches the request to the matching route and logs the result.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	rw := core.NewResponseRecorder(w)
	s.mux.ServeHTTP(rw, r)
	s.logger.Info("request", core.RequestLogArgs(r, rw)...)
}

func keyRecordToResponse(k KeyRecord) keyResponse {
	return keyResponse{
		ID:                     k.ID,
		CreatedAt:              core.FormatRFC3339Nano(k.CreatedAt),
		ModifiedAt:             core.FormatRFC3339Nano(k.ModifiedAt),
		ServiceClass:           "cloud/kms/key",
		Name:                   k.Name,
		Description:            k.Description,
		KeyOrigin:              k.KeyOrigin,
		LatestVersion:          k.LatestVersion,
		Status:                 k.Status,
		DeletionScheduledAfter: formatOptionalTime(k.DeletionScheduledAfter),
		Tags:                   k.Tags,
	}
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := core.FormatRFC3339Nano(*t)
	return &v
}

// handleListKeys returns keys oldest first. From is the 0-based index of the
// first key returned (default 0) and Count the page size (0 or absent: all).
func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	p, err := core.ParsePage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	keys := s.store.List()
	page := core.Paginate(keys, p)
	items := make([]keyResponse, len(page))
	for i, k := range page {
		items[i] = keyRecordToResponse(k)
	}
	s.logger.Debug("keys listed", "count", len(items), "from", p.From, "total", len(keys))
	core.WriteJSON(w, http.StatusOK, paginatedKeyList{
		Count: len(items),
		From:  p.From,
		Total: len(keys),
		Keys:  items,
		IsOK:  true,
	})
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	var req createKeyRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	origin := "generated"
	if req.Key.PlainKey != "" {
		origin = "imported"
	}
	k, err := s.store.Create(req.Key.Name, req.Key.Description, origin, req.Key.Tags)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Debug("key created", "id", k.ID, "name", k.Name)
	core.WriteJSON(w, http.StatusCreated, wrappedKey{Key: keyRecordToResponse(k), IsOK: true})
}

func (s *Server) handleReadKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	k, err := s.store.Read(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedKey{Key: keyRecordToResponse(k), IsOK: true})
}

func (s *Server) handleUpdateKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req updateKeyRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	k, err := s.store.Update(id, req.Key.Name, req.Key.Description, req.Key.Tags)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedKey{Key: keyRecordToResponse(k), IsOK: true})
}

func (s *Server) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	if err := s.store.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	k, err := s.store.Rotate(id)
	if err != nil {
		if k.ID == "" {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusForbidden, err.Error())
		}
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedKey{Key: keyRecordToResponse(k), IsOK: true})
}

func (s *Server) handleChangeStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req changeStatusRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.ChangeStatus(id, req.Key.Status); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedChangeKeyState{
		Key:  changeKeyStateResponse{Status: req.Key.Status},
		IsOK: true,
	})
}

func (s *Server) handleScheduleDestruction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req scheduleDestructionRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	days := defaultPendingDays
	if req.Key.PendingDays != nil {
		days = *req.Key.PendingDays
	}
	k, err := s.store.ScheduleDestruction(id, time.Now().AddDate(0, 0, days))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedKeyScheduledDestruction{
		Key: keyScheduledDestructionResponse{
			Status:                 k.Status,
			DeletionScheduledAfter: formatOptionalTime(k.DeletionScheduledAfter),
		},
		IsOK: true,
	})
}

func (s *Server) handleEncrypt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req encryptRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.store.Read(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	ciphertext, err := s.store.Encrypt(id, []byte(req.Key.Plain))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedKeyCipher{
		Key:  keyCipherResponse{Cipher: ciphertext},
		IsOK: true,
	})
}

func (s *Server) handleDecrypt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req decryptRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.store.Read(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	plaintext, err := s.store.Decrypt(id, req.Key.Cipher)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedKeyPlain{
		Key:  keyPlainResponse{Plain: string(plaintext)},
		IsOK: true,
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	core.WriteJSON(w, status, map[string]string{"error": msg})
}
