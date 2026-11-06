package securitycontrol

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sacloud/sakumock/core"
)

// JSON types matching the Security Control OpenAPI spec.

type activationCreateRequest struct {
	ServicePrincipalID string `json:"servicePrincipalId"`
}

type activationUpdateRequest struct {
	ServicePrincipalID string `json:"servicePrincipalId"`
	IsActive           bool   `json:"isActive"`
}

type activationJSON struct {
	ServicePrincipalID   string `json:"servicePrincipalId"`
	IsActive             bool   `json:"isActive"`
	AutomatedActionLimit int    `json:"automatedActionLimit"`
}

type automatedActionRequest struct {
	Name               string          `json:"name"`
	Description        *string         `json:"description"`
	Action             json.RawMessage `json:"action"`
	ExecutionCondition string          `json:"executionCondition"`
	IsEnabled          bool            `json:"isEnabled"`
}

type automatedActionJSON struct {
	AutomatedActionID  string          `json:"automatedActionId"`
	Name               string          `json:"name"`
	Description        *string         `json:"description,omitempty"`
	Action             json.RawMessage `json:"action"`
	ExecutionCondition string          `json:"executionCondition"`
	IsEnabled          bool            `json:"isEnabled"`
	CreatedAt          string          `json:"createdAt"`
}

type evaluationRuleRequest struct {
	Rule      json.RawMessage `json:"rule"`
	IsEnabled bool            `json:"isEnabled"`
}

// evaluationRuleHead is the discriminator of the EvaluationRuleUnion.
type evaluationRuleHead struct {
	EvaluationRuleID string `json:"evaluationRuleId"`
}

type evaluationRuleJSON struct {
	Rule             json.RawMessage `json:"rule"`
	Description      string          `json:"description"`
	IAMRolesRequired []string        `json:"iamRolesRequired"`
	IsEnabled        bool            `json:"isEnabled"`
}

// listJSON is the paginated list envelope shared by the list operations.
type listJSON[T any] struct {
	Items         []T    `json:"items"`
	NextPageToken string `json:"nextPageToken,omitempty"`
	PrevPageToken string `json:"prevPageToken,omitempty"`
	TotalSize     int    `json:"totalSize"`
}

// problemJSON is the RFC 7807 problem details envelope of the spec's error
// schemas (BadRequest, NotFound, ..., UnexpectedError).
type problemJSON struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// problemDetails maps a status to the fixed detail its error schema declares.
var problemDetails = map[int]string{
	http.StatusBadRequest:      "invalid",
	http.StatusUnauthorized:    "authentication_failed",
	http.StatusForbidden:       "forbidden",
	http.StatusNotFound:        "not_found",
	http.StatusConflict:        "conflict",
	http.StatusTooManyRequests: "too_many_requests",
}

// writeError writes a problem details body. The spec fixes detail for each
// declared error status, so the occurrence-specific message goes in title;
// other statuses (UnexpectedError) carry it in detail. The body is sent as
// application/json, which the spec allows beside application/problem+json and
// which the SDK decodes.
func writeError(w http.ResponseWriter, status int, msg string) {
	p := problemJSON{Title: msg, Status: status}
	if d, ok := problemDetails[status]; ok {
		p.Detail = d
	} else {
		p.Title, p.Detail = http.StatusText(status), msg
	}
	if p.Title == "" {
		p.Title = http.StatusText(status)
	}
	core.WriteJSON(w, status, p)
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
	core.APILatency(r, s.latency)
	rw := core.NewResponseRecorder(w)
	s.mux.ServeHTTP(rw, r)
	s.logger.Info("request", core.RequestLogArgs(r, rw)...)
}

// Activation

func (s *Server) activationToJSON(a Activation) activationJSON {
	return activationJSON{
		ServicePrincipalID:   a.ServicePrincipalID,
		IsActive:             a.IsActive,
		AutomatedActionLimit: s.actionLimit,
	}
}

func (s *Server) handleCreateActivation(w http.ResponseWriter, r *http.Request) {
	var req activationCreateRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a, err := s.store.CreateActivation(req.ServicePrincipalID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusCreated, s.activationToJSON(a))
}

func (s *Server) handleReadActivation(w http.ResponseWriter, r *http.Request) {
	a, err := s.store.ReadActivation()
	if err != nil {
		writeError(w, http.StatusNotFound, "security control has never been activated")
		return
	}
	core.WriteJSON(w, http.StatusOK, s.activationToJSON(a))
}

func (s *Server) handleUpdateActivation(w http.ResponseWriter, r *http.Request) {
	var req activationUpdateRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a, err := s.store.UpdateActivation(Activation{ServicePrincipalID: req.ServicePrincipalID, IsActive: req.IsActive})
	if err != nil {
		writeError(w, http.StatusNotFound, "security control has never been activated")
		return
	}
	core.WriteJSON(w, http.StatusOK, s.activationToJSON(a))
}

// Automated actions

func automatedActionToJSON(a AutomatedAction) automatedActionJSON {
	return automatedActionJSON{
		AutomatedActionID:  a.ID,
		Name:               a.Name,
		Description:        a.Description,
		Action:             a.Action,
		ExecutionCondition: a.ExecutionCondition,
		IsEnabled:          a.IsEnabled,
		CreatedAt:          core.FormatRFC3339(a.CreatedAt),
	}
}

func readAutomatedActionInput(r *http.Request) (AutomatedActionInput, error) {
	var req automatedActionRequest
	if err := core.ReadJSON(r, &req); err != nil {
		return AutomatedActionInput{}, err
	}
	return AutomatedActionInput{
		Name:               req.Name,
		Description:        req.Description,
		Action:             req.Action,
		ExecutionCondition: req.ExecutionCondition,
		IsEnabled:          req.IsEnabled,
	}, nil
}

