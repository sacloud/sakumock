package simplenotification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// storeSnapshot is the store's form in the state file (--state-dir).
type storeSnapshot struct {
	// Items holds the destinations, groups and routings, keyed by ID.
	Items map[string]*ServiceItem `json:"items"`
	// Messages is the accepted notifications, in send order.
	Messages []MessageRecord `json:"messages"`
	// NextMessageID is the ID the next accepted message gets. It is kept apart
	// from Messages because clearing them does not restart the numbering.
	NextMessageID int64 `json:"next_message_id"`
}

// Snapshot returns every item and accepted message, with the message ID
// counter, as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.MarshalIndent(storeSnapshot{
		Items:         s.items,
		Messages:      s.messages,
		NextMessageID: s.nextID,
	}, "", "  ")
}

// Restore replaces the store's items and messages with a snapshot, and
// advances the ID generator past the restored item IDs so new items never
// reuse one.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Items == nil {
		snap.Items = make(map[string]*ServiceItem)
	}
	for id, it := range snap.Items {
		if it == nil || it.ID != id {
			return fmt.Errorf("item %q: record is missing or has a different id", id)
		}
		// The snapshot is indented; the store keeps raw JSON as it is echoed
		// back in responses.
		var err error
		if it.Settings, err = compactJSON(it.Settings); err != nil {
			return fmt.Errorf("item %q settings: %w", id, err)
		}
		if it.Icon, err = compactJSON(it.Icon); err != nil {
			return fmt.Errorf("item %q icon: %w", id, err)
		}
	}
	if snap.NextMessageID < 1 {
		return fmt.Errorf("next_message_id %d: must be at least 1", snap.NextMessageID)
	}
	for _, m := range snap.Messages {
		if n, err := strconv.ParseInt(m.ID, 10, 64); err != nil || n >= snap.NextMessageID {
			return fmt.Errorf("message %q: id is not below next_message_id %d", m.ID, snap.NextMessageID)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = snap.Items
	s.messages = snap.Messages
	s.nextID = snap.NextMessageID
	for id := range s.items {
		s.ids.Observe(id)
	}
	return nil
}

// compactJSON strips the insignificant whitespace from raw, keeping nil as nil.
func compactJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
