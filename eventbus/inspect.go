package eventbus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// InspectionClient is an HTTP client for the mock-only /_sakumock/ endpoints
// exposed by the eventbus server. It lets integration tests drive and observe
// the data plane over the network (e.g. against a running sakumock process or
// container) without in-process access.
type InspectionClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewInspectionClient returns a client that talks to the eventbus inspection
// endpoints at baseURL (e.g. "http://localhost:18085").
func NewInspectionClient(baseURL string) *InspectionClient {
	return &InspectionClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: http.DefaultClient,
	}
}

// InjectEvent posts an event and returns the resulting deliveries from
// matching triggers.
func (c *InspectionClient) InjectEvent(ctx context.Context, ev Event) ([]Delivery, error) {
	body, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("eventbus: marshal event: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/_sakumock/events", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doDeliveries(req)
}

// Tick forces a scheduler evaluation. If at is non-zero, schedules are
// evaluated at that time; otherwise the server uses its current time.
func (c *InspectionClient) Tick(ctx context.Context, at time.Time) ([]Delivery, error) {
	u := c.baseURL + "/_sakumock/tick"
	if !at.IsZero() {
		u += "?at=" + url.QueryEscape(at.Format(time.RFC3339))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return nil, err
	}
	return c.doDeliveries(req)
}

// Deliveries returns all recorded firings.
func (c *InspectionClient) Deliveries(ctx context.Context) ([]Delivery, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/_sakumock/deliveries", nil)
	if err != nil {
		return nil, err
	}
	return c.doDeliveries(req)
}

// ClearDeliveries discards all recorded firings.
func (c *InspectionClient) ClearDeliveries(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/_sakumock/deliveries", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("eventbus: DELETE /_sakumock/deliveries: unexpected status %d", resp.StatusCode)
	}
	return nil
}

type inspectDeliveriesResponse struct {
	Deliveries []Delivery `json:"Deliveries"`
	Count      int        `json:"Count"`
}

func (c *InspectionClient) doDeliveries(req *http.Request) ([]Delivery, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eventbus: %s %s: unexpected status %d", req.Method, req.URL.Path, resp.StatusCode)
	}
	var result inspectDeliveriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("eventbus: decode response: %w", err)
	}
	return result.Deliveries, nil
}
