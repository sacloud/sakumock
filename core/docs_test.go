package core

import (
	"reflect"
	"testing"
)

const sampleDoc = `# sakumock/kms

A KMS compatible mock server
for local development.

## Install

` + "```bash" + `
# not a heading
go install example
` + "```" + `

## Fixed keys

Use --key.

### Rotation

Rotate with --key ID=SECRET@N.

## Fault Injection

See the root README.
`

func TestHeadings(t *testing.T) {
	got := Headings(sampleDoc)
	want := []Heading{
		{Level: 1, Text: "sakumock/kms", Line: 1},
		{Level: 2, Text: "Install", Line: 6},
		{Level: 2, Text: "Fixed keys", Line: 13},
		{Level: 3, Text: "Rotation", Line: 17},
		{Level: 2, Text: "Fault Injection", Line: 21},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Headings() = %+v, want %+v", got, want)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Fault Injection":                   "fault-injection",
		"Data plane (Docker reverse proxy)": "data-plane-docker-reverse-proxy",
		"  sakumock/kms ":                   "sakumockkms",
		"Use with sacloud-sdk-go":           "use-with-sacloud-sdk-go",
	} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSection(t *testing.T) {
	tests := []struct {
		name string
		want string
		ok   bool
	}{
		{"Fixed keys", "## Fixed keys\n\nUse --key.\n\n### Rotation\n\nRotate with --key ID=SECRET@N.\n", true},
		{"fixed-keys", "## Fixed keys\n\nUse --key.\n\n### Rotation\n\nRotate with --key ID=SECRET@N.\n", true},
		{"FIXED KEYS", "## Fixed keys\n\nUse --key.\n\n### Rotation\n\nRotate with --key ID=SECRET@N.\n", true},
		{"rotation", "### Rotation\n\nRotate with --key ID=SECRET@N.\n", true},
		{"Fault Injection", "## Fault Injection\n\nSee the root README.\n", true},
		{"not a heading", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := Section(sampleDoc, tt.name)
		if ok != tt.ok || got != tt.want {
			t.Errorf("Section(%q) = %q, %v; want %q, %v", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

func TestTitleAndSummary(t *testing.T) {
	if got := Title(sampleDoc); got != "sakumock/kms" {
		t.Errorf("Title() = %q", got)
	}
	if got := Summary(sampleDoc); got != "A KMS compatible mock server for local development." {
		t.Errorf("Summary() = %q", got)
	}
	if got := Summary("no title\n\nsecond"); got != "no title" {
		t.Errorf("Summary(no title) = %q", got)
	}
	if got := Title("plain text"); got != "" {
		t.Errorf("Title(plain) = %q", got)
	}
}

func TestSearchDoc(t *testing.T) {
	got := SearchDoc(sampleDoc, "--KEY")
	want := []Match{
		{Line: 15, Text: "Use --key."},
		{Line: 19, Text: "Rotate with --key ID=SECRET@N."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SearchDoc() = %+v, want %+v", got, want)
	}
	if got := SearchDoc(sampleDoc, ""); got != nil {
		t.Errorf("SearchDoc(empty) = %+v, want nil", got)
	}
}
