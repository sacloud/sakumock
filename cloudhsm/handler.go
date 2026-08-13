package cloudhsm

import (
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

// JSON types matching the CloudHSM OpenAPI spec.

type localRouterResponse struct {
	ResourceID string `json:"ResourceID"`
	SecretKey  string `json:"SecretKey"`
}

type cloudhsmResponse struct {
	ID                 string               `json:"ID"`
	CreatedAt          string               `json:"CreatedAt"`
	ModifiedAt         string               `json:"ModifiedAt"`
	ServiceClass       string               `json:"ServiceClass"`
	Availability       string               `json:"Availability"`
	Name               string               `json:"Name"`
	Description        string               `json:"Description"`
	Tags               []string             `json:"Tags"`
	Ipv4NetworkAddress string               `json:"Ipv4NetworkAddress"`
	Ipv4PrefixLength   int                  `json:"Ipv4PrefixLength"`
	Ipv4Address        string               `json:"Ipv4Address"`
	LocalRouter        *localRouterResponse `json:"LocalRouter"`
}

type createCloudHSMResponse struct {
	ID                 string   `json:"ID"`
	CreatedAt          string   `json:"CreatedAt"`
	ModifiedAt         string   `json:"ModifiedAt"`
	ServiceClass       string   `json:"ServiceClass"`
	Availability       string   `json:"Availability"`
	Name               string   `json:"Name"`
	Description        string   `json:"Description"`
	Tags               []string `json:"Tags"`
	Ipv4NetworkAddress string   `json:"Ipv4NetworkAddress"`
	Ipv4PrefixLength   int      `json:"Ipv4PrefixLength"`
	Ipv4Address        string   `json:"Ipv4Address"`
}

type wrappedCloudHSM struct {
	CloudHSM cloudhsmResponse `json:"CloudHSM"`
}

type wrappedCreateCloudHSM struct {
	CloudHSM createCloudHSMResponse `json:"CloudHSM"`
}

type paginatedCloudHSMList struct {
	Count     int                `json:"Count"`
	From      int                `json:"From"`
	Total     int                `json:"Total"`
	CloudHSMs []cloudhsmResponse `json:"CloudHSMs"`
}

type createCloudHSMRequest struct {
	CloudHSM struct {
		Name               string   `json:"Name"`
		Description        string   `json:"Description"`
		Tags               []string `json:"Tags"`
		Ipv4NetworkAddress string   `json:"Ipv4NetworkAddress"`
		Ipv4PrefixLength   int      `json:"Ipv4PrefixLength"`
	} `json:"CloudHSM"`
}

type updateCloudHSMRequest struct {
	CloudHSM struct {
		Name               string   `json:"Name"`
		Description        string   `json:"Description"`
		Tags               []string `json:"Tags"`
		Ipv4NetworkAddress string   `json:"Ipv4NetworkAddress"`
		Ipv4PrefixLength   int      `json:"Ipv4PrefixLength"`
	} `json:"CloudHSM"`
}

type clientResponse struct {
	ID           string `json:"ID"`
	CreatedAt    string `json:"CreatedAt"`
	ModifiedAt   string `json:"ModifiedAt"`
	Availability string `json:"Availability"`
	Name         string `json:"Name"`
	Certificate  string `json:"Certificate"`
}

type wrappedClient struct {
	Client clientResponse `json:"Client"`
}

type paginatedClientList struct {
	Count   int              `json:"Count"`
	From    int              `json:"From"`
	Total   int              `json:"Total"`
	Clients []clientResponse `json:"Clients"`
}

type clientRequest struct {
	Client struct {
		Name        string `json:"Name"`
		Certificate string `json:"Certificate"`
	} `json:"Client"`
}

type peerResponse struct {
	ID     string   `json:"ID"`
	Index  int      `json:"Index"`
	Status string   `json:"Status"`
	Routes []string `json:"Routes"`
}

type peerListResponse struct {
	Peers []peerResponse `json:"Peers"`
}

type createPeerRequest struct {
	Peer struct {
		ID        string `json:"ID"`
		SecretKey string `json:"SecretKey"`
	} `json:"Peer"`
}

type licenseResponse struct {
	ID           string   `json:"ID"`
	CreatedAt    string   `json:"CreatedAt"`
	ModifiedAt   string   `json:"ModifiedAt"`
	ServiceClass string   `json:"ServiceClass"`
	Name         string   `json:"Name"`
	Description  string   `json:"Description"`
	Tags         []string `json:"Tags"`
}

type wrappedLicense struct {
	License licenseResponse `json:"License"`
}

type paginatedLicenseList struct {
	Count    int               `json:"Count"`
	From     int               `json:"From"`
	Total    int               `json:"Total"`
	Licenses []licenseResponse `json:"Licenses"`
}

type licenseRequest struct {
	License struct {
		Name        string   `json:"Name"`
		Description string   `json:"Description"`
		Tags        []string `json:"Tags"`
	} `json:"License"`
}

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, r := range s.routeTable() {
		mux.HandleFunc(r.Method+" "+r.Path, r.Handler)
	}
	return mux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	rw := core.NewResponseRecorder(w)
	s.mux.ServeHTTP(rw, r)
	s.logger.Info("request", core.RequestLogArgs(r, rw)...)
}

