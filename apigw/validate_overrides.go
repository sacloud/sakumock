package apigw

// bodyNonEmptyFields declares spec gaps: string fields the spec marks
// required (or that the real API rejects when empty) but gives no minLength.
// core.WithNonEmpty overlays MinLength 1 onto these paths at construction.
// Remove an entry once the upstream spec gains the minLength.
var bodyNonEmptyFields = map[string][]string{
	"POST /oidc":                          {"issuer", "clientId", "clientSecret"},
	"PUT /oidc/{oidcId}":                  {"issuer", "clientId", "clientSecret"},
	"POST /subscriptions":                 {"planId", "name"},
	"PUT /subscriptions/{subscriptionId}": {"name"},
	"PUT /users/{userId}/authentication": {
		"basicAuth.userName", "basicAuth.password",
		"jwt.key", "jwt.secret",
		"hmacAuth.userName", "hmacAuth.secret",
	},
}
