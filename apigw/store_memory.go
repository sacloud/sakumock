package apigw

import (
	"log/slog"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sacloud/sakumock/core"
)

// maxSubscriptions caps the number of concurrent subscriptions per account.
const maxSubscriptions = 10

type MemoryStore struct {
	mu            sync.RWMutex
	services      map[string]*Service
	routes        map[string]*Route
	users         map[string]*User
	groups        map[string]*Group
	domains       map[string]*Domain
	certificates  map[string]*Certificate
	plans         []Plan
	subscriptions map[string]*Subscription
	oidcs         map[string]*OidcConfig
	ids           *core.IDGenerator
	logger        *slog.Logger
}

var _ Store = (*MemoryStore)(nil)

func NewMemoryStore(logger *slog.Logger) *MemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	s := &MemoryStore{
		services:      make(map[string]*Service),
		routes:        make(map[string]*Route),
		users:         make(map[string]*User),
		groups:        make(map[string]*Group),
		domains:       make(map[string]*Domain),
		certificates:  make(map[string]*Certificate),
		subscriptions: make(map[string]*Subscription),
		oidcs:         make(map[string]*OidcConfig),
		ids:           core.NewIDGenerator(core.DefaultIDBase()),
		logger:        logger,
	}
	s.plans = seedPlans()
	return s
}

// seedPlans returns the fixed plan catalog. IDs are deterministic (derived
// from the plan name) so lookups stay stable across restarts. The values
// mirror the real service's public pricing.
func seedPlans() []Plan {
	now := time.Now().UTC()
	plan := func(name, price, description string, maxServices, maxRequests int, unit string, overage *Overage) Plan {
		return Plan{
			ID:              uuid.NewSHA1(uuid.NameSpaceURL, []byte("sakumock-apigw-plan-"+name)).String(),
			CreatedAt:       now,
			UpdatedAt:       now,
			Name:            name,
			Price:           price,
			Description:     description,
			MaxServices:     maxServices,
			MaxRequests:     maxRequests,
			MaxRequestsUnit: unit,
			Overage:         overage,
		}
	}
	return []Plan{
		plan("トライアル", "0", "APIゲートウェイ for トライアル", 1, 10, "second", nil),
		plan("エンタープライズ", "22000", "APIゲートウェイ for エンタープライズ", 10, 10000000, "month",
			&Overage{UnitRequests: 1000000, UnitPrice: "1100"}),
	}
}

func (s *MemoryStore) Close() {}

// newRouteHost derives the auto-issued gateway hostname for a service. The
// .localhost suffix resolves to loopback on typical systems, so the data
// plane is reachable without DNS setup (see apprun for the same convention).
func newRouteHost(serviceID string) string {
	hex := strings.ReplaceAll(serviceID, "-", "")
	return "site-" + hex[:12] + ".localhost"
}

func now() time.Time { return time.Now().UTC() }

func ptr[T any](v T) *T { return &v }

// --- copy helpers (stores hand out copies so callers never share memory) ---

func copyService(v *Service) Service {
	c := *v
	c.Tags = append([]string(nil), v.Tags...)
	if v.Oidc != nil {
		c.Oidc = ptr(*v.Oidc)
	}
	if v.CorsConfig != nil {
		cc := *v.CorsConfig
		cc.AccessControlAllowMethods = append([]string(nil), v.CorsConfig.AccessControlAllowMethods...)
		c.CorsConfig = &cc
	}
	if v.ObjectStorage != nil {
		c.ObjectStorage = ptr(*v.ObjectStorage)
	}
	if v.Retries != nil {
		c.Retries = ptr(*v.Retries)
	}
	return c
}

