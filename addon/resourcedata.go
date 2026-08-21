package addon

import (
	"encoding/json"
	"strconv"
)

// The add-on API's GetResourceResponse.data is free-form in the OpenAPI spec:
// it carries the deployed Azure resource, whose shape differs per family. The
// Terraform provider recovers the create parameters from it (see
// sacloud/terraform-provider-sakura internal/service/addon), so these types
// reproduce the fields it reads:
//
//	ai          data.location.name
//	datalake    data.location, data.sku.name ("<Performance>_<Redundancy>")
//	dwh/etl/query
//	            data.location
//	search      data.location, data.sku.name, data.properties.{partitionCount,
//	            replicaCount, hostingMode}
//	streaming   data.location, data.sku.capacity
//	cdn/waf/ddos
//	            data.location, data.sku.name ("<Level>_AzureFrontDoor"),
//	            data.endpoints[0].routes[0].properties.patternsToMatch,
//	            data.originGroups[0].origins[0].properties.{hostName,
//	            originHostHeader}
//
// The mock rebuilds them from the create request it stored, so a client that
// decodes them gets back exactly what it sent.

type provisioningProperties struct {
	ProvisioningState string `json:"provisioningState"`
}

type skuData struct {
	Name string `json:"name"`
	// Capacity carries the Stream Analytics streaming unit count; it is
	// omitted for the families whose SKU has no capacity.
	Capacity *int `json:"capacity,omitempty"`
}

type aiResourceData struct {
	Name       string                 `json:"name"`
	Location   azureLocation          `json:"location"`
	Sku        skuData                `json:"sku"`
	Properties provisioningProperties `json:"properties"`
}

// locationResourceData is the payload of the families whose only recoverable
// setting is the location (data warehouse, data ETL, query, vulnerability
// detection).
type locationResourceData struct {
	Name       string                 `json:"name"`
	Location   string                 `json:"location"`
	Properties provisioningProperties `json:"properties"`
}

type dataLakeResourceData struct {
	Name       string                 `json:"name"`
	Location   string                 `json:"location"`
	Sku        skuData                `json:"sku"`
	Kind       string                 `json:"kind"`
	Properties provisioningProperties `json:"properties"`
}

type searchProperties struct {
	ProvisioningState string `json:"provisioningState"`
	PartitionCount    int    `json:"partitionCount"`
	ReplicaCount      int    `json:"replicaCount"`
	HostingMode       string `json:"hostingMode"`
}

type searchResourceData struct {
	Name       string           `json:"name"`
	Location   string           `json:"location"`
	Sku        skuData          `json:"sku"`
	Properties searchProperties `json:"properties"`
}

type streamingResourceData struct {
	Name       string                 `json:"name"`
	Location   string                 `json:"location"`
	Sku        skuData                `json:"sku"`
	Properties provisioningProperties `json:"properties"`
}

type frontDoorRouteProperties struct {
	PatternsToMatch    []string `json:"patternsToMatch"`
	SupportedProtocols []string `json:"supportedProtocols"`
}

type frontDoorRouteData struct {
	Name       string                   `json:"name"`
	Properties frontDoorRouteProperties `json:"properties"`
}

type frontDoorEndpointData struct {
	Name   string               `json:"name"`
	Routes []frontDoorRouteData `json:"routes"`
}

type frontDoorOriginProperties struct {
	HostName         string `json:"hostName"`
	OriginHostHeader string `json:"originHostHeader"`
}

type frontDoorOriginData struct {
	Name       string                    `json:"name"`
	Properties frontDoorOriginProperties `json:"properties"`
}

type frontDoorOriginGroupData struct {
	Name    string                `json:"name"`
	Origins []frontDoorOriginData `json:"origins"`
}

type frontDoorResourceData struct {
	Name         string                     `json:"name"`
	Location     string                     `json:"location"`
	Sku          skuData                    `json:"sku"`
	Properties   provisioningProperties     `json:"properties"`
	Endpoints    []frontDoorEndpointData    `json:"endpoints"`
	OriginGroups []frontDoorOriginGroupData `json:"originGroups"`
}

// Create request bodies, parsed to rebuild the deployed resource. Only the
// fields that survive into the response are declared.

type aiCreateRequest struct {
	Sku int `json:"sku"`
}

type dataLakeCreateRequest struct {
	Performance int `json:"performance"`
	Redundancy  int `json:"redundancy"`
}

type searchCreateRequest struct {
	Sku            int `json:"sku"`
	ReplicaCount   int `json:"replicaCount"`
	PartitionCount int `json:"partitionCount"`
}

type streamingCreateRequest struct {
	UnitCount string `json:"unitCount"`
}

type frontDoorCreateRequest struct {
	Profile struct {
		Level int `json:"level"`
	} `json:"profile"`
	Endpoint struct {
		Route struct {
			Patterns    []string `json:"patterns"`
			OriginGroup struct {
				Origin struct {
					HostName   string `json:"hostName"`
					HostHeader string `json:"hostHeader"`
				} `json:"origin"`
			} `json:"originGroup"`
		} `json:"route"`
	} `json:"endpoint"`
}

