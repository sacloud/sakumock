// Command openapidrift compares each service's committed OpenAPI spec
// (`<service>/openapi/`) with the spec published on the SAKURA Cloud API
// portal, which is the external contract sakumock follows. sources.json maps
// every local spec to its portal URL.
//
// Run it from the repository root:
//
//	go run ./internal/openapidrift            # report only
//	go run ./internal/openapidrift -issues    # also sync GitHub issues
//
// The report (a summary table plus per-spec differences) goes to stdout and,
// in GitHub Actions, to the job summary. The command exits 0 whenever the
// comparison completes, even with differences; a non-zero status means an
// execution error (download, parse, or GitHub API failure).
//
// Documentation-only keywords (description, summary, example(s), title, tags,
// x-*) are ignored, so only changes the mock may have to follow are reported.
//
// With -issues, all drift is tracked in a single issue labeled openapi-drift
// (the specs are followed in one batch), identified by a hidden fingerprint
// marker so scheduled runs never pile up duplicates: the open issue is
// rewritten when the differences change and closed when they disappear, and an
// issue closed by hand is not reopened until the differences change. SDK
// incompatibilities found while following the portal are filed separately,
// one issue each; reporting them to sacloud-sdk-go stays manual.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type manifest struct {
	Sources []source  `json:"sources"`
	Specs   []apiSpec `json:"specs"`
}

type source struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// apiSpec pairs a local spec with its portal source. SkipPaths and
// SkipComponentSetChanges are for portal specs shared by several services
// (KMS, CloudHSM and SecretManager): only the components both sides define
// are compared.
type apiSpec struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Local                   string `json:"local"`
	Source                  string `json:"source"`
	SkipPaths               bool   `json:"skipPaths,omitempty"`
	SkipComponentSetChanges bool   `json:"skipComponentSetChanges,omitempty"`
}

func main() {
	manifestPath := flag.String("manifest", "internal/openapidrift/sources.json", "path to the source manifest")
	syncIssues := flag.Bool("issues", false, "create, update, or close openapi-drift GitHub issues (uses the gh CLI)")
	flag.Parse()
	if err := run(*manifestPath, *syncIssues); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(manifestPath string, syncIssues bool) error {
	cfg, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	upstream := make(map[string]map[string]any, len(cfg.Sources))
	for _, src := range cfg.Sources {
		body, err := download(client, src.URL)
		if err != nil {
			return fmt.Errorf("download %s: %w", src.URL, err)
		}
		doc, err := parseSpec(body)
		if err != nil {
			return fmt.Errorf("parse portal spec %s: %w", src.ID, err)
		}
		upstream[src.ID] = doc
	}

	results := make([]result, 0, len(cfg.Specs))
	for _, spec := range cfg.Specs {
		data, err := os.ReadFile(spec.Local)
		if err != nil {
			return fmt.Errorf("read local spec: %w", err)
		}
		local, err := parseSpec(data)
		if err != nil {
			return fmt.Errorf("parse local spec %s: %w", spec.Local, err)
		}
		results = append(results, compareSpec(spec, cfg.sourceURL(spec.Source), local, upstream[spec.Source]))
	}

	if err := writeReport(results); err != nil {
		return err
	}
	if syncIssues {
		return syncDriftIssues(ghClient{}, results)
	}
	return nil
}

func loadManifest(path string) (*manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var cfg manifest
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	sources := make(map[string]bool, len(cfg.Sources))
	for _, src := range cfg.Sources {
		if src.ID == "" || src.URL == "" {
			return nil, fmt.Errorf("source entry must have an id and url")
		}
		if sources[src.ID] {
			return nil, fmt.Errorf("duplicate source id %q", src.ID)
		}
		sources[src.ID] = true
	}
	ids := make(map[string]bool, len(cfg.Specs))
	for _, spec := range cfg.Specs {
		if spec.ID == "" || spec.Name == "" || spec.Local == "" {
			return nil, fmt.Errorf("spec entry must have an id, name and local path")
		}
		if ids[spec.ID] {
			return nil, fmt.Errorf("duplicate spec id %q", spec.ID)
		}
		ids[spec.ID] = true
		if !sources[spec.Source] {
			return nil, fmt.Errorf("spec %q references unknown source %q", spec.ID, spec.Source)
		}
	}
	return &cfg, nil
}

func (m *manifest) sourceURL(id string) string {
	for _, src := range m.Sources {
		if src.ID == id {
			return src.URL
		}
	}
	return ""
}

func download(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sakumock-openapi-drift-check")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status %s", res.Status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(body)) == "" {
		return nil, fmt.Errorf("empty response")
	}
	return body, nil
}

// parseSpec decodes a YAML or JSON (a YAML subset) OpenAPI document.
func parseSpec(body []byte) (map[string]any, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if len(doc) == 0 {
		return nil, fmt.Errorf("empty document")
	}
	if nestedString(doc, "openapi") == "" && nestedString(doc, "swagger") == "" {
		return nil, fmt.Errorf("document has no OpenAPI or Swagger version")
	}
	if nestedString(doc, "info", "version") == "" {
		return nil, fmt.Errorf("document has no info.version")
	}
	return doc, nil
}

func nestedMap(root map[string]any, keys ...string) map[string]any {
	var current any = root
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[key]
	}
	result, _ := current.(map[string]any)
	return result
}

func nestedString(root map[string]any, keys ...string) string {
	var current any = root
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[key]
	}
	result, _ := current.(string)
	return result
}
