package seg

import (
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

// JSON types matching the Service Endpoint Gateway OpenAPI spec.

type planJSON struct {
	ID int `json:"ID"`
}

type switchRemarkJSON struct {
	ID string `json:"ID"`
}

type networkRemarkJSON struct {
	NetworkMaskLen int `json:"NetworkMaskLen"`
}

type serverRemarkJSON struct {
	IPAddress string `json:"IPAddress"`
}

type zoneRemarkJSON struct {
	ID string `json:"ID"`
}

type applianceCreateRemarkJSON struct {
	Switch  switchRemarkJSON   `json:"Switch"`
	Network networkRemarkJSON  `json:"Network"`
	Servers []serverRemarkJSON `json:"Servers"`
}

type applianceRemarkJSON struct {
	Switch  switchRemarkJSON   `json:"Switch"`
	Network networkRemarkJSON  `json:"Network"`
	Servers []serverRemarkJSON `json:"Servers"`
	Zone    zoneRemarkJSON     `json:"Zone"`
}

type applianceCreateBodyJSON struct {
	Class  string                    `json:"Class"`
	Plan   planJSON                  `json:"Plan"`
	Remark applianceCreateRemarkJSON `json:"Remark"`
}

type createServiceEndpointGatewayRequest struct {
	Appliance applianceCreateBodyJSON `json:"Appliance"`
}

type hostInfoJSON struct {
	Name    *string `json:"Name"`
	InfoURL *string `json:"InfoURL"`
}

type instanceJSON struct {
	Status          *string        `json:"Status"`
	StatusChangedAt *string        `json:"StatusChangedAt"`
	Host            hostInfoJSON   `json:"Host"`
	Hosts           []hostInfoJSON `json:"Hosts"`
}

type instanceForPowerJSON struct {
	Status string `json:"Status"`
}

type encryptionKeyJSON struct {
	KMSKeyID *string `json:"KMSKeyID"`
}

type dedicatedStorageContractJSON struct {
	ID *string `json:"ID"`
}

type diskJSON struct {
	EncryptionAlgorithm      *string                       `json:"EncryptionAlgorithm"`
	EncryptionKey            *encryptionKeyJSON            `json:"EncryptionKey"`
	DedicatedStorageContract *dedicatedStorageContractJSON `json:"DedicatedStorageContract"`
}

type internetInfoJSON struct {
	BandWidthMbps *int `json:"BandWidthMbps"`
}

type regionJSON struct {
	ID   int    `json:"ID"`
	Name string `json:"Name"`
}

type zoneJSON struct {
	ID     int        `json:"ID"`
	Name   string     `json:"Name"`
	Region regionJSON `json:"Region"`
}

type switchJSON struct {
	ID           string            `json:"ID"`
	Name         string            `json:"Name"`
	Internet     *internetInfoJSON `json:"Internet"`
	Scope        string            `json:"Scope"`
	Availability string            `json:"Availability"`
	Zone         zoneJSON          `json:"Zone"`
}

type subnetJSON struct {
	NetworkAddress string           `json:"NetworkAddress"`
	NetworkMaskLen int              `json:"NetworkMaskLen"`
	DefaultRoute   string           `json:"DefaultRoute"`
	Internet       internetInfoJSON `json:"Internet"`
}

type userSubnetJSON struct {
	DefaultRoute   string `json:"DefaultRoute"`
	NetworkMaskLen int    `json:"NetworkMaskLen"`
}

type interfaceSwitchJSON struct {
	ID         string         `json:"ID"`
	Name       string         `json:"Name"`
	Scope      string         `json:"Scope"`
	Subnet     *subnetJSON    `json:"Subnet"`
	UserSubnet userSubnetJSON `json:"UserSubnet"`
}

type interfaceJSON struct {
	IPAddress     *string             `json:"IPAddress"`
	UserIPAddress *string             `json:"UserIPAddress"`
	HostName      *string             `json:"HostName"`
	Switch        interfaceSwitchJSON `json:"Switch"`
}

type simpleInterfaceSwitchJSON struct {
	Scope string `json:"Scope"`
}

type simpleInterfaceJSON struct {
	IPAddress     *string                   `json:"IPAddress"`
	UserIPAddress *string                   `json:"UserIPAddress"`
	Switch        simpleInterfaceSwitchJSON `json:"Switch"`
}

type serviceConfigJSON struct {
	Endpoints []string `json:"Endpoints,omitempty"`
	Mode      string   `json:"Mode,omitempty"`
}

type enabledServiceJSON struct {
	Type   string            `json:"Type"`
	Config serviceConfigJSON `json:"Config"`
}

type monitoringSuiteSettingsJSON struct {
	Enabled string `json:"Enabled"`
}

type dnsForwardingSettingsJSON struct {
	Enabled           string `json:"Enabled"`
	PrivateHostedZone string `json:"PrivateHostedZone"`
	UpstreamDNS1      string `json:"UpstreamDNS1"`
	UpstreamDNS2      string `json:"UpstreamDNS2"`
}

type serviceEndpointGatewaySettingsJSON struct {
	EnabledServices []enabledServiceJSON         `json:"EnabledServices"`
	MonitoringSuite *monitoringSuiteSettingsJSON `json:"MonitoringSuite,omitempty"`
	DNSForwarding   *dnsForwardingSettingsJSON   `json:"DNSForwarding,omitempty"`
}

type applianceSettingsJSON struct {
	ServiceEndpointGateway serviceEndpointGatewaySettingsJSON `json:"ServiceEndpointGateway"`
}

type applianceJSON struct {
	ID           string                 `json:"ID"`
	Class        string                 `json:"Class"`
	Name         string                 `json:"Name"`
	Description  string                 `json:"Description"`
	Plan         planJSON               `json:"Plan"`
	Settings     *applianceSettingsJSON `json:"Settings"`
	SettingsHash *string                `json:"SettingsHash"`
	Remark       applianceRemarkJSON    `json:"Remark"`
	Availability string                 `json:"Availability"`
	Instance     instanceJSON           `json:"Instance"`
	Disk         diskJSON               `json:"Disk"`
	ServiceClass string                 `json:"ServiceClass"`
	Generation   int                    `json:"Generation"`
	CreatedAt    string                 `json:"CreatedAt"`
	Icon         any                    `json:"Icon"`
	Switch       switchJSON             `json:"Switch"`
	Interfaces   []interfaceJSON        `json:"Interfaces"`
	Tags         []string               `json:"Tags"`
}

type applianceUpdateBodyJSON struct {
	Settings applianceSettingsJSON `json:"Settings"`
}

type updateServiceEndpointGatewayRequest struct {
	Appliance applianceUpdateBodyJSON `json:"Appliance"`
}

type readServiceEndpointGatewayResponseBody struct {
	Appliance applianceJSON `json:"Appliance"`
	IsOk      bool          `json:"is_ok"`
}

type listServiceEndpointGatewaysResponseBody struct {
	From       int             `json:"From"`
	Count      int             `json:"Count"`
	Total      int             `json:"Total"`
	Appliances []applianceJSON `json:"Appliances"`
	IsOk       bool            `json:"is_ok"`
}

type readServiceEndpointGatewayInterfaceResponseBody struct {
	Interface simpleInterfaceJSON `json:"Interface"`
	IsOk      bool                `json:"is_ok"`
}

type readServiceEndpointGatewayPowerStatusResponseBody struct {
	Instance instanceForPowerJSON `json:"Instance"`
	IsOk     bool                 `json:"is_ok"`
}

type updateServiceEndpointGatewayPowerStatusResponseBody struct {
	Success bool `json:"Success"`
	IsOk    bool `json:"is_ok"`
}

type applyResponseHeadersJSON struct {
	Status string  `json:"Status"`
	Cause  *string `json:"Cause"`
}

type applianceApplyResponse struct {
	Success         bool                     `json:"Success"`
	ReturnCode      int                      `json:"ReturnCode"`
	ResponseHeaders applyResponseHeadersJSON `json:"ResponseHeaders"`
	Out             *string                  `json:"Out"`
	IsOk            bool                     `json:"is_ok"`
}

func strPtr(s string) *string { return &s }

func settingsToJSON(settings *SettingsRecord) *applianceSettingsJSON {
	if settings == nil {
		return nil
	}
	services := make([]enabledServiceJSON, len(settings.EnabledServices))
	for i, es := range settings.EnabledServices {
		services[i] = enabledServiceJSON{
			Type: es.Type,
			Config: serviceConfigJSON{
				Endpoints: es.Config.Endpoints,
				Mode:      es.Config.Mode,
			},
		}
	}
	out := serviceEndpointGatewaySettingsJSON{EnabledServices: services}
	if settings.MonitoringSuite != nil {
		enabled := "False"
		if *settings.MonitoringSuite {
			enabled = "True"
		}
		out.MonitoringSuite = &monitoringSuiteSettingsJSON{Enabled: enabled}
	}
	if settings.DNSForwarding != nil {
		enabled := "False"
		if settings.DNSForwarding.Enabled {
			enabled = "True"
		}
		out.DNSForwarding = &dnsForwardingSettingsJSON{
			Enabled:           enabled,
			PrivateHostedZone: settings.DNSForwarding.PrivateHostedZone,
			UpstreamDNS1:      settings.DNSForwarding.UpstreamDNS1,
			UpstreamDNS2:      settings.DNSForwarding.UpstreamDNS2,
		}
	}
	return &applianceSettingsJSON{ServiceEndpointGateway: out}
}

func settingsFromJSON(in applianceSettingsJSON) SettingsRecord {
	sg := in.ServiceEndpointGateway
	services := make([]EnabledServiceRecord, len(sg.EnabledServices))
	for i, es := range sg.EnabledServices {
		services[i] = EnabledServiceRecord{
			Type: es.Type,
			Config: EnabledServiceConfig{
				Endpoints: es.Config.Endpoints,
				Mode:      es.Config.Mode,
			},
		}
	}
	out := SettingsRecord{EnabledServices: services}
	if sg.MonitoringSuite != nil {
		enabled := sg.MonitoringSuite.Enabled == "True"
		out.MonitoringSuite = &enabled
	}
	if sg.DNSForwarding != nil {
		out.DNSForwarding = &DNSForwardingRecord{
			Enabled:           sg.DNSForwarding.Enabled == "True",
			PrivateHostedZone: sg.DNSForwarding.PrivateHostedZone,
			UpstreamDNS1:      sg.DNSForwarding.UpstreamDNS1,
			UpstreamDNS2:      sg.DNSForwarding.UpstreamDNS2,
		}
	}
	return out
}

func interfaceToJSON(iface InterfaceRecord) interfaceJSON {
	out := interfaceJSON{
		Switch: interfaceSwitchJSON{
			ID:         iface.SwitchID,
			Name:       iface.SwitchName,
			Scope:      iface.Scope,
			UserSubnet: userSubnetJSON{DefaultRoute: iface.DefaultRoute, NetworkMaskLen: iface.NetworkMaskLen},
		},
	}
	if iface.IPAddress != "" {
		out.IPAddress = strPtr(iface.IPAddress)
	}
	if iface.UserIPAddress != "" {
		out.UserIPAddress = strPtr(iface.UserIPAddress)
	}
	if iface.HasSubnet {
		out.Switch.Subnet = &subnetJSON{
			NetworkAddress: iface.NetworkAddress,
			NetworkMaskLen: iface.NetworkMaskLen,
			DefaultRoute:   iface.DefaultRoute,
			Internet:       internetInfoJSON{BandWidthMbps: intPtr(100)},
		}
	}
	return out
}

func intPtr(n int) *int { return &n }

func simpleInterfaceToJSON(iface InterfaceRecord) simpleInterfaceJSON {
	out := simpleInterfaceJSON{Switch: simpleInterfaceSwitchJSON{Scope: iface.Scope}}
	if iface.IPAddress != "" {
		out.IPAddress = strPtr(iface.IPAddress)
	}
	if iface.UserIPAddress != "" {
		out.UserIPAddress = strPtr(iface.UserIPAddress)
	}
	return out
}

func applianceToJSON(a ApplianceRecord) applianceJSON {
	interfaces := make([]interfaceJSON, len(a.Interfaces))
	for i, iface := range a.Interfaces {
		interfaces[i] = interfaceToJSON(iface)
	}

	var settingsHashPtr *string
	if a.SettingsHash != "" {
		settingsHashPtr = strPtr(a.SettingsHash)
	}

	hostName := strPtr("sac-mock-" + a.ID)
	hostInfoURL := strPtr("")
	host := hostInfoJSON{Name: hostName, InfoURL: hostInfoURL}

	servers := make([]serverRemarkJSON, len(a.Servers))
	for i, sv := range a.Servers {
		servers[i] = serverRemarkJSON(sv)
	}

	return applianceJSON{
		ID:           a.ID,
		Class:        "serviceendpointgateway",
		Name:         a.Name,
		Description:  a.Description,
		Plan:         planJSON{ID: 1},
		Settings:     settingsToJSON(a.Settings),
		SettingsHash: settingsHashPtr,
		Remark: applianceRemarkJSON{
			Switch:  switchRemarkJSON{ID: a.Switch.ID},
			Network: networkRemarkJSON{NetworkMaskLen: a.Network.NetworkMaskLen},
			Servers: servers,
			Zone:    zoneRemarkJSON{ID: a.ZoneID},
		},
		Availability: a.Availability,
		Instance: instanceJSON{
			Status:          strPtr(a.PowerStatus),
			StatusChangedAt: strPtr(core.FormatRFC3339(a.StatusChangedAt)),
			Host:            host,
			Hosts:           []hostInfoJSON{host},
		},
		Disk:         diskJSON{},
		ServiceClass: "cloud/appliance/serviceendpointgateway/1",
		Generation:   100,
		CreatedAt:    core.FormatRFC3339(a.CreatedAt),
		Icon:         nil,
		Switch: switchJSON{
			ID:           a.Switch.ID,
			Name:         "Switch1",
			Scope:        "user",
			Availability: "available",
			Zone:         zoneJSON{ID: mustAtoi(a.ZoneID), Name: "is1a", Region: regionJSON{ID: 310, Name: "石狩"}},
		},
		Interfaces: interfaces,
		Tags:       a.Tags,
	}
}

func mustAtoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, r := range s.routeTable() {
		mux.HandleFunc(r.Method+" "+r.Path, r.Handler)
	}
	return mux
}

