package core

import (
	"fmt"
	"net/http"
	"strconv"
)

// Page holds the Count/From query parameters of a paginated list endpoint:
// From is the 0-based index of the first item, Count the page size (0 or
// absent returns every item from From on).
type Page struct {
	From  int
	Count int
}

// ParsePage reads the Count and From query parameters of r. A value that is
// not a non-negative integer is an error, to be reported as 400.
func ParsePage(r *http.Request) (Page, error) {
	var p Page
	q := r.URL.Query()
	for _, f := range []struct {
		name string
		dst  *int
	}{{"From", &p.From}, {"Count", &p.Count}} {
		v := q.Get(f.name)
		if v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return Page{}, fmt.Errorf("invalid %s: %q", f.name, v)
		}
		*f.dst = n
	}
	return p, nil
}

// Paginate returns the slice of items selected by p (never nil).
func Paginate[T any](items []T, p Page) []T {
	if p.From >= len(items) {
		return []T{}
	}
	end := len(items)
	if p.Count > 0 {
		end = min(p.From+p.Count, end)
	}
	return items[p.From:end]
}
