package coretest

import (
	"slices"
	"testing"

	"github.com/sacloud/sakumock/core"
)

func TestRouteCoverageProblems(t *testing.T) {
	schemas := map[string]map[int]*core.BodySchema{
		"GET /a": nil,
		"GET /b": nil,
		"GET /c": nil,
	}
	tests := []struct {
		name     string
		answered []string
		untested []string
		want     []string
	}{
		{
			name:     "every route answered or listed",
			answered: []string{"GET /a", "GET /b"},
			untested: []string{"GET /c"},
		},
		{
			name:     "a route nobody calls",
			answered: []string{"GET /a"},
			untested: []string{"GET /c"},
			want:     []string{`no test gets a successful response from "GET /b"; add one (SDK client against NewTestServer)`},
		},
		{
			name:     "a listed route that is answered",
			answered: []string{"GET /a", "GET /b", "GET /c"},
			untested: []string{"GET /c"},
			want:     []string{`untested route "GET /c" is now answered by a test; remove it`},
		},
		{
			name:     "a listed route the spec does not have",
			answered: []string{"GET /a", "GET /b", "GET /c"},
			untested: []string{"GET /gone"},
			want:     []string{`untested route "GET /gone" is not in the spec's response schemas; remove it`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answered := map[string]bool{}
			for _, key := range tt.answered {
				answered[key] = true
			}
			if got := routeCoverageProblems(schemas, answered, tt.untested); !slices.Equal(got, tt.want) {
				t.Errorf("problems = %q, want %q", got, tt.want)
			}
		})
	}
}
