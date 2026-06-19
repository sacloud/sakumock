package iam

import (
	"net/http"
	"time"

	"github.com/sacloud/sakumock/core"
)

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	for _, r := range s.routeTable() {
		mux.HandleFunc(r.Method+" "+r.Path, r.Handler)
	}
	return mux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.latency > 0 {
		time.Sleep(s.latency)
	}
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	s.mux.ServeHTTP(rw, r)
	s.logger.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"status", rw.statusCode,
	)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

type problemDetail struct {
	Type   string `json:"type"`
	Status int    `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

func writeError(w http.ResponseWriter, status int, detail string) {
	core.WriteJSON(w, status, problemDetail{
		Type:   "about:blank",
		Status: status,
		Title:  http.StatusText(status),
		Detail: detail,
	})
}

type paginatedList[T any] struct {
	Items    []T     `json:"items"`
	Count    int     `json:"count"`
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
}

func writePage[T any](w http.ResponseWriter, items []T) {
	if items == nil {
		items = []T{}
	}
	core.WriteJSON(w, http.StatusOK, paginatedList[T]{
		Items:    items,
		Count:    len(items),
		Next:     nil,
		Previous: nil,
	})
}
