package monitoringsuite

// bodyNonEmptyFields lists request-body string fields whose spec schema
// declares required but no minLength, while the real API rejects an empty
// string. core.WithNonEmpty overlays a MinLength-1 constraint for them on the
// generated bodySchemas; remove an entry once the upstream spec gains a
// minLength for the field.
var bodyNonEmptyFields = map[string][]string{
	"POST /alerts/projects/":     {"name"},
	"POST /dashboards/projects/": {"name"},
	"POST /logs/storages/":       {"name"},
	"POST /metrics/storages/":    {"name"},
	"POST /traces/storages/":     {"name"},
}
