package simplenotification

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// CommonServiceItem control-plane JSON types. Settings and Icon are passed
// through verbatim (json.RawMessage), so the mock does not model the polymorphic
// per-class settings of destinations, groups, and routings.

var validProviderClasses = map[string]bool{
	"saknoticedestination": true,
	"saknoticegroup":       true,
	"saknoticerouting":     true,
}

type csiProvider struct {
	Class        string `json:"Class"`
	Name         string `json:"Name,omitempty"`
	ServiceClass string `json:"ServiceClass,omitempty"`
}

type csiResponse struct {
	ID           string          `json:"ID"`
	Name         string          `json:"Name"`
	Description  string          `json:"Description"`
	Settings     json.RawMessage `json:"Settings"`
	Provider     csiProvider     `json:"Provider"`
	ServiceClass string          `json:"ServiceClass"`
	Icon         json.RawMessage `json:"Icon"`
	Tags         []string        `json:"Tags"`
	CreatedAt    string          `json:"CreatedAt"`
	ModifiedAt   string          `json:"ModifiedAt"`
}

type csiCreateRequest struct {
	CommonServiceItem struct {
		Name         string          `json:"Name"`
		Description  string          `json:"Description"`
		Settings     json.RawMessage `json:"Settings"`
		Provider     csiProvider     `json:"Provider"`
		ServiceClass string          `json:"ServiceClass"`
		Icon         json.RawMessage `json:"Icon"`
		Tags         []string        `json:"Tags"`
	} `json:"CommonServiceItem"`
}

type csiUpdateRequest struct {
	CommonServiceItem struct {
		Name        string          `json:"Name"`
		Description string          `json:"Description"`
		Settings    json.RawMessage `json:"Settings"`
		Icon        json.RawMessage `json:"Icon"`
		Tags        []string        `json:"Tags"`
	} `json:"CommonServiceItem"`
}

func toCSI(it ServiceItem) csiResponse {
	tags := it.Tags
	if tags == nil {
		tags = []string{}
	}
	return csiResponse{
		ID:           it.ID,
		Name:         it.Name,
		Description:  it.Description,
		Settings:     it.Settings,
		Provider:     csiProvider{Class: it.ProviderClass},
		ServiceClass: it.ServiceClass,
		Icon:         it.Icon,
		Tags:         tags,
		CreatedAt:    it.CreatedAt.Format(time.RFC3339),
		ModifiedAt:   it.ModifiedAt.Format(time.RFC3339),
	}
}

// providerClassFilter extracts Provider.Class from the SDK's JSON query string,
// which has the shape {"Filter":{"Provider.Class":"saknoticegroup"}}. Returns
// "" when no class filter is present.
func providerClassFilter(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	candidates := []string{rawQuery}
	if dec, err := url.QueryUnescape(rawQuery); err == nil && dec != rawQuery {
		candidates = append(candidates, dec)
	}
	for _, c := range candidates {
		var q struct {
			Filter map[string]string `json:"Filter"`
		}
		if json.Unmarshal([]byte(c), &q) == nil {
			return q.Filter["Provider.Class"]
		}
	}
	return ""
}

func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	var req csiCreateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	csi := req.CommonServiceItem
	if csi.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if !validProviderClasses[csi.Provider.Class] {
		writeError(w, http.StatusBadRequest, "Provider.Class must be one of saknoticedestination, saknoticegroup, saknoticerouting")
		return
	}
	it := s.store.CreateItem(ServiceItem{
		Name:          csi.Name,
		Description:   csi.Description,
		Tags:          csi.Tags,
		ProviderClass: csi.Provider.Class,
		ServiceClass:  csi.ServiceClass,
		Settings:      csi.Settings,
		Icon:          csi.Icon,
	})
	slog.Debug("service item created", "id", it.ID, "class", it.ProviderClass)
	writeJSON(w, http.StatusCreated, map[string]any{"CommonServiceItem": toCSI(it)})
}

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request) {
	class := providerClassFilter(r.URL.RawQuery)
	items := s.store.ListItems(class)
	out := make([]csiResponse, len(items))
	for i, it := range items {
		out[i] = toCSI(it)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"From":               0,
		"Count":              len(out),
		"Total":              len(out),
		"CommonServiceItems": out,
	})
}

func (s *Server) handleGetItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, ok := s.store.GetItem(id)
	if !ok {
		writeError(w, http.StatusNotFound, "対象が見つかりません。")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"CommonServiceItem": toCSI(it)})
}

func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req csiUpdateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	csi := req.CommonServiceItem
	it, ok := s.store.UpdateItem(id, csi.Name, csi.Description, csi.Tags, csi.Settings, csi.Icon)
	if !ok {
		writeError(w, http.StatusNotFound, "対象が見つかりません。")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"CommonServiceItem": toCSI(it)})
}

func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, ok := s.store.DeleteItem(id)
	if !ok {
		writeError(w, http.StatusNotFound, "対象が見つかりません。")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"CommonServiceItem": toCSI(it)})
}
