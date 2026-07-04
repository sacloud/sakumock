package apigw

import (
	"fmt"
	"time"
)

// Domain types mirror the wire JSON of the API Gateway OpenAPI spec so the
// store can serve them directly. Fields the API never returns (internal
// references, stored credentials/PEMs) carry `json:"-"`.

// Service is an upstream definition (Kong "service"). The gateway forwards
// requests matched by the service's routes to protocol://host:port/path.
type Service struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Name           string       `json:"name"`
	Tags           []string     `json:"tags,omitempty"`
	Protocol       string       `json:"protocol"`
	Host           string       `json:"host"`
	Path           string       `json:"path"`
	Port           int          `json:"port"`
	Retries        *int         `json:"retries"`
	ConnectTimeout int          `json:"connectTimeout"`
	WriteTimeout   int          `json:"writeTimeout"`
	ReadTimeout    int          `json:"readTimeout"`
	Authentication string       `json:"authentication"`
	Oidc           *OidcSummary `json:"oidc,omitempty"`
	RouteHost      string       `json:"routeHost"`
	CorsConfig     *CorsConfig  `json:"corsConfig,omitempty"`
	ObjectStorage  *ObjectStore `json:"objectStorageConfig,omitempty"`
	SubscriptionID string       `json:"-"`
}

// OidcSummary references an OIDC configuration from a service.
type OidcSummary struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// CorsConfig holds the per-service CORS settings (enforced by the data plane
// in a later phase; stored and echoed as-is until then).
type CorsConfig struct {
	Credentials                 *bool    `json:"credentials,omitempty"`
	AccessControlExposedHeaders string   `json:"accessControlExposedHeaders,omitempty"`
	AccessControlAllowHeaders   string   `json:"accessControlAllowHeaders,omitempty"`
	MaxAge                      int      `json:"maxAge,omitempty"`
	AccessControlAllowMethods   []string `json:"accessControlAllowMethods,omitempty"`
	AccessControlAllowOrigins   string   `json:"accessControlAllowOrigins,omitempty"`
	PreflightContinue           *bool    `json:"preflightContinue,omitempty"`
	PrivateNetwork              *bool    `json:"privateNetwork,omitempty"`
}

// ObjectStore is the S3-compatible backend configuration of a service. All
// required fields (including the credentials) are echoed back in responses
// because the SDK's generated client requires them on decode.
type ObjectStore struct {
	BucketName       string `json:"bucketName"`
	FolderName       string `json:"folderName,omitempty"`
	Endpoint         string `json:"endpoint"`
	Region           string `json:"region"`
	AccessKeyID      string `json:"accessKeyID"`
	SecretAccessKey  string `json:"secretAccessKey"`
	UseDocumentIndex *bool  `json:"useDocumentIndex"`
}

// Route is an entrypoint of a service (Kong "route"). Requests are matched by
// hosts, path, and methods, then forwarded to the owning service's upstream.
type Route struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	ServiceID               string               `json:"serviceId"`
	Name                    string               `json:"name"`
	Tags                    []string             `json:"tags,omitempty"`
	Protocols               string               `json:"protocols"`
	Path                    string               `json:"path,omitempty"`
	Host                    string               `json:"host"`
	Hosts                   []string             `json:"hosts,omitempty"`
	Methods                 []string             `json:"methods"`
	HTTPSRedirectStatusCode int                  `json:"httpsRedirectStatusCode"`
	RegexPriority           int                  `json:"regexPriority"`
	StripPath               *bool                `json:"stripPath"`
	PreserveHost            bool                 `json:"preserveHost"`
	RequestBuffering        *bool                `json:"requestBuffering"`
	ResponseBuffering       *bool                `json:"responseBuffering"`
	IPRestriction           *IPRestrictionConfig `json:"ipRestrictionConfig,omitempty"`

	Authorization     *RouteAuthorizationConfig `json:"-"`
	RequestTransform  *RequestTransformation    `json:"-"`
	ResponseTransform *ResponseTransformation   `json:"-"`
}

