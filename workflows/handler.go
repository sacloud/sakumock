package workflows

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/workflows/runbook"
)

// JSON request types

type createWorkflowRequest struct {
	Name               string    `json:"Name"`
	Description        string    `json:"Description"`
	Runbook            string    `json:"Runbook"`
	Publish            bool      `json:"Publish"`
	Logging            bool      `json:"Logging"`
	Tags               []tagJSON `json:"Tags"`
	RevisionAlias      string    `json:"RevisionAlias"`
	ServicePrincipalId string    `json:"ServicePrincipalId"`
	ConcurrencyMode    string    `json:"ConcurrencyMode"`
}

type updateWorkflowRequest struct {
	Name            *string    `json:"Name,omitempty"`
	Description     *string    `json:"Description,omitempty"`
	Publish         *bool      `json:"Publish,omitempty"`
	Logging         *bool      `json:"Logging,omitempty"`
	Tags            *[]tagJSON `json:"Tags,omitempty"`
	ConcurrencyMode *string    `json:"ConcurrencyMode,omitempty"`
}

type createRevisionRequest struct {
	Runbook       string `json:"Runbook"`
	RevisionAlias string `json:"RevisionAlias"`
}

type updateRevisionAliasRequest struct {
	RevisionAlias string `json:"RevisionAlias"`
}

type createExecutionRequest struct {
	RevisionId    *int   `json:"RevisionId,omitempty"`
	RevisionAlias string `json:"RevisionAlias"`
	Args          string `json:"Args"`
	Name          string `json:"Name"`
}

type createSubscriptionRequest struct {
	PlanId int `json:"PlanId"`
}

// JSON response types

type tagJSON struct {
	Name string `json:"Name"`
}

type workflowJSON struct {
	Id                 string    `json:"Id"`
	Name               string    `json:"Name"`
	Description        string    `json:"Description,omitempty"`
	Publish            bool      `json:"Publish"`
	Logging            bool      `json:"Logging"`
	Tags               []tagJSON `json:"Tags"`
	ServicePrincipalId string    `json:"ServicePrincipalId,omitempty"`
	CreatedAt          string    `json:"CreatedAt"`
	UpdatedAt          string    `json:"UpdatedAt"`
	ConcurrencyMode    string    `json:"ConcurrencyMode,omitempty"`
}

type revisionJSON struct {
	RevisionId    int    `json:"RevisionId"`
	WorkflowId    string `json:"WorkflowId"`
	RevisionAlias string `json:"RevisionAlias"`
	Runbook       string `json:"Runbook"`
	CreatedAt     string `json:"CreatedAt"`
	UpdatedAt     string `json:"UpdatedAt"`
}

type executionWorkflowJSON struct {
	Id                 string    `json:"Id"`
	Name               string    `json:"Name"`
	Description        string    `json:"Description,omitempty"`
	Publish            bool      `json:"Publish"`
	Logging            bool      `json:"Logging"`
	Tags               []tagJSON `json:"Tags"`
	ServicePrincipalId string    `json:"ServicePrincipalId,omitempty"`
	CreatedAt          string    `json:"CreatedAt"`
	UpdatedAt          string    `json:"UpdatedAt"`
	ConcurrencyMode    string    `json:"ConcurrencyMode,omitempty"`
}

type executionJSON struct {
	ExecutionId       string                `json:"ExecutionId"`
	Name              string                `json:"Name"`
	Workflow          executionWorkflowJSON `json:"Workflow"`
	Status            string                `json:"Status"`
	Revision          int                   `json:"Revision"`
	RevisionAlias     string                `json:"RevisionAlias"`
	Args              string                `json:"Args"`
	StepCount         int                   `json:"StepCount"`
	Result            string                `json:"Result"`
	Error             string                `json:"Error"`
	CreatedAt         string                `json:"CreatedAt"`
	UpdatedAt         string                `json:"UpdatedAt"`
	RunAt             *string               `json:"RunAt,omitempty"`
	FailedAt          *string               `json:"FailedAt,omitempty"`
	SucceededAt       *string               `json:"SucceededAt,omitempty"`
	CancelRequestedAt *string               `json:"CancelRequestedAt,omitempty"`
	CanceledAt        *string               `json:"CanceledAt,omitempty"`
}

