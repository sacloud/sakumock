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

type initialDataResponse struct {
	PartitionName string `json:"PartitionName"`
	PartitionID   string `json:"PartitionID"`
	Certificate   string `json:"Certificate"`
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
	IPv4NetworkAddress string               `json:"IPv4NetworkAddress"`
	IPv4PrefixLength   int                  `json:"IPv4PrefixLength"`
	IPv4Address        string               `json:"IPv4Address"`
	LocalRouter        *localRouterResponse `json:"LocalRouter"`
	InitialData        *initialDataResponse `json:"InitialData"`
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
	IPv4NetworkAddress string   `json:"IPv4NetworkAddress"`
	IPv4PrefixLength   int      `json:"IPv4PrefixLength"`
	IPv4Address        string   `json:"IPv4Address"`
}

type wrappedCloudHSM struct {
	CloudHSM cloudhsmResponse `json:"CloudHSM"`
	IsOk     bool             `json:"is_ok"`
}

type wrappedCreateCloudHSM struct {
	CloudHSM createCloudHSMResponse `json:"CloudHSM"`
	IsOk     bool                   `json:"is_ok"`
}

type paginatedCloudHSMList struct {
	Count     int                `json:"Count"`
	From      int                `json:"From"`
	Total     int                `json:"Total"`
	CloudHSMs []cloudhsmResponse `json:"CloudHSMs"`
	IsOk      bool               `json:"is_ok"`
}

type createCloudHSMRequest struct {
	CloudHSM struct {
		Name               string   `json:"Name"`
		Description        string   `json:"Description"`
		Tags               []string `json:"Tags"`
		IPv4NetworkAddress string   `json:"IPv4NetworkAddress"`
		IPv4PrefixLength   int      `json:"IPv4PrefixLength"`
	} `json:"CloudHSM"`
}

type updateCloudHSMRequest struct {
	CloudHSM struct {
		Name               string   `json:"Name"`
		Description        string   `json:"Description"`
		Tags               []string `json:"Tags"`
		IPv4NetworkAddress string   `json:"IPv4NetworkAddress"`
		IPv4PrefixLength   int      `json:"IPv4PrefixLength"`
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
	IsOk   bool           `json:"is_ok"`
}

type paginatedClientList struct {
	Count   int              `json:"Count"`
	From    int              `json:"From"`
	Total   int              `json:"Total"`
	Clients []clientResponse `json:"Clients"`
	IsOk    bool             `json:"is_ok"`
}

type createClientRequest struct {
	Client struct {
		Name        string `json:"Name"`
		Certificate string `json:"Certificate"`
	} `json:"Client"`
}

type updateClientRequest struct {
	Client struct {
		Name string `json:"Name"`
	} `json:"Client"`
}

type peerResponse struct {
	ID          string `json:"ID"`
	SecretKey   string `json:"SecretKey"`
	Enabled     bool   `json:"Enabled"`
	Description string `json:"Description"`
}

type peerListResponse struct {
	Peers []peerResponse `json:"Peers"`
	IsOk  bool           `json:"is_ok"`
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
	IsOk    bool            `json:"is_ok"`
}

type paginatedLicenseList struct {
	Count    int               `json:"Count"`
	From     int               `json:"From"`
	Total    int               `json:"Total"`
	Licenses []licenseResponse `json:"Licenses"`
	IsOk     bool              `json:"is_ok"`
}

type documentResponse struct {
	ID         string `json:"ID"`
	CreatedAt  string `json:"CreatedAt"`
	ModifiedAt string `json:"ModifiedAt"`
	Name       string `json:"Name"`
}

type paginatedDocumentList struct {
	Count             int                `json:"Count"`
	From              int                `json:"From"`
	Total             int                `json:"Total"`
	CloudHSMDocuments []documentResponse `json:"CloudHSMDocuments"`
	IsOk              bool               `json:"is_ok"`
}

type documentDownloadResponse struct {
	URL string `json:"URL"`
}

type wrappedDocumentDownload struct {
	Document documentDownloadResponse `json:"Document"`
	IsOk     bool                     `json:"is_ok"`
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

// ServeHTTP dispatches the request to the matching route and logs the result.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	rw := core.NewResponseRecorder(w)
	s.mux.ServeHTTP(rw, r)
	s.logger.Info("request", core.RequestLogArgs(r, rw)...)
}

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
		IPv4NetworkAddress: h.IPv4NetworkAddress,
		IPv4PrefixLength:   h.IPv4PrefixLength,
		IPv4Address:        h.IPv4Address,
		LocalRouter:        nil,
		InitialData:        nil,
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
		IPv4NetworkAddress: h.IPv4NetworkAddress,
		IPv4PrefixLength:   h.IPv4PrefixLength,
		IPv4Address:        h.IPv4Address,
	}
}

func (s *Server) handleListCloudHSMs(w http.ResponseWriter, r *http.Request) {
	hsms := s.store.ListCloudHSMs()
	p, err := core.ParsePage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	page := core.Paginate(hsms, p)
	items := make([]cloudhsmResponse, len(page))
	for i, h := range page {
		items[i] = cloudhsmRecordToResponse(h)
	}
	core.WriteJSON(w, http.StatusOK, paginatedCloudHSMList{
		Count:     len(items),
		From:      p.From,
		Total:     len(hsms),
		CloudHSMs: items,
		IsOk:      true,
	})
}

