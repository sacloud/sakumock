package apigw

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// Request/response transformations follow the semantics of the real
// gateway's transformer plugins: within each transformation the phases apply
// in the order allow → remove → rename → replace → add → append; replace only
// touches existing keys, add only missing ones, and append accumulates.
// Body operations apply to JSON object bodies only (dotted keys address
// nested fields, per the spec's JSONKey pattern); non-JSON and
// content-encoded bodies are passed through untouched.

// applyRequestTransformation rewrites the outbound request in place. It runs
// inside the proxy's Director, after the standard forwarding headers are set.
func applyRequestTransformation(req *http.Request, tr *RequestTransformation) {
	if tr == nil {
		return
	}
	if tr.HTTPMethod != "" {
		req.Method = tr.HTTPMethod
	}

	// Headers: remove → rename → replace → add → append.
	if tr.Remove != nil {
		for _, k := range tr.Remove.HeaderKeys {
			req.Header.Del(k)
		}
	}
	if tr.Rename != nil {
		for _, p := range tr.Rename.Headers {
			renameHeader(req.Header, p.From, p.To)
		}
	}
	if tr.Replace != nil {
		for _, kv := range tr.Replace.Headers {
			if headerExists(req.Header, kv.Key) {
				req.Header.Set(kv.Key, kv.Value)
			}
		}
	}
	if tr.Add != nil {
		for _, kv := range tr.Add.Headers {
			if !headerExists(req.Header, kv.Key) {
				req.Header.Set(kv.Key, kv.Value)
			}
		}
	}
	if tr.Append != nil {
		for _, kv := range tr.Append.Headers {
			req.Header.Add(kv.Key, kv.Value)
		}
	}

	// Query parameters, same phase order.
	q := req.URL.Query()
	changed := false
	if tr.Remove != nil {
		for _, k := range tr.Remove.QueryParams {
			q.Del(k)
			changed = true
		}
	}
	if tr.Rename != nil {
		for _, p := range tr.Rename.QueryParams {
			if vs, ok := q[p.From]; ok {
				delete(q, p.From)
				q[p.To] = vs
				changed = true
			}
		}
	}
	if tr.Replace != nil {
		for _, kv := range tr.Replace.QueryParams {
			if q.Has(kv.Key) {
				q.Set(kv.Key, kv.Value)
				changed = true
			}
		}
	}
	if tr.Add != nil {
		for _, kv := range tr.Add.QueryParams {
			if !q.Has(kv.Key) {
				q.Set(kv.Key, kv.Value)
				changed = true
			}
		}
	}
	if tr.Append != nil {
		for _, kv := range tr.Append.QueryParams {
			q.Add(kv.Key, kv.Value)
			changed = true
		}
	}
	if changed {
		req.URL.RawQuery = q.Encode()
	}

	if !requestHasBodyOps(tr) || !isTransformableJSON(req.Header, req.Body != nil) {
		return
	}
	raw, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		// The body is already damaged; forward what was read with a
		// consistent Content-Length so the transport fails predictably.
		setRequestBody(req, raw)
		return
	}
	transformed, ok := transformJSONObject(raw, func(obj map[string]any) {
		if tr.Allow != nil && len(tr.Allow.Body) > 0 {
			retainPaths(obj, tr.Allow.Body)
		}
		if tr.Remove != nil {
			for _, k := range tr.Remove.Body {
				deletePath(obj, k)
			}
		}
		if tr.Rename != nil {
			for _, p := range tr.Rename.Body {
				if v, ok := getPath(obj, p.From); ok {
					deletePath(obj, p.From)
					setPath(obj, p.To, v)
				}
			}
		}
		if tr.Replace != nil {
			for _, kv := range tr.Replace.Body {
				if _, ok := getPath(obj, kv.Key); ok {
					setPath(obj, kv.Key, kv.Value)
				}
			}
		}
		if tr.Add != nil {
			for _, kv := range tr.Add.Body {
				if _, ok := getPath(obj, kv.Key); !ok {
					setPath(obj, kv.Key, kv.Value)
				}
			}
		}
		if tr.Append != nil {
			for _, kv := range tr.Append.Body {
				appendPath(obj, kv.Key, kv.Value)
			}
		}
	})
	if !ok {
		transformed = raw
	}
	setRequestBody(req, transformed)
}

