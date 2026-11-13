package apigw

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// storeSnapshot is the store's form in the state file (--state-dir). Every map
// is keyed by resource ID. The plan catalog is not part of it: it is seeded
// with deterministic IDs at every start, so subscriptions keep referring to
// the same plans.
//
// The record types are shaped after the API's JSON and hide what the API
// never returns (`json:"-"`); the *Snapshot wrappers below add those fields
// back, so nothing the store needs is lost in the file.
type storeSnapshot struct {
	Services      map[string]serviceSnapshot     `json:"services"`
	Routes        map[string]routeSnapshot       `json:"routes"`
	Users         map[string]userSnapshot        `json:"users"`
	Groups        map[string]*Group              `json:"groups"`
	Domains       map[string]*Domain             `json:"domains"`
	Certificates  map[string]certificateSnapshot `json:"certificates"`
	Subscriptions map[string]*Subscription       `json:"subscriptions"`
	Oidcs         map[string]*OidcConfig         `json:"oidcs"`
}

// serviceSnapshot is a Service with the subscription it is bound to. That
// binding is saved only here: Subscription.BoundServiceID and
// Subscription.Service are rebuilt from it on restore.
type serviceSnapshot struct {
	*Service
	SubscriptionID string `json:"subscriptionId"`
}

// routeSnapshot is a Route with its authorization and transformations, which
// the API serves from separate endpoints.
type routeSnapshot struct {
	*Route
	Authorization     *RouteAuthorizationConfig `json:"authorization,omitempty"`
	RequestTransform  *RequestTransformation    `json:"requestTransformation,omitempty"`
	ResponseTransform *ResponseTransformation   `json:"responseTransformation,omitempty"`
}

// userSnapshot is a User with its credentials and group memberships.
type userSnapshot struct {
	*User
	// GroupIDs takes the place of User.Groups (resolved from the group store
	// on read) under the "groups" key: the member group IDs, in assignment
	// order.
	GroupIDs []string            `json:"groups"`
	Auth     *UserAuthentication `json:"authentication,omitempty"`
}

// certificateSnapshot is a Certificate with its PEM material.
type certificateSnapshot struct {
	*Certificate
	RSA   *certificateDetailsSnapshot `json:"rsa,omitempty"`
	ECDSA *certificateDetailsSnapshot `json:"ecdsa,omitempty"`
}

// certificateDetailsSnapshot is CertificateDetails with the write-only PEMs
// included. It must keep the same fields so the two convert into each other.
type certificateDetailsSnapshot struct {
	ExpiredAt time.Time `json:"expiredAt"`
	CertPEM   string    `json:"cert"`
	KeyPEM    string    `json:"key"`
}

// Snapshot returns every resource of the store, credentials and certificate
// keys included, as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := storeSnapshot{
		Services:      make(map[string]serviceSnapshot, len(s.services)),
		Routes:        make(map[string]routeSnapshot, len(s.routes)),
		Users:         make(map[string]userSnapshot, len(s.users)),
		Groups:        s.groups,
		Domains:       s.domains,
		Certificates:  make(map[string]certificateSnapshot, len(s.certificates)),
		Subscriptions: s.subscriptions,
		Oidcs:         s.oidcs,
	}
	for id, svc := range s.services {
		snap.Services[id] = serviceSnapshot{Service: svc, SubscriptionID: svc.SubscriptionID}
	}
	for id, rt := range s.routes {
		snap.Routes[id] = routeSnapshot{
			Route:             rt,
			Authorization:     rt.Authorization,
			RequestTransform:  rt.RequestTransform,
			ResponseTransform: rt.ResponseTransform,
		}
	}
	for id, u := range s.users {
		snap.Users[id] = userSnapshot{User: u, GroupIDs: u.GroupIDs, Auth: u.Auth}
	}
	for id, c := range s.certificates {
		snap.Certificates[id] = certificateSnapshot{
			Certificate: c,
			RSA:         (*certificateDetailsSnapshot)(c.RSA),
			ECDSA:       (*certificateDetailsSnapshot)(c.ECDSA),
		}
	}
	return json.MarshalIndent(snap, "", "  ")
}