func (s *Server) handleCreateCloudHSM(w http.ResponseWriter, r *http.Request) {
	var req createCloudHSMRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h, err := s.store.CreateCloudHSM(req.CloudHSM.Name, req.CloudHSM.Description, req.CloudHSM.Tags, req.CloudHSM.IPv4NetworkAddress, req.CloudHSM.IPv4PrefixLength)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Debug("cloudhsm created", "id", h.ID, "name", h.Name)
	core.WriteJSON(w, http.StatusCreated, wrappedCreateCloudHSM{CloudHSM: cloudhsmRecordToCreateResponse(h), IsOk: true})
}

func (s *Server) handleReadCloudHSM(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	h, err := s.store.ReadCloudHSM(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedCloudHSM{CloudHSM: cloudhsmRecordToResponse(h), IsOk: true})
}

func (s *Server) handleUpdateCloudHSM(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	var req updateCloudHSMRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h, err := s.store.UpdateCloudHSM(id, req.CloudHSM.Name, req.CloudHSM.Description, req.CloudHSM.Tags, req.CloudHSM.IPv4NetworkAddress, req.CloudHSM.IPv4PrefixLength)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedCloudHSM{CloudHSM: cloudhsmRecordToResponse(h), IsOk: true})
}

func (s *Server) handleDeleteCloudHSM(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	if err := s.store.DeleteCloudHSM(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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
	p, err := core.ParsePage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	page := core.Paginate(clients, p)
	items := make([]clientResponse, len(page))
	for i, c := range page {
		items[i] = clientRecordToResponse(c)
	}
	core.WriteJSON(w, http.StatusOK, paginatedClientList{
		Count:   len(items),
		From:    p.From,
		Total:   len(clients),
		Clients: items,
		IsOk:    true,
	})
}

func (s *Server) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("cloudhsm_resource_id")
	var req createClientRequest
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
	core.WriteJSON(w, http.StatusCreated, wrappedClient{Client: clientRecordToResponse(c), IsOk: true})
}

func (s *Server) handleReadClient(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("cloudhsm_resource_id")
	id := r.PathValue("id")
	c, err := s.store.ReadClient(hsmID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedClient{Client: clientRecordToResponse(c), IsOk: true})
}

func (s *Server) handleUpdateClient(w http.ResponseWriter, r *http.Request) {
	hsmID := r.PathValue("cloudhsm_resource_id")
	id := r.PathValue("id")
	var req updateClientRequest
	if err := core.ReadJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.store.UpdateClient(hsmID, id, req.Client.Name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedClient{Client: clientRecordToResponse(c), IsOk: true})
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

func peerRecordToResponse(p PeerRecord) peerResponse {
	return peerResponse{
		ID:          p.ID,
		SecretKey:   p.SecretKey,
		Enabled:     p.Enabled,
		Description: p.Description,
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
	core.WriteJSON(w, http.StatusOK, peerListResponse{Peers: items, IsOk: true})
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
	p, err := s.store.CreatePeer(hsmID, req.Peer.ID, req.Peer.SecretKey)
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
	p, err := core.ParsePage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	page := core.Paginate(licenses, p)
	items := make([]licenseResponse, len(page))
	for i, l := range page {
		items[i] = licenseRecordToResponse(l)
	}
	core.WriteJSON(w, http.StatusOK, paginatedLicenseList{
		Count:    len(items),
		From:     p.From,
		Total:    len(licenses),
		Licenses: items,
		IsOk:     true,
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
	core.WriteJSON(w, http.StatusCreated, wrappedLicense{License: licenseRecordToResponse(l), IsOk: true})
}

func (s *Server) handleReadLicense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	l, err := s.store.ReadLicense(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedLicense{License: licenseRecordToResponse(l), IsOk: true})
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
	core.WriteJSON(w, http.StatusOK, wrappedLicense{License: licenseRecordToResponse(l), IsOk: true})
}

func (s *Server) handleDeleteLicense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("resource_id")
	if err := s.store.DeleteLicense(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	docs, err := s.store.ListDocuments(r.PathValue("license_resource_id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	p, err := core.ParsePage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	page := core.Paginate(docs, p)
	items := make([]documentResponse, len(page))
	for i, d := range page {
		items[i] = documentResponse{
			ID:         d.ID,
			CreatedAt:  core.FormatRFC3339Nano(d.CreatedAt),
			ModifiedAt: core.FormatRFC3339Nano(d.ModifiedAt),
			Name:       d.Name,
		}
	}
	core.WriteJSON(w, http.StatusOK, paginatedDocumentList{
		Count:             len(items),
		From:              p.From,
		Total:             len(docs),
		CloudHSMDocuments: items,
		IsOk:              true,
	})
}

func (s *Server) handleDownloadDocument(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.ReadDocument(r.PathValue("license_resource_id"), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	core.WriteJSON(w, http.StatusOK, wrappedDocumentDownload{
		Document: documentDownloadResponse{URL: documentURL(d)},
		IsOk:     true,
	})
}

// documentURL returns the download URL handed out for a license document.
// The mock hosts no files, so it points at a reserved example.com name.
func documentURL(d DocumentRecord) string {
	return "https://example.com/cloudhsm/licenses/" + d.LicenseID + "/documents/" + d.ID + "/" + d.Name
}

func writeError(w http.ResponseWriter, status int, msg string) {
	core.WriteJSON(w, status, map[string]string{"error": msg})
}
