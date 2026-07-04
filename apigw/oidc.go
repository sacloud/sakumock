package apigw

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// authenticateOidc validates a Bearer token against the service's OIDC
// configuration: the issuer's discovery document and JWKS are fetched (and
// cached) via go-oidc, so any IdP — including the real Google — works. This
// covers the accessToken authentication method; authorizationCodeFlow
// (browser login + session) arrives in a later phase.
//
// The error messages carry the underlying reason: a mock should make token
// problems easy to debug, and the real API's generic responses hide them.
func (dp *dataPlane) authenticateOidc(w http.ResponseWriter, r *http.Request, m *matchResult) bool {
	if m.service.Oidc == nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized: service has no OIDC configuration")
		return false
	}
	cfg, _, err := dp.store.GetOidc(m.service.Oidc.ID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized: OIDC configuration not found")
		return false
	}

	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return false
	}

	provider, err := dp.oidcProvider(cfg.Issuer)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized: OIDC discovery failed: "+err.Error())
		return false
	}

	// Without tokenAudiences the token's aud must contain the client ID;
	// with them, the aud check is done by hand against the configured list.
	vcfg := &oidc.Config{ClientID: cfg.ClientID}
	if len(cfg.TokenAudiences) > 0 {
		vcfg = &oidc.Config{SkipClientIDCheck: true}
	}
	idToken, err := provider.Verifier(vcfg).Verify(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
		return false
	}
	if len(cfg.TokenAudiences) > 0 && !slices.ContainsFunc(idToken.Audience, func(aud string) bool {
		return slices.Contains(cfg.TokenAudiences, aud)
	}) {
		writeError(w, http.StatusUnauthorized, "Unauthorized: token audience is not allowed")
		return false
	}

	if cfg.HideCredentials {
		m.stripAuthorization = true
	}
	return true
}

// oidcProvider returns the cached provider for the issuer, performing the
// discovery on first use. Failed discoveries are not cached so a temporarily
// unreachable IdP is retried on the next request. The provider keeps its own
// JWKS cache; it is created with a background context because key refreshes
// outlive the triggering request.
func (dp *dataPlane) oidcProvider(issuer string) (*oidc.Provider, error) {
	dp.mu.Lock()
	if p, ok := dp.oidcProviders[issuer]; ok {
		dp.mu.Unlock()
		return p, nil
	}
	dp.mu.Unlock()

	// The discovery request runs outside the lock so a slow IdP does not
	// stall unrelated requests; concurrent first uses may race, and the
	// second result simply replaces the first (both are valid).
	p, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		return nil, err
	}
	dp.mu.Lock()
	dp.oidcProviders[issuer] = p
	dp.mu.Unlock()
	return p, nil
}
