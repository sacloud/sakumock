package apigw

import (
	"net/http"
	"strconv"
	"strings"
)

// corsDefaultMethods is the real gateway's default allow-list when
// accessControlAllowMethods is not configured.
const corsDefaultMethods = "GET,HEAD,PUT,PATCH,POST,DELETE,OPTIONS,TRACE,CONNECT"

func isCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions &&
		r.Header.Get("Origin") != "" &&
		r.Header.Get("Access-Control-Request-Method") != ""
}

// handleCORSPreflight answers a CORS preflight for a service with corsConfig.
// It runs before authentication — browsers send preflights without
// credentials, so an auth-protected route must still answer them (the real
// gateway's CORS handling likewise precedes its auth plugins). It reports
// whether the response was written.
func (dp *dataPlane) handleCORSPreflight(w http.ResponseWriter, r *http.Request, m *matchResult) bool {
	cfg := m.service.CorsConfig
	if cfg == nil || !isCORSPreflight(r) {
		return false
	}
	if cfg.PreflightContinue != nil && *cfg.PreflightContinue {
		// The upstream answers preflights itself; proxy untouched.
		return false
	}

	h := w.Header()
	origin, ok := corsAllowOrigin(cfg, r.Header.Get("Origin"))
	if !ok {
		// Disallowed origin: no CORS headers; the browser blocks the caller.
		w.WriteHeader(http.StatusOK)
		return true
	}
	h.Set("Access-Control-Allow-Origin", origin)
	h.Add("Vary", "Origin")

	if len(cfg.AccessControlAllowMethods) > 0 {
		h.Set("Access-Control-Allow-Methods", strings.Join(cfg.AccessControlAllowMethods, ","))
	} else {
		h.Set("Access-Control-Allow-Methods", corsDefaultMethods)
	}
	if cfg.AccessControlAllowHeaders != "" {
		h.Set("Access-Control-Allow-Headers", cfg.AccessControlAllowHeaders)
	} else if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
		// Reflect the requested headers when none are configured, as the
		// real gateway does.
		h.Set("Access-Control-Allow-Headers", reqHeaders)
		h.Add("Vary", "Access-Control-Request-Headers")
	}
	if cfg.MaxAge > 0 {
		h.Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
	}
	if cfg.Credentials != nil && *cfg.Credentials {
		h.Set("Access-Control-Allow-Credentials", "true")
	}
	if cfg.PrivateNetwork != nil && *cfg.PrivateNetwork &&
		r.Header.Get("Access-Control-Request-Private-Network") == "true" {
		h.Set("Access-Control-Allow-Private-Network", "true")
	}
	w.WriteHeader(http.StatusOK)
	return true
}

// applyCORSResponseHeaders decorates an actual (non-preflight) response.
func applyCORSResponseHeaders(h http.Header, cfg *CorsConfig, requestOrigin string) {
	if cfg == nil || requestOrigin == "" {
		return
	}
	origin, ok := corsAllowOrigin(cfg, requestOrigin)
	if !ok {
		return
	}
	h.Set("Access-Control-Allow-Origin", origin)
	h.Add("Vary", "Origin")
	if cfg.Credentials != nil && *cfg.Credentials {
		h.Set("Access-Control-Allow-Credentials", "true")
	}
	if cfg.AccessControlExposedHeaders != "" {
		h.Set("Access-Control-Expose-Headers", cfg.AccessControlExposedHeaders)
	}
}

// corsAllowOrigin resolves the Access-Control-Allow-Origin value:
// accessControlAllowOrigins is a comma-separated list or "*". A wildcard with
// credentials echoes the request origin ("*" is invalid with credentials).
func corsAllowOrigin(cfg *CorsConfig, requestOrigin string) (string, bool) {
	credentials := cfg.Credentials != nil && *cfg.Credentials
	for origin := range strings.SplitSeq(cfg.AccessControlAllowOrigins, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "*" {
			if credentials {
				return requestOrigin, true
			}
			return "*", true
		}
		if strings.EqualFold(origin, requestOrigin) {
			return requestOrigin, true
		}
	}
	return "", false
}
