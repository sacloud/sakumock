package apigw

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"hash"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// hmacClockSkew is the maximum allowed difference between the request's Date
// (or X-Date) header and the server clock, matching the real gateway's
// default.
const hmacClockSkew = 300 * time.Second

// authorize enforces the matched service's authentication scheme, IP
// restrictions, and the route's group allow-list, in the same order as the
// real gateway: route IP restriction, authentication, consumer IP
// restriction, ACL. It writes the error response and returns false when the
// request must not be proxied.
func (dp *dataPlane) authorize(w http.ResponseWriter, r *http.Request, m *matchResult) bool {
	clientIP := clientIPOf(r)
	if !dp.allowIP(w, m.route.IPRestriction, clientIP) {
		return false
	}
	user, ok := dp.authenticate(w, r, m)
	if !ok {
		return false
	}
	if user != nil && !dp.allowIP(w, user.IPRestriction, clientIP) {
		return false
	}
	return dp.checkACL(w, m.route.Authorization, user)
}

// authenticate verifies the request against the service's authentication
// scheme. It returns the authenticated consumer (nil for scheme "none") and
// whether the request may proceed; on failure the 401 response is written.
func (dp *dataPlane) authenticate(w http.ResponseWriter, r *http.Request, m *matchResult) (*User, bool) {
	switch m.service.Authentication {
	case "", "none":
		return nil, true
	case "basic":
		return dp.authenticateBasic(w, r)
	case "hmac":
		return dp.authenticateHmac(w, r)
	case "jwt":
		return dp.authenticateJwt(w, r)
	case "oidc":
		// OIDC token validation arrives in a later phase.
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return nil, false
	default:
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return nil, false
	}
}

func (dp *dataPlane) authenticateBasic(w http.ResponseWriter, r *http.Request) (*User, bool) {
	userName, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="apigw"`)
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return nil, false
	}
	u, found := dp.store.UserByBasicUserName(userName)
	if !found || subtle.ConstantTimeCompare([]byte(u.Auth.BasicAuth.Password), []byte(password)) != 1 {
		writeError(w, http.StatusUnauthorized, "Invalid authentication credentials")
		return nil, false
	}
	return &u, true
}

// hmacAlgorithms maps the algorithm names of the Authorization header to
// their hash constructors (the set the real gateway's hmac-auth accepts).
var hmacAlgorithms = map[string]func() hash.Hash{
	"hmac-sha1":   sha1.New,
	"hmac-sha256": sha256.New,
	"hmac-sha384": sha512.New384,
	"hmac-sha512": sha512.New,
}

// authenticateHmac verifies a draft-cavage style HMAC signature:
//
//	Authorization: hmac username="u",algorithm="hmac-sha256",headers="date request-line",signature="base64"
func (dp *dataPlane) authenticateHmac(w http.ResponseWriter, r *http.Request) (*User, bool) {
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(authz, "hmac ") {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return nil, false
	}
	params := parseHmacParams(strings.TrimPrefix(authz, "hmac "))
	newHash, ok := hmacAlgorithms[params["algorithm"]]
	if !ok {
		writeError(w, http.StatusUnauthorized, "HMAC signature cannot be verified")
		return nil, false
	}
	u, found := dp.store.UserByHmacUserName(params["username"])
	if !found {
		writeError(w, http.StatusUnauthorized, "HMAC signature cannot be verified")
		return nil, false
	}

	headers := params["headers"]
	if headers == "" {
		headers = "date"
	}
	var lines []string
	for name := range strings.FieldsSeq(headers) {
		name = strings.ToLower(name)
		switch name {
		case "request-line":
			lines = append(lines, r.Method+" "+r.URL.RequestURI()+" "+r.Proto)
		case "@request-target":
			lines = append(lines, "@request-target: "+strings.ToLower(r.Method)+" "+r.URL.RequestURI())
		case "date", "x-date":
			value := r.Header.Get(name)
			if !dateWithinSkew(value) {
				writeError(w, http.StatusUnauthorized, "HMAC signature cannot be verified")
				return nil, false
			}
			lines = append(lines, name+": "+value)
		default:
			lines = append(lines, name+": "+r.Header.Get(name))
		}
	}

	sig, err := base64.StdEncoding.DecodeString(params["signature"])
	if err != nil {
		writeError(w, http.StatusUnauthorized, "HMAC signature cannot be verified")
		return nil, false
	}
	mac := hmac.New(newHash, []byte(u.Auth.HmacAuth.Secret))
	mac.Write([]byte(strings.Join(lines, "\n")))
	if !hmac.Equal(mac.Sum(nil), sig) {
		writeError(w, http.StatusUnauthorized, "HMAC signature cannot be verified")
		return nil, false
	}
	return &u, true
}

// parseHmacParams splits `k1="v1",k2="v2"` into a map. Base64 signatures
// contain no commas or quotes, so a simple split suffices.
func parseHmacParams(s string) map[string]string {
	params := make(map[string]string)
	for part := range strings.SplitSeq(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		params[k] = strings.Trim(v, `"`)
	}
	return params
}

