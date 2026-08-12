package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestGenerateGolden(t *testing.T) {
	cases := []struct {
		name      string
		specs     []string
		mapping   *Mapping
		responses bool
	}{
		{"spec30", []string{"testdata/spec30.json"}, nil, false},
		{"spec31", []string{"testdata/spec31.yaml"}, nil, false},
		{"mapped", []string{"testdata/spec30.json"}, &Mapping{
			Prefix:       "/{site}",
			PathRewrites: map[string]string{"{id}": "{widget_id}"},
			Routes:       map[string]string{"POST /widgets/": "POST /v2/widgets/"},
			SkipPaths:    []string{"/nodes/"},
		}, false},
		{"respspec", []string{"testdata/respspec.json"}, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var warn bytes.Buffer
			got, err := Generate(c.specs, "fixture", "bodySchemas", c.mapping, c.responses, &warn)
			if err != nil {
				t.Fatal(err)
			}
			again, err := Generate(c.specs, "fixture", "bodySchemas", c.mapping, c.responses, &warn)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, again) {
				t.Fatal("Generate is not deterministic")
			}

			golden := filepath.Join("testdata", c.name+".go.golden")
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("generated source differs from %s; run `go test ./internal/genvalidate/ -update` and review the diff.\n--- got ---\n%s", golden, got)
			}
		})
	}
}

func TestGenerateDuplicateRoute(t *testing.T) {
	_, err := Generate([]string{"testdata/spec30.json", "testdata/spec30.json"}, "fixture", "bodySchemas", nil, false, os.Stderr)
	if err == nil {
		t.Fatal("expected duplicate-route error")
	}
}

func TestRouteKey(t *testing.T) {
	m := &Mapping{
		Prefix:       "/{site}",
		PathRewrites: map[string]string{"{resource_id}": "{vault_resource_id}"},
		Routes:       map[string]string{"POST /odd": "PUT /elsewhere"},
		SkipPaths:    []string{"/skipped"},
	}
	cases := []struct {
		method, path string
		want         string
		skip         bool
	}{
		{"POST", "/vaults/{resource_id}", "POST /{site}/vaults/{vault_resource_id}", false},
		{"POST", "/odd", "PUT /elsewhere", false},
		{"POST", "/skipped", "", true},
	}
	for _, c := range cases {
		got, skip := routeKey(c.method, c.path, m)
		if got != c.want || skip != c.skip {
			t.Errorf("routeKey(%s, %s) = (%q, %v), want (%q, %v)", c.method, c.path, got, skip, c.want, c.skip)
		}
	}
	if got, skip := routeKey("POST", "/plain", nil); got != "POST /plain" || skip {
		t.Errorf("routeKey without mapping = (%q, %v)", got, skip)
	}
}