// ServeHTTP dispatches the request to the matching route and logs the result.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	rw := core.NewResponseRecorder(w)
	s.mux.ServeHTTP(rw, r)
	s.logger.Info("request", core.RequestLogArgs(r, rw)...)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	appliances := s.store.List()
	items := make([]applianceJSON, len(appliances))
	for i, a := range appliances {
		items[i] = applianceToJSON(a)
	}
	core.WriteJSON(w, http.StatusOK, listServiceEndpointGatewaysResponseBody{
		From:       0,
		Count:      len(items),
		Total:      len(items),
		Appliances: items,
		IsOk:       true,
	})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createServiceEndpointGatewayRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	remark := req.Appliance.Remark
	servers := make([]ServerRemark, len(remark.Servers))
	for i, sv := range remark.Servers {
		servers[i] = ServerRemark(sv)
	}
	a, err := s.store.Create("", "", nil,
		SwitchRemark{ID: remark.Switch.ID},
		NetworkRemark{NetworkMaskLen: remark.Network.NetworkMaskLen},
		servers,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Debug("seg appliance created", "id", a.ID)
	core.WriteJSON(w, http.StatusAccepted, readServiceEndpointGatewayResponseBody{Appliance: applianceToJSON(a), IsOk: true})
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applianceID")
	a, err := s.store.Read(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, readServiceEndpointGatewayResponseBody{Appliance: applianceToJSON(a), IsOk: true})
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applianceID")
	var req updateServiceEndpointGatewayRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a, err := s.store.Update(id, settingsFromJSON(req.Appliance.Settings))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, readServiceEndpointGatewayResponseBody{Appliance: applianceToJSON(a), IsOk: true})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applianceID")
	a, err := s.store.Read(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := s.store.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, readServiceEndpointGatewayResponseBody{Appliance: applianceToJSON(a), IsOk: true})
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applianceID")
	if _, err := s.store.Apply(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, applianceApplyResponse{
		Success:         true,
		ReturnCode:      0,
		ResponseHeaders: applyResponseHeadersJSON{Status: "200 OK"},
		IsOk:            true,
	})
}