func copyRoute(v *Route) Route {
	c := *v
	c.Tags = append([]string(nil), v.Tags...)
	c.Hosts = append([]string(nil), v.Hosts...)
	c.Methods = append([]string(nil), v.Methods...)
	if v.StripPath != nil {
		c.StripPath = ptr(*v.StripPath)
	}
	if v.RequestBuffering != nil {
		c.RequestBuffering = ptr(*v.RequestBuffering)
	}
	if v.ResponseBuffering != nil {
		c.ResponseBuffering = ptr(*v.ResponseBuffering)
	}
	if v.IPRestriction != nil {
		ip := *v.IPRestriction
		ip.IPs = append([]string(nil), v.IPRestriction.IPs...)
		c.IPRestriction = &ip
	}
	if v.Authorization != nil {
		a := *v.Authorization
		a.Groups = append([]RouteAuthorization(nil), v.Authorization.Groups...)
		c.Authorization = &a
	}
	// Transformations are read-modify-free after storage; a shallow copy of
	// the pointer target keeps callers from mutating the stored value's top
	// level, which is all the handlers do.
	if v.RequestTransform != nil {
		c.RequestTransform = ptr(*v.RequestTransform)
	}
	if v.ResponseTransform != nil {
		c.ResponseTransform = ptr(*v.ResponseTransform)
	}
	return c
}

func copyUser(v *User) User {
	c := *v
	c.Tags = append([]string(nil), v.Tags...)
	c.GroupIDs = append([]string(nil), v.GroupIDs...)
	c.Groups = append([]Group(nil), v.Groups...)
	if v.IPRestriction != nil {
		ip := *v.IPRestriction
		ip.IPs = append([]string(nil), v.IPRestriction.IPs...)
		c.IPRestriction = &ip
	}
	if v.Auth != nil {
		a := UserAuthentication{}
		if v.Auth.BasicAuth != nil {
			a.BasicAuth = ptr(*v.Auth.BasicAuth)
		}
		if v.Auth.Jwt != nil {
			a.Jwt = ptr(*v.Auth.Jwt)
		}
		if v.Auth.HmacAuth != nil {
			a.HmacAuth = ptr(*v.Auth.HmacAuth)
		}
		c.Auth = &a
	}
	return c
}

func copyGroup(v *Group) Group {
	c := *v
	c.Tags = append([]string(nil), v.Tags...)
	return c
}

func copyCertificate(v *Certificate) Certificate {
	c := *v
	if v.RSA != nil {
		c.RSA = ptr(*v.RSA)
	}
	if v.ECDSA != nil {
		c.ECDSA = ptr(*v.ECDSA)
	}
	return c
}

func copySubscription(v *Subscription) Subscription {
	c := *v
	if v.Service != nil {
		c.Service = ptr(*v.Service)
	}
	return c
}

func copyOidc(v *OidcConfig) OidcConfig {
	c := *v
	c.AuthenticationMethods = append([]string(nil), v.AuthenticationMethods...)
	c.Scopes = append([]string(nil), v.Scopes...)
	c.TokenAudiences = append([]string(nil), v.TokenAudiences...)
	return c
}

// --- Services ---

func (s *MemoryStore) CreateService(svc Service, subscriptionID string) (Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[subscriptionID]
	if !ok {
		return Service{}, errNotFound("subscription %s not found", subscriptionID)
	}
	if sub.BoundServiceID != "" {
		return Service{}, errConflict("subscription %s is already bound to another service", subscriptionID)
	}
	for _, other := range s.services {
		if other.Name == svc.Name {
			return Service{}, errConflict("service %s already exists", svc.Name)
		}
	}
	if err := s.resolveOidcLocked(&svc); err != nil {
		return Service{}, err
	}

	svc.ID = uuid.NewString()
	svc.CreatedAt = now()
	svc.UpdatedAt = svc.CreatedAt
	svc.RouteHost = newRouteHost(svc.ID)
	svc.SubscriptionID = subscriptionID
	applyServiceDefaults(&svc)

	stored := copyService(&svc)
	s.services[svc.ID] = &stored
	sub.BoundServiceID = svc.ID
	sub.Service = &SubscriptionService{ID: svc.ID, Name: svc.Name}
	sub.UpdatedAt = now()
	s.logger.Debug("service created", "id", svc.ID, "name", svc.Name, "routeHost", svc.RouteHost)
	return svc, nil
}

// resolveOidcLocked validates the OIDC reference and fills its display name.
func (s *MemoryStore) resolveOidcLocked(svc *Service) error {
	if svc.Oidc == nil || svc.Oidc.ID == "" {
		svc.Oidc = nil
		if svc.Authentication == "oidc" {
			return errBadRequest("authentication oidc requires an oidc reference")
		}
		return nil
	}
	o, ok := s.oidcs[svc.Oidc.ID]
	if !ok {
		return errNotFound("oidc %s not found", svc.Oidc.ID)
	}
	svc.Oidc = &OidcSummary{ID: o.ID, Name: o.Name}
	return nil
}