// applyResponseTransformation rewrites the upstream response in place (the
// proxy's ModifyResponse hook). Operations carrying ifStatusCode apply only
// when the response status is listed.
func applyResponseTransformation(resp *http.Response, tr *ResponseTransformation) error {
	if tr == nil {
		return nil
	}
	status := resp.StatusCode

	if tr.Remove != nil && statusMatches(tr.Remove.IfStatusCode, status) {
		for _, k := range tr.Remove.HeaderKeys {
			resp.Header.Del(k)
		}
	}
	if tr.Rename != nil && statusMatches(tr.Rename.IfStatusCode, status) {
		for _, p := range tr.Rename.Headers {
			renameHeader(resp.Header, p.From, p.To)
		}
	}
	if tr.Replace != nil && statusMatches(tr.Replace.IfStatusCode, status) {
		for _, kv := range tr.Replace.Headers {
			if headerExists(resp.Header, kv.Key) {
				resp.Header.Set(kv.Key, kv.Value)
			}
		}
	}
	if tr.Add != nil && statusMatches(tr.Add.IfStatusCode, status) {
		for _, kv := range tr.Add.Headers {
			if !headerExists(resp.Header, kv.Key) {
				resp.Header.Set(kv.Key, kv.Value)
			}
		}
	}
	if tr.Append != nil && statusMatches(tr.Append.IfStatusCode, status) {
		for _, kv := range tr.Append.Headers {
			resp.Header.Add(kv.Key, kv.Value)
		}
	}

	// Whole-body replacement takes precedence over JSON field operations.
	// Presence (not emptiness) triggers it, so replacing with "" works.
	if tr.Replace != nil && tr.Replace.Body != nil && statusMatches(tr.Replace.IfStatusCode, status) {
		return setResponseBody(resp, []byte(*tr.Replace.Body))
	}

	if !responseHasJSONOps(tr) || !isTransformableJSON(resp.Header, resp.Body != nil) {
		return nil
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	transformed, ok := transformJSONObject(raw, func(obj map[string]any) {
		if tr.Allow != nil && len(tr.Allow.JSONKeys) > 0 {
			retainPaths(obj, tr.Allow.JSONKeys)
		}
		if tr.Remove != nil && statusMatches(tr.Remove.IfStatusCode, status) {
			for _, k := range tr.Remove.JSONKeys {
				deletePath(obj, k)
			}
		}
		if tr.Rename != nil && statusMatches(tr.Rename.IfStatusCode, status) {
			for _, p := range tr.Rename.JSON {
				if v, ok := getPath(obj, p.From); ok {
					deletePath(obj, p.From)
					setPath(obj, p.To, v)
				}
			}
		}
		if tr.Replace != nil && statusMatches(tr.Replace.IfStatusCode, status) {
			for _, kv := range tr.Replace.JSON {
				if _, ok := getPath(obj, kv.Key); ok {
					setPath(obj, kv.Key, kv.Value)
				}
			}
		}
		if tr.Add != nil && statusMatches(tr.Add.IfStatusCode, status) {
			for _, kv := range tr.Add.JSON {
				if _, ok := getPath(obj, kv.Key); !ok {
					setPath(obj, kv.Key, kv.Value)
				}
			}
		}
		if tr.Append != nil && statusMatches(tr.Append.IfStatusCode, status) {
			for _, kv := range tr.Append.JSON {
				appendPath(obj, kv.Key, kv.Value)
			}
		}
	})
	if !ok {
		transformed = raw
	}
	return setResponseBody(resp, transformed)
}

func requestHasBodyOps(tr *RequestTransformation) bool {
	return (tr.Allow != nil && len(tr.Allow.Body) > 0) ||
		(tr.Remove != nil && len(tr.Remove.Body) > 0) ||
		(tr.Rename != nil && len(tr.Rename.Body) > 0) ||
		(tr.Replace != nil && len(tr.Replace.Body) > 0) ||
		(tr.Add != nil && len(tr.Add.Body) > 0) ||
		(tr.Append != nil && len(tr.Append.Body) > 0)
}

func responseHasJSONOps(tr *ResponseTransformation) bool {
	return (tr.Allow != nil && len(tr.Allow.JSONKeys) > 0) ||
		(tr.Remove != nil && len(tr.Remove.JSONKeys) > 0) ||
		(tr.Rename != nil && len(tr.Rename.JSON) > 0) ||
		(tr.Replace != nil && len(tr.Replace.JSON) > 0) ||
		(tr.Add != nil && len(tr.Add.JSON) > 0) ||
		(tr.Append != nil && len(tr.Append.JSON) > 0)
}

// isTransformableJSON reports whether the body may be parsed and rewritten:
// declared JSON, present, and not content-encoded (a compressed body would
// need decoding this mock does not attempt).
func isTransformableJSON(h http.Header, hasBody bool) bool {
	if !hasBody || h.Get("Content-Encoding") != "" {
		return false
	}
	ct := h.Get("Content-Type")
	return strings.HasPrefix(ct, "application/json") || strings.Contains(ct, "+json")
}

// transformJSONObject parses raw as a JSON object, applies f, and re-encodes.
// Non-object JSON (arrays, scalars) is reported as not transformable.
func transformJSONObject(raw []byte, f func(map[string]any)) ([]byte, bool) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, false
	}
	f(obj)
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, false
	}
	return out, true
}

