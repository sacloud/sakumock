package addon

// family describes one add-on service family: the path prefix it is served
// under and whether it exposes a deployment status endpoint.
type family struct {
	kind Kind
	// path is the base path of the family, matching the spec exactly — the
	// addon SDK client uses SAKURA_ENDPOINTS_ADDON as the whole API root URL,
	// so the mock serves the spec paths unprefixed.
	path string
	// label names the family in route descriptions.
	label string
	// hasStatus is false only for vulnerability detection, which deploys an
	// agent rather than an Azure deployment and has no status endpoint.
	hasStatus bool
}

// families is the single source of truth for the served resource families,
// driving both routeTable() and the handlers.
var families = []family{
	{kind: KindAI, path: "/ai", label: "AI service", hasStatus: true},
	{kind: KindCDN, path: "/cdn", label: "CDN service", hasStatus: true},
	{kind: KindDDoS, path: "/security/ddos", label: "DDoS protection service", hasStatus: true},
	{kind: KindWAF, path: "/security/waf", label: "WAF service", hasStatus: true},
	{kind: KindVulnerability, path: "/security/vulnerability", label: "vulnerability detection service"},
	{kind: KindDataLake, path: "/analytics/datalake", label: "data lake", hasStatus: true},
	{kind: KindDWH, path: "/analytics/dwh", label: "data warehouse", hasStatus: true},
	{kind: KindETL, path: "/analytics/etl", label: "data ETL", hasStatus: true},
	{kind: KindQuery, path: "/analytics/query", label: "query service", hasStatus: true},
	{kind: KindSearch, path: "/analytics/search", label: "search service", hasStatus: true},
	{kind: KindStreaming, path: "/analytics/streaming", label: "streaming service", hasStatus: true},
}

// createRequest is the part of every family's create body the mock reads
// itself. Everything else is echoed back verbatim from the raw body.
type createRequest struct {
	Location string `json:"location"`
}

// Server OS types, in the numbering the spec's ServerOsType enum uses.
const (
	osTypeWindows = 1
	osTypeLinux   = 2
)

type vulnerabilityCreateRequest struct {
	OS int `json:"os"`
}
