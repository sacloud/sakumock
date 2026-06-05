package simplemq

import (
	"os"
	"testing"

	"github.com/alecthomas/kong"
)

func TestStrictAPIKeyMutuallyExclusive(t *testing.T) {
	// Ensure ambient env vars don't perturb the flag-set detection.
	for _, e := range []string{"SIMPLEMQ_API_KEY", "SIMPLEMQ_STRICT"} {
		if v, ok := os.LookupEnv(e); ok {
			os.Unsetenv(e)
			t.Cleanup(func() { os.Setenv(e, v) })
		}
	}

	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"neither", nil, false},
		{"only api-key", []string{"--api-key", "k"}, false},
		{"only strict", []string{"--strict"}, false},
		{"both", []string{"--api-key", "k", "--strict"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cmd Command
			parser, err := kong.New(&cmd)
			if err != nil {
				t.Fatalf("kong.New: %v", err)
			}
			_, err = parser.Parse(tc.args)
			if tc.wantErr != (err != nil) {
				t.Fatalf("args=%v: wantErr=%v, got err=%v", tc.args, tc.wantErr, err)
			}
		})
	}
}
