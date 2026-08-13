package simplenotification

import (
	"encoding/json"
	"testing"
)

func TestParseMessagePayload(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want notificationMessageJSON
	}{
		{
			name: "plain text",
			raw:  "hello",
			want: notificationMessageJSON{Body: "hello", Color: "default", ColorCode: "#7d7d7d"},
		},
		{
			name: "rich payload",
			raw:  `{"body":"b","title":"t","color":"info","color_code":"#00f"}`,
			want: notificationMessageJSON{Body: "b", Title: "t", Color: "info", ColorCode: "#00f"},
		},
		{
			name: "empty body keeps rich fields",
			raw:  `{"body":"","title":"t"}`,
			want: notificationMessageJSON{Body: "", Title: "t", Color: "default", ColorCode: "#7d7d7d"},
		},
		{
			name: "JSON without body key is plain text",
			raw:  `{"foo":1}`,
			want: notificationMessageJSON{Body: `{"foo":1}`, Color: "default", ColorCode: "#7d7d7d"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseMessagePayload(c.raw); got != c.want {
				t.Errorf("parseMessagePayload(%q) = %+v, want %+v", c.raw, got, c.want)
			}
		})
	}
}

func TestGroupDestinationsFiltersInvalidIDs(t *testing.T) {
	store := NewMemoryStore(nil)
	s := &Server{store: store}
	it := store.CreateItem(ServiceItem{
		Name:          "g",
		ProviderClass: "saknoticegroup",
		Settings:      json.RawMessage(`{"Destinations":["123456789012","abc","1"]}`),
	})
	got := s.groupDestinations(it.ID)
	if len(got) != 1 || got[0] != "123456789012" {
		t.Errorf("expected only the conformant destination ID, got %v", got)
	}
}