func dateWithinSkew(value string) bool {
	t, err := http.ParseTime(value)
	if err != nil {
		return false
	}
	diff := time.Since(t)
	if diff < 0 {
		diff = -diff
	}
	return diff <= hmacClockSkew
}

// authenticateJwt resolves the credential by the token's iss claim (the
// credential key) and verifies the HMAC signature with the credential's
// algorithm, as the real gateway's jwt plugin does.
func (dp *dataPlane) authenticateJwt(w http.ResponseWriter, r *http.Request) (*User, bool) {
	authz := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(authz, "Bearer ")
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return nil, false
	}

	unverified, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Bad token; invalid JSON")
		return nil, false
	}
	iss, err := unverified.Claims.GetIssuer()
	if err != nil || iss == "" {
		writeError(w, http.StatusUnauthorized, "No mandatory 'iss' in claims")
		return nil, false
	}
	u, found := dp.store.UserByJwtKey(iss)
	if !found {
		writeError(w, http.StatusUnauthorized, "No credentials found for given 'iss'")
		return nil, false
	}
	cred := u.Auth.Jwt

	// Restricting the accepted method to the credential's algorithm prevents
	// algorithm-confusion attacks and matches the gateway's behavior.
	_, err = jwt.Parse(token, func(*jwt.Token) (any, error) {
		return []byte(cred.Secret), nil
	}, jwt.WithValidMethods([]string{cred.Algorithm}))
	switch {
	case err == nil:
		return &u, true
	case errors.Is(err, jwt.ErrTokenExpired):
		writeError(w, http.StatusUnauthorized, "token expired")
	default:
		writeError(w, http.StatusUnauthorized, "Invalid signature")
	}
	return nil, false
}

// checkACL enforces the route's group allow-list against the authenticated
// consumer's groups.
func (dp *dataPlane) checkACL(w http.ResponseWriter, cfg *RouteAuthorizationConfig, user *User) bool {
	if cfg == nil || !cfg.IsACLEnabled {
		return true
	}
	if user == nil {
		// ACL without an authenticated consumer can never pass.
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return false
	}
	for _, g := range cfg.Groups {
		if g.Enabled && slices.Contains(user.GroupIDs, g.ID) {
			return true
		}
	}
	writeError(w, http.StatusForbidden, "You cannot consume this service")
	return false
}

// allowIP enforces an allow/deny client-IP list. The restriction only applies
// when its protocols setting covers the scheme this listener serves.
func (dp *dataPlane) allowIP(w http.ResponseWriter, cfg *IPRestrictionConfig, clientIP string) bool {
	if cfg == nil || !protocolsInclude(cfg.Protocols, dp.scheme) {
		return true
	}
	listed := slices.Contains(cfg.IPs, clientIP)
	allowed := listed
	if cfg.RestrictedBy == "denyIps" {
		allowed = !listed
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "Your IP address is not allowed")
		return false
	}
	return true
}

func clientIPOf(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
