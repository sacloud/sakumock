package simplenotification

import "github.com/sacloud/sakumock/core"

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
// the portal spec forbids it; core.WithNullable accepts null for them. The
// Terraform provider sends null Tags when no tags are set.
var bodyNullableFields = map[string][]core.SDKFix{
	"POST /commonserviceitem":     {tagsNullable},
	"PUT /commonserviceitem/{id}": {tagsNullable},
}

var tagsNullable = core.SDKFix{
	Path:   "CommonServiceItem.Tags",
	Source: "https://github.com/sacloud/simple-notification-api-go/commit/2b5de682",
}