// applyServiceDefaults fills the spec-declared defaults so responses always
// carry effective values, as the real API does.
func applyServiceDefaults(svc *Service) {
	if svc.Path == "" {
		svc.Path = "/"
	}
	if svc.Port == 0 {
		if svc.Protocol == "https" {
			svc.Port = 443
		} else {
			svc.Port = 80
		}
	}
	if svc.Retries == nil {
		svc.Retries = ptr(5)
	}
	if svc.ConnectTimeout == 0 {
		svc.ConnectTimeout = 60000
	}
	if svc.WriteTimeout == 0 {
		svc.WriteTimeout = 60000
	}
	if svc.ReadTimeout == 0 {
		svc.ReadTimeout = 60000
	}
	if svc.Authentication == "" {
		svc.Authentication = "none"
	}
	if svc.ObjectStorage != nil && svc.ObjectStorage.UseDocumentIndex == nil {
		svc.ObjectStorage.UseDocumentIndex = ptr(true)
	}
}

func (s *MemoryStore) ListServices() []Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Service, 0, len(s.services))
	for _, v := range s.services {
		out = append(out, copyService(v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *MemoryStore) GetService(id string) (Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.services[id]
	if !ok {
		return Service{}, errNotFound("service %s not found", id)
	}
	return copyService(v), nil
}

func (s *MemoryStore) UpdateService(id string, svc Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.services[id]
	if !ok {
		return errNotFound("service %s not found", id)
	}
	for _, other := range s.services {
		if other.ID != id && other.Name == svc.Name {
			return errConflict("service %s already exists", svc.Name)
		}
	}
	if err := s.resolveOidcLocked(&svc); err != nil {
		return err
	}
	svc.ID = cur.ID
	svc.CreatedAt = cur.CreatedAt
	svc.UpdatedAt = now()
	svc.RouteHost = cur.RouteHost
	svc.SubscriptionID = cur.SubscriptionID
	applyServiceDefaults(&svc)
	stored := copyService(&svc)
	s.services[id] = &stored
	if sub, ok := s.subscriptions[cur.SubscriptionID]; ok && sub.Service != nil {
		sub.Service.Name = svc.Name
	}
	return nil
}

func (s *MemoryStore) DeleteService(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.services[id]
	if !ok {
		return errNotFound("service %s not found", id)
	}
	for _, rt := range s.routes {
		if rt.ServiceID == id {
			return errBadRequest("an existing route references this service")
		}
	}
	if sub, ok := s.subscriptions[cur.SubscriptionID]; ok {
		sub.BoundServiceID = ""
		sub.Service = nil
		sub.UpdatedAt = now()
	}
	delete(s.services, id)
	return nil
}

func (s *MemoryStore) SubscriptionNameOf(serviceID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	svc, ok := s.services[serviceID]
	if !ok {
		return ""
	}
	if sub, ok := s.subscriptions[svc.SubscriptionID]; ok {
		return sub.Name
	}
	return ""
}

// --- Routes ---

func (s *MemoryStore) CreateRoute(serviceID string, rt Route) (Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	svc, ok := s.services[serviceID]
	if !ok {
		return Route{}, errNotFound("service %s not found", serviceID)
	}
	for _, other := range s.routes {
		if other.ServiceID == serviceID && other.Name == rt.Name {
			return Route{}, errConflict("route %s already exists", rt.Name)
		}
	}
	if err := s.validateRouteHostsLocked(svc, rt.Hosts); err != nil {
		return Route{}, err
	}

	rt.ID = uuid.NewString()
	rt.CreatedAt = now()
	rt.UpdatedAt = rt.CreatedAt
	rt.ServiceID = serviceID
	rt.Host = svc.RouteHost
	applyRouteDefaults(&rt)

	stored := copyRoute(&rt)
	s.routes[rt.ID] = &stored
	s.logger.Debug("route created", "id", rt.ID, "name", rt.Name, "service", serviceID)
	return rt, nil
}

// validateRouteHostsLocked enforces the spec rule that each entry of hosts is
// either the service's auto-issued routeHost or a registered custom domain.
func (s *MemoryStore) validateRouteHostsLocked(svc *Service, hosts []string) error {
	for _, h := range hosts {
		if h == svc.RouteHost {
			continue
		}
		found := false
		for _, d := range s.domains {
			if d.DomainName == h {
				found = true
				break
			}
		}
		if !found {
			return errBadRequest("host %s is neither the service routeHost nor a registered domain", h)
		}
	}
	return nil
}

func applyRouteDefaults(rt *Route) {
	if rt.HTTPSRedirectStatusCode == 0 {
		rt.HTTPSRedirectStatusCode = 426
	}
	if rt.StripPath == nil {
		rt.StripPath = ptr(true)
	}
	if rt.RequestBuffering == nil {
		rt.RequestBuffering = ptr(true)
	}
	if rt.ResponseBuffering == nil {
		rt.ResponseBuffering = ptr(true)
	}
	// The spec allows all methods when none are specified.
	if len(rt.Methods) == 0 {
		rt.Methods = allHTTPMethods()
	}
}

func allHTTPMethods() []string {
	return []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD", "CONNECT", "TRACE"}
}

// RoutesByHost returns the routes whose effective host set (hosts, or the
// auto-issued host when hosts is empty) contains host, in creation order.
// The data plane calls this per request.
func (s *MemoryStore) RoutesByHost(host string) []Route {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Route
	for _, rt := range s.routes {
		hosts := rt.Hosts
		if len(hosts) == 0 {
			hosts = []string{rt.Host}
		}
		if slices.Contains(hosts, host) {
			out = append(out, copyRoute(rt))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *MemoryStore) ListRoutes(serviceID string) ([]Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.services[serviceID]; !ok {
		return nil, errNotFound("service %s not found", serviceID)
	}
	var out []Route
	for _, rt := range s.routes {
		if rt.ServiceID == serviceID {
			out = append(out, copyRoute(rt))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *MemoryStore) getRouteLocked(serviceID, routeID string) (*Route, error) {
	if _, ok := s.services[serviceID]; !ok {
		return nil, errNotFound("service %s not found", serviceID)
	}
	rt, ok := s.routes[routeID]
	if !ok || rt.ServiceID != serviceID {
		return nil, errNotFound("route %s not found", routeID)
	}
	return rt, nil
}

func (s *MemoryStore) GetRoute(serviceID, routeID string) (Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rt, err := s.getRouteLocked(serviceID, routeID)
	if err != nil {
		return Route{}, err
	}
	return copyRoute(rt), nil
}

func (s *MemoryStore) UpdateRoute(serviceID, routeID string, rt Route) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.getRouteLocked(serviceID, routeID)
	if err != nil {
		return err
	}
	for _, other := range s.routes {
		if other.ServiceID == serviceID && other.ID != routeID && other.Name == rt.Name {
			return errConflict("route %s already exists", rt.Name)
		}
	}
	if err := s.validateRouteHostsLocked(s.services[serviceID], rt.Hosts); err != nil {
		return err
	}
	rt.ID = cur.ID
	rt.CreatedAt = cur.CreatedAt
	rt.UpdatedAt = now()
	rt.ServiceID = cur.ServiceID
	rt.Host = cur.Host
	rt.Authorization = cur.Authorization
	rt.RequestTransform = cur.RequestTransform
	rt.ResponseTransform = cur.ResponseTransform
	applyRouteDefaults(&rt)
	stored := copyRoute(&rt)
	s.routes[routeID] = &stored
	return nil
}

func (s *MemoryStore) DeleteRoute(serviceID, routeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.getRouteLocked(serviceID, routeID); err != nil {
		return err
	}
	delete(s.routes, routeID)
	return nil
}

func (s *MemoryStore) SetRouteAuthorization(serviceID, routeID string, cfg *RouteAuthorizationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt, err := s.getRouteLocked(serviceID, routeID)
	if err != nil {
		return err
	}
	if cfg != nil {
		for i, g := range cfg.Groups {
			stored, ok := s.groups[g.ID]
			if !ok {
				return errNotFound("group %s not found", g.ID)
			}
			cfg.Groups[i].Name = stored.Name
		}
	}
	rt.Authorization = cfg
	rt.UpdatedAt = now()
	return nil
}

func (s *MemoryStore) SetRequestTransformation(serviceID, routeID string, tr *RequestTransformation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt, err := s.getRouteLocked(serviceID, routeID)
	if err != nil {
		return err
	}
	rt.RequestTransform = tr
	rt.UpdatedAt = now()
	return nil
}

func (s *MemoryStore) SetResponseTransformation(serviceID, routeID string, tr *ResponseTransformation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt, err := s.getRouteLocked(serviceID, routeID)
	if err != nil {
		return err
	}
	rt.ResponseTransform = tr
	rt.UpdatedAt = now()
	return nil
}

// --- Users ---

func (s *MemoryStore) CreateUser(u User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, other := range s.users {
		if other.Name == u.Name {
			return User{}, errConflict("user %s already exists", u.Name)
		}
	}
	u.ID = uuid.NewString()
	u.CreatedAt = now()
	u.UpdatedAt = u.CreatedAt
	u.Groups = nil
	u.GroupIDs = nil
	u.Auth = nil
	stored := copyUser(&u)
	s.users[u.ID] = &stored
	return s.userWithGroupsLocked(&stored), nil
}

// userWithGroupsLocked returns a copy with Groups resolved from GroupIDs.
func (s *MemoryStore) userWithGroupsLocked(u *User) User {
	c := copyUser(u)
	c.Groups = []Group{}
	for _, gid := range u.GroupIDs {
		if g, ok := s.groups[gid]; ok {
			c.Groups = append(c.Groups, copyGroup(g))
		}
	}
	return c
}

func (s *MemoryStore) ListUsers() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, s.userWithGroupsLocked(u))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *MemoryStore) GetUser(id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, errNotFound("user %s not found", id)
	}
	return s.userWithGroupsLocked(u), nil
}

func (s *MemoryStore) UpdateUser(id string, u User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.users[id]
	if !ok {
		return errNotFound("user %s not found", id)
	}
	for _, other := range s.users {
		if other.ID != id && other.Name == u.Name {
			return errConflict("user %s already exists", u.Name)
		}
	}
	cur.Name = u.Name
	cur.CustomID = u.CustomID
	cur.Tags = append([]string(nil), u.Tags...)
	if u.IPRestriction != nil {
		ip := *u.IPRestriction
		ip.IPs = append([]string(nil), u.IPRestriction.IPs...)
		cur.IPRestriction = &ip
	} else {
		cur.IPRestriction = nil
	}
	cur.UpdatedAt = now()
	return nil
}

func (s *MemoryStore) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return errNotFound("user %s not found", id)
	}
	delete(s.users, id)
	return nil
}

