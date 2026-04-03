package simplemq

import "testing"

func TestMaskAuthorization(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"Bearer abc", "Bearer ****"},
		{"Bearer abcd", "Bearer ****"},
		{"Bearer abcde", "Bearer abcd****"},
		{"Bearer my-secret-api-key-12345", "Bearer my-s****"},
		{"InvalidHeader", "Bearer Inva****"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := maskAuthorization(tt.input)
			if got != tt.want {
				t.Errorf("maskAuthorization(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
