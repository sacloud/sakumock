package coretest

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/sacloud/sakumock/core"
)

// RouteCoverageMain is a service's TestMain: it runs the tests, then fails
// the run when a route of schemas (the generated responseSchemas) never
// answered a request successfully. The response validator only checks what a
// test makes a handler write, so a route no test calls can violate the spec
// unnoticed.
//
// untested lists the routes known to lack a test ("METHOD /path"); an entry
// that is answered after all, or that names no route, fails the run too, so
// the list only shrinks. The check is skipped when the run is partial
// (-run, -skip, -list, -short) or already failing.
func RouteCoverageMain(m *testing.M, schemas map[string]map[int]*core.BodySchema, untested []string) {
	code := m.Run()
	if code == 0 && fullRun() {
		if problems := routeCoverageProblems(schemas, core.AnsweredRoutes(schemas), untested); len(problems) != 0 {
			for _, p := range problems {
				fmt.Fprintln(os.Stderr, "route coverage: "+p)
			}
			fmt.Fprintln(os.Stderr, "FAIL")
			code = 1
		}
	}
	os.Exit(code)
}

func fullRun() bool {
	for _, name := range []string{"test.run", "test.skip", "test.list"} {
		if f := flag.Lookup(name); f != nil && f.Value.String() != "" {
			return false
		}
	}
	return !testing.Short()
}

func routeCoverageProblems(schemas map[string]map[int]*core.BodySchema, answered map[string]bool, untested []string) []string {
	var problems []string
	for _, key := range untested {
		switch _, ok := schemas[key]; {
		case !ok:
			problems = append(problems, fmt.Sprintf("untested route %q is not in the spec's response schemas; remove it", key))
		case answered[key]:
			problems = append(problems, fmt.Sprintf("untested route %q is now answered by a test; remove it", key))
		}
	}
	for key := range schemas {
		if !answered[key] && !slices.Contains(untested, key) {
			problems = append(problems, fmt.Sprintf("no test gets a successful response from %q; add one (SDK client against NewTestServer)", key))
		}
	}
	slices.Sort(problems)
	return problems
}
