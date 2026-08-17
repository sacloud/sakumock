package seg

// bodyNonEmptyFields lists request-body string fields whose spec schema
// declares required but no minLength, while the real API rejects an empty
// string. core.WithNonEmpty overlays a MinLength-1 constraint for them on the
// generated bodySchemas; remove an entry once the upstream spec gains a
// minLength for the field.
//
// Servers[].IPAddress has the same gap but core.WithNonEmpty only walks
// object properties, not array items, so it cannot be overlaid here.
var bodyNonEmptyFields = map[string][]string{
	"POST /appliance": {"Appliance.Remark.Switch.ID"},
}
