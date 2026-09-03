package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/sacloud/sakumock/core"
)

// TestDocsCoverEveryService checks that every service registered in
// serviceConfigs yields a docs topic with a non-empty README, in the same
// order, so a service whose Doc is empty (a missing embed) cannot slip in.
func TestDocsCoverEveryService(t *testing.T) {
	var sc serviceConfigs
	cfgs := sc.configs()
	if len(serviceTopics) != len(cfgs) {
		t.Fatalf("%d service topics for %d services", len(serviceTopics), len(cfgs))
	}
	for i, cfg := range cfgs {
		topic := serviceTopics[i]
		if topic.Name != cfg.Name() {
			t.Errorf("topic %d is %q, want %q", i, topic.Name, cfg.Name())
		}
		if !strings.HasPrefix(topic.Body, "# ") {
			t.Errorf("service %q: Doc() does not start with a Markdown title: %.40q", cfg.Name(), topic.Body)
		}
	}
}

// TestDocsMatchRepositoryFiles checks each embedded body against the file in
// the source tree it is supposed to mirror.
func TestDocsMatchRepositoryFiles(t *testing.T) {
	files := map[string]string{
		"readme":    filepath.Join("..", "..", "README.md"),
		"changelog": filepath.Join("..", "..", "CHANGELOG.md"),
		"compose":   filepath.Join("..", "..", "examples", "compose.yaml"),
	}
	for _, topic := range serviceTopics {
		files[topic.Name] = filepath.Join("..", "..", topic.alias(), "README.md")
	}
	for _, topic := range allTopics() {
		if topic.Name == "terraform" {
			continue // composed from several files; see TestDocsTerraform
		}
		data, err := os.ReadFile(files[topic.Name])
		if err != nil {
			t.Errorf("%s: %v", topic.Name, err)
			continue
		}
		if string(data) != topic.Body {
			t.Errorf("topic %q does not match %s", topic.Name, files[topic.Name])
		}
	}
}

// TestDocsTerraform checks the composed terraform topic carries every file of
// the end-to-end test, with a section per service so --section works.
func TestDocsTerraform(t *testing.T) {
	topic, err := findTopic("terraform")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("..", "..", "test", "terraform")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".tf") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(topic.Body, "`"+name+"`:\n") || !strings.Contains(topic.Body, strings.TrimRight(string(data), "\n")) {
			t.Errorf("terraform topic is missing %s", name)
		}
	}
	if strings.Contains(topic.Body, "write-env-file") {
		t.Error("terraform topic refers to the removed --write-env-file flag")
	}
	for _, svc := range serviceTopics {
		sec, ok := core.Section(topic.Body, svc.Name)
		if !ok {
			if _, err := os.Stat(filepath.Join(dir, svc.alias()+".tf")); os.IsNotExist(err) {
				// A service without an example is listed instead of silently absent.
				if !strings.Contains(topic.Body, "## Not covered\n\nNo Terraform example exists for: ") || !strings.Contains(topic.Body, svc.Name) {
					t.Errorf("terraform topic should list %s as not covered", svc.Name)
				}
				continue
			}
			t.Errorf("terraform topic has no section for %s", svc.Name)
			continue
		}
		if !strings.Contains(sec, "resource \"sakura_") {
			t.Errorf("terraform section %s declares no sakura_ resource:\n%s", svc.Name, sec)
		}
	}
	// The runbook referenced by workflows.tf lives in the workflows section.
	sec, _ := core.Section(topic.Body, "workflows")
	if !strings.Contains(sec, "`workflows-runbook.yaml`:") {
		t.Errorf("workflows section lacks the runbook:\n%s", sec)
	}
	// Every declared type gets a provider-documentation link, derived from the
	// declaration alone.
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".tf") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		for _, m := range tfDeclaration.FindAllStringSubmatch(string(data), -1) {
			segment := "resources"
			if m[1] == "data" {
				segment = "data-sources"
			}
			want := "`" + m[2] + "`: " + providerDocsURL + "/" + segment + "/" + strings.TrimPrefix(m[2], "sakura_")
			if !strings.Contains(topic.Body, want) {
				t.Errorf("terraform topic lacks the doc link for %s %s", m[1], m[2])
			}
		}
	}
	out, err := runDocs(t, "terraform", "--section", "apigw")
	if err != nil || !strings.HasPrefix(out, "## apigw\n") || !strings.Contains(out, "sakura_apigw_service") ||
		!strings.Contains(out, "- data source `sakura_apigw_plan`: "+providerDocsURL+"/data-sources/apigw_plan") ||
		!strings.Contains(out, "- resource `sakura_apigw_route`: "+providerDocsURL+"/resources/apigw_route") {
		t.Errorf("docs terraform --section apigw: err=%v\n%s", err, out)
	}
}

func runDocs(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("sakumock"))
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	kctx, err := parser.Parse(append([]string{"docs"}, args...))
	if err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	var buf bytes.Buffer
	cli.Docs.out = &buf
	err = kctx.Run()
	return buf.String(), err
}

func TestDocsIndex(t *testing.T) {
	out, err := runDocs(t)
	if err != nil {
		t.Fatalf("docs: %v", err)
	}
	if !strings.HasPrefix(out, "# sakumock\n\n> ") {
		t.Errorf("index is not llms.txt shaped:\n%s", out)
	}
	for _, name := range topicNames() {
		if !strings.Contains(out, "- ["+name+"](sakumock docs "+name+"): ") {
			t.Errorf("index missing topic %q:\n%s", name, out)
		}
	}
}

