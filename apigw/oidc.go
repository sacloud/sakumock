package apigw

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// oidcDiscoveryTimeout bounds the discovery request so an unreachable or
// stalled IdP fails fast instead of tying up request goroutines.
const oidcDiscoveryTimeout = 10 * time.Second

// authenticateOidc enforces the service's OIDC configuration. Bearer tokens
// (the accessToken method) are verified against the issuer's JWKS; requests
// without one fall through to the authorizationCodeFlow method (session
// cookie, or a browser round-trip to the IdP) when the configuration allows
// it.
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
	allowBearer := slices.Contains(cfg.AuthenticationMethods, "accessToken")
	allowCode := slices.Contains(cfg.AuthenticationMethods, "authorizationCodeFlow")

	if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		if !allowBearer {
			writeError(w, http.StatusUnauthorized, "Unauthorized: the OIDC configuration does not allow the accessToken method")
			return false
		}
		return dp.authenticateBearer(w, r, m, cfg, token)
	}
	if allowCode {
		return dp.authenticateSession(w, r, m, cfg)
	}
	writeError(w, http.StatusUnauthorized, "Unauthorized")
	return false
}

func (dp *dataPlane) authenticateBearer(w http.ResponseWriter, r *http.Request, m *matchResult, cfg OidcConfig, token string) bool {
	provider, err := dp.oidcProvider(cfg.Issuer)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized: OIDC discovery failed: "+err.Error())
		return false
	}
	if _, err := dp.verifyIDToken(r.Context(), provider, cfg, token); err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error())
		return false
	}
	if cfg.HideCredentials {
		m.stripAuthorization = true
	}
	return true
}

// verifyIDToken verifies signature, issuer, expiry, and audience: without
// tokenAudiences the token's aud must contain the client ID; with them, the
// aud is checked against the configured list instead.
func (dp *dataPlane) verifyIDToken(ctx context.Context, provider *oidc.Provider, cfg OidcConfig, raw string) (*oidc.IDToken, error) {
	vcfg := &oidc.Config{ClientID: cfg.ClientID}
	if len(cfg.TokenAudiences) > 0 {
		vcfg = &oidc.Config{SkipClientIDCheck: true}
	}
	idToken, err := provider.Verifier(vcfg).Verify(ctx, raw)
	if err != nil {
		return nil, err
	}
	if len(cfg.TokenAudiences) > 0 && !slices.ContainsFunc(idToken.Audience, func(aud string) bool {
		return slices.Contains(cfg.TokenAudiences, aud)
	}) {
		return nil, errors.New("token audience is not allowed")
	}
	return idToken, nil
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
	// second result simply replaces the first (both are valid). The timeout
	// only bounds the discovery: the provider's JWKS key set uses its own
	// background context internally.
	ctx, cancel := context.WithTimeout(context.Background(), oidcDiscoveryTimeout)
	defer cancel()
	p, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	dp.mu.Lock()
	dp.oidcProviders[issuer] = p
	dp.mu.Unlock()
	return p, nil
}
