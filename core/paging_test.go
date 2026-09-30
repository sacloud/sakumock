package core

import (
	"net/http/httptest"
	"slices"
	"testing"
)

func TestParsePage(t *testing.T) {
	tests := []struct {
		query   string
		want    Page
		wantErr bool
	}{
		{"", Page{}, false},
		{"From=2&Count=3", Page{From: 2, Count: 3}, false},
		{"Count=0", Page{}, false},
		{"From=-1", Page{}, true},
		{"Count=x", Page{}, true},
	}
	for _, tt := range tests {
		r := httptest.NewRequest("GET", "/?"+tt.query, nil)
		got, err := ParsePage(r)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q: err = %v, wantErr %v", tt.query, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("%q: got %+v, want %+v", tt.query, got, tt.want)
		}
	}
}

func TestPaginate(t *testing.T) {
	items := []int{0, 1, 2, 3, 4}
	tests := []struct {
		page Page
		want []int
	}{
		{Page{}, []int{0, 1, 2, 3, 4}},
		{Page{From: 1, Count: 2}, []int{1, 2}},
		{Page{From: 3}, []int{3, 4}},
		{Page{From: 4, Count: 10}, []int{4}},
		{Page{From: 5}, []int{}},
		{Page{From: 9, Count: 1}, []int{}},
	}
	for _, tt := range tests {
		got := Paginate(items, tt.page)
		if got == nil || !slices.Equal(got, tt.want) {
			t.Errorf("%+v: got %v, want %v", tt.page, got, tt.want)
		}
	}
}
