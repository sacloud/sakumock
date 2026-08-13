package cloudhsm

// bodyNonEmptyFields lists request-body string fields whose spec schema
// declares required but no minLength, while the real API rejects an empty
// string. core.WithNonEmpty overlays a MinLength-1 constraint for them on the
// generated bodySchemas; remove an entry once the upstream spec gains a
// minLength for the field.
var bodyNonEmptyFields = map[string][]string{
	"POST " + zonePrefix + "/cloudhsm/cloudhsms":                                    {"CloudHSM.Name", "CloudHSM.Ipv4NetworkAddress"},
	"PUT " + zonePrefix + "/cloudhsm/cloudhsms/{resource_id}":                       {"CloudHSM.Name", "CloudHSM.Ipv4NetworkAddress"},
	"POST " + zonePrefix + "/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients":     {"Client.Name", "Client.Certificate"},
	"PUT " + zonePrefix + "/cloudhsm/cloudhsms/{cloudhsm_resource_id}/clients/{id}": {"Client.Name"},
	"POST " + zonePrefix + "/cloudhsm/cloudhsms/{resource_id}/peers":                {"Peer.ID", "Peer.SecretKey"},
	"POST " + zonePrefix + "/cloudhsm/licenses":                                     {"License.Name"},
	"PUT " + zonePrefix + "/cloudhsm/licenses/{resource_id}":                        {"License.Name"},
}