type historyJSON struct {
	WorkflowExecutionId string `json:"WorkflowExecutionId"`
	JobId               string `json:"JobId"`
	ThreadId            string `json:"ThreadId"`
	Type                string `json:"Type"`
	CreatedAt           string `json:"CreatedAt"`
	Meta                string `json:"Meta"`
	StackTrace          string `json:"StackTrace"`
	Variables           string `json:"Variables"`
}

type planJSON struct {
	Id                  int    `json:"id"`
	Name                string `json:"name"`
	Grade               int    `json:"grade"`
	ServiceClassPath    string `json:"serviceClassPath"`
	BasePrice           int    `json:"basePrice"`
	IncludedSteps       int    `json:"includedSteps"`
	OverageStepUnit     int    `json:"overageStepUnit"`
	OveragePricePerUnit int    `json:"overagePricePerUnit"`
}

type subscriptionJSON struct {
	Id            string  `json:"id"`
	AccountId     string  `json:"accountId"`
	ContractId    string  `json:"contractId"`
	PlanId        int     `json:"planId"`
	ActivateFrom  string  `json:"activateFrom"`
	ActivateUntil *string `json:"activateUntil"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
	PlanName      string  `json:"planName"`
}

type monthAppliedPlanJSON struct {
	Id                  string  `json:"id"`
	AccountId           string  `json:"accountId"`
	ContractId          string  `json:"contractId"`
	PlanId              int     `json:"planId"`
	ActivateFrom        string  `json:"activateFrom"`
	ActivateUntil       *string `json:"activateUntil"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
	PlanName            string  `json:"planName"`
	PlanGrade           int     `json:"planGrade"`
	BasePrice           int     `json:"basePrice"`
	IncludedSteps       int     `json:"includedSteps"`
	OverageStepUnit     int     `json:"overageStepUnit"`
	OveragePricePerUnit int     `json:"overagePricePerUnit"`
}

type suggestItemJSON struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

// Helper functions

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

func writeError(w http.ResponseWriter, status int, msg string) {
	core.WriteJSON(w, status, map[string]any{
		"is_ok":   false,
		"Message": msg,
	})
}

func toTagJSON(tags []Tag) []tagJSON {
	result := make([]tagJSON, len(tags))
	for i, t := range tags {
		result[i] = tagJSON{Name: t.Name}
	}
	return result
}

func fromTagJSON(tags []tagJSON) []Tag {
	result := make([]Tag, len(tags))
	for i, t := range tags {
		result[i] = Tag{Name: t.Name}
	}
	return result
}

func toWorkflowJSON(w *WorkflowRecord) workflowJSON {
	return workflowJSON{
		Id:                 w.ID,
		Name:               w.Name,
		Description:        w.Description,
		Publish:            w.Publish,
		Logging:            w.Logging,
		Tags:               toTagJSON(w.Tags),
		ServicePrincipalId: w.ServicePrincipalID,
		CreatedAt:          core.FormatRFC3339Nano(w.CreatedAt),
		UpdatedAt:          core.FormatRFC3339Nano(w.UpdatedAt),
		ConcurrencyMode:    w.ConcurrencyMode,
	}
}

func toRevisionJSON(r *RevisionRecord) revisionJSON {
	return revisionJSON{
		RevisionId:    r.RevisionID,
		WorkflowId:    r.WorkflowID,
		RevisionAlias: r.RevisionAlias,
		Runbook:       r.Runbook,
		CreatedAt:     core.FormatRFC3339Nano(r.CreatedAt),
		UpdatedAt:     core.FormatRFC3339Nano(r.UpdatedAt),
	}
}

func optionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := core.FormatRFC3339Nano(*t)
	return &s
}