// SKU names as the deployed Azure resource reports them, keyed by the enum
// value the spec's request bodies use.
var (
	aiSkuNames               = map[int]string{1: "S0"}
	dataLakePerformanceNames = map[int]string{1: "Standard", 2: "Premium"}
	dataLakeRedundancyNames  = map[int]string{1: "LRS", 2: "GRS", 3: "ZRS", 4: "GZRS"}
	frontDoorSkuNames        = map[int]string{1: "Standard_AzureFrontDoor", 2: "Premium_AzureFrontDoor"}
	// Standard3HD (6) is Standard3 deployed in high-density hosting mode,
	// which is how it is told apart from Standard3 (5) on read.
	searchSkuNames = map[int]string{
		1: "free", 2: "basic", 3: "standard1", 4: "standard2",
		5: "standard3", 6: "standard3", 7: "storage_optimized_l1", 8: "storage_optimized_l2",
	}
)

const searchHighDensitySku = 6

// resourceData builds the deployed-resource payload of GetResourceResponse.data
// from the stored create request. The request body was validated against the
// spec before it was stored, so decoding it cannot fail meaningfully; a field
// the client omitted simply stays at its zero value.
func resourceData(r Resource) any {
	succeeded := provisioningProperties{ProvisioningState: stateSucceeded}

	switch r.Kind {
	case KindAI:
		var req aiCreateRequest
		_ = json.Unmarshal(r.Parameters, &req)
		return aiResourceData{
			Name:       r.ResourceGroupName,
			Location:   locationOf(r.Location),
			Sku:        skuData{Name: aiSkuNames[req.Sku]},
			Properties: succeeded,
		}

	case KindDataLake:
		var req dataLakeCreateRequest
		_ = json.Unmarshal(r.Parameters, &req)
		return dataLakeResourceData{
			Name:     r.ResourceGroupName,
			Location: r.Location,
			// The storage account SKU is "<Performance>_<Redundancy>".
			Sku:        skuData{Name: dataLakePerformanceNames[req.Performance] + "_" + dataLakeRedundancyNames[req.Redundancy]},
			Kind:       "StorageV2",
			Properties: succeeded,
		}

	case KindSearch:
		var req searchCreateRequest
		_ = json.Unmarshal(r.Parameters, &req)
		hostingMode := "default"
		if req.Sku == searchHighDensitySku {
			hostingMode = "highDensity"
		}
		return searchResourceData{
			Name:     r.ResourceGroupName,
			Location: r.Location,
			Sku:      skuData{Name: searchSkuNames[req.Sku]},
			Properties: searchProperties{
				ProvisioningState: stateSucceeded,
				PartitionCount:    req.PartitionCount,
				ReplicaCount:      req.ReplicaCount,
				HostingMode:       hostingMode,
			},
		}

	case KindStreaming:
		var req streamingCreateRequest
		_ = json.Unmarshal(r.Parameters, &req)
		capacity, _ := strconv.Atoi(req.UnitCount)
		return streamingResourceData{
			Name:       r.ResourceGroupName,
			Location:   r.Location,
			Sku:        skuData{Name: "Standard", Capacity: &capacity},
			Properties: succeeded,
		}

	case KindCDN, KindWAF, KindDDoS:
		var req frontDoorCreateRequest
		_ = json.Unmarshal(r.Parameters, &req)
		origin := req.Endpoint.Route.OriginGroup.Origin
		patterns := req.Endpoint.Route.Patterns
		if patterns == nil {
			patterns = []string{}
		}
		return frontDoorResourceData{
			Name:       r.ResourceGroupName,
			Location:   r.Location,
			Sku:        skuData{Name: frontDoorSkuNames[req.Profile.Level]},
			Properties: succeeded,
			Endpoints: []frontDoorEndpointData{{
				Name: r.ResourceGroupName + "-endpoint",
				Routes: []frontDoorRouteData{{
					Name: r.ResourceGroupName + "-route",
					Properties: frontDoorRouteProperties{
						PatternsToMatch:    patterns,
						SupportedProtocols: []string{"Http", "Https"},
					},
				}},
			}},
			OriginGroups: []frontDoorOriginGroupData{{
				Name: r.ResourceGroupName + "-origin-group",
				Origins: []frontDoorOriginData{{
					Name: r.ResourceGroupName + "-origin",
					Properties: frontDoorOriginProperties{
						HostName:         origin.HostName,
						OriginHostHeader: origin.HostHeader,
					},
				}},
			}},
		}

	default: // data warehouse, data ETL, query, vulnerability detection
		return locationResourceData{
			Name:       r.ResourceGroupName,
			Location:   r.Location,
			Properties: succeeded,
		}
	}
}
