package addon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sacloud/sakumock/core"
)

// JSON types matching the Add-on OpenAPI spec.

type errorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Errors []errorInfo `json:"errors"`
}

type resourceType struct {
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
}

type azureLocation struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// resourceIdentifier is the spec's ResourceIdentifier. Its recursive "parent"
// property is left out: a resource group has no parent resource.
type resourceIdentifier struct {
	ResourceType      resourceType  `json:"resourceType"`
	Name              string        `json:"name"`
	SubscriptionID    string        `json:"subscriptionId"`
	Provider          string        `json:"provider"`
	Location          azureLocation `json:"location"`
	ResourceGroupName string        `json:"resourceGroupName"`
}

type systemData struct {
	CreatedOn      string `json:"createdOn"`
	LastModifiedOn string `json:"lastModifiedOn"`
}

type resourceGroupData struct {
	ID                             resourceIdentifier `json:"id"`
	Name                           string             `json:"name"`
	ResourceType                   resourceType       `json:"resourceType"`
	SystemData                     systemData         `json:"systemData"`
	Location                       azureLocation      `json:"location"`
	ResourceGroupProvisioningState string             `json:"resourceGroupProvisioningState"`
}

type resourceGroupResource struct {
	ID      resourceIdentifier `json:"id"`
	HasData bool               `json:"hasData"`
	Data    resourceGroupData  `json:"data"`
	URL     string             `json:"url"`
}

type listResourcesResponse struct {
	Resources []resourceGroupResource `json:"resources"`
}

// getResourceResponse carries the deployed Azure resource in Data, whose
// shape is free-form in the spec and differs per family (see resourcedata.go).
type getResourceResponse struct {
	Data any    `json:"data"`
	URL  string `json:"url"`
}

type deploymentStatusProperties struct {
	ProvisioningState string `json:"provisioningState"`
	Timestamp         string `json:"timestamp"`
}

type deploymentStatus struct {
	ID         string                     `json:"id"`
	Name       string                     `json:"name"`
	Type       string                     `json:"type"`
	Properties deploymentStatusProperties `json:"properties"`
}

type postDeploymentResponse struct {
	ResourceGroupName string `json:"resourceGroupName"`
	DeploymentName    string `json:"deploymentName"`
}

type vulnerabilityResponse struct {
	ResourceGroupName string `json:"resourceGroupName"`
	InstallScript     string `json:"installScript"`
}

const (
	// subscriptionID is the dummy Azure subscription every mock resource
	// belongs to. It is the all-zero UUID so it is recognizable as synthetic.
	subscriptionID = "00000000-0000-0000-0000-000000000000"
	// resourceGroupProvider / resourceGroupTypeName are the ARM resource type
	// of a resource group, which is what the list and get endpoints return.
	resourceGroupProvider = "Microsoft.Resources"
	resourceGroupTypeName = "resourceGroups"
	// deploymentTypeName is the ARM resource type the status endpoint reports
	// on.
	deploymentTypeName = "Microsoft.Resources/deployments"
	// portalBaseURL is the base of the synthetic management portal URL the
	// mock reports; the real API returns a link into the SAKURA Cloud portal.
	portalBaseURL = "https://secure.sakura.ad.jp/cloud/addon"

	// Deployment provisioning states, in ARM's vocabulary.
	stateRunning   = "Running"
	stateSucceeded = "Succeeded"
)

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

// locationDisplayNames gives the regions the add-on API deploys to their
// human-readable name; an unknown region falls back to its own name.
var locationDisplayNames = map[string]string{"japaneast": "Japan East"}

func locationOf(name string) azureLocation {
	displayName, ok := locationDisplayNames[name]
	if !ok {
		displayName = name
	}
	return azureLocation{Name: name, DisplayName: displayName}
}

func identifierOf(r Resource) resourceIdentifier {
	return resourceIdentifier{
		ResourceType:      resourceType{Namespace: resourceGroupProvider, Type: resourceGroupTypeName},
		Name:              r.ResourceGroupName,
		SubscriptionID:    subscriptionID,
		Provider:          resourceGroupProvider,
		Location:          locationOf(r.Location),
		ResourceGroupName: r.ResourceGroupName,
	}
}

func resourceGroupDataOf(r Resource) resourceGroupData {
	return resourceGroupData{
		ID:           identifierOf(r),
		Name:         r.ResourceGroupName,
		ResourceType: resourceType{Namespace: resourceGroupProvider, Type: resourceGroupTypeName},
		SystemData: systemData{
			CreatedOn:      core.FormatRFC3339Nano(r.CreatedAt),
			LastModifiedOn: core.FormatRFC3339Nano(r.CreatedAt),
		},
		Location:                       locationOf(r.Location),
		ResourceGroupProvisioningState: stateSucceeded,
	}
}

