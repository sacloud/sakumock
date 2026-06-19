package eventbus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sacloud/sakumock/core"
)

// CommonServiceItem control-plane JSON types. Settings and Icon are passed
// through verbatim (json.RawMessage); only the fields the mock validates
// (per-class required settings) are inspected.

const (
	classProcessConfiguration = "eventbusprocessconfiguration"
	classSchedule             = "eventbusschedule"
	classTrigger              = "eventbustrigger"
)

var validProviderClasses = map[string]bool{
	classProcessConfiguration: true,
	classSchedule:             true,
	classTrigger:              true,
}

var validDestinations = map[string]bool{
	"simplenotification": true,
	"simplemq":           true,
	"autoscale":          true,
}

// validRecurringUnits is the RecurringUnit enum from the OpenAPI ScheduleSettings.
var validRecurringUnits = map[string]bool{
	"min":  true,
	"hour": true,
	"day":  true,
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
	SettingsHash string          `json:"SettingsHash"`
	Provider     csiProvider     `json:"Provider"`
	ServiceClass string          `json:"ServiceClass"`
	Availability string          `json:"Availability"`
	Icon         json.RawMessage `json:"Icon"`
	Tags         []string        `json:"Tags"`
	Status       *csiStatus      `json:"Status,omitempty"`
	CreatedAt    string          `json:"CreatedAt"`
	ModifiedAt   string          `json:"ModifiedAt"`
}

// csiStatus is the Status block the API returns once a schedule or trigger has
// fired. It is omitted entirely until the data plane records an outcome.
type csiStatus struct {
	Success   bool   `json:"Success"`
	Message   string `json:"Message"`
	UpdatedAt string `json:"UpdatedAt"`
}

type csiCreateRequest struct {
	CommonServiceItem struct {
		Name        string          `json:"Name"`
		Description string          `json:"Description"`
		Settings    json.RawMessage `json:"Settings"`
		Provider    csiProvider     `json:"Provider"`
		Icon        json.RawMessage `json:"Icon"`
		Tags        []string        `json:"Tags"`
	} `json:"CommonServiceItem"`
}

type csiUpdateRequest struct {
	CommonServiceItem struct {
		Name        string          `json:"Name"`
		Description string          `json:"Description"`
		Settings    json.RawMessage `json:"Settings"`
		Provider    csiProvider     `json:"Provider"`
		Icon        json.RawMessage `json:"Icon"`
		Tags        []string        `json:"Tags"`
	} `json:"CommonServiceItem"`
}

type setSecretRequest struct {
	Secret json.RawMessage `json:"Secret"`
}

func toCSI(it ServiceItem) csiResponse {
	tags := it.Tags
	if tags == nil {
		tags = []string{}
	}
	var status *csiStatus
	if it.Status != nil {
		status = &csiStatus{
			Success:   it.Status.Success,
			Message:   it.Status.Message,
			UpdatedAt: it.Status.UpdatedAt.Format(time.RFC3339),
		}
	}
	return csiResponse{
		ID:           it.ID,
		Name:         it.Name,
		Description:  it.Description,
		Settings:     it.Settings,
		SettingsHash: settingsHash(it.Settings),
		Provider:     csiProvider{Class: it.ProviderClass},
		ServiceClass: it.ServiceClass,
		Availability: "available",
		Icon:         it.Icon,
		Tags:         tags,
		Status:       status,
		CreatedAt:    it.CreatedAt.Format(time.RFC3339),
		ModifiedAt:   it.ModifiedAt.Format(time.RFC3339),
	}
}

// settingsHash derives the opaque SettingsHash the API includes in every
// CommonServiceItem response. The real value is a server-side hash of the
// stored settings; the mock computes a deterministic stand-in so the field is
// present and changes when the settings change.
func settingsHash(settings json.RawMessage) string {
	sum := sha256.Sum256(settings)
	return hex.EncodeToString(sum[:16])
}

// providerClassFilter extracts Provider.Class from the SDK's JSON query string,
// which has the shape {"Filter":{"Provider.Class":"eventbusschedule"}}. Returns
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

