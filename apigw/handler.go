package apigw

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

type errorJSON struct {
	Message string `json:"message"`
}

// writeError writes the spec's ErrorSchema shape ({"message": ...}).
func writeError(w http.ResponseWriter, status int, message string) {
	core.WriteJSON(w, status, errorJSON{Message: message})
}

func (s *Server) writeStoreError(w http.ResponseWriter, err error) {
	var se *StoreError
	if errors.As(err, &se) {
		writeError(w, se.Status, se.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

// envelope is the {"apigw": ...} wrapper every success response uses.
type envelope[T any] struct {
	Apigw T `json:"apigw"`
}

func writeEnvelope[T any](w http.ResponseWriter, status int, v T) {
	core.WriteJSON(w, status, envelope[T]{Apigw: v})
}

func (s *Server) readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := core.ReadJSON(r, v); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

// --- Services ---

type subscriptionRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// serviceRequest is the ServiceDetailRequest body: a service plus the
// subscription reference (POST only; PUT bodies carry no subscription).
type serviceRequest struct {
	Service
	Subscription *subscriptionRef `json:"subscription"`
}

// serviceResponse is the ServiceDetailResponse shape.
type serviceResponse struct {
	Service
	Subscription subscriptionRef `json:"subscription"`
}

type servicesBody struct {
	Services []serviceResponse `json:"services"`
}

type serviceBody struct {
	Service serviceResponse `json:"service"`
}

func (s *Server) renderService(svc Service) serviceResponse {
	return serviceResponse{
		Service: svc,
		Subscription: subscriptionRef{
			ID:   svc.SubscriptionID,
			Name: s.store.SubscriptionNameOf(svc.ID),
		},
	}
}

func (s *Server) handleAddService(w http.ResponseWriter, r *http.Request) {
	var req serviceRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if req.Subscription == nil || req.Subscription.ID == "" {
		writeError(w, http.StatusBadRequest, "subscription is required")
		return
	}
	created, err := s.store.CreateService(req.Service, req.Subscription.ID)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusCreated, serviceBody{Service: s.renderService(created)})
}

func (s *Server) handleListServices(w http.ResponseWriter, _ *http.Request) {
	services := s.store.ListServices()
	out := make([]serviceResponse, 0, len(services))
	for _, svc := range services {
		out = append(out, s.renderService(svc))
	}
	writeEnvelope(w, http.StatusOK, servicesBody{Services: out})
}

func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	svc, err := s.store.GetService(r.PathValue("serviceId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, serviceBody{Service: s.renderService(svc)})
}

func (s *Server) handleUpdateService(w http.ResponseWriter, r *http.Request) {
	var req serviceRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.store.UpdateService(r.PathValue("serviceId"), req.Service); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteService(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteService(r.PathValue("serviceId")); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Routes ---

type routesBody struct {
	Routes []Route `json:"routes"`
}

type routeBody struct {
	Route Route `json:"route"`
}

func (s *Server) handleAddRoute(w http.ResponseWriter, r *http.Request) {
	var req Route
	if !s.readJSON(w, r, &req) {
		return
	}
	created, err := s.store.CreateRoute(r.PathValue("serviceId"), req)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusCreated, routeBody{Route: created})
}

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := s.store.ListRoutes(r.PathValue("serviceId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	if routes == nil {
		routes = []Route{}
	}
	writeEnvelope(w, http.StatusOK, routesBody{Routes: routes})
}

func (s *Server) handleGetRoute(w http.ResponseWriter, r *http.Request) {
	rt, err := s.store.GetRoute(r.PathValue("serviceId"), r.PathValue("routeId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, routeBody{Route: rt})
}

func (s *Server) handleUpdateRoute(w http.ResponseWriter, r *http.Request) {
	var req Route
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.store.UpdateRoute(r.PathValue("serviceId"), r.PathValue("routeId"), req); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteRoute(r.PathValue("serviceId"), r.PathValue("routeId")); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Route authorization & transformations ---

type routeAuthorizationBody struct {
	RouteAuthorization *RouteAuthorizationConfig `json:"routeAuthorization,omitempty"`
}

// routeAuthorizationRequest is the RouteAuthorizationDetail oneOf: either
// {isACLEnabled: false} or {isACLEnabled: true, groups: [...]}. The oneOf is
// degraded by the generated validator, so the shape is enforced here.
type routeAuthorizationRequest struct {
	IsACLEnabled *bool                `json:"isACLEnabled"`
	Groups       []RouteAuthorization `json:"groups"`
}

func (s *Server) handleUpsertRouteAuthorization(w http.ResponseWriter, r *http.Request) {
	var req routeAuthorizationRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if req.IsACLEnabled == nil {
		writeError(w, http.StatusBadRequest, "isACLEnabled is required")
		return
	}
	var cfg *RouteAuthorizationConfig
	if *req.IsACLEnabled {
		if len(req.Groups) == 0 {
			writeError(w, http.StatusBadRequest, "groups must contain at least one group when isACLEnabled is true")
			return
		}
		cfg = &RouteAuthorizationConfig{IsACLEnabled: true, Groups: req.Groups}
	}
	if err := s.store.SetRouteAuthorization(r.PathValue("serviceId"), r.PathValue("routeId"), cfg); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetRouteAuthorization(w http.ResponseWriter, r *http.Request) {
	rt, err := s.store.GetRoute(r.PathValue("serviceId"), r.PathValue("routeId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, routeAuthorizationBody{RouteAuthorization: rt.Authorization})
}

type requestTransformationBody struct {
	RequestTransformation *RequestTransformation `json:"requestTransformation,omitempty"`
}

func (s *Server) handleUpsertRequestTransformation(w http.ResponseWriter, r *http.Request) {
	var req RequestTransformation
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.store.SetRequestTransformation(r.PathValue("serviceId"), r.PathValue("routeId"), &req); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetRequestTransformation(w http.ResponseWriter, r *http.Request) {
	rt, err := s.store.GetRoute(r.PathValue("serviceId"), r.PathValue("routeId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, requestTransformationBody{RequestTransformation: rt.RequestTransform})
}

type responseTransformationBody struct {
	ResponseTransformation *ResponseTransformation `json:"responseTransformation,omitempty"`
}

func (s *Server) handleUpsertResponseTransformation(w http.ResponseWriter, r *http.Request) {
	var req ResponseTransformation
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.store.SetResponseTransformation(r.PathValue("serviceId"), r.PathValue("routeId"), &req); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetResponseTransformation(w http.ResponseWriter, r *http.Request) {
	rt, err := s.store.GetRoute(r.PathValue("serviceId"), r.PathValue("routeId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, responseTransformationBody{ResponseTransformation: rt.ResponseTransform})
}

// --- Users ---

type usersBody struct {
	Users []User `json:"users"`
}

type userBody struct {
	User User `json:"user"`
}

func (s *Server) handleAddUser(w http.ResponseWriter, r *http.Request) {
	var req User
	if !s.readJSON(w, r, &req) {
		return
	}
	created, err := s.store.CreateUser(req)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusCreated, userBody{User: created})
}

func (s *Server) handleListUsers(w http.ResponseWriter, _ *http.Request) {
	writeEnvelope(w, http.StatusOK, usersBody{Users: s.store.ListUsers()})
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.GetUser(r.PathValue("userId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, userBody{User: u})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	var req User
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.store.UpdateUser(r.PathValue("userId"), req); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteUser(r.PathValue("userId")); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- User groups ---

type userGroupDetail struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsAssigned bool   `json:"isAssigned"`
}

type userGroupsBody struct {
	Groups []userGroupDetail `json:"groups"`
}

func (s *Server) handleGetUserGroups(w http.ResponseWriter, r *http.Request) {
	groups, assigned, err := s.store.ListUserGroups(r.PathValue("userId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	out := make([]userGroupDetail, 0, len(groups))
	for _, g := range groups {
		out = append(out, userGroupDetail{ID: g.ID, Name: g.Name, IsAssigned: assigned[g.ID]})
	}
	writeEnvelope(w, http.StatusOK, userGroupsBody{Groups: out})
}

// userGroupUpdate is one UserGroupDetailUpdate entry: isAssigned plus either
// the group id or its name (a oneOf the generated validator degrades).
type userGroupUpdate struct {
	IsAssigned *bool  `json:"isAssigned"`
	ID         string `json:"id"`
	Name       string `json:"name"`
}

func (s *Server) handleUpdateUserGroups(w http.ResponseWriter, r *http.Request) {
	var req []userGroupUpdate
	if !s.readJSON(w, r, &req) {
		return
	}
	userID := r.PathValue("userId")
	for _, entry := range req {
		if entry.IsAssigned == nil {
			writeError(w, http.StatusBadRequest, "isAssigned is required")
			return
		}
		ref := entry.ID
		if ref == "" {
			ref = entry.Name
		}
		if ref == "" {
			writeError(w, http.StatusBadRequest, "either id or name is required")
			return
		}
		if err := s.store.AssignUserGroup(userID, ref, *entry.IsAssigned); err != nil {
			s.writeStoreError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- User authentication ---

type userAuthenticationBody struct {
	UserAuthentication UserAuthentication `json:"userAuthentication"`
}

func (s *Server) handleGetUserAuthentication(w http.ResponseWriter, r *http.Request) {
	auth, err := s.store.GetUserAuthentication(r.PathValue("userId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, userAuthenticationBody{UserAuthentication: *auth})
}

func (s *Server) handleUpsertUserAuthentication(w http.ResponseWriter, r *http.Request) {
	var req UserAuthentication
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.store.UpsertUserAuthentication(r.PathValue("userId"), req); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Groups ---

type groupsBody struct {
	Groups []Group `json:"groups"`
}

type groupBody struct {
	Group Group `json:"group"`
}

func (s *Server) handleAddGroup(w http.ResponseWriter, r *http.Request) {
	var req Group
	if !s.readJSON(w, r, &req) {
		return
	}
	created, err := s.store.CreateGroup(req)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusCreated, groupBody{Group: created})
}

func (s *Server) handleListGroups(w http.ResponseWriter, _ *http.Request) {
	writeEnvelope(w, http.StatusOK, groupsBody{Groups: s.store.ListGroups()})
}

func (s *Server) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	g, err := s.store.GetGroup(r.PathValue("groupId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, groupBody{Group: g})
}

func (s *Server) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	var req Group
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.store.UpdateGroup(r.PathValue("groupId"), req); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteGroup(r.PathValue("groupId")); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Domains ---

type domainsBody struct {
	Domains []Domain `json:"domains"`
}

type domainBody struct {
	Domain Domain `json:"domain"`
}

func (s *Server) handleAddDomain(w http.ResponseWriter, r *http.Request) {
	var req Domain
	if !s.readJSON(w, r, &req) {
		return
	}
	created, err := s.store.CreateDomain(req)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusCreated, domainBody{Domain: created})
}

func (s *Server) handleListDomains(w http.ResponseWriter, _ *http.Request) {
	writeEnvelope(w, http.StatusOK, domainsBody{Domains: s.store.ListDomains()})
}

type domainUpdateRequest struct {
	CertificateID string `json:"certificateId"`
}

func (s *Server) handleUpdateDomain(w http.ResponseWriter, r *http.Request) {
	var req domainUpdateRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.store.UpdateDomain(r.PathValue("domainId"), req.CertificateID); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteDomain(r.PathValue("domainId")); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Certificates ---

type certificatesBody struct {
	Certificates []Certificate `json:"certificates"`
}

type certificateBody struct {
	Certificate Certificate `json:"certificate"`
}

// certificateRequest carries the uploaded PEM material (cert and key are
// writeOnly in the spec, so the stored Certificate type excludes them from
// JSON and a separate request shape is needed).
type certificateRequest struct {
	Name  string                     `json:"name"`
	RSA   *certificateDetailsRequest `json:"rsa"`
	ECDSA *certificateDetailsRequest `json:"ecdsa"`
}

type certificateDetailsRequest struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

// parseCertificate derives expiredAt from the leaf certificate in the PEM.
func parseCertificate(req *certificateDetailsRequest) (*CertificateDetails, error) {
	block, _ := pem.Decode([]byte(req.Cert))
	if block == nil {
		return nil, errBadRequest("invalid certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errBadRequest("invalid certificate: %v", err)
	}
	return &CertificateDetails{
		ExpiredAt: cert.NotAfter.UTC(),
		CertPEM:   req.Cert,
		KeyPEM:    req.Key,
	}, nil
}

func (s *Server) certificateFromRequest(w http.ResponseWriter, req certificateRequest) (Certificate, bool) {
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return Certificate{}, false
	}
	if req.RSA == nil && req.ECDSA == nil {
		writeError(w, http.StatusBadRequest, "either rsa or ecdsa is required")
		return Certificate{}, false
	}
	c := Certificate{Name: req.Name}
	for _, pair := range []struct {
		in  *certificateDetailsRequest
		out **CertificateDetails
	}{{req.RSA, &c.RSA}, {req.ECDSA, &c.ECDSA}} {
		if pair.in == nil {
			continue
		}
		details, err := parseCertificate(pair.in)
		if err != nil {
			s.writeStoreError(w, err)
			return Certificate{}, false
		}
		*pair.out = details
	}
	return c, true
}

func (s *Server) handleAddCertificate(w http.ResponseWriter, r *http.Request) {
	var req certificateRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	c, ok := s.certificateFromRequest(w, req)
	if !ok {
		return
	}
	created, err := s.store.CreateCertificate(c)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusCreated, certificateBody{Certificate: created})
}

func (s *Server) handleListCertificates(w http.ResponseWriter, _ *http.Request) {
	writeEnvelope(w, http.StatusOK, certificatesBody{Certificates: s.store.ListCertificates()})
}

func (s *Server) handleUpdateCertificate(w http.ResponseWriter, r *http.Request) {
	var req certificateRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	c, ok := s.certificateFromRequest(w, req)
	if !ok {
		return
	}
	if err := s.store.UpdateCertificate(r.PathValue("certificateId"), c); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteCertificate(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteCertificate(r.PathValue("certificateId")); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Plans & subscriptions ---

type plansBody struct {
	Plans []Plan `json:"plans"`
}

func (s *Server) handleListPlans(w http.ResponseWriter, _ *http.Request) {
	writeEnvelope(w, http.StatusOK, plansBody{Plans: s.store.ListPlans()})
}

type subscribeRequest struct {
	PlanID string `json:"planId"`
	Name   string `json:"name"`
}

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	var req subscribeRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if _, err := s.store.CreateSubscription(req.PlanID, req.Name); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type subscriptionsBody struct {
	Subscriptions   []Subscription `json:"subscriptions"`
	MaxSubscription int            `json:"maxSubscription"`
}

func (s *Server) handleListSubscriptions(w http.ResponseWriter, _ *http.Request) {
	writeEnvelope(w, http.StatusOK, subscriptionsBody{
		Subscriptions:   s.store.ListSubscriptions(),
		MaxSubscription: maxSubscriptions,
	})
}

type subscriptionPlanResponse struct {
	PlanID          string   `json:"planID"`
	PlanName        string   `json:"planName"`
	Price           string   `json:"price"`
	MaxServices     int      `json:"maxServices"`
	MaxRequests     int      `json:"maxRequests"`
	MaxRequestsUnit string   `json:"maxRequestsUnit"`
	Overage         *Overage `json:"overage,omitempty"`
}

type subscriptionDetailResponse struct {
	Subscription
	Plan *subscriptionPlanResponse `json:"plan,omitempty"`
}

type subscriptionBody struct {
	Subscription subscriptionDetailResponse `json:"subscription"`
}

func (s *Server) handleGetSubscription(w http.ResponseWriter, r *http.Request) {
	sub, plan, err := s.store.GetSubscription(r.PathValue("subscriptionId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	detail := subscriptionDetailResponse{Subscription: sub}
	if plan.ID != "" {
		detail.Plan = &subscriptionPlanResponse{
			PlanID:          plan.ID,
			PlanName:        plan.Name,
			Price:           plan.Price,
			MaxServices:     plan.MaxServices,
			MaxRequests:     plan.MaxRequests,
			MaxRequestsUnit: plan.MaxRequestsUnit,
			Overage:         plan.Overage,
		}
	}
	writeEnvelope(w, http.StatusOK, subscriptionBody{Subscription: detail})
}

type subscriptionUpdateRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleUpdateSubscription(w http.ResponseWriter, r *http.Request) {
	var req subscriptionUpdateRequest
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.store.UpdateSubscription(r.PathValue("subscriptionId"), req.Name); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteSubscription(r.PathValue("subscriptionId")); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- OIDC ---

type oidcsBody struct {
	Oidcs []OidcConfig `json:"oidcs"`
}

type oidcBody struct {
	Oidc OidcConfig `json:"oidc"`
}

// oidcDetailResponse adds the services using this OIDC configuration.
type oidcDetailResponse struct {
	OidcConfig
	Services []OidcServiceSummary `json:"services,omitempty"`
}

// OidcServiceSummary is the ServiceSummary shape (id and name only).
type OidcServiceSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type oidcDetailBody struct {
	Oidc oidcDetailResponse `json:"oidc"`
}

func (s *Server) handleAddOidc(w http.ResponseWriter, r *http.Request) {
	var req OidcConfig
	if !s.readJSON(w, r, &req) {
		return
	}
	created, err := s.store.CreateOidc(req)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeEnvelope(w, http.StatusCreated, oidcBody{Oidc: created})
}

func (s *Server) handleListOidcs(w http.ResponseWriter, _ *http.Request) {
	writeEnvelope(w, http.StatusOK, oidcsBody{Oidcs: s.store.ListOidcs()})
}

func (s *Server) handleGetOidc(w http.ResponseWriter, r *http.Request) {
	o, services, err := s.store.GetOidc(r.PathValue("oidcId"))
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	detail := oidcDetailResponse{OidcConfig: o}
	for _, svc := range services {
		detail.Services = append(detail.Services, OidcServiceSummary{ID: svc.ID, Name: svc.Name})
	}
	writeEnvelope(w, http.StatusOK, oidcDetailBody{Oidc: detail})
}

func (s *Server) handleUpdateOidc(w http.ResponseWriter, r *http.Request) {
	var req OidcConfig
	if !s.readJSON(w, r, &req) {
		return
	}
	if err := s.store.UpdateOidc(r.PathValue("oidcId"), req); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteOidc(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteOidc(r.PathValue("oidcId")); err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- HTTP plumbing ---

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
