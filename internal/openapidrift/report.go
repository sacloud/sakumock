package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// maxIssueBody keeps the issue body under GitHub's 65536-character limit.
const maxIssueBody = 60000

func renderReport(results []result) string {
	var b strings.Builder
	b.WriteString("# OpenAPI drift from the portal\n\n")
	writeSummary(&b, results)
	writeDetails(&b, results, 30, false)
	return b.String()
}

func writeSummary(b *strings.Builder, results []result) {
	versionDiffs, structuralDiffs := 0, 0
	for _, r := range results {
		if r.LocalVersion != r.PortalVersion {
			versionDiffs++
		}
		if r.StructuralCount() > 0 {
			structuralDiffs++
		}
	}
	fmt.Fprintf(b, "Compared specs: %d; version differences: %d; specs with structural differences: %d. "+
		"Documentation-only keywords (description, summary, example(s), title, tags, x-*) are ignored.\n\n",
		len(results), versionDiffs, structuralDiffs)
	b.WriteString("| Spec | Local | sakumock version | Portal version | Status | Structural differences |\n")
	b.WriteString("|---|---|---:|---:|---|---:|\n")
	for _, r := range results {
		status := ":white_check_mark:"
		if r.Drifted() {
			status = ":x:"
		}
		fmt.Fprintf(b, "| %s | `%s` | %s | %s | %s | %d |\n",
			r.Spec.Name, r.Spec.Local, r.LocalVersion, r.PortalVersion, status, r.StructuralCount())
	}
}

// writeDetails lists up to limit differences per drifting spec, folded into
// <details> blocks when collapsed is set.
func writeDetails(b *strings.Builder, results []result, limit int, collapsed bool) {
	for _, r := range results {
		if !r.Drifted() {
			continue
		}
		if collapsed {
			fmt.Fprintf(b, "\n<details><summary>%s (%d)</summary>\n\n", r.Spec.Name, len(r.Differences))
		} else {
			fmt.Fprintf(b, "\n## %s\n\n", r.Spec.Name)
		}
		for i, d := range r.Differences {
			if i == limit {
				fmt.Fprintf(b, "- ... (showing %d of %d differences)\n", limit, len(r.Differences))
				break
			}
			fmt.Fprintf(b, "- %s\n", d)
		}
		fmt.Fprintf(b, "- Local: `%s`\n- Portal: %s\n", r.Spec.Local, r.PortalURL)
		if collapsed {
			b.WriteString("\n</details>\n")
		}
	}
}

// writeReport prints the report and appends it to the GitHub Actions job
// summary when running in Actions.
func writeReport(results []result) error {
	text := renderReport(results)
	fmt.Print(text)
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open job summary: %w", err)
	}
	if _, err := io.WriteString(f, text); err != nil {
		f.Close()
		return fmt.Errorf("write job summary: %w", err)
	}
	return f.Close()
}

// issueBody renders the single drift issue, shrinking the per-spec detail
// lists until the body fits; the full report is in the workflow run summary.
func issueBody(results []result, fp string) string {
	for _, limit := range []int{100, 50, 20, 0} {
		var b strings.Builder
		fmt.Fprintf(&b, "%s\nThe portal OpenAPI specs differ from the specs sakumock implements. "+
			"sakumock follows the portal; update the specs below and the handlers together.\n\n", marker(fp))
		writeSummary(&b, results)
		if limit > 0 {
			writeDetails(&b, results, limit, true)
		}
		b.WriteString("\nIf following the portal breaks the SDK-based tests, file each incompatibility as its own issue; " +
			"reporting it to sacloud-sdk-go is done by hand after review.\n\n")
		b.WriteString("_Maintained by the `OpenAPI drift` workflow: this issue is updated while the differences change " +
			"and closed when they disappear. Closing it by hand silences it until the differences change._\n")
		if b.Len() <= maxIssueBody || limit == 0 {
			return b.String()
		}
	}
	panic("unreachable")
}