// validateSettings checks the per-class required settings fields. It returns a
// message suitable for a 400 response, or "" when the settings are acceptable.
// pcExists reports whether a process configuration with the given ID exists,
// so schedules and triggers cannot reference a missing one.
func (s *Server) validateSettings(class string, settings json.RawMessage) string {
	if len(settings) == 0 {
		return "Settings is required"
	}
	switch class {
	case classProcessConfiguration:
		var st struct {
			Destination string  `json:"Destination"`
			Parameters  *string `json:"Parameters"`
		}
		if err := json.Unmarshal(settings, &st); err != nil {
			return "invalid Settings: " + err.Error()
		}
		if !validDestinations[st.Destination] {
			return "Settings.Destination must be one of simplenotification, simplemq, autoscale"
		}
		if st.Parameters == nil {
			return "Settings.Parameters is required"
		}
	case classSchedule:
		var st struct {
			ProcessConfigurationID string          `json:"ProcessConfigurationID"`
			StartsAt               json.RawMessage `json:"StartsAt"`
			Crontab                string          `json:"Crontab"`
			RecurringStep          int             `json:"RecurringStep"`
			RecurringUnit          string          `json:"RecurringUnit"`
		}
		if err := json.Unmarshal(settings, &st); err != nil {
			return "invalid Settings: " + err.Error()
		}
		if st.ProcessConfigurationID == "" {
			return "Settings.ProcessConfigurationID is required"
		}
		if len(st.StartsAt) == 0 || string(st.StartsAt) == "null" {
			return "Settings.StartsAt is required"
		}
		// The OpenAPI marks only ProcessConfigurationID and StartsAt as required,
		// but a schedule's type is exactly one of Crontab or recurring
		// (RecurringStep + RecurringUnit): the control panel presents them as a
		// mutually exclusive choice. This rule is not expressible in the spec, so
		// the mock enforces it (both-or-neither) to match the real API.
		hasCron := st.Crontab != ""
		hasRecurring := st.RecurringStep > 0 && st.RecurringUnit != ""
		if hasCron == hasRecurring {
			return "Settings must specify exactly one of Crontab or RecurringStep with RecurringUnit"
		}
		if hasRecurring && !validRecurringUnits[st.RecurringUnit] {
			return "Settings.RecurringUnit must be one of min, hour, day"
		}
		if hasCron {
			if _, err := ParseCrontab(st.Crontab); err != nil {
				return "invalid Settings.Crontab: " + err.Error()
			}
		}
		if msg := s.checkProcessConfiguration(st.ProcessConfigurationID); msg != "" {
			return msg
		}
	case classTrigger:
		var st struct {
			Source                 string `json:"Source"`
			ProcessConfigurationID string `json:"ProcessConfigurationID"`
		}
		if err := json.Unmarshal(settings, &st); err != nil {
			return "invalid Settings: " + err.Error()
		}
		if st.Source == "" {
			return "Settings.Source is required"
		}
		if st.ProcessConfigurationID == "" {
			return "Settings.ProcessConfigurationID is required"
		}
		if msg := s.checkProcessConfiguration(st.ProcessConfigurationID); msg != "" {
			return msg
		}
	}
	return ""
}

// checkProcessConfiguration verifies the referenced process configuration
// exists and has the right provider class.
func (s *Server) checkProcessConfiguration(id string) string {
	it, ok := s.store.GetItem(id)
	if !ok || it.ProviderClass != classProcessConfiguration {
		return fmt.Sprintf("Settings.ProcessConfigurationID %q does not reference an existing process configuration", id)
	}
	return ""
}

// normalizeScheduleSettings rewrites a numeric StartsAt to its string form.
// The API accepts StartsAt as an integer (epoch milliseconds) on requests but
// returns it as a string, and the mock stores what it will respond with.
func normalizeScheduleSettings(settings json.RawMessage) json.RawMessage {
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(settings))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return settings
	}
	n, ok := m["StartsAt"].(json.Number)
	if !ok {
		return settings
	}
	m["StartsAt"] = n.String()
	out, err := json.Marshal(m)
	if err != nil {
		return settings
	}
	return out
}

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