// IPRestrictionConfig is an allow/deny client-IP list scoped to protocols.
type IPRestrictionConfig struct {
	Protocols    string   `json:"protocols"`
	RestrictedBy string   `json:"restrictedBy"`
	IPs          []string `json:"ips"`
}

// RouteAuthorizationConfig is the group allow-list of a route (Kong ACL).
type RouteAuthorizationConfig struct {
	IsACLEnabled bool                 `json:"isACLEnabled"`
	Groups       []RouteAuthorization `json:"groups"`
}

// RouteAuthorization is one group entry in a route's allow-list.
type RouteAuthorization struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled"`
}

// User is an API consumer (Kong "consumer"). Groups is resolved from the
// group store on read; credentials live in Auth.
type User struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Name          string               `json:"name"`
	CustomID      string               `json:"customID,omitempty"`
	Groups        []Group              `json:"groups"`
	Tags          []string             `json:"tags,omitempty"`
	IPRestriction *IPRestrictionConfig `json:"ipRestrictionConfig,omitempty"`

	GroupIDs []string            `json:"-"`
	Auth     *UserAuthentication `json:"-"`
}

// UserAuthentication bundles the per-user credentials. Secrets are echoed in
// responses because the spec marks them required on the credential schemas.
type UserAuthentication struct {
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`
	Jwt       *JwtAuth   `json:"jwt,omitempty"`
	HmacAuth  *HmacAuth  `json:"hmacAuth,omitempty"`
}

// BasicAuth is a Basic authentication credential.
type BasicAuth struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	UserName  string    `json:"userName"`
	Password  string    `json:"password"`
}

// JwtAuth is a JWT credential: tokens whose iss claim equals Key are verified
// with Secret using Algorithm.
type JwtAuth struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Key       string    `json:"key"`
	Secret    string    `json:"secret"`
	Algorithm string    `json:"algorithm"`
}

// HmacAuth is an HMAC signature credential (draft-cavage style).
type HmacAuth struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	UserName  string    `json:"userName"`
	Secret    string    `json:"secret"`
}

// Group is a consumer group referenced by users and route authorizations.
type Group struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Name      string    `json:"name"`
	Tags      []string  `json:"tags,omitempty"`
}

// Domain is a custom hostname routes can answer on. CertificateName is
// resolved from the certificate store on read.
type Domain struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	DomainName      string `json:"domainName"`
	CertificateID   string `json:"certificateId,omitempty"`
	CertificateName string `json:"certificateName,omitempty"`
}

// Certificate is a TLS certificate pair (RSA and/or ECDSA).
type Certificate struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Name  string              `json:"name"`
	RSA   *CertificateDetails `json:"rsa,omitempty"`
	ECDSA *CertificateDetails `json:"ecdsa,omitempty"`
}

// CertificateDetails holds one uploaded certificate. The PEM material is
// stored but never returned (cert and key are writeOnly in the spec).
type CertificateDetails struct {
	ExpiredAt time.Time `json:"expiredAt"`
	CertPEM   string    `json:"-"`
	KeyPEM    string    `json:"-"`
}

// Plan is a billing plan. Plans are seeded at store construction and
// read-only through the API.
type Plan struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Name            string   `json:"name"`
	Price           string   `json:"price"`
	Description     string   `json:"description,omitempty"`
	MaxServices     int      `json:"maxServices"`
	MaxRequests     int      `json:"maxRequests"`
	MaxRequestsUnit string   `json:"maxRequestsUnit"`
	Overage         *Overage `json:"overage,omitempty"`
}

// Overage is the extra-request pricing of a plan.
type Overage struct {
	UnitRequests int    `json:"unitRequests"`
	UnitPrice    string `json:"unitPrice"`
}

// Subscription is a plan contract. At most one service binds to a
// subscription; ResourceID is a SAKURA Cloud numeric resource ID. Other
// SAKURA Cloud APIs serialize resource IDs as JSON strings, but the apigw
// spec declares resourceId as integer (int64) and the generated SDK client
// decodes it as such, so the number is intentional here.
type Subscription struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Name           string               `json:"name"`
	PlanID         string               `json:"planId"`
	ResourceID     int64                `json:"resourceId"`
	MonthlyRequest int                  `json:"monthlyRequest"`
	Service        *SubscriptionService `json:"service,omitempty"`

	BoundServiceID string `json:"-"`
}

// SubscriptionService is the service bound to a subscription.
type SubscriptionService struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OidcConfig is an OpenID Connect relying-party configuration attachable to
// services with authentication "oidc".
type OidcConfig struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Name                  string   `json:"name"`
	AuthenticationMethods []string `json:"authenticationMethods"`
	Issuer                string   `json:"issuer"`
	ClientID              string   `json:"clientId"`
	ClientSecret          string   `json:"clientSecret"`
	Scopes                []string `json:"scopes,omitempty"`
	HideCredentials       bool     `json:"hideCredentials"`
	TokenAudiences        []string `json:"tokenAudiences,omitempty"`
	UseSession            bool     `json:"useSession"`
	RefreshTokenParamName string   `json:"refreshTokenParamName,omitempty"`
}

// RequestTransformation transforms matched requests before proxying
// (enforced by the data plane in a later phase; stored and echoed until then).
type RequestTransformation struct {
	HTTPMethod string                     `json:"httpMethod,omitempty"`
	Allow      *RequestAllowDetail        `json:"allow,omitempty"`
	Remove     *RequestRemoveDetail       `json:"remove,omitempty"`
	Rename     *RequestRenameDetail       `json:"rename,omitempty"`
	Replace    *RequestModificationDetail `json:"replace,omitempty"`
	Add        *RequestModificationDetail `json:"add,omitempty"`
	Append     *RequestModificationDetail `json:"append,omitempty"`
}

type RequestAllowDetail struct {
	Body []string `json:"body,omitempty"`
}

type RequestRemoveDetail struct {
	HeaderKeys  []string `json:"headerKeys,omitempty"`
	QueryParams []string `json:"queryParams,omitempty"`
	Body        []string `json:"body,omitempty"`
}

type RenamePair struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

type RequestRenameDetail struct {
	Headers     []RenamePair `json:"headers,omitempty"`
	QueryParams []RenamePair `json:"queryParams,omitempty"`
	Body        []RenamePair `json:"body,omitempty"`
}

type KeyValuePair struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type RequestModificationDetail struct {
	Headers     []KeyValuePair `json:"headers,omitempty"`
	QueryParams []KeyValuePair `json:"queryParams,omitempty"`
	Body        []KeyValuePair `json:"body,omitempty"`
}

// ResponseTransformation transforms upstream responses before returning them.
type ResponseTransformation struct {
	Allow   *ResponseAllowDetail        `json:"allow,omitempty"`
	Remove  *ResponseRemoveDetail       `json:"remove,omitempty"`
	Rename  *ResponseRenameDetail       `json:"rename,omitempty"`
	Replace *ResponseReplaceDetail      `json:"replace,omitempty"`
	Add     *ResponseModificationDetail `json:"add,omitempty"`
	Append  *ResponseModificationDetail `json:"append,omitempty"`
}

type ResponseAllowDetail struct {
	JSONKeys []string `json:"jsonKeys,omitempty"`
}

type ResponseRemoveDetail struct {
	IfStatusCode []int    `json:"ifStatusCode,omitempty"`
	HeaderKeys   []string `json:"headerKeys,omitempty"`
	JSONKeys     []string `json:"jsonKeys,omitempty"`
}

type ResponseRenameDetail struct {
	IfStatusCode []int        `json:"ifStatusCode,omitempty"`
	Headers      []RenamePair `json:"headers,omitempty"`
	JSON         []RenamePair `json:"json,omitempty"`
}

type ResponseReplaceDetail struct {
	IfStatusCode []int          `json:"ifStatusCode,omitempty"`
	Headers      []KeyValuePair `json:"headers,omitempty"`
	JSON         []KeyValuePair `json:"json,omitempty"`
	Body         string         `json:"body,omitempty"`
}

type ResponseModificationDetail struct {
	IfStatusCode []int          `json:"ifStatusCode,omitempty"`
	Headers      []KeyValuePair `json:"headers,omitempty"`
	JSON         []KeyValuePair `json:"json,omitempty"`
}

// StoreError carries an HTTP status alongside the message so handlers can map
// store failures to the spec's error responses without string matching.
type StoreError struct {
	Status  int
	Message string
}

func (e *StoreError) Error() string { return e.Message }

func errNotFound(format string, args ...any) *StoreError {
	return &StoreError{Status: 404, Message: fmt.Sprintf(format, args...)}
}

func errConflict(format string, args ...any) *StoreError {
	return &StoreError{Status: 409, Message: fmt.Sprintf(format, args...)}
}

func errBadRequest(format string, args ...any) *StoreError {
	return &StoreError{Status: 400, Message: fmt.Sprintf(format, args...)}
}

// Store is the persistence interface of the apigw mock.
type Store interface {
	// Services
	CreateService(svc Service, subscriptionID string) (Service, error)
	ListServices() []Service
	GetService(id string) (Service, error)
	UpdateService(id string, svc Service) error
	DeleteService(id string) error
	SubscriptionNameOf(serviceID string) string

	// Routes
	CreateRoute(serviceID string, rt Route) (Route, error)
	RoutesByHost(host string) []Route
	UserByBasicUserName(name string) (User, bool)
	UserByHmacUserName(name string) (User, bool)
	UserByJwtKey(key string) (User, bool)
	ListRoutes(serviceID string) ([]Route, error)
	GetRoute(serviceID, routeID string) (Route, error)
	UpdateRoute(serviceID, routeID string, rt Route) error
	DeleteRoute(serviceID, routeID string) error
	SetRouteAuthorization(serviceID, routeID string, cfg *RouteAuthorizationConfig) error
	SetRequestTransformation(serviceID, routeID string, tr *RequestTransformation) error
	SetResponseTransformation(serviceID, routeID string, tr *ResponseTransformation) error

	// Users
	CreateUser(u User) (User, error)
	ListUsers() []User
	GetUser(id string) (User, error)
	UpdateUser(id string, u User) error
	DeleteUser(id string) error
	ListUserGroups(userID string) ([]Group, map[string]bool, error)
	AssignUserGroup(userID, groupIDOrName string, assigned bool) error
	GetUserAuthentication(userID string) (*UserAuthentication, error)
	UpsertUserAuthentication(userID string, auth UserAuthentication) error

	// Groups
	CreateGroup(g Group) (Group, error)
	ListGroups() []Group
	GetGroup(id string) (Group, error)
	UpdateGroup(id string, g Group) error
	DeleteGroup(id string) error

	// Domains
	CreateDomain(d Domain) (Domain, error)
	ListDomains() []Domain
	UpdateDomain(id string, certificateID string) error
	DeleteDomain(id string) error

	// Certificates
	CreateCertificate(c Certificate) (Certificate, error)
	ListCertificates() []Certificate
	UpdateCertificate(id string, c Certificate) error
	DeleteCertificate(id string) error

	// Plans & subscriptions
	ListPlans() []Plan
	CreateSubscription(planID, name string) (Subscription, error)
	ListSubscriptions() []Subscription
	GetSubscription(id string) (Subscription, Plan, error)
	UpdateSubscription(id, name string) error
	DeleteSubscription(id string) error

	// OIDC
	CreateOidc(o OidcConfig) (OidcConfig, error)
	ListOidcs() []OidcConfig
	GetOidc(id string) (OidcConfig, []Service, error)
	UpdateOidc(id string, o OidcConfig) error
	DeleteOidc(id string) error

	Close()
}
