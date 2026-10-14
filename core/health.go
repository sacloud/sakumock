package core

import "net/http"

// HealthPath is the mock-only liveness endpoint served by every service's
// control plane and by the shared `sakumock all` listener.
const HealthPath = InspectionPathPrefix + "health"

// HealthStatus is the body HealthHandler responds with.
type HealthStatus struct {
	Status   string   `json:"status"`
	Services []string `json:"services,omitempty"`
}

// HealthHandler responds 200 with {"status":"ok"}, listing services when
// given (the shared listener reports every mounted service).
func HealthHandler(services []string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, HealthStatus{Status: "ok", Services: services})
	}
}

// HealthRoute returns the GET HealthPath inspection route for a service's
// routeTable().
func HealthRoute() RegisteredRoute {
	return RegisteredRoute{
		Route:   Route{Method: "GET", Path: HealthPath, Description: "Liveness check", Kind: "inspection"},
		Handler: HealthHandler(nil),
	}
}

// InspectionRoutes returns the inspection routes every service appends to its
// routeTable() as raw entries: HealthRoute and SpecViolationRoutes(rv).
func InspectionRoutes(rv *ResponseValidator) []RegisteredRoute {
	return append([]RegisteredRoute{HealthRoute()}, SpecViolationRoutes(rv)...)
}