func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	var req csiCreateRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	csi := req.CommonServiceItem
	if csi.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if !validProviderClasses[csi.Provider.Class] {
		writeError(w, http.StatusBadRequest, "Provider.Class must be one of eventbusprocessconfiguration, eventbusschedule, eventbustrigger")
		return
	}
	if msg := s.validateSettings(csi.Provider.Class, csi.Settings); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	settings := csi.Settings
	if csi.Provider.Class == classSchedule {
		settings = normalizeScheduleSettings(settings)
	}
	it := s.store.CreateItem(ServiceItem{
		Name:          csi.Name,
		Description:   csi.Description,
		Tags:          csi.Tags,
		ProviderClass: csi.Provider.Class,
		Settings:      settings,
		Icon:          csi.Icon,
	})
	s.logger.Debug("service item created", "id", it.ID, "class", it.ProviderClass)
	core.WriteJSON(w, http.StatusCreated, map[string]any{
		"CommonServiceItem": toCSI(it),
		"Success":           true,
		"is_ok":             true,
	})
}

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request) {
	class := providerClassFilter(r.URL.RawQuery)
	items := s.store.ListItems(class)
	out := make([]csiResponse, len(items))
	for i, it := range items {
		out[i] = toCSI(it)
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"From":               0,
		"Count":              len(out),
		"Total":              len(out),
		"CommonServiceItems": out,
		"is_ok":              true,
	})
}

func (s *Server) handleGetItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, ok := s.store.GetItem(id)
	if !ok {
		writeError(w, http.StatusNotFound, "対象が見つかりません。")
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"CommonServiceItem": toCSI(it),
		"is_ok":             true,
	})
}

func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req csiUpdateRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	csi := req.CommonServiceItem
	current, ok := s.store.GetItem(id)
	if !ok {
		writeError(w, http.StatusNotFound, "対象が見つかりません。")
		return
	}
	if csi.Provider.Class != "" && csi.Provider.Class != current.ProviderClass {
		writeError(w, http.StatusBadRequest, "Provider.Class does not match the resource")
		return
	}
	settings := csi.Settings
	if settings != nil {
		if msg := s.validateSettings(current.ProviderClass, settings); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		if current.ProviderClass == classSchedule {
			settings = normalizeScheduleSettings(settings)
		}
	}
	it, ok := s.store.UpdateItem(id, csi.Name, csi.Description, csi.Tags, settings, csi.Icon)
	if !ok {
		writeError(w, http.StatusNotFound, "対象が見つかりません。")
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"CommonServiceItem": toCSI(it),
		"Success":           true,
		"is_ok":             true,
	})
}

func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, ok := s.store.DeleteItem(id)
	if !ok {
		writeError(w, http.StatusNotFound, "対象が見つかりません。")
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"CommonServiceItem": toCSI(it),
		"Success":           true,
		"is_ok":             true,
	})
}

func (s *Server) handleSetSecret(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, ok := s.store.GetItem(id)
	if !ok {
		writeError(w, http.StatusNotFound, "対象が見つかりません。")
		return
	}
	if it.ProviderClass != classProcessConfiguration {
		writeError(w, http.StatusBadRequest, "the resource is not a process configuration")
		return
	}
	var req setSecretRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if msg := validateSecret(req.Secret); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if _, ok := s.store.SetSecret(id, req.Secret); !ok {
		writeError(w, http.StatusNotFound, "対象が見つかりません。")
		return
	}
	s.logger.Debug("process configuration secret set", "id", id)
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"process": map[string]any{"result": "ok"},
		"is_ok":   true,
	})
}

// validateSecret checks the secret matches one of the spec's shapes:
// SacloudAPISecret (AccessToken + AccessTokenSecret) or SimpleMQSecret (APIKey).
func validateSecret(secret json.RawMessage) string {
	if len(secret) == 0 || string(secret) == "null" {
		return "Secret is required"
	}
	var sec struct {
		AccessToken       string `json:"AccessToken"`
		AccessTokenSecret string `json:"AccessTokenSecret"`
		APIKey            string `json:"APIKey"`
	}
	if err := json.Unmarshal(secret, &sec); err != nil {
		return "invalid Secret: " + err.Error()
	}
	if sec.APIKey == "" && (sec.AccessToken == "" || sec.AccessTokenSecret == "") {
		return "Secret must contain APIKey, or AccessToken and AccessTokenSecret"
	}
	return ""
}

func writeError(w http.ResponseWriter, status int, msg string) {
	core.WriteStandardError(w, status, "", msg)
}
