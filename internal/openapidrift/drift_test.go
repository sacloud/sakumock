package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) map[string]any {
	t.Helper()
	doc, err := parseSpec([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

const localSpec = `{"openapi":"3.0.3","info":{"version":"1.0.0"},
"paths":{"/a":{"get":{"operationId":"getA"}}},
"components":{"schemas":{"A":{"type":"string"},"Gone":{"type":"object"}}}}`

const portalSpec = `
openapi: 3.0.3
info: {version: 1.1.0}
paths:
  /a: {get: {operationId: getA}}
  /b: {get: {operationId: getB}}
components:
  schemas:
    A: {type: integer}
    Other: {type: object}
`

func TestCompareSpec(t *testing.T) {
	r := compareSpec(apiSpec{ID: "x", Name: "X"}, "u", mustParse(t, localSpec), mustParse(t, portalSpec))
	want := []string{
		`components.schemas.A.type: value differs (local "string"; portal "integer")`,
		"components.schemas.Gone: present locally, missing from portal",
		"components.schemas.Other: present in portal, missing locally",
		"paths./b: present in portal, missing locally",
		`info.version: local "1.0.0"; portal "1.1.0"`,
	}
	if !slices.Equal(r.Differences, want) {
		t.Errorf("differences:\n got %q\nwant %q", r.Differences, want)
	}
	if r.StructuralCount() != 4 {
		t.Errorf("StructuralCount = %d, want 4", r.StructuralCount())
	}
}

func TestCompareSpecSharedSource(t *testing.T) {
	spec := apiSpec{ID: "x", Name: "X", SkipPaths: true, SkipComponentSetChanges: true}
	r := compareSpec(spec, "u", mustParse(t, localSpec), mustParse(t, portalSpec))
	want := []string{
		`components.schemas.A.type: value differs (local "string"; portal "integer")`,
		`info.version: local "1.0.0"; portal "1.1.0"`,
	}
	if !slices.Equal(r.Differences, want) {
		t.Errorf("differences:\n got %q\nwant %q", r.Differences, want)
	}
}

func TestFingerprint(t *testing.T) {
	a := compareSpec(apiSpec{ID: "x"}, "u", mustParse(t, localSpec), mustParse(t, portalSpec))
	b := compareSpec(apiSpec{ID: "x"}, "u", mustParse(t, localSpec), mustParse(t, portalSpec))
	if fingerprint([]result{a}) == "" || fingerprint([]result{a}) != fingerprint([]result{b}) {
		t.Error("fingerprint must be non-empty and stable for identical drift")
	}
	same := compareSpec(apiSpec{ID: "x"}, "u", mustParse(t, localSpec), mustParse(t, localSpec))
	if same.Drifted() || fingerprint([]result{same}) != "" {
		t.Errorf("identical specs drifted: %q", same.Differences)
	}
}

func TestCompareSpecIgnoresDocumentation(t *testing.T) {
	local := mustParse(t, `{"openapi":"3.0.3","info":{"version":"1"},"paths":{"/a":{"get":{"summary":"old","x-ogen":1}}},
"components":{"schemas":{"A":{"description":"old","example":1,"properties":{"description":{"type":"string"}}}}}}`)
	portal := mustParse(t, `{"openapi":"3.0.3","info":{"version":"1"},"paths":{"/a":{"get":{"summary":"new","tags":["t"]}}},
"components":{"schemas":{"A":{"description":"new","properties":{"description":{"type":"integer"},"title":{"type":"string"}}}}}}`)
	r := compareSpec(apiSpec{ID: "x"}, "u", local, portal)
	want := []string{
		`components.schemas.A.properties.description.type: value differs (local "string"; portal "integer")`,
		"components.schemas.A.properties.title: present in portal, missing locally",
	}
	if !slices.Equal(r.Differences, want) {
		t.Errorf("differences:\n got %q\nwant %q", r.Differences, want)
	}
}

// TestManifest checks the committed manifest without network access: every
// local spec exists and parses.
func TestManifest(t *testing.T) {
	cfg, err := loadManifest("sources.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range cfg.Specs {
		data, err := os.ReadFile(filepath.Join("..", "..", spec.Local))
		if err != nil {
			t.Errorf("%s: %v", spec.ID, err)
			continue
		}
		if _, err := parseSpec(data); err != nil {
			t.Errorf("%s: %v", spec.ID, err)
		}
	}
}

type fakeClient struct {
	issues []issue
	log    []string
}

func (f *fakeClient) List() ([]issue, error) { return f.issues, nil }
func (f *fakeClient) EnsureLabel() error {
	f.log = append(f.log, "label")
	return nil
}
func (f *fakeClient) Create(title, body string) error {
	f.log = append(f.log, "create "+title)
	f.issues = append(f.issues, issue{Number: 100 + len(f.issues), State: "OPEN", Body: body})
	return nil
}
func (f *fakeClient) Update(number int, body, comment string) error {
	f.log = append(f.log, fmt.Sprintf("update #%d", number))
	f.find(number).Body = body
	return nil
}
func (f *fakeClient) Close(number int, body, comment string) error {
	f.log = append(f.log, fmt.Sprintf("close #%d", number))
	is := f.find(number)
	is.Body, is.State = body, "CLOSED"
	return nil
}
func (f *fakeClient) find(number int) *issue {
	for i := range f.issues {
		if f.issues[i].Number == number {
			return &f.issues[i]
		}
	}
	panic("no issue")
}

func driftResult(id string, diffs ...string) result {
	return result{Spec: apiSpec{ID: id, Name: strings.ToUpper(id)}, Differences: diffs}
}

func TestSyncDriftIssues(t *testing.T) {
	f := &fakeClient{}
	sync := func(results ...result) []string {
		t.Helper()
		f.log = nil
		if err := syncDriftIssues(f, results); err != nil {
			t.Fatal(err)
		}
		return f.log
	}
	check := func(got []string, want ...string) {
		t.Helper()
		if !slices.Equal(got, want) {
			t.Errorf("actions = %q, want %q", got, want)
		}
	}
	create := "create " + issueTitle

	check(sync(driftResult("a")))
	// Several drifting specs share one issue.
	check(sync(driftResult("a", "d1"), driftResult("b", "d2")), "label", create)
	// Repeated runs with the same drift do nothing.
	check(sync(driftResult("a", "d1"), driftResult("b", "d2")))
	check(sync(driftResult("a", "d1"), driftResult("b", "d3")), "update #100")
	check(sync(driftResult("a"), driftResult("b")), "close #100")
	check(sync(driftResult("a")))
	// A recurring drift after resolution opens a new issue.
	check(sync(driftResult("a", "d1"), driftResult("b", "d3")), "label", create)

	// Closed by hand: silenced until the differences change.
	f.find(101).State = "CLOSED"
	check(sync(driftResult("a", "d1"), driftResult("b", "d3")))
	check(sync(driftResult("a", "d4")), "label", create)
}

func TestIssueBodyFits(t *testing.T) {
	long := strings.Repeat("x", 300)
	var results []result
	for i := range 20 {
		var diffs []string
		for j := range 200 {
			diffs = append(diffs, fmt.Sprintf("%s %d", long, j))
		}
		results = append(results, driftResult(fmt.Sprintf("s%d", i), diffs...))
	}
	body := issueBody(results, "fp")
	if len(body) > maxIssueBody {
		t.Errorf("body length %d exceeds %d", len(body), maxIssueBody)
	}
	if !strings.Contains(body, marker("fp")) {
		t.Error("body lacks the marker")
	}
}