// --- CloudHSM ---

func cloudhsmRecordToResponse(h CloudHSMRecord) cloudhsmResponse {
	return cloudhsmResponse{
		ID:                 h.ID,
		CreatedAt:          core.FormatRFC3339Nano(h.CreatedAt),
		ModifiedAt:         core.FormatRFC3339Nano(h.ModifiedAt),
		ServiceClass:       "cloud/cloudhsm/partition",
		Availability:       h.Availability,
		Name:               h.Name,
		Description:        h.Description,
		Tags:               h.Tags,
		Ipv4NetworkAddress: h.Ipv4NetworkAddress,
		Ipv4PrefixLength:   h.Ipv4PrefixLength,
		Ipv4Address:        h.Ipv4Address,
		LocalRouter:        nil,
	}
}

func cloudhsmRecordToCreateResponse(h CloudHSMRecord) createCloudHSMResponse {
	return createCloudHSMResponse{
		ID:                 h.ID,
		CreatedAt:          core.FormatRFC3339Nano(h.CreatedAt),
		ModifiedAt:         core.FormatRFC3339Nano(h.ModifiedAt),
		ServiceClass:       "cloud/cloudhsm/partition",
		Availability:       h.Availability,
		Name:               h.Name,
		Description:        h.Description,
		Tags:               h.Tags,
		Ipv4NetworkAddress: h.Ipv4NetworkAddress,
		Ipv4PrefixLength:   h.Ipv4PrefixLength,
		Ipv4Address:        h.Ipv4Address,
	}
}

func (s *Server) handleListCloudHSMs(w http.ResponseWriter, r *http.Request) {
	hsms := s.store.ListCloudHSMs()
	items := make([]cloudhsmResponse, len(hsms))
	for i, h := range hsms {
		items[i] = cloudhsmRecordToResponse(h)
	}
	core.WriteJSON(w, http.StatusOK, paginatedCloudHSMList{
		Count:     len(items),
		From:      0,
		Total:     len(items),
		CloudHSMs: items,
	})
}

func (s *Server) handleCreateCloudHSM(w http.ResponseWriter, r *http.Request) {
	var req createCloudHSMRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h, err := s.store.CreateCloudHSM(req.CloudHSM.Name, req.CloudHSM.Description, req.CloudHSM.Tags, req.CloudHSM.Ipv4NetworkAddress, req.CloudHSM.Ipv4PrefixLength)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Debug("cloudhsm created", "id", h.ID, "name", h.Name)
	core.WriteJSON(w, http.StatusCreated, wrappedCreateCloudHSM{CloudHSM: cloudhsmRecordToCreateResponse(h)})
}

func (s *Server) handleReadCloudHSM(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	h, err := s.store.ReadCloudHSM(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedCloudHSM{CloudHSM: cloudhsmRecordToResponse(h)})
}

func (s *Server) handleUpdateCloudHSM(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req updateCloudHSMRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h, err := s.store.UpdateCloudHSM(id, req.CloudHSM.Name, req.CloudHSM.Description, req.CloudHSM.Tags, req.CloudHSM.Ipv4NetworkAddress, req.CloudHSM.Ipv4PrefixLength)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedCloudHSM{CloudHSM: cloudhsmRecordToResponse(h)})
}

