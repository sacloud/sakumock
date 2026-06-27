package simplenotification

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// InspectionClient is an HTTP client for the mock-only /_sakumock/ endpoints
// exposed by the simplenotification server. It lets integration tests inspect
// accepted messages over the network (e.g. against a running sakumock process
// or container) without in-process access.
type InspectionClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewInspectionClient returns a client that talks to the simplenotification
// inspection endpoints at baseURL (e.g. "http://localhost:18083").
func NewInspectionClient(baseURL string) *InspectionClient {
	return &InspectionClient{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
	}
}

// Messages returns all accepted notification messages.
func (c *InspectionClient) Messages(ctx context.Context) ([]MessageRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/_sakumock/messages", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("simplenotification: GET /_sakumock/messages: unexpected status %d", resp.StatusCode)
	}
	var list inspectMessageList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("simplenotification: decode response: %w", err)
	}
	records := make([]MessageRecord, len(list.Messages))
	for i, m := range list.Messages {
		t, err := time.Parse(time.RFC3339Nano, m.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("simplenotification: parse created_at %q: %w", m.CreatedAt, err)
		}
		records[i] = MessageRecord{
			ID:        m.ID,
			GroupID:   m.GroupID,
			Message:   m.Message,
			CreatedAt: t,
		}
	}
	return records, nil
}

// ClearMessages discards all accepted messages.
func (c *InspectionClient) ClearMessages(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/_sakumock/messages", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("simplenotification: DELETE /_sakumock/messages: unexpected status %d", resp.StatusCode)
	}
	return nil
}
