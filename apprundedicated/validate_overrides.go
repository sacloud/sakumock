package apprundedicated

// bodyNonEmptyFields lists request-body string fields whose spec schema
// declares required but no minLength, while the real API rejects an empty
// string. core.WithNonEmpty overlays a MinLength-1 constraint for them on the
// generated bodySchemas; remove an entry once the upstream spec gains a
// minLength for the field.
var bodyNonEmptyFields = map[string][]string{
	"POST /applications/{applicationID}/versions":            {"image"},
	"POST /clusters/{clusterID}/asg":                         {"zone"},
	"POST /clusters/{clusterID}/certificates":                {"certificatePem", "privatekeyPem"},
	"PUT /clusters/{clusterID}/certificates/{certificateID}": {"certificatePem", "privatekeyPem"},
}