func (s *Server) toExecutionJSON(e *ExecutionRecord) executionJSON {
	w, _ := s.store.GetWorkflow(e.WorkflowID)
	var wj executionWorkflowJSON
	if w != nil {
		wj = executionWorkflowJSON{
			Id:                 w.ID,
			Name:               w.Name,
			Description:        w.Description,
			Publish:            w.Publish,
			Logging:            w.Logging,
			Tags:               toTagJSON(w.Tags),
			ServicePrincipalId: w.ServicePrincipalID,
			CreatedAt:          core.FormatRFC3339Nano(w.CreatedAt),
			UpdatedAt:          core.FormatRFC3339Nano(w.UpdatedAt),
			ConcurrencyMode:    w.ConcurrencyMode,
		}
	}
	return executionJSON{
		ExecutionId:       e.ExecutionID,
		Name:              e.Name,
		Workflow:          wj,
		Status:            e.Status,
		Revision:          e.Revision,
		RevisionAlias:     e.RevisionAlias,
		Args:              e.Args,
		StepCount:         e.StepCount,
		Result:            e.Result,
		Error:             e.Error,
		CreatedAt:         core.FormatRFC3339Nano(e.CreatedAt),
		UpdatedAt:         core.FormatRFC3339Nano(e.UpdatedAt),
		RunAt:             optionalTime(e.RunAt),
		FailedAt:          optionalTime(e.FailedAt),
		SucceededAt:       optionalTime(e.SucceededAt),
		CancelRequestedAt: optionalTime(e.CancelRequestedAt),
		CanceledAt:        optionalTime(e.CanceledAt),
	}
}

// Handlers — Workflow

func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req createWorkflowRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.Runbook == "" {
		writeError(w, http.StatusBadRequest, "Runbook is required")
		return
	}

	var tags []Tag
	if req.Tags != nil {
		tags = fromTagJSON(req.Tags)
	}

	wf := s.store.CreateWorkflow(
		req.Name, req.Description, req.Runbook,
		req.Publish, req.Logging,
		tags, req.ServicePrincipalId, req.ConcurrencyMode, req.RevisionAlias,
	)
	core.WriteJSON(w, http.StatusCreated, map[string]any{
		"is_ok":    true,
		"Workflow": toWorkflowJSON(wf),
	})
}

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows := s.store.ListWorkflows()

	nameFilter := r.URL.Query().Get("Name")
	matchType := r.URL.Query().Get("NameMatchType")
	publishedFilter := r.URL.Query().Get("Published")

	filtered := make([]*WorkflowRecord, 0, len(workflows))
	for _, wf := range workflows {
		if nameFilter != "" {
			switch matchType {
			case "prefix":
				if !strings.HasPrefix(wf.Name, nameFilter) {
					continue
				}
			default:
				if !strings.Contains(wf.Name, nameFilter) {
					continue
				}
			}
		}
		if publishedFilter != "" {
			pub, err := strconv.ParseBool(publishedFilter)
			if err == nil && wf.Publish != pub {
				continue
			}
		}
		filtered = append(filtered, wf)
	}

	items := make([]workflowJSON, len(filtered))
	for i, wf := range filtered {
		items[i] = toWorkflowJSON(wf)
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":     true,
		"Total":     len(items),
		"From":      0,
		"Count":     len(items),
		"Workflows": items,
	})
}

func (s *Server) handleListWorkflowSuggest(w http.ResponseWriter, r *http.Request) {
	workflows := s.store.ListWorkflows()
	nameFilter := r.URL.Query().Get("Name")

	suggests := make([]suggestItemJSON, 0)
	for _, wf := range workflows {
		if nameFilter != "" && !strings.Contains(wf.Name, nameFilter) {
			continue
		}
		suggests = append(suggests, suggestItemJSON{
			Id:   wf.ID,
			Name: wf.Name,
		})
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":    true,
		"Total":    len(suggests),
		"From":     0,
		"Count":    len(suggests),
		"Suggests": suggests,
	})
}

func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wf, ok := s.store.GetWorkflow(id)
	if !ok {
		writeError(w, http.StatusNotFound, "Workflow not found.")
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":    true,
		"Workflow": toWorkflowJSON(wf),
	})
}

func (s *Server) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req updateWorkflowRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updates := WorkflowUpdates{
		Name:            req.Name,
		Description:     req.Description,
		Publish:         req.Publish,
		Logging:         req.Logging,
		ConcurrencyMode: req.ConcurrencyMode,
	}
	if req.Tags != nil {
		tags := fromTagJSON(*req.Tags)
		updates.Tags = &tags
	}

	wf, ok := s.store.UpdateWorkflow(id, updates)
	if !ok {
		writeError(w, http.StatusNotFound, "Workflow not found.")
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":    true,
		"Workflow": toWorkflowJSON(wf),
	})
}

func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteWorkflow(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "Workflow not found.")
		} else {
			writeError(w, http.StatusConflict, err.Error())
		}
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"is_ok": true})
}