func (s *MemoryStore) ListUserGroups(userID string) ([]Group, map[string]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[userID]
	if !ok {
		return nil, nil, errNotFound("user %s not found", userID)
	}
	assigned := make(map[string]bool, len(u.GroupIDs))
	for _, gid := range u.GroupIDs {
		assigned[gid] = true
	}
	groups := make([]Group, 0, len(s.groups))
	for _, g := range s.groups {
		groups = append(groups, copyGroup(g))
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].CreatedAt.Before(groups[j].CreatedAt) })
	return groups, assigned, nil
}

func (s *MemoryStore) AssignUserGroup(userID, groupIDOrName string, assigned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[userID]
	if !ok {
		return errNotFound("user %s not found", userID)
	}
	var group *Group
	if g, ok := s.groups[groupIDOrName]; ok {
		group = g
	} else {
		for _, g := range s.groups {
			if g.Name == groupIDOrName {
				group = g
				break
			}
		}
	}
	if group == nil {
		return errNotFound("group %s not found", groupIDOrName)
	}
	idx := -1
	for i, gid := range u.GroupIDs {
		if gid == group.ID {
			idx = i
			break
		}
	}
	switch {
	case assigned && idx < 0:
		u.GroupIDs = append(u.GroupIDs, group.ID)
	case !assigned && idx >= 0:
		u.GroupIDs = append(u.GroupIDs[:idx], u.GroupIDs[idx+1:]...)
	}
	u.UpdatedAt = now()
	return nil
}

