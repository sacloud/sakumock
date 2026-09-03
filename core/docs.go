package core

import (
	"strings"
)

// Heading is an ATX heading (`# Title`) found in a Markdown document.
// Headings inside fenced code blocks are not reported.
type Heading struct {
	// Level is the number of leading '#' characters (1-6).
	Level int
	// Text is the heading text with the marker and surrounding space removed.
	Text string
	// Line is the 1-based line number of the heading in the document.
	Line int
}

// Slug converts heading text to the anchor form GitHub uses: lower-case, with
// spaces replaced by '-' and punctuation other than '-' and '_' dropped; a
// Markdown link keeps only its text. It lets a section be addressed as either
// "Fault Injection" or "fault-injection".
func Slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(stripLinks(text))) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r > 0x7f:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stripLinks replaces every Markdown link "[text](url)" in s with "text".
func stripLinks(s string) string {
	for {
		open := strings.Index(s, "[")
		if open < 0 {
			return s
		}
		close := strings.Index(s[open:], "](")
		if close < 0 {
			return s
		}
		end := strings.Index(s[open+close:], ")")
		if end < 0 {
			return s
		}
		s = s[:open] + s[open+1:open+close] + s[open+close+end+1:]
	}
}

// Headings returns every ATX heading in doc, in document order, skipping those
// inside fenced code blocks (a shell comment in a ```sh block is not a heading).
func Headings(doc string) []Heading {
	var hs []Heading
	inFence := false
	fence := ""
	for i, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		if inFence {
			if strings.HasPrefix(trimmed, fence) {
				inFence = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = true
			fence = trimmed[:3]
			continue
		}
		level := 0
		for level < len(line) && line[level] == '#' {
			level++
		}
		if level == 0 || level > 6 || level == len(line) || line[level] != ' ' {
			continue
		}
		hs = append(hs, Heading{Level: level, Text: strings.TrimSpace(line[level+1:]), Line: i + 1})
	}
	return hs
}

// Section returns the part of doc under the heading named name (matched
// case-insensitively against either the heading text or its Slug), from the
// heading line up to the next heading of the same or a higher level. The
// second result is false when no heading matches.
func Section(doc, name string) (string, bool) {
	want := Slug(name)
	if want == "" {
		return "", false
	}
	hs := Headings(doc)
	for i, h := range hs {
		if Slug(h.Text) != want && !strings.EqualFold(h.Text, strings.TrimSpace(name)) {
			continue
		}
		lines := strings.Split(doc, "\n")
		end := len(lines)
		for _, next := range hs[i+1:] {
			if next.Level <= h.Level {
				end = next.Line - 1
				break
			}
		}
		return strings.TrimRight(strings.Join(lines[h.Line-1:end], "\n"), "\n") + "\n", true
	}
	return "", false
}

// Title returns the text of the first level-1 heading in doc, or "" when it
// has none.
func Title(doc string) string {
	for _, h := range Headings(doc) {
		if h.Level == 1 {
			return h.Text
		}
	}
	return ""
}

// Summary returns the first paragraph of doc that follows the title (or the
// first paragraph at all when there is no title), joined onto a single line.
// It is the one-line description a document index shows next to the title.
func Summary(doc string) string {
	lines := strings.Split(doc, "\n")
	start := 0
	for _, h := range Headings(doc) {
		if h.Level == 1 {
			start = h.Line
			break
		}
	}
	var para []string
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, "[![") {
			if len(para) > 0 {
				break
			}
			continue
		}
		para = append(para, trimmed)
	}
	return strings.Join(para, " ")
}

// Match is one line of a document that contains a search query.
type Match struct {
	// Line is the 1-based line number.
	Line int
	// Text is the matching line, with surrounding whitespace trimmed.
	Text string
}

// SearchDoc returns every line of doc containing query, compared
// case-insensitively. An empty query matches nothing.
func SearchDoc(doc, query string) []Match {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var ms []Match
	for i, line := range strings.Split(doc, "\n") {
		if strings.Contains(strings.ToLower(line), q) {
			ms = append(ms, Match{Line: i + 1, Text: strings.TrimSpace(line)})
		}
	}
	return ms
}