// Restore replaces the store's resources with a snapshot, rebuilds the
// subscription-to-service bindings from the services, and advances the ID
// generator past the restored subscription resource IDs so new subscriptions
// never reuse one. A snapshot with dangling references that requests would
// trip over (a route without its service, a service without its subscription,
// a subscription to an unknown plan) is rejected as a whole.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	groups, err := restoredRecords("group", snap.Groups, func(g *Group) string { return g.ID })
	if err != nil {
		return err
	}
	domains, err := restoredRecords("domain", snap.Domains, func(d *Domain) string { return d.ID })
	if err != nil {
		return err
	}
	oidcs, err := restoredRecords("oidc", snap.Oidcs, func(o *OidcConfig) string { return o.ID })
	if err != nil {
		return err
	}
	subscriptions, err := restoredRecords("subscription", snap.Subscriptions, func(sub *Subscription) string { return sub.ID })
	if err != nil {
		return err
	}
	for id, sub := range subscriptions {
		if !s.hasPlan(sub.PlanID) {
			return fmt.Errorf("subscription %q: unknown plan %q", id, sub.PlanID)
		}
		sub.BoundServiceID = ""
		sub.Service = nil
	}

	services := make(map[string]*Service, len(snap.Services))
	for id, v := range snap.Services {
		svc := v.Service
		if svc == nil || svc.ID != id {
			return fmt.Errorf("service %q: record is missing or has a different id", id)
		}
		svc.SubscriptionID = v.SubscriptionID
		sub, ok := subscriptions[svc.SubscriptionID]
		if !ok {
			return fmt.Errorf("service %q: unknown subscription %q", id, svc.SubscriptionID)
		}
		if sub.BoundServiceID != "" {
			return fmt.Errorf("subscription %q: bound to both service %q and %q", sub.ID, sub.BoundServiceID, id)
		}
		sub.BoundServiceID = id
		sub.Service = &SubscriptionService{ID: id, Name: svc.Name}
		services[id] = svc
	}

	routes := make(map[string]*Route, len(snap.Routes))
	for id, v := range snap.Routes {
		rt := v.Route
		if rt == nil || rt.ID != id {
			return fmt.Errorf("route %q: record is missing or has a different id", id)
		}
		if _, ok := services[rt.ServiceID]; !ok {
			return fmt.Errorf("route %q: unknown service %q", id, rt.ServiceID)
		}
		rt.Authorization = v.Authorization
		rt.RequestTransform = v.RequestTransform
		rt.ResponseTransform = v.ResponseTransform
		routes[id] = rt
	}

	users := make(map[string]*User, len(snap.Users))
	for id, v := range snap.Users {
		u := v.User
		if u == nil || u.ID != id {
			return fmt.Errorf("user %q: record is missing or has a different id", id)
		}
		u.GroupIDs = v.GroupIDs
		u.Auth = v.Auth
		users[id] = u
	}

	certificates := make(map[string]*Certificate, len(snap.Certificates))
	for id, v := range snap.Certificates {
		c := v.Certificate
		if c == nil || c.ID != id {
			return fmt.Errorf("certificate %q: record is missing or has a different id", id)
		}
		c.RSA = (*CertificateDetails)(v.RSA)
		c.ECDSA = (*CertificateDetails)(v.ECDSA)
		certificates[id] = c
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.services = services
	s.routes = routes
	s.users = users
	s.groups = groups
	s.domains = domains
	s.certificates = certificates
	s.subscriptions = subscriptions
	s.oidcs = oidcs
	for _, sub := range s.subscriptions {
		s.ids.Observe(strconv.FormatInt(sub.ResourceID, 10))
	}
	return nil
}

// hasPlan reports whether the seeded catalog has the plan. The catalog never
// changes after construction, so no lock is needed.
func (s *MemoryStore) hasPlan(id string) bool {
	for _, p := range s.plans {
		if p.ID == id {
			return true
		}
	}
	return false
}

// restoredRecords checks that every record of a restored map is present under
// its own ID, and returns the map, empty rather than nil when the snapshot
// had none.
func restoredRecords[T any](kind string, records map[string]*T, idOf func(*T) string) (map[string]*T, error) {
	if records == nil {
		return make(map[string]*T), nil
	}
	for id, r := range records {
		if r == nil || idOf(r) != id {
			return nil, fmt.Errorf("%s %q: record is missing or has a different id", kind, id)
		}
	}
	return records, nil
}