func portalURL(r Resource) string {
	return fmt.Sprintf("%s/%s/%s", portalBaseURL, r.Kind, r.ResourceGroupName)
}

func (s *Server) handleList(f family) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resources := s.store.List(f.kind, time.Now())
		items := make([]resourceGroupResource, len(resources))
		for i, res := range resources {
			items[i] = resourceGroupResource{
				ID:      identifierOf(res),
				HasData: true,
				Data:    resourceGroupDataOf(res),
				URL:     portalURL(res),
			}
		}
		core.WriteJSON(w, http.StatusOK, listResourcesResponse{Resources: items})
	}
}

func (s *Server) handleCreate(f family) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		if len(body) == 0 {
			writeError(w, http.StatusBadRequest, "empty request body")
			return
		}
		var req createRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		res := s.store.Create(f.kind, req.Location, json.RawMessage(body))

		if f.kind == KindVulnerability {
			var vreq vulnerabilityCreateRequest
			if err := json.Unmarshal(body, &vreq); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			core.WriteJSON(w, http.StatusAccepted, vulnerabilityResponse{
				ResourceGroupName: res.ResourceGroupName,
				InstallScript:     installScript(vreq.OS, res.ResourceGroupName),
			})
			return
		}
		core.WriteJSON(w, http.StatusAccepted, postDeploymentResponse{
			ResourceGroupName: res.ResourceGroupName,
			DeploymentName:    res.DeploymentName,
		})
	}
}

func (s *Server) handleRead(f family) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("resourceGroupName")
		res, err := s.store.Read(f.kind, name, time.Now())
		if err != nil {
			// The real API 404s a resource group whose deployment is still
			// running, so a still-provisioning resource lands here too.
			writeNotFound(w, f, name)
			return
		}
		core.WriteJSON(w, http.StatusOK, getResourceResponse{
			Data: resourceData(res),
			URL:  portalURL(res),
		})
	}
}

func (s *Server) handleDelete(f family) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("resourceGroupName")
		if err := s.store.Delete(f.kind, name); err != nil {
			writeNotFound(w, f, name)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleStatus(f family) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("resourceGroupName")
		deployment := r.PathValue("deploymentName")
		res, err := s.store.ReadAny(f.kind, name)
		if err != nil || res.DeploymentName != deployment {
			writeNotFound(w, f, name)
			return
		}
		state := stateSucceeded
		if !res.Provisioned(time.Now()) {
			state = stateRunning
		}
		core.WriteJSON(w, http.StatusOK, deploymentStatus{
			ID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/%s/%s",
				subscriptionID, res.ResourceGroupName, deploymentTypeName, res.DeploymentName),
			Name: res.DeploymentName,
			Type: deploymentTypeName,
			Properties: deploymentStatusProperties{
				ProvisioningState: state,
				Timestamp:         core.FormatRFC3339Nano(res.CreatedAt),
			},
		})
	}
}

// installScript is the agent install script POST /security/vulnerability
// returns. The real API returns a script that installs the detection agent on
// the target VM; the mock returns a harmless stand-in that says so.
func installScript(osType int, resourceGroupName string) string {
	if osType == osTypeWindows {
		return strings.Join([]string{
			"# sakumock: dummy vulnerability detection agent install script (Windows)",
			fmt.Sprintf("Write-Host 'sakumock: no agent is installed for resource group %s'", resourceGroupName),
			"",
		}, "\n")
	}
	return strings.Join([]string{
		"#!/bin/sh",
		"# sakumock: dummy vulnerability detection agent install script (Linux)",
		fmt.Sprintf("echo 'sakumock: no agent is installed for resource group %s'", resourceGroupName),
		"",
	}, "\n")
}

func writeNotFound(w http.ResponseWriter, f family, name string) {
	writeErrorCode(w, http.StatusNotFound, "ResourceGroupNotFound",
		fmt.Sprintf("%s resource group %q not found", f.label, name))
}

// writeError responds with the spec's ErrorResponse envelope, deriving the
// error code from the status text.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeErrorCode(w, status, strings.ReplaceAll(http.StatusText(status), " ", ""), msg)
}

func writeErrorCode(w http.ResponseWriter, status int, code, msg string) {
	core.WriteJSON(w, status, errorResponse{Errors: []errorInfo{{Code: code, Message: msg}}})
}
