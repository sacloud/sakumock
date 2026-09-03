package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/sacloud/sakumock"
	"github.com/sacloud/sakumock/core"
)

// docTopic is one document embedded in the binary that `sakumock docs` can
// print. Service topics are named after their subcommand, so `sakumock docs kms`
// documents `sakumock kms`.
type docTopic struct {
	Name string
	Body string
	// Lang is the fenced-code language for a non-Markdown body ("" for
	// Markdown). Such a topic has no headings, so --toc/--section are refused
	// and --all wraps it in a fence.
	Lang string
	// Summary is the one-line description shown in the index; when empty it is
	// derived from the body's first paragraph.
	Summary string
}

// alias is the topic name with hyphens removed (apprundedicated for
// apprun-dedicated), matching the Go package directory.
func (t docTopic) alias() string { return strings.ReplaceAll(t.Name, "-", "") }

func (t docTopic) matches(name string) bool {
	return strings.EqualFold(name, t.Name) || strings.EqualFold(name, t.alias())
}

func (t docTopic) summary() string {
	if t.Summary != "" {
		return t.Summary
	}
	return firstSentence(core.Summary(t.Body))
}

// suiteTopics are the repository-level documents. changelogTopic is kept apart
// so allTopics can place it last: hits from release history then trail the
// current documentation.
var (
	suiteTopics = []docTopic{
		{Name: "readme", Body: sakumock.README, Summary: "Suite overview: install, quick start, configuration, fault injection, TLS, service link, tracing, inspection, Docker, library use"},
		{Name: "terraform", Body: terraformDoc(), Summary: "Known-good Terraform (sacloud/sakura provider) configuration for every service, one section per service with links to the provider documentation, taken from the end-to-end test"},
		{Name: "compose", Body: sakumock.ComposeExample, Lang: "yaml", Summary: "docker compose example running sakumock next to an SDK / Terraform client"},
	}
	changelogTopic = docTopic{Name: "changelog", Body: sakumock.Changelog, Summary: "Release history (searched last, so its hits trail the current documentation)"}
)

// serviceTopics derives one topic per service from serviceConfigs — the same
// registry `all` and `env` iterate — through core.ServiceConfig.Name and Doc,
// so registering a service there is all it takes to document it here.
var serviceTopics = func() []docTopic {
	var sc serviceConfigs
	var topics []docTopic
	for _, cfg := range sc.configs() {
		topics = append(topics, docTopic{Name: cfg.Name(), Body: cfg.Doc()})
	}
	return topics
}()

// providerDocsURL is the Terraform Registry documentation of the
// sacloud/sakura provider. A resource or data source page lives under it at
// resources/<name> or data-sources/<name>, where <name> is the type without the
// `sakura_` prefix — the same layout as the provider repository's docs/ tree.
const providerDocsURL = "https://registry.terraform.io/providers/sacloud/sakura/latest/docs"

// tfDeclaration matches the type of every resource and data-source block in a
// .tf file.
var tfDeclaration = regexp.MustCompile(`(?m)^(resource|data)\s+"(sakura_[a-z0-9_]+)"`)

// providerDocLinks lists a Markdown link to the provider documentation of every
// resource and data-source type declared in tf, in order of first appearance.
// It is derived from the declarations alone, so a new .tf gets its links
// without any registration.
func providerDocLinks(tf string) []string {
	var links []string
	seen := map[string]bool{}
	for _, m := range tfDeclaration.FindAllStringSubmatch(tf, -1) {
		kind, typ := m[1], m[2]
		if seen[kind+typ] {
			continue
		}
		seen[kind+typ] = true
		segment, label := "resources", "resource"
		if kind == "data" {
			segment, label = "data-sources", "data source"
		}
		links = append(links, fmt.Sprintf("- %s `%s`: %s/%s/%s", label, typ, providerDocsURL, segment, strings.TrimPrefix(typ, "sakura_")))
	}
	return links
}