func (s *Server) handleListAutomatedActions(w http.ResponseWriter, r *http.Request) {
	p, err := parsePageRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, paginate(s.store.ListAutomatedActions(), p, automatedActionToJSON))
}

func (s *Server) handleCreateAutomatedAction(w http.ResponseWriter, r *http.Request) {
	in, err := readAutomatedActionInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a, err := s.store.CreateAutomatedAction(in)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s (%d)", err, s.actionLimit))
		return
	}
	core.WriteJSON(w, http.StatusCreated, automatedActionToJSON(a))
}

func (s *Server) handleReadAutomatedAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("actionId")
	a, err := s.store.ReadAutomatedAction(id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("automated action %q not found", id))
		return
	}
	core.WriteJSON(w, http.StatusOK, automatedActionToJSON(a))
}

func (s *Server) handleUpdateAutomatedAction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("actionId")
	in, err := readAutomatedActionInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a, err := s.store.UpdateAutomatedAction(id, in)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("automated action %q not found", id))
		return
	}
	core.WriteJSON(w, http.StatusOK, automatedActionToJSON(a))
}

// handleDeleteAutomatedAction answers 204 even for an unknown ID: the spec
// declares no 404 for this operation.
func (s *Server) handleDeleteAutomatedAction(w http.ResponseWriter, r *http.Request) {
	s.store.DeleteAutomatedAction(r.PathValue("actionId"))
	w.WriteHeader(http.StatusNoContent)
}

// Evaluation rules

func evaluationRuleToJSON(r EvaluationRule) evaluationRuleJSON {
	return evaluationRuleJSON{
		Rule:             r.Rule,
		Description:      ruleDescription(r.ID),
		IAMRolesRequired: []string{},
		IsEnabled:        r.IsEnabled,
	}
}

func (s *Server) handleListEvaluationRules(w http.ResponseWriter, r *http.Request) {
	p, err := parsePageRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, paginate(s.store.ListEvaluationRules(), p, evaluationRuleToJSON))
}

func (s *Server) handleReadEvaluationRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("evaluationRuleId")
	rule, err := s.store.ReadEvaluationRule(id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("evaluation rule %q not found", id))
		return
	}
	core.WriteJSON(w, http.StatusOK, evaluationRuleToJSON(rule))
}

func (s *Server) handleUpdateEvaluationRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("evaluationRuleId")
	if _, err := s.store.ReadEvaluationRule(id); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("evaluation rule %q not found", id))
		return
	}
	var req evaluationRuleRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var head evaluationRuleHead
	if err := json.Unmarshal(req.Rule, &head); err != nil {
		writeError(w, http.StatusBadRequest, "rule must be an object")
		return
	}
	if head.EvaluationRuleID != id {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("rule.evaluationRuleId %q does not match the path %q", head.EvaluationRuleID, id))
		return
	}
	rule, err := s.store.UpdateEvaluationRule(EvaluationRule{ID: id, Rule: req.Rule, IsEnabled: req.IsEnabled})
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("evaluation rule %q not found", id))
		return
	}
	core.WriteJSON(w, http.StatusOK, evaluationRuleToJSON(rule))
}

// Pagination (page_size / next / prev)

const (
	defaultPageSize = 50
	maxPageSize     = 100
)

// pageRequest selects size items starting at index start.
type pageRequest struct {
	start int
	size  int
}

// parsePageRequest reads page_size, next and prev. A page token encodes the
// start index and the page size it was issued for; the spec requires the
// other parameters of a paginated request to match the original, so a token
// replayed with a different page_size is rejected.
func parsePageRequest(r *http.Request) (pageRequest, error) {
	q := r.URL.Query()
	p := pageRequest{size: defaultPageSize}
	if v := q.Get("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return pageRequest{}, fmt.Errorf("invalid page_size: %q", v)
		}
		p.size = min(n, maxPageSize)
	}
	next, prev := q.Get("next"), q.Get("prev")
	token := next
	switch {
	case next != "" && prev != "":
		return pageRequest{}, errors.New("next and prev cannot be specified together")
	case prev != "":
		token = prev
	case next == "":
		return p, nil
	}
	start, size, err := decodePageToken(token)
	if err != nil {
		return pageRequest{}, err
	}
	if size != p.size {
		return pageRequest{}, errors.New("page_size does not match the page token")
	}
	p.start = start
	return p, nil
}

// paginate converts the page of items selected by p into the list envelope,
// with the tokens of the following and preceding pages when they exist.
func paginate[T, J any](items []T, p pageRequest, conv func(T) J) listJSON[J] {
	start := min(p.start, len(items))
	end := min(start+p.size, len(items))
	out := listJSON[J]{Items: make([]J, 0, end-start), TotalSize: len(items)}
	for _, it := range items[start:end] {
		out.Items = append(out.Items, conv(it))
	}
	if end < len(items) {
		out.NextPageToken = encodePageToken(end, p.size)
	}
	if start > 0 {
		out.PrevPageToken = encodePageToken(max(start-p.size, 0), p.size)
	}
	return out
}

func encodePageToken(start, size int) string {
	return base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "%d:%d", start, size))
}

func decodePageToken(token string) (start, size int, err error) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err == nil {
		_, err = fmt.Sscanf(string(b), "%d:%d", &start, &size)
	}
	// Sscanf ignores trailing input, so only the canonical encoding is valid.
	if err != nil || start < 0 || size < 1 || encodePageToken(start, size) != token {
		return 0, 0, fmt.Errorf("invalid page token: %q", token)
	}
	return start, size, nil
}
