package simplenotification

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

const routingClass = "saknoticerouting"

// Routing JSON types matching the OpenAPI spec.

type sourceJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type listSourcesResponse struct {
	Sources []sourceJSON `json:"Sources"`
}

// builtinSources is the mock's static notification-source catalog; routings
// reference these via Settings.SourceID. The real catalog is the set of
// SAKURA Cloud services that can emit notifications, which the mock cannot
// know, so it offers one stable entry.
var builtinSources = []sourceJSON{
	{ID: "1", Name: "sakumock"},
}

func (s *Server) handleListSources(w http.ResponseWriter, _ *http.Request) {
	core.WriteJSON(w, http.StatusOK, listSourcesResponse{Sources: builtinSources})
}

type reorderRequest struct {
	Orders []struct {
		RoutingID    string `json:"RoutingID"`
		PriorityRank int    `json:"PriorityRank"`
	} `json:"Orders"`
}

type reorderResponse struct {
	IsOk bool `json:"is_ok"`
}

// handleReorderRouting patches PriorityRank inside each referenced routing's
// verbatim Settings JSON. Schema-level constraints (required fields, rank
// range, ID pattern) are enforced by the generated body validator; the
// handler adds the referential and uniqueness checks.
func (s *Server) handleReorderRouting(w http.ResponseWriter, r *http.Request) {
	var req reorderRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	seenID := map[string]bool{}
	seenRank := map[int]bool{}
	for _, o := range req.Orders {
		if seenID[o.RoutingID] {
			writeError(w, http.StatusBadRequest, "duplicate RoutingID: "+o.RoutingID)
			return
		}
		if seenRank[o.PriorityRank] {
			writeError(w, http.StatusBadRequest, "PriorityRank values must be unique")
			return
		}
		seenID[o.RoutingID] = true
		seenRank[o.PriorityRank] = true
		it, ok := s.store.GetItem(o.RoutingID)
		if !ok || it.ProviderClass != routingClass {
			writeError(w, http.StatusNotFound, "対象が見つかりません。")
			return
		}
	}
	for _, o := range req.Orders {
		it, _ := s.store.GetItem(o.RoutingID)
		var settings map[string]any
		if err := json.Unmarshal(it.Settings, &settings); err != nil || settings == nil {
			settings = map[string]any{}
		}
		settings["PriorityRank"] = o.PriorityRank
		raw, err := json.Marshal(settings)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.store.UpdateItemSettings(o.RoutingID, raw)
	}
	s.logger.Debug("routings reordered", "count", len(req.Orders))
	core.WriteJSON(w, http.StatusAccepted, reorderResponse{IsOk: true})
}

// Status of a destination or group: the mock reports an item as valid unless
// its Settings carry Disabled: true.

type notificationStatusInfoJSON struct {
	IsValid    bool   `json:"IsValid"`
	ModifiedAt string `json:"ModifiedAt"`
}

type getItemStatusResponse struct {
	NotificationStatus notificationStatusInfoJSON `json:"NotificationStatus"`
}

func (s *Server) handleGetItemStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, ok := s.store.GetItem(id)
	if !ok {
		writeError(w, http.StatusNotFound, "対象が見つかりません。")
		return
	}
	var settings struct {
		Disabled bool `json:"Disabled"`
	}
	_ = json.Unmarshal(it.Settings, &settings)
	core.WriteJSON(w, http.StatusOK, getItemStatusResponse{
		NotificationStatus: notificationStatusInfoJSON{
			IsValid:    !settings.Disabled,
			ModifiedAt: it.ModifiedAt.Format(time.RFC3339),
		},
	})
}
