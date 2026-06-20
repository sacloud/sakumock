package apprun

import "testing"

func TestExtractAppID(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"abc-123.localhost:28088", "abc-123"},
		{"abc-123.localhost", "abc-123"},
		{"localhost:28088", ""},
		{"example.com:28088", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := extractAppID(tt.host); got != tt.want {
			t.Errorf("extractAppID(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}

func TestParseDockerPort(t *testing.T) {
	tests := []struct {
		output string
		want   string
	}{
		{"0.0.0.0:32768", "32768"},
		{"0.0.0.0:32768\n:::32768", "32768"},
		{"0.0.0.0:12345\n:::12345\n", "12345"},
		{"", ""},
		{"invalid", ""},
	}
	for _, tt := range tests {
		if got := parseDockerPort(tt.output); got != tt.want {
			t.Errorf("parseDockerPort(%q) = %q, want %q", tt.output, got, tt.want)
		}
	}
}