func TestDocsTopic(t *testing.T) {
	kmsDoc, _ := findTopic("kms")
	for _, name := range []string{"kms", "KMS"} {
		out, err := runDocs(t, name)
		if err != nil {
			t.Fatalf("docs %s: %v", name, err)
		}
		if out != kmsDoc.Body {
			t.Errorf("docs %s printed something other than the kms README", name)
		}
	}
	// The hyphen-less alias matches the Go package directory.
	if out, err := runDocs(t, "apprundedicated"); err != nil || !strings.HasPrefix(out, "# sakumock/apprundedicated") {
		t.Errorf("docs apprundedicated: err=%v out=%.40q", err, out)
	}
	_, err := runDocs(t, "nope")
	if err == nil || !strings.Contains(err.Error(), `unknown topic "nope"`) || !strings.Contains(err.Error(), "apprun-dedicated") {
		t.Errorf("unknown topic error = %v; want the topic list", err)
	}
}

func TestDocsTOCAndSection(t *testing.T) {
	out, err := runDocs(t, "kms", "--toc")
	if err != nil {
		t.Fatalf("--toc: %v", err)
	}
	if !strings.Contains(out, "# sakumock/kms\n") || !strings.Contains(out, "### Fixed keys\n") {
		t.Errorf("--toc output:\n%s", out)
	}
	if strings.Contains(out, "\n\n") {
		t.Errorf("--toc should print headings only:\n%s", out)
	}

	for _, name := range []string{"Fixed keys", "fixed-keys"} {
		out, err := runDocs(t, "kms", "--section", name)
		if err != nil {
			t.Fatalf("--section %q: %v", name, err)
		}
		if !strings.HasPrefix(out, "### Fixed keys\n") || !strings.Contains(out, "--key") || strings.Contains(out, "## Use with sacloud-sdk-go") {
			t.Errorf("--section %q output:\n%s", name, out)
		}
	}
	_, err = runDocs(t, "kms", "--section", "nope")
	if err == nil || !strings.Contains(err.Error(), "fixed-keys") {
		t.Errorf("missing section error = %v; want the section list", err)
	}
	if _, err := runDocs(t, "compose", "--toc"); err == nil {
		t.Error("--toc on a yaml topic should fail")
	}
	if _, err := runDocs(t, "--toc"); err == nil {
		t.Error("--toc without a topic should fail")
	}
}

func TestDocsAll(t *testing.T) {
	out, err := runDocs(t, "--all")
	if err != nil {
		t.Fatalf("--all: %v", err)
	}
	for _, topic := range allTopics() {
		if !strings.Contains(out, "<!-- sakumock docs "+topic.Name+" -->\n") {
			t.Errorf("--all missing topic %q", topic.Name)
		}
	}
	if !strings.Contains(out, "# compose\n\n```yaml\n") {
		t.Errorf("--all should fence the compose example:\n%.200s", out)
	}
}

func TestDocsSearch(t *testing.T) {
	out, err := runDocs(t, "--search", "FIXED KEYS")
	if err != nil {
		t.Fatalf("--search: %v", err)
	}
	if !strings.Contains(out, "kms:33:fixed-keys: ### Fixed keys\n") || !strings.Contains(out, "kms:27:options: ") {
		t.Errorf("--search output:\n%s", out)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if !strings.Contains(strings.ToLower(line), "fixed keys") {
			t.Errorf("non-matching line %q", line)
		}
	}
	// With a topic, the search is confined to it.
	out, err = runDocs(t, "eventbus", "--search", "simplemq")
	if err != nil {
		t.Fatalf("eventbus --search: %v", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if !strings.HasPrefix(line, "eventbus:") {
			t.Errorf("hit outside the eventbus topic: %q", line)
		}
	}
	if !strings.Contains(out, ":service-link: ") {
		t.Errorf("eventbus --search simplemq should hit the service-link section:\n%s", out)
	}
	if _, err := runDocs(t, "--search", "no such phrase anywhere zzz"); err == nil {
		t.Error("--search with no hits should fail")
	}
}

func TestProviderDocLinks(t *testing.T) {
	tf := `resource "sakura_kms_key" "a" {}
data "sakura_apigw_plan" "p" {}
resource "sakura_kms_key" "b" {}
  resource "sakura_indented" "no" {}
`
	got := providerDocLinks(tf)
	want := []string{
		"- resource `sakura_kms_key`: " + providerDocsURL + "/resources/kms_key",
		"- data source `sakura_apigw_plan`: " + providerDocsURL + "/data-sources/apigw_plan",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("providerDocLinks() =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got := providerDocLinks("# nothing"); got != nil {
		t.Errorf("providerDocLinks(no declarations) = %v", got)
	}
}

func TestFirstSentence(t *testing.T) {
	for in, want := range map[string]string{
		"A mock. It stores data.":     "A mock.",
		"Uses e.g. this. Then that.":  "Uses e.g. this.",
		"No boundary here":            "No boundary here",
		"Trailing period at the end.": "Trailing period at the end.",
	} {
		if got := firstSentence(in); got != want {
			t.Errorf("firstSentence(%q) = %q, want %q", in, got, want)
		}
	}
}