// terraformDoc renders the embedded Terraform test configuration as one
// Markdown document: the provider block first, then one section per service in
// port order (so `--section apigw` returns that service's resources), then any
// remaining file. A file a .tf references (the workflows runbook) is appended to
// its service's section, and each section ends with links to the provider
// documentation of the types it declares.
func terraformDoc() string {
	const dir = "test/terraform"
	files := map[string]string{}
	entries, err := fs.ReadDir(sakumock.TerraformExamples, dir)
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		data, err := fs.ReadFile(sakumock.TerraformExamples, path.Join(dir, e.Name()))
		if err != nil {
			panic(err)
		}
		files[e.Name()] = string(data)
	}
	fence := func(b *strings.Builder, name, body string) {
		lang := "hcl"
		if strings.HasSuffix(name, ".yaml") {
			lang = "yaml"
		}
		fmt.Fprintf(b, "`%s`:\n\n```%s\n%s```\n\n", name, lang, strings.TrimRight(body, "\n")+"\n")
	}
	docLinks := func(b *strings.Builder, tf string) {
		if links := providerDocLinks(tf); len(links) > 0 {
			fmt.Fprintf(b, "Provider documentation:\n\n%s\n\n", strings.Join(links, "\n"))
		}
	}
	var b strings.Builder
	b.WriteString("# Terraform examples\n\n")
	b.WriteString("These are the configurations sakumock's own end-to-end test applies with the [sacloud/sakura](https://registry.terraform.io/providers/sacloud/sakura/latest) provider against `sakumock all` (apply, plan with no diff, destroy), so every resource below is known to work on the mock. Point the provider at the mock with `eval \"$(sakumock env --export)\"` in the shell that runs Terraform; the provider reads the endpoints and dummy credentials from the environment. Each section links the provider's documentation for the types it declares; the full list of resources and data sources is at " + providerDocsURL + ".\n\n")
	b.WriteString("## Provider\n\n")
	fence(&b, "main.tf", files["main.tf"])
	delete(files, "main.tf")
	var uncovered []string
	for _, t := range serviceTopics {
		name := t.alias() + ".tf"
		body, ok := files[name]
		if !ok {
			uncovered = append(uncovered, t.Name)
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", t.Name)
		fence(&b, name, body)
		delete(files, name)
		// Files a service's .tf references, e.g. workflows-runbook.yaml.
		for _, extra := range sortedKeys(files) {
			if strings.HasPrefix(extra, t.alias()+"-") {
				fence(&b, extra, files[extra])
				delete(files, extra)
			}
		}
		docLinks(&b, body)
	}
	for _, name := range sortedKeys(files) {
		fmt.Fprintf(&b, "## %s\n\n", name)
		fence(&b, name, files[name])
		docLinks(&b, files[name])
	}
	if len(uncovered) > 0 {
		fmt.Fprintf(&b, "## Not covered\n\nNo Terraform example exists for: %s.\n", strings.Join(uncovered, ", "))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func allTopics() []docTopic {
	topics := append(append([]docTopic{}, suiteTopics...), serviceTopics...)
	return append(topics, changelogTopic)
}

// cliVars supplies the kong.Vars interpolated into help text: the version and
// the topic list, so `sakumock docs --help` names every topic without a
// hand-maintained list.
func cliVars() kong.Vars {
	return kong.Vars{
		"version": sakumock.Version,
		"topics":  strings.Join(topicNames(), ", "),
	}
}

func topicNames() []string {
	var names []string
	for _, t := range allTopics() {
		names = append(names, t.Name)
	}
	return names
}

func findTopic(name string) (docTopic, error) {
	for _, t := range allTopics() {
		if t.matches(name) {
			return t, nil
		}
	}
	return docTopic{}, fmt.Errorf("unknown topic %q; available topics: %s", name, strings.Join(topicNames(), ", "))
}

// firstSentence cuts s at the first sentence boundary (". " followed by an
// upper-case letter) so a multi-sentence README opening becomes an index line.
func firstSentence(s string) string {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == '.' && s[i+1] == ' ' && s[i+2] >= 'A' && s[i+2] <= 'Z' {
			return s[:i+1]
		}
	}
	return s
}

// DocsCmd prints the documentation embedded in the binary — the repository
// README, CHANGELOG, compose example, and every service README — so a user or
// an LLM agent can read it from the CLI without the source checkout. With no
// topic it prints an index in the llms.txt convention.
type DocsCmd struct {
	Topic   string `arg:"" optional:"" help:"Topic to print (${topics}); omit for the index"`
	TOC     bool   `name:"toc" help:"Print only the headings of the topic"`
	Section string `placeholder:"NAME" help:"Print only the section under this heading; matched case-insensitively by text or slug (e.g. 'Fixed keys' or fixed-keys)"`
	All     bool   `help:"Print every topic concatenated (an llms-full.txt style dump)"`
	Search  string `placeholder:"QUERY" help:"Search for a string (case-insensitive) in every topic, or only in the given topic; prints 'topic:line:section: text' per hit, where section is the slug to pass to --section"`

	out io.Writer
}

func (c *DocsCmd) writer() io.Writer {
	if c.out != nil {
		return c.out
	}
	return os.Stdout
}

func (c *DocsCmd) Run() error {
	w := c.writer()
	switch {
	case c.Search != "" && c.Topic == "":
		return writeSearch(w, allTopics(), c.Search)
	case c.All:
		return writeAll(w)
	case c.Topic == "":
		if c.TOC || c.Section != "" {
			return fmt.Errorf("--toc and --section need a topic")
		}
		return writeIndex(w)
	}
	t, err := findTopic(c.Topic)
	if err != nil {
		return err
	}
	if (c.TOC || c.Section != "") && t.Lang != "" {
		return fmt.Errorf("topic %q is %s, not Markdown; it has no sections", t.Name, t.Lang)
	}
	switch {
	case c.Search != "":
		return writeSearch(w, []docTopic{t}, c.Search)
	case c.TOC:
		for _, h := range core.Headings(t.Body) {
			if _, err := fmt.Fprintf(w, "%s %s\n", strings.Repeat("#", h.Level), h.Text); err != nil {
				return err
			}
		}
		return nil
	case c.Section != "":
		sec, ok := core.Section(t.Body, c.Section)
		if !ok {
			var names []string
			for _, h := range core.Headings(t.Body) {
				names = append(names, core.Slug(h.Text))
			}
			return fmt.Errorf("no section %q in topic %q; sections: %s", c.Section, t.Name, strings.Join(names, ", "))
		}
		_, err := io.WriteString(w, sec)
		return err
	}
	_, err = io.WriteString(w, t.Body)
	return err
}

func writeIndex(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# sakumock\n\n")
	fmt.Fprintf(&b, "> Local mock server suite for SAKURA Cloud APIs (version %s). Start every mock with `sakumock all`, then point the SDK or Terraform at it with `eval \"$(sakumock env --export)\"`.\n\n", sakumock.Version)
	b.WriteString("Read a topic with `sakumock docs <topic>`. Narrow it with `--toc` (headings only) or `--section NAME`, search every topic with `sakumock docs --search QUERY` (or one topic with `sakumock docs <topic> --search QUERY`; each hit names the section slug to read next), or dump everything with `sakumock docs --all`. `sakumock <service> --docs` prints the same document as `sakumock docs <service>`, `sakumock <service> --routes` lists the HTTP endpoints it serves, and `sakumock all --help` lists every per-service flag with its environment variable.\n\n")
	b.WriteString("## Suite\n\n")
	for _, t := range append(append([]docTopic{}, suiteTopics...), changelogTopic) {
		fmt.Fprintf(&b, "- [%s](sakumock docs %s): %s\n", t.Name, t.Name, t.summary())
	}
	b.WriteString("\n## Services\n\n")
	for _, t := range serviceTopics {
		fmt.Fprintf(&b, "- [%s](sakumock docs %s): %s\n", t.Name, t.Name, t.summary())
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeAll(w io.Writer) error {
	for i, t := range allTopics() {
		if i > 0 {
			if _, err := io.WriteString(w, "\n\n"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "<!-- sakumock docs %s -->\n\n", t.Name); err != nil {
			return err
		}
		body := t.Body
		if t.Lang != "" {
			body = fmt.Sprintf("# %s\n\n```%s\n%s```\n", t.Name, t.Lang, body)
		}
		if _, err := io.WriteString(w, strings.TrimRight(body, "\n")+"\n"); err != nil {
			return err
		}
	}
	return nil
}

// writeSearch prints every line of the given topics containing query,
// grep-style, with the slug of the enclosing section so the reader knows what
// to pass to --section next. It fails when nothing matches so a caller can tell
// from the exit status.
func writeSearch(w io.Writer, topics []docTopic, query string) error {
	found := false
	for _, t := range topics {
		var headings []core.Heading
		if t.Lang == "" {
			headings = core.Headings(t.Body)
		}
		for _, m := range core.SearchDoc(t.Body, query) {
			found = true
			section := "-"
			for _, h := range headings {
				if h.Line > m.Line {
					break
				}
				section = core.Slug(h.Text)
			}
			if _, err := fmt.Fprintf(w, "%s:%d:%s: %s\n", t.Name, m.Line, section, m.Text); err != nil {
				return err
			}
		}
	}
	if !found {
		return fmt.Errorf("no matches for %q", query)
	}
	return nil
}