func (s *MemoryStore) GetUserAuthentication(userID string) (*UserAuthentication, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[userID]
	if !ok {
		return nil, errNotFound("user %s not found", userID)
	}
	if u.Auth == nil {
		return &UserAuthentication{}, nil
	}
	c := copyUser(u)
	return c.Auth, nil
}

// UpsertUserAuthentication merges the provided credentials into the user's
// stored ones: a credential type present in auth replaces (or creates) the
// stored credential of that type, keeping its ID; absent types are untouched.
func (s *MemoryStore) UpsertUserAuthentication(userID string, auth UserAuthentication) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[userID]
	if !ok {
		return errNotFound("user %s not found", userID)
	}
	if u.Auth == nil {
		u.Auth = &UserAuthentication{}
	}
	t := now()
	if auth.BasicAuth != nil {
		if u.Auth.BasicAuth == nil {
			u.Auth.BasicAuth = &BasicAuth{ID: uuid.NewString(), CreatedAt: t}
		}
		u.Auth.BasicAuth.UserName = auth.BasicAuth.UserName
		u.Auth.BasicAuth.Password = auth.BasicAuth.Password
		u.Auth.BasicAuth.UpdatedAt = t
	}
	if auth.Jwt != nil {
		if u.Auth.Jwt == nil {
			u.Auth.Jwt = &JwtAuth{ID: uuid.NewString(), CreatedAt: t}
		}
		u.Auth.Jwt.Key = auth.Jwt.Key
		u.Auth.Jwt.Secret = auth.Jwt.Secret
		u.Auth.Jwt.Algorithm = auth.Jwt.Algorithm
		u.Auth.Jwt.UpdatedAt = t
	}
	if auth.HmacAuth != nil {
		if u.Auth.HmacAuth == nil {
			u.Auth.HmacAuth = &HmacAuth{ID: uuid.NewString(), CreatedAt: t}
		}
		u.Auth.HmacAuth.UserName = auth.HmacAuth.UserName
		u.Auth.HmacAuth.Secret = auth.HmacAuth.Secret
		u.Auth.HmacAuth.UpdatedAt = t
	}
	u.UpdatedAt = t
	return nil
}