func setResponseBody(resp *http.Response, body []byte) error {
	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

// headerExists distinguishes a missing header from one present with an empty
// value (Header.Get returns "" for both).
func headerExists(h http.Header, key string) bool {
	_, ok := h[http.CanonicalHeaderKey(key)]
	return ok
}

func setRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

func statusMatches(codes []int, status int) bool {
	return len(codes) == 0 || slices.Contains(codes, status)
}

func renameHeader(h http.Header, from, to string) {
	if from == "" || to == "" {
		return
	}
	vs := h.Values(from)
	if len(vs) == 0 {
		return
	}
	values := slices.Clone(vs)
	h.Del(from)
	for _, v := range values {
		h.Add(to, v)
	}
}

// walkPath descends to the parent object of the path's final segment,
// returning it and that segment.
func walkPath(obj map[string]any, path string, create bool) (map[string]any, string, bool) {
	segs := strings.Split(path, ".")
	cur := obj
	for _, seg := range segs[:len(segs)-1] {
		next, ok := cur[seg].(map[string]any)
		if !ok {
			if !create {
				return nil, "", false
			}
			next = map[string]any{}
			cur[seg] = next
		}
		cur = next
	}
	return cur, segs[len(segs)-1], true
}

func getPath(obj map[string]any, path string) (any, bool) {
	parent, key, ok := walkPath(obj, path, false)
	if !ok {
		return nil, false
	}
	v, ok := parent[key]
	return v, ok
}

func setPath(obj map[string]any, path string, value any) {
	parent, key, _ := walkPath(obj, path, true)
	parent[key] = value
}

func deletePath(obj map[string]any, path string) {
	if parent, key, ok := walkPath(obj, path, false); ok {
		delete(parent, key)
	}
}

// appendPath sets a missing value, and otherwise accumulates into an array.
func appendPath(obj map[string]any, path string, value any) {
	cur, ok := getPath(obj, path)
	switch {
	case !ok:
		setPath(obj, path, value)
	default:
		if arr, isArr := cur.([]any); isArr {
			setPath(obj, path, append(arr, value))
		} else {
			setPath(obj, path, []any{cur, value})
		}
	}
}

// retainPaths keeps only the listed paths (and their ancestors) in obj.
func retainPaths(obj map[string]any, paths []string) {
	kept := map[string]any{}
	for _, p := range paths {
		if v, ok := getPath(obj, p); ok {
			setPath(kept, p, v)
		}
	}
	for k := range obj {
		delete(obj, k)
	}
	maps.Copy(obj, kept)
}
