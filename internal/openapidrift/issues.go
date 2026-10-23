package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	issueLabel = "openapi-drift"
	issueTitle = "OpenAPI drift from the portal"
	// resolvedFingerprint replaces the fingerprint of an issue closed because
	// the drift disappeared, so a later identical drift opens a new issue
	// instead of being mistaken for a hand-closed (silenced) one.
	resolvedFingerprint = "resolved"
)

var markerRe = regexp.MustCompile(`<!-- openapi-drift fingerprint=(\S+) -->`)

func marker(fingerprint string) string {
	return fmt.Sprintf("<!-- openapi-drift fingerprint=%s -->", fingerprint)
}

// fingerprint identifies the exact set of differences across all specs, so
// the issue is only rewritten (or a hand-closed one superseded) when the drift
// actually changes. It is empty when nothing drifts.
func fingerprint(results []result) string {
	h := sha256.New()
	drifted := false
	for _, r := range results {
		if !r.Drifted() {
			continue
		}
		drifted = true
		fmt.Fprintf(h, "[%s]\n", r.Spec.ID)
		for _, d := range r.Differences {
			fmt.Fprintln(h, d)
		}
	}
	if !drifted {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

type issue struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Body   string `json:"body"`
}

func (i issue) fingerprint() (string, bool) {
	m := markerRe.FindStringSubmatch(i.Body)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func (i issue) open() bool { return strings.EqualFold(i.State, "open") }

type issueClient interface {
	List() ([]issue, error)
	EnsureLabel() error
	Create(title, body string) error
	Update(number int, body, comment string) error
	Close(number int, body, comment string) error
}

// syncDriftIssues keeps a single open issue covering every drifting spec:
//   - drift, no open issue: create one, unless a closed issue already carries
//     the same fingerprint (closed by hand = acknowledged).
//   - drift, open issue with another fingerprint: rewrite it and comment.
//   - no drift, open issue: mark it resolved and close it.
func syncDriftIssues(c issueClient, results []result) error {
	issues, err := c.List()
	if err != nil {
		return err
	}
	var cur *issue
	closedFPs := map[string]bool{}
	for i, is := range issues {
		fp, ok := is.fingerprint()
		if !ok {
			continue
		}
		if !is.open() {
			closedFPs[fp] = true
			continue
		}
		if cur == nil || is.Number < cur.Number {
			cur = &issues[i]
		}
	}

	fp := fingerprint(results)
	switch {
	case fp == "" && cur == nil:
		return nil
	case fp == "":
		curFP, _ := cur.fingerprint()
		body := strings.Replace(cur.Body, marker(curFP), marker(resolvedFingerprint), 1)
		if err := c.Close(cur.Number, body, "The portal specs and the local specs no longer differ."); err != nil {
			return err
		}
		fmt.Printf("closed #%d: drift resolved\n", cur.Number)
	case cur != nil:
		if curFP, _ := cur.fingerprint(); curFP == fp {
			return nil
		}
		if err := c.Update(cur.Number, issueBody(results, fp), "The differences changed; the description has been updated."); err != nil {
			return err
		}
		fmt.Printf("updated #%d\n", cur.Number)
	case closedFPs[fp]:
		fmt.Println("skipped: an identical drift issue was closed by hand")
	default:
		if err := c.EnsureLabel(); err != nil {
			return err
		}
		if err := c.Create(issueTitle, issueBody(results, fp)); err != nil {
			return err
		}
		fmt.Println("created a drift issue")
	}
	return nil
}

// ghClient drives the gh CLI against the repository of the working directory
// (or GH_REPO); it needs GH_TOKEN with issues: write.
type ghClient struct{}

func (ghClient) List() ([]issue, error) {
	out, err := gh(nil, "issue", "list", "--label", issueLabel, "--state", "all", "--limit", "1000", "--json", "number,state,body")
	if err != nil {
		return nil, err
	}
	var issues []issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parse gh issue list: %w", err)
	}
	return issues, nil
}

func (ghClient) EnsureLabel() error {
	_, err := gh(nil, "label", "create", issueLabel, "--force", "--color", "fbca04",
		"--description", "Portal OpenAPI spec differs from sakumock's")
	return err
}

func (ghClient) Create(title, body string) error {
	_, err := gh(strings.NewReader(body), "issue", "create", "--title", title, "--label", issueLabel, "--body-file", "-")
	return err
}

func (ghClient) Update(number int, body, comment string) error {
	n := strconv.Itoa(number)
	if _, err := gh(strings.NewReader(body), "issue", "edit", n, "--body-file", "-"); err != nil {
		return err
	}
	_, err := gh(nil, "issue", "comment", n, "--body", comment)
	return err
}

func (ghClient) Close(number int, body, comment string) error {
	n := strconv.Itoa(number)
	if _, err := gh(strings.NewReader(body), "issue", "edit", n, "--body-file", "-"); err != nil {
		return err
	}
	_, err := gh(nil, "issue", "close", n, "--reason", "completed", "--comment", comment)
	return err
}

func gh(stdin *strings.Reader, args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args[:2], " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