// --- Groups ---

func (s *MemoryStore) CreateGroup(g Group) (Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, other := range s.groups {
		if other.Name == g.Name {
			return Group{}, errConflict("group %s already exists", g.Name)
		}
	}
	g.ID = uuid.NewString()
	g.CreatedAt = now()
	g.UpdatedAt = g.CreatedAt
	stored := copyGroup(&g)
	s.groups[g.ID] = &stored
	return g, nil
}

func (s *MemoryStore) ListGroups() []Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Group, 0, len(s.groups))
	for _, g := range s.groups {
		out = append(out, copyGroup(g))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *MemoryStore) GetGroup(id string) (Group, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.groups[id]
	if !ok {
		return Group{}, errNotFound("group %s not found", id)
	}
	return copyGroup(g), nil
}

func (s *MemoryStore) UpdateGroup(id string, g Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.groups[id]
	if !ok {
		return errNotFound("group %s not found", id)
	}
	for _, other := range s.groups {
		if other.ID != id && other.Name == g.Name {
			return errConflict("group %s already exists", g.Name)
		}
	}
	cur.Name = g.Name
	cur.Tags = append([]string(nil), g.Tags...)
	cur.UpdatedAt = now()
	// Route authorizations denormalize the group name; keep them in sync.
	for _, rt := range s.routes {
		if rt.Authorization == nil {
			continue
		}
		for i := range rt.Authorization.Groups {
			if rt.Authorization.Groups[i].ID == id {
				rt.Authorization.Groups[i].Name = g.Name
			}
		}
	}
	return nil
}

// DeleteGroup removes the group and cascades: users lose the membership and
// route authorizations drop the entry (the spec offers no refusal status for
// referenced groups, so cascading keeps the mock usable).
func (s *MemoryStore) DeleteGroup(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[id]; !ok {
		return errNotFound("group %s not found", id)
	}
	delete(s.groups, id)
	for _, u := range s.users {
		for i, gid := range u.GroupIDs {
			if gid == id {
				u.GroupIDs = append(u.GroupIDs[:i], u.GroupIDs[i+1:]...)
				break
			}
		}
	}
	for _, rt := range s.routes {
		if rt.Authorization == nil {
			continue
		}
		groups := rt.Authorization.Groups[:0]
		for _, g := range rt.Authorization.Groups {
			if g.ID != id {
				groups = append(groups, g)
			}
		}
		rt.Authorization.Groups = groups
	}
	return nil
}