func (s *Server) handleDeleteCloudHSM(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	if err := s.store.DeleteCloudHSM(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Clients ---

func clientRecordToResponse(c ClientRecord) clientResponse {
	return clientResponse{
		ID:           c.ID,
		CreatedAt:    core.FormatRFC3339Nano(c.CreatedAt),
		ModifiedAt:   core.FormatRFC3339Nano(c.ModifiedAt),
		Availability: c.Availability,
		Name:         c.Name,
		Certificate:  c.Certificate,
	}
}

func (s *Server) handleListClients(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("cloudhsm_resource_id")
	clients, err := s.store.ListClients(hsmID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	items := make([]clientResponse, len(clients))
	for i, c := range clients {
		items[i] = clientRecordToResponse(c)
	}
	core.WriteJSON(w, http.StatusOK, paginatedClientList{
		Count:   len(items),
		From:    0,
		Total:   len(items),
		Clients: items,
	})
}

func (s *Server) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("cloudhsm_resource_id")
	var req clientRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.store.CreateClient(hsmID, req.Client.Name, req.Client.Certificate)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.logger.Debug("cloudhsm client created", "hsm_id", hsmID, "id", c.ID, "name", c.Name)
	core.WriteJSON(w, http.StatusCreated, wrappedClient{Client: clientRecordToResponse(c)})
}

func (s *Server) handleReadClient(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("cloudhsm_resource_id")
	id := r.PathValue("id")
	c, err := s.store.ReadClient(hsmID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedClient{Client: clientRecordToResponse(c)})
}

func (s *Server) handleUpdateClient(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("cloudhsm_resource_id")
	id := r.PathValue("id")
	var req clientRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.store.UpdateClient(hsmID, id, req.Client.Name, req.Client.Certificate)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedClient{Client: clientRecordToResponse(c)})
}

func (s *Server) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("cloudhsm_resource_id")
	id := r.PathValue("id")
	if err := s.store.DeleteClient(hsmID, id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Peers ---

func peerRecordToResponse(p PeerRecord) peerResponse {
	return peerResponse{
		ID:     p.ID,
		Index:  p.Index,
		Status: p.Status,
		Routes: p.Routes,
	}
}

func (s *Server) handleListPeers(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("resource_id")
	peers, err := s.store.ListPeers(hsmID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	items := make([]peerResponse, len(peers))
	for i, p := range peers {
		items[i] = peerRecordToResponse(p)
	}
	core.WriteJSON(w, http.StatusOK, peerListResponse{Peers: items})
}

func (s *Server) handleCreatePeer(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("resource_id")
	if _, err := s.store.ReadCloudHSM(hsmID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req createPeerRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err := s.store.CreatePeer(hsmID, req.Peer.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Debug("cloudhsm peer created", "hsm_id", hsmID, "id", p.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeletePeer(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("resource_id")
	peerID := r.PathValue("peer_id")
	if err := s.store.DeletePeer(hsmID, peerID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Licenses ---

func licenseRecordToResponse(l LicenseRecord) licenseResponse {
	return licenseResponse{
		ID:           l.ID,
		CreatedAt:    core.FormatRFC3339Nano(l.CreatedAt),
		ModifiedAt:   core.FormatRFC3339Nano(l.ModifiedAt),
		ServiceClass: "cloud/cloudhsm/license/l7",
		Name:         l.Name,
		Description:  l.Description,
		Tags:         l.Tags,
	}
}

func (s *Server) handleListLicenses(w http.ResponseWriter, r *http.Request) {
	licenses := s.store.ListLicenses()
	items := make([]licenseResponse, len(licenses))
	for i, l := range licenses {
		items[i] = licenseRecordToResponse(l)
	}
	core.WriteJSON(w, http.StatusOK, paginatedLicenseList{
		Count:    len(items),
		From:     0,
		Total:    len(items),
		Licenses: items,
	})
}

func (s *Server) handleCreateLicense(w http.ResponseWriter, r *http.Request) {
	var req licenseRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	l, err := s.store.CreateLicense(req.License.Name, req.License.Description, req.License.Tags)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Debug("cloudhsm license created", "id", l.ID, "name", l.Name)
	core.WriteJSON(w, http.StatusCreated, wrappedLicense{License: licenseRecordToResponse(l)})
}

func (s *Server) handleReadLicense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	l, err := s.store.ReadLicense(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedLicense{License: licenseRecordToResponse(l)})
}

func (s *Server) handleUpdateLicense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req licenseRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	l, err := s.store.UpdateLicense(id, req.License.Name, req.License.Description, req.License.Tags)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedLicense{License: licenseRecordToResponse(l)})
}

func (s *Server) handleDeleteLicense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	if err := s.store.DeleteLicense(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	core.WriteJSON(w, status, map[string]string{"error": msg})
}
