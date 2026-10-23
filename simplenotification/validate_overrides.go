package simplenotification

// bodyNonEmptyFields lists request-body string fields whose spec schema
// declares required but no minLength, while the real API rejects an empty
// string. core.WithNonEmpty overlays a MinLength-1 constraint for them on the
// generated bodySchemas; remove an entry once the upstream spec gains a
// minLength for the field.
var bodyNonEmptyFields = map[string][]string{
	"POST /commonserviceitem":                                 {"CommonServiceItem.Name"},
	"POST /commonserviceitem/{id}/simplenotification/message": {"Message"},
}

// bodyNullableFields lists request-body fields the SDK sends as null although
// the portal spec forbids it. core.WithNullable accepts null for them: the
// SDK's hand-fixed spec (sacloud/simple-notification-api-go 2b5de682, now in
// sacloud-sdk-go) marks the request Tags nullable, and the Terraform provider
// sends null when no tags are set.
var bodyNullableFields = map[string][]string{
	"POST /commonserviceitem":     {"CommonServiceItem.Tags"},
	"PUT /commonserviceitem/{id}": {"CommonServiceItem.Tags"},
}