// --- Domains ---

func (s *MemoryStore) CreateDomain(d Domain) (Domain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, other := range s.domains {
		if other.DomainName == d.DomainName {
			return Domain{}, errConflict("domain %s already exists", d.DomainName)
		}
	}
	if err := s.resolveCertificateLocked(&d); err != nil {
		return Domain{}, err
	}
	d.ID = uuid.NewString()
	d.CreatedAt = now()
	d.UpdatedAt = d.CreatedAt
	stored := d
	s.domains[d.ID] = &stored
	return d, nil
}

func (s *MemoryStore) resolveCertificateLocked(d *Domain) error {
	if d.CertificateID == "" {
		d.CertificateName = ""
		return nil
	}
	c, ok := s.certificates[d.CertificateID]
	if !ok {
		return errNotFound("certificate %s not found", d.CertificateID)
	}
	d.CertificateName = c.Name
	return nil
}

func (s *MemoryStore) ListDomains() []Domain {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Domain, 0, len(s.domains))
	for _, d := range s.domains {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *MemoryStore) UpdateDomain(id string, certificateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.domains[id]
	if !ok {
		return errNotFound("domain %s not found", id)
	}
	d.CertificateID = certificateID
	if err := s.resolveCertificateLocked(d); err != nil {
		return err
	}
	d.UpdatedAt = now()
	return nil
}

func (s *MemoryStore) DeleteDomain(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.domains[id]
	if !ok {
		return errNotFound("domain %s not found", id)
	}
	for _, rt := range s.routes {
		if slices.Contains(rt.Hosts, d.DomainName) {
			return errBadRequest("an existing route references this domain")
		}
	}
	delete(s.domains, id)
	return nil
}

// --- Certificates ---

func (s *MemoryStore) CreateCertificate(c Certificate) (Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, other := range s.certificates {
		if other.Name == c.Name {
			return Certificate{}, errConflict("certificate %s already exists", c.Name)
		}
	}
	c.ID = uuid.NewString()
	c.CreatedAt = now()
	c.UpdatedAt = c.CreatedAt
	stored := copyCertificate(&c)
	s.certificates[c.ID] = &stored
	return c, nil
}

func (s *MemoryStore) ListCertificates() []Certificate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Certificate, 0, len(s.certificates))
	for _, c := range s.certificates {
		out = append(out, copyCertificate(c))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *MemoryStore) UpdateCertificate(id string, c Certificate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.certificates[id]
	if !ok {
		return errNotFound("certificate %s not found", id)
	}
	for _, other := range s.certificates {
		if other.ID != id && other.Name == c.Name {
			return errConflict("certificate %s already exists", c.Name)
		}
	}
	cur.Name = c.Name
	if c.RSA != nil {
		cur.RSA = ptr(*c.RSA)
	}
	if c.ECDSA != nil {
		cur.ECDSA = ptr(*c.ECDSA)
	}
	cur.UpdatedAt = now()
	// Domains denormalize the certificate name via resolveCertificateLocked
	// on read paths; nothing to sync here because CertificateName is
	// recomputed when domains are listed.
	for _, d := range s.domains {
		if d.CertificateID == id {
			d.CertificateName = c.Name
		}
	}
	return nil
}

func (s *MemoryStore) DeleteCertificate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.certificates[id]; !ok {
		return errNotFound("certificate %s not found", id)
	}
	for _, d := range s.domains {
		if d.CertificateID == id {
			return errBadRequest("an existing domain references this certificate")
		}
	}
	delete(s.certificates, id)
	return nil
}

// --- Plans & subscriptions ---

func (s *MemoryStore) ListPlans() []Plan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Plan, len(s.plans))
	copy(out, s.plans)
	return out
}