// Handlers — Revision

func (s *Server) handleCreateRevision(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	var req createRevisionRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Runbook == "" {
		writeError(w, http.StatusBadRequest, "Runbook is required")
		return
	}

	rev, err := s.store.CreateRevision(workflowID, req.Runbook, req.RevisionAlias)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusConflict, err.Error())
		}
		return
	}
	core.WriteJSON(w, http.StatusCreated, map[string]any{
		"is_ok":    true,
		"Revision": toRevisionJSON(rev),
	})
}

func (s *Server) handleListRevisions(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	if _, ok := s.store.GetWorkflow(workflowID); !ok {
		writeError(w, http.StatusNotFound, "Workflow not found.")
		return
	}

	revisions := s.store.ListRevisions(workflowID)
	items := make([]revisionJSON, len(revisions))
	for i, rev := range revisions {
		items[i] = toRevisionJSON(rev)
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":     true,
		"Total":     len(items),
		"From":      0,
		"Count":     len(items),
		"Revisions": items,
	})
}

func (s *Server) handleGetRevision(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	revisionID, err := strconv.Atoi(r.PathValue("revisionId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid revision ID")
		return
	}

	rev, ok := s.store.GetRevision(workflowID, revisionID)
	if !ok {
		writeError(w, http.StatusNotFound, "Revision not found.")
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":    true,
		"Revision": toRevisionJSON(rev),
	})
}

func (s *Server) handleUpdateRevisionAlias(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	revisionID, err := strconv.Atoi(r.PathValue("revisionId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid revision ID")
		return
	}

	var req updateRevisionAliasRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rev, err := s.store.UpdateRevisionAlias(workflowID, revisionID, req.RevisionAlias)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusConflict, err.Error())
		}
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":    true,
		"Revision": toRevisionJSON(rev),
	})
}

func (s *Server) handleDeleteRevisionAlias(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	revisionID, err := strconv.Atoi(r.PathValue("revisionId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid revision ID")
		return
	}

	rev, ok := s.store.DeleteRevisionAlias(workflowID, revisionID)
	if !ok {
		writeError(w, http.StatusNotFound, "Revision not found.")
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":    true,
		"Revision": toRevisionJSON(rev),
	})
}

// Handlers — Execution

func (s *Server) handleCreateExecution(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	var req createExecutionRequest
	if err := core.ReadJSON(r, &req); err != nil && err.Error() != "empty request body" {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	input := ExecutionInput{
		RevisionID:    req.RevisionId,
		RevisionAlias: req.RevisionAlias,
		Args:          req.Args,
		Name:          req.Name,
	}
	if s.dataPlaneEnabled() {
		input.InitialStatus = "Queued"
	}

	exec, err := s.store.CreateExecution(workflowID, input)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else if strings.Contains(err.Error(), "not executable") {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	if s.dataPlaneEnabled() {
		rev, ok := s.store.GetRevision(workflowID, exec.Revision)
		if ok {
			rb, parseErr := runbook.Parse([]byte(rev.Runbook))
			if parseErr != nil {
				s.logger.Error("failed to parse runbook", "error", parseErr)
			} else {
				s.executor.submit(s.ctx, workflowID, exec.ExecutionID, rb, exec.Args)
			}
		}
	}

	core.WriteJSON(w, http.StatusCreated, map[string]any{
		"is_ok":     true,
		"Execution": s.toExecutionJSON(exec),
	})
}

func (s *Server) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	if _, ok := s.store.GetWorkflow(workflowID); !ok {
		writeError(w, http.StatusNotFound, "Workflow not found.")
		return
	}

	executions := s.store.ListExecutions(workflowID)
	items := make([]executionJSON, len(executions))
	for i, e := range executions {
		items[i] = s.toExecutionJSON(e)
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":      true,
		"Total":      len(items),
		"From":       0,
		"Count":      len(items),
		"Executions": items,
	})
}

func (s *Server) handleGetExecution(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	executionID := r.PathValue("executionId")

	exec, ok := s.store.GetExecution(workflowID, executionID)
	if !ok {
		writeError(w, http.StatusNotFound, "Execution not found.")
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":     true,
		"Execution": s.toExecutionJSON(exec),
	})
}

