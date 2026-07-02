package objectstorage

// bodyNonEmptyFields lists request-body string fields whose spec schema
// declares required but no minLength, while the real API rejects an empty
// string. core.WithNonEmpty overlays a MinLength-1 constraint for them on the
// generated bodySchemas; remove an entry once the upstream spec gains a
// minLength for the field.
var bodyNonEmptyFields = map[string][]string{
	"POST /fed/v1/buckets/{name}/replication": {"dest_bucket"},
	"PUT /fed/v1/buckets/{name}":              {"cluster_id"},
	"PUT /{site}/v2/buckets/{name}/plan":      {"new_plan.service_class_path"},
}
