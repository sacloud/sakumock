package simplenotification

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/sacloud/sakumock/core"
)

// Notification-history JSON types matching the OpenAPI spec. Histories are
// derived on the fly from the accepted-message records (the same data behind
// /_sakumock/messages), so a send is immediately visible as a history entry.

type notificationMessageJSON struct {
	Body      string `json:"body"`
	Color     string `json:"color"`
	ColorCode string `json:"color_code"`
	IconURL   string `json:"icon_url"`
	ImageURL  string `json:"image_url"`
	Title     string `json:"title"`
}

type notificationStatusJSON struct {
	ID                    string `json:"id"`
	Status                int    `json:"status"`
	ErrorInfo             string `json:"error_info"`
	NotificationRequestID string `json:"notification_request_id"`
	GroupID               string `json:"group_id"`
	DestinationID         string `json:"destination_id"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

type notificationHistoryJSON struct {
	RequestID  string                   `json:"request_id"`
	SourceID   string                   `json:"source_id"`
	Statuses   []notificationStatusJSON `json:"statuses"`
	ReceivedAt string                   `json:"received_at"`
	Message    notificationMessageJSON  `json:"message"`
}

type listHistoriesResponse struct {
	NotificationHistories []notificationHistoryJSON `json:"NotificationHistories"`
}

type getHistoryResponse struct {
	NotificationHistory notificationHistoryJSON `json:"NotificationHistory"`
}

const (
	// historyLimit and historyWindow mirror the real API's documented
	// bounds: the latest 100 entries within the last 30 days.
	historyLimit  = 100
	historyWindow = 30 * 24 * time.Hour

	// statusSent is the NotificationStatus.status value for 送信済 (sent);
	// the mock accepts every message, so deliveries are always "sent".
	statusSent = 2
)

// parseMessagePayload maps the accepted Message string onto the spec's
// message shape. A message that is itself a JSON object with a "body" key
// (the real service's rich-message payload) keeps its own fields; anything
// else becomes the body of a default-styled message. The defaults match the
// spec's documented example ("default" / "#7d7d7d").
func parseMessagePayload(raw string) notificationMessageJSON {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &keys); err == nil {
		if _, hasBody := keys["body"]; hasBody {
			var parsed notificationMessageJSON
			if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
				if parsed.Color == "" {
					parsed.Color = "default"
				}
				if parsed.ColorCode == "" {
					parsed.ColorCode = "#7d7d7d"
				}
				return parsed
			}
		}
	}
	return notificationMessageJSON{
		Body:      raw,
		Color:     "default",
		ColorCode: "#7d7d7d",
	}
}

// groupDestinations extracts the Destinations list from a group item's
// verbatim Settings JSON; a missing group or unparsable settings yield nil.
// Entries not matching the spec's 12-digit ID pattern are dropped: group
// settings are stored permissively (oneOf), but the history response schema
// requires conformant destination IDs.
func (s *Server) groupDestinations(groupID string) []string {
	it, ok := s.store.GetItem(groupID)
	if !ok {
		return nil
	}
	var settings struct {
		Destinations []string `json:"Destinations"`
	}
	if err := json.Unmarshal(it.Settings, &settings); err != nil {
		return nil
	}
	out := settings.Destinations[:0]
	for _, dest := range settings.Destinations {
		if groupIDRe.MatchString(dest) {
			out = append(out, dest)
		}
	}
	return out
}

// historyOf renders one accepted message as a spec-shaped history entry.
// source_id is empty for API-initiated sends, matching the spec's example.
func (s *Server) historyOf(rec MessageRecord) notificationHistoryJSON {
	at := rec.CreatedAt.Format(time.RFC3339)
	statuses := []notificationStatusJSON{}
	for i, dest := range s.groupDestinations(rec.GroupID) {
		statuses = append(statuses, notificationStatusJSON{
			ID:                    rec.ID + "-" + strconv.Itoa(i+1),
			Status:                statusSent,
			ErrorInfo:             "",
			NotificationRequestID: rec.ID,
			GroupID:               rec.GroupID,
			DestinationID:         dest,
			CreatedAt:             at,
			UpdatedAt:             at,
		})
	}
	return notificationHistoryJSON{
		RequestID:  rec.ID,
		SourceID:   "",
		Statuses:   statuses,
		ReceivedAt: at,
		Message:    parseMessagePayload(rec.Message),
	}
}

func (s *Server) handleListHistories(w http.ResponseWriter, _ *http.Request) {
	records := s.store.List()
	cutoff := time.Now().Add(-historyWindow)
	out := []notificationHistoryJSON{}
	// Newest first, capped at the documented limit.
	for i := len(records) - 1; i >= 0 && len(out) < historyLimit; i-- {
		if records[i].CreatedAt.Before(cutoff) {
			continue
		}
		out = append(out, s.historyOf(records[i]))
	}
	core.WriteJSON(w, http.StatusOK, listHistoriesResponse{NotificationHistories: out})
}

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("request_id")
	for _, rec := range s.store.List() {
		if rec.ID == requestID {
			core.WriteJSON(w, http.StatusOK, getHistoryResponse{NotificationHistory: s.historyOf(rec)})
			return
		}
	}
	writeError(w, http.StatusNotFound, "対象が見つかりません。")
}