func (s *Server) handleCancelExecution(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	executionID := r.PathValue("executionId")

	if s.dataPlaneEnabled() {
		s.executor.cancel(executionID)
	}

	exec, err := s.store.CancelExecution(workflowID, executionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusConflict, err.Error())
		}
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":     true,
		"Execution": s.toExecutionJSON(exec),
	})
}

func (s *Server) handleDeleteExecution(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	executionID := r.PathValue("executionId")

	if err := s.store.DeleteExecution(workflowID, executionID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusConflict, err.Error())
		}
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"is_ok": true})
}

func (s *Server) handleListExecutionHistory(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	executionID := r.PathValue("executionId")

	histories, err := s.store.ListExecutionHistory(workflowID, executionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	items := make([]historyJSON, len(histories))
	for i, h := range histories {
		items[i] = historyJSON{
			WorkflowExecutionId: h.WorkflowExecutionID,
			JobId:               h.JobID,
			ThreadId:            h.ThreadID,
			Type:                h.Type,
			CreatedAt:           core.FormatRFC3339Nano(h.CreatedAt),
			Meta:                h.Meta,
			StackTrace:          h.StackTrace,
			Variables:           h.Variables,
		}
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":     true,
		"Total":     len(items),
		"From":      0,
		"Count":     len(items),
		"Histories": items,
	})
}

// Handlers — Plans & Subscription

func (s *Server) handleListPlans(w http.ResponseWriter, _ *http.Request) {
	plans := make([]planJSON, len(staticPlans))
	for i, p := range staticPlans {
		plans[i] = planJSON{
			Id:                  p.ID,
			Name:                p.Name,
			Grade:               p.Grade,
			ServiceClassPath:    p.ServiceClassPath,
			BasePrice:           p.BasePrice,
			IncludedSteps:       p.IncludedSteps,
			OverageStepUnit:     p.OverageStepUnit,
			OveragePricePerUnit: p.OveragePricePerUnit,
		}
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":   true,
		"Plans":   plans,
		"TaxRate": 10,
	})
}

func (s *Server) handleGetSubscription(w http.ResponseWriter, _ *http.Request) {
	sub := s.store.GetSubscription()
	var currentPlan any
	var monthPlan any
	if sub != nil {
		var activateUntil *string
		if sub.ActivateUntil != nil {
			s := core.FormatRFC3339Nano(*sub.ActivateUntil)
			activateUntil = &s
		}
		currentPlan = subscriptionJSON{
			Id:            sub.ID,
			AccountId:     sub.AccountID,
			ContractId:    sub.ContractID,
			PlanId:        sub.PlanID,
			ActivateFrom:  core.FormatRFC3339Nano(sub.ActivateFrom),
			ActivateUntil: activateUntil,
			CreatedAt:     core.FormatRFC3339Nano(sub.CreatedAt),
			UpdatedAt:     core.FormatRFC3339Nano(sub.UpdatedAt),
			PlanName:      sub.PlanName,
		}
		plan := plansByID[sub.PlanID]
		monthPlan = monthAppliedPlanJSON{
			Id:                  sub.ID,
			AccountId:           sub.AccountID,
			ContractId:          sub.ContractID,
			PlanId:              sub.PlanID,
			ActivateFrom:        core.FormatRFC3339Nano(sub.ActivateFrom),
			ActivateUntil:       activateUntil,
			CreatedAt:           core.FormatRFC3339Nano(sub.CreatedAt),
			UpdatedAt:           core.FormatRFC3339Nano(sub.UpdatedAt),
			PlanName:            sub.PlanName,
			PlanGrade:           plan.Grade,
			BasePrice:           plan.BasePrice,
			IncludedSteps:       plan.IncludedSteps,
			OverageStepUnit:     plan.OverageStepUnit,
			OveragePricePerUnit: plan.OveragePricePerUnit,
		}
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"is_ok":            true,
		"CurrentPlan":      currentPlan,
		"MonthAppliedPlan": monthPlan,
	})
}

func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req createSubscriptionRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.PlanId == 0 {
		writeError(w, http.StatusBadRequest, "PlanId is required")
		return
	}

	if err := s.store.CreateSubscription(req.PlanId); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteSubscription(w http.ResponseWriter, _ *http.Request) {
	if !s.store.DeleteSubscription() {
		writeError(w, http.StatusNotFound, "Subscription not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
