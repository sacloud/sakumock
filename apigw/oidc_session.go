package apigw

import (
	"crypto/rand"
	"net/http"
	"slices"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	// sessionCookieName carries the gateway session issued after a completed
	// authorization code flow. The cookie is gateway-internal and stripped
	// before proxying.
	sessionCookieName = "apigw_session"
	// loginStateTTL bounds how long a redirect to the IdP may take to come
	// back before its state is forgotten.
	loginStateTTL = 10 * time.Minute
	// sessionFallbackTTL applies when the ID token carries no usable expiry.
	sessionFallbackTTL = time.Hour
	// sweepThreshold caps how many entries the state/session maps may hold
	// before expired ones are swept out on insert.
	sweepThreshold = 1000
)

// pendingLogin is an authorization code flow in progress: issued when
// redirecting to the IdP, consumed by the callback.
type pendingLogin struct {
	nonce       string
	redirectURI string
	expires     time.Time
}

type gwSession struct {
	expires time.Time
}

// authenticateSession handles the authorizationCodeFlow method: an
// established session cookie passes; a callback (code+state) is exchanged for
// tokens; anything else is redirected to the IdP's authorization endpoint.
// It returns true only when the request may proceed to the upstream.
func (dp *dataPlane) authenticateSession(w http.ResponseWriter, r *http.Request, m *matchResult, cfg OidcConfig) bool {
	if c, err := r.Cookie(sessionCookieName); err == nil && dp.validSession(c.Value) {
		// The session cookie is gateway-internal; never leak it upstream.
		m.stripSessionCookie = true
		if cfg.HideCredentials {
			m.stripAuthorization = true
		}
		return true
	}

	q := r.URL.Query()
	if q.Get("code") != "" && q.Get("state") != "" {
		dp.finishLogin(w, r, cfg)
		return false
	}
	dp.startLogin(w, r, cfg)
	return false
}

// startLogin stores a state/nonce pair and redirects to the IdP. The
// redirect_uri is the protected URL itself (as the real gateway does), so the
// user lands back where they started.
func (dp *dataPlane) startLogin(w http.ResponseWriter, r *http.Request, cfg OidcConfig) {
	provider, err := dp.oidcProvider(cfg.Issuer)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized: OIDC discovery failed: "+err.Error())
		return
	}

	state := randomToken()
	nonce := randomToken()
	redirectURI := dp.scheme + "://" + r.Host + r.URL.RequestURI()

	dp.mu.Lock()
	sweepExpired(dp.pendingLogins, func(p pendingLogin) time.Time { return p.expires })
	dp.pendingLogins[state] = pendingLogin{
		nonce:       nonce,
		redirectURI: redirectURI,
		expires:     time.Now().Add(loginStateTTL),
	}
	dp.mu.Unlock()

	authURL := oauth2Config(cfg, provider, redirectURI).AuthCodeURL(state, oidc.Nonce(nonce))
	http.Redirect(w, r, authURL, http.StatusFound)
}

// finishLogin validates the callback, exchanges the code for tokens, verifies
// the ID token (including the nonce), establishes the session, and redirects
// back to the originally requested URL.
func (dp *dataPlane) finishLogin(w http.ResponseWriter, r *http.Request, cfg OidcConfig) {
	q := r.URL.Query()

	dp.mu.Lock()
	pending, ok := dp.pendingLogins[q.Get("state")]
	delete(dp.pendingLogins, q.Get("state"))
	dp.mu.Unlock()
	if !ok || time.Now().After(pending.expires) {
		writeError(w, http.StatusUnauthorized, "Unauthorized: unknown or expired login state")
		return
	}

	provider, err := dp.oidcProvider(cfg.Issuer)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized: OIDC discovery failed: "+err.Error())
		return
	}
	token, err := oauth2Config(cfg, provider, pending.redirectURI).Exchange(r.Context(), q.Get("code"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized: token exchange failed: "+err.Error())
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized: the token response carried no id_token")
		return
	}
	idToken, err := dp.verifyIDToken(r.Context(), provider, cfg, rawIDToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
		return
	}
	if idToken.Nonce != pending.nonce {
		writeError(w, http.StatusUnauthorized, "Unauthorized: nonce mismatch")
		return
	}

	expires := idToken.Expiry
	if expires.IsZero() || time.Until(expires) <= 0 {
		expires = time.Now().Add(sessionFallbackTTL)
	}
	sessionID := randomToken()
	dp.mu.Lock()
	sweepExpired(dp.sessions, func(s gwSession) time.Time { return s.expires })
	dp.sessions[sessionID] = gwSession{expires: expires}
	dp.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   dp.scheme == "https",
	})
	http.Redirect(w, r, pending.redirectURI, http.StatusFound)
}

func (dp *dataPlane) validSession(id string) bool {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	s, ok := dp.sessions[id]
	if !ok {
		return false
	}
	if time.Now().After(s.expires) {
		delete(dp.sessions, id)
		return false
	}
	return true
}

// oauth2Config builds the token-exchange configuration from the OIDC
// configuration and the discovered endpoints. The openid scope is always
// requested.
func oauth2Config(cfg OidcConfig, provider *oidc.Provider, redirectURI string) *oauth2.Config {
	scopes := cfg.Scopes
	if !slices.Contains(scopes, oidc.ScopeOpenID) {
		// The openid scope is what makes the IdP return an id_token; the
		// flow cannot complete without it.
		scopes = append([]string{oidc.ScopeOpenID}, scopes...)
	}
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}
}

// randomToken returns an unguessable value for state/nonce/session IDs.
// crypto/rand.Text cannot fail, unlike error-returning Read patterns.
func randomToken() string {
	return rand.Text()
}

// sweepExpired drops expired entries once the map grows past sweepThreshold,
// keeping abandoned logins and sessions from accumulating without a
// background goroutine.
func sweepExpired[V any](m map[string]V, expiry func(V) time.Time) {
	if len(m) < sweepThreshold {
		return
	}
	now := time.Now()
	for k, v := range m {
		if now.After(expiry(v)) {
			delete(m, k)
		}
	}
}

func removeCookie(r *http.Request, name string) {
	cookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, c := range cookies {
		if c.Name != name {
			r.AddCookie(c)
		}
	}
}