func (s *MemoryStore) CreateSubscription(planID, name string) (Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var plan *Plan
	for i := range s.plans {
		if s.plans[i].ID == planID {
			plan = &s.plans[i]
			break
		}
	}
	if plan == nil {
		return Subscription{}, errBadRequest("plan %s not found", planID)
	}
	if len(s.subscriptions) >= maxSubscriptions {
		return Subscription{}, errBadRequest("subscription limit (%d) reached", maxSubscriptions)
	}
	for _, other := range s.subscriptions {
		if other.Name == name {
			return Subscription{}, errBadRequest("subscription %s already exists", name)
		}
	}
	resourceID, err := strconv.ParseInt(s.ids.Next(), 10, 64)
	if err != nil {
		return Subscription{}, &StoreError{Status: 500, Message: "generate resource ID: " + err.Error()}
	}
	sub := Subscription{
		ID:         uuid.NewString(),
		CreatedAt:  now(),
		UpdatedAt:  now(),
		Name:       name,
		PlanID:     planID,
		ResourceID: resourceID,
	}
	stored := copySubscription(&sub)
	s.subscriptions[sub.ID] = &stored
	s.logger.Debug("subscription created", "id", sub.ID, "name", name, "plan", plan.Name)
	return sub, nil
}

func (s *MemoryStore) ListSubscriptions() []Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Subscription, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		out = append(out, copySubscription(sub))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *MemoryStore) GetSubscription(id string) (Subscription, Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.subscriptions[id]
	if !ok {
		return Subscription{}, Plan{}, errNotFound("subscription %s not found", id)
	}
	var plan Plan
	for _, p := range s.plans {
		if p.ID == sub.PlanID {
			plan = p
			break
		}
	}
	return copySubscription(sub), plan, nil
}

func (s *MemoryStore) UpdateSubscription(id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subscriptions[id]
	if !ok {
		// PUT /subscriptions/{id} declares no 404, so an unknown id is a 400.
		return errBadRequest("subscription %s not found", id)
	}
	for _, other := range s.subscriptions {
		if other.ID != id && other.Name == name {
			return errBadRequest("subscription %s already exists", name)
		}
	}
	sub.Name = name
	sub.UpdatedAt = now()
	return nil
}

func (s *MemoryStore) DeleteSubscription(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subscriptions[id]
	if !ok {
		return errNotFound("subscription %s not found", id)
	}
	if sub.BoundServiceID != "" {
		return errBadRequest("subscription %s is bound to a service", id)
	}
	delete(s.subscriptions, id)
	return nil
}

// --- OIDC ---

func (s *MemoryStore) CreateOidc(o OidcConfig) (OidcConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, other := range s.oidcs {
		if other.Name == o.Name {
			return OidcConfig{}, errConflict("oidc %s already exists", o.Name)
		}
	}
	o.ID = uuid.NewString()
	o.CreatedAt = now()
	o.UpdatedAt = o.CreatedAt
	stored := copyOidc(&o)
	s.oidcs[o.ID] = &stored
	return o, nil
}

func (s *MemoryStore) ListOidcs() []OidcConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]OidcConfig, 0, len(s.oidcs))
	for _, o := range s.oidcs {
		out = append(out, copyOidc(o))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *MemoryStore) GetOidc(id string) (OidcConfig, []Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.oidcs[id]
	if !ok {
		return OidcConfig{}, nil, errNotFound("oidc %s not found", id)
	}
	var services []Service
	for _, svc := range s.services {
		if svc.Oidc != nil && svc.Oidc.ID == id {
			services = append(services, copyService(svc))
		}
	}
	sort.Slice(services, func(i, j int) bool { return services[i].CreatedAt.Before(services[j].CreatedAt) })
	return copyOidc(o), services, nil
}

func (s *MemoryStore) UpdateOidc(id string, o OidcConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.oidcs[id]
	if !ok {
		return errNotFound("oidc %s not found", id)
	}
	for _, other := range s.oidcs {
		if other.ID != id && other.Name == o.Name {
			return errConflict("oidc %s already exists", o.Name)
		}
	}
	o.ID = cur.ID
	o.CreatedAt = cur.CreatedAt
	o.UpdatedAt = now()
	stored := copyOidc(&o)
	s.oidcs[id] = &stored
	// Services denormalize the OIDC name in their summary.
	for _, svc := range s.services {
		if svc.Oidc != nil && svc.Oidc.ID == id {
			svc.Oidc.Name = o.Name
		}
	}
	return nil
}

func (s *MemoryStore) DeleteOidc(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.oidcs[id]; !ok {
		return errNotFound("oidc %s not found", id)
	}
	for _, svc := range s.services {
		if svc.Oidc != nil && svc.Oidc.ID == id {
			return errConflict("an existing service references this oidc")
		}
	}
	delete(s.oidcs, id)
	return nil
}