func (s *Server) handleReadInterface(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applianceID")
	interfaceID := r.PathValue("interfaceID")
	iface, err := s.store.ReadInterface(id, interfaceID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, readServiceEndpointGatewayInterfaceResponseBody{Interface: simpleInterfaceToJSON(iface), IsOk: true})
}

func (s *Server) handleReadPowerStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applianceID")
	a, err := s.store.Read(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, readServiceEndpointGatewayPowerStatusResponseBody{Instance: instanceForPowerJSON{Status: a.PowerStatus}, IsOk: true})
}

func (s *Server) handlePowerOn(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applianceID")
	if _, err := s.store.PowerOn(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, updateServiceEndpointGatewayPowerStatusResponseBody{Success: true, IsOk: true})
}

func (s *Server) handlePowerOff(w http.ResponseWriter, r *http.Request) {
	// The spec's {"Force": bool} body is accepted (validated by the spec-derived
	// validator middleware) but not read: the mock powers off unconditionally.
	id := r.PathValue("applianceID")
	if _, err := s.store.PowerOff(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, updateServiceEndpointGatewayPowerStatusResponseBody{Success: true, IsOk: true})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("applianceID")
	if _, err := s.store.Reset(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, updateServiceEndpointGatewayPowerStatusResponseBody{Success: true, IsOk: true})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	core.WriteStandardError(w, status, "", msg)
}
