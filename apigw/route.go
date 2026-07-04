package apigw

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	route := func(method, path, desc string, h http.HandlerFunc) core.RegisteredRoute {
		return core.RegisteredRoute{
			Route: core.Route{Method: method, Path: path, Description: desc, Kind: "api"},
			// Rate limit outermost, then spec-derived body validation, then
			// the handler.
			Handler: rl(s.validator.Middleware(method, path, h)),
		}
	}
	return []core.RegisteredRoute{
		// Services
		route("POST", "/services", "Create a service", s.handleAddService),
		route("GET", "/services", "List services", s.handleListServices),
		route("GET", "/services/{serviceId}", "Get a service", s.handleGetService),
		route("PUT", "/services/{serviceId}", "Update a service", s.handleUpdateService),
		route("DELETE", "/services/{serviceId}", "Delete a service", s.handleDeleteService),

		// Routes
		route("POST", "/services/{serviceId}/routes", "Create a route", s.handleAddRoute),
		route("GET", "/services/{serviceId}/routes", "List routes of a service", s.handleListRoutes),
		route("GET", "/services/{serviceId}/routes/{routeId}", "Get a route", s.handleGetRoute),
		route("PUT", "/services/{serviceId}/routes/{routeId}", "Update a route", s.handleUpdateRoute),
		route("DELETE", "/services/{serviceId}/routes/{routeId}", "Delete a route", s.handleDeleteRoute),

		// Route authorization & transformations
		route("PUT", "/services/{serviceId}/routes/{routeId}/authorization", "Upsert route authorization", s.handleUpsertRouteAuthorization),
		route("GET", "/services/{serviceId}/routes/{routeId}/authorization", "Get route authorization", s.handleGetRouteAuthorization),
		route("PUT", "/services/{serviceId}/routes/{routeId}/request", "Upsert request transformation", s.handleUpsertRequestTransformation),
		route("GET", "/services/{serviceId}/routes/{routeId}/request", "Get request transformation", s.handleGetRequestTransformation),
		route("PUT", "/services/{serviceId}/routes/{routeId}/response", "Upsert response transformation", s.handleUpsertResponseTransformation),
		route("GET", "/services/{serviceId}/routes/{routeId}/response", "Get response transformation", s.handleGetResponseTransformation),

		// Users
		route("POST", "/users", "Create a user", s.handleAddUser),
		route("GET", "/users", "List users", s.handleListUsers),
		route("GET", "/users/{userId}", "Get a user", s.handleGetUser),
		route("PUT", "/users/{userId}", "Update a user", s.handleUpdateUser),
		route("DELETE", "/users/{userId}", "Delete a user", s.handleDeleteUser),
		route("GET", "/users/{userId}/groups", "List group assignments of a user", s.handleGetUserGroups),
		route("PUT", "/users/{userId}/groups", "Update group assignments of a user", s.handleUpdateUserGroups),
		route("GET", "/users/{userId}/authentication", "Get user credentials", s.handleGetUserAuthentication),
		route("PUT", "/users/{userId}/authentication", "Upsert user credentials", s.handleUpsertUserAuthentication),

		// Groups
		route("POST", "/groups", "Create a group", s.handleAddGroup),
		route("GET", "/groups", "List groups", s.handleListGroups),
		route("GET", "/groups/{groupId}", "Get a group", s.handleGetGroup),
		route("PUT", "/groups/{groupId}", "Update a group", s.handleUpdateGroup),
		route("DELETE", "/groups/{groupId}", "Delete a group", s.handleDeleteGroup),

		// Domains
		route("POST", "/domains", "Create a domain", s.handleAddDomain),
		route("GET", "/domains", "List domains", s.handleListDomains),
		route("PUT", "/domains/{domainId}", "Update a domain", s.handleUpdateDomain),
		route("DELETE", "/domains/{domainId}", "Delete a domain", s.handleDeleteDomain),

		// Certificates
		route("POST", "/certificates", "Create a certificate", s.handleAddCertificate),
		route("GET", "/certificates", "List certificates", s.handleListCertificates),
		route("PUT", "/certificates/{certificateId}", "Update a certificate", s.handleUpdateCertificate),
		route("DELETE", "/certificates/{certificateId}", "Delete a certificate", s.handleDeleteCertificate),

		// Plans & subscriptions
		route("GET", "/plans", "List plans", s.handleListPlans),
		route("POST", "/subscriptions", "Create a subscription", s.handleSubscribe),
		route("GET", "/subscriptions", "List subscriptions", s.handleListSubscriptions),
		route("GET", "/subscriptions/{subscriptionId}", "Get a subscription", s.handleGetSubscription),
		route("PUT", "/subscriptions/{subscriptionId}", "Update a subscription", s.handleUpdateSubscription),
		route("DELETE", "/subscriptions/{subscriptionId}", "Delete a subscription", s.handleUnsubscribe),

		// OIDC
		route("POST", "/oidc", "Create an OIDC configuration", s.handleAddOidc),
		route("GET", "/oidc", "List OIDC configurations", s.handleListOidcs),
		route("GET", "/oidc/{oidcId}", "Get an OIDC configuration", s.handleGetOidc),
		route("PUT", "/oidc/{oidcId}", "Update an OIDC configuration", s.handleUpdateOidc),
		route("DELETE", "/oidc/{oidcId}", "Delete an OIDC configuration", s.handleDeleteOidc),
	}
}

func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
