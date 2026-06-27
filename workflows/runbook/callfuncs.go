package runbook

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sacloud/sakumock/workflows/expr"
)

const maxResponseBodySize = 10 * 1024 * 1024 // 10 MiB
const maxRedirects = 10

func NewHTTPClient(allowLocalNet bool) *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if !allowLocalNet && isBlockedHost(req.URL.Host) {
				return fmt.Errorf("redirect to blocked host: %s", req.URL.Host)
			}
			return nil
		},
	}
}

func defaultCallFuncs() map[string]CallFunc {
	return map[string]CallFunc{
		"http.get":       httpCall("GET"),
		"http.post":      httpCall("POST"),
		"http.put":       httpCall("PUT"),
		"http.delete":    httpCall("DELETE"),
		"http.patch":     httpCall("PATCH"),
		"http.request":   httpRequestCall,
		"sys.sleep":      sysSleep,
		"sys.sleepUntil": sysSleepUntil,
	}
}

func evalCallArgs(env *expr.Env, raw map[string]string) (map[string]expr.Value, error) {
	result := make(map[string]expr.Value, len(raw))
	for k, v := range raw {
		val, err := expr.EvalInterpolated(v, env)
		if err != nil {
			return nil, fmt.Errorf("arg %s: %w", k, err)
		}
		result[k] = val
	}
	return result, nil
}

func httpCall(method string) CallFunc {
	return func(ctx context.Context, env *expr.Env, call *CallStep, opts CallOpts) (expr.Value, error) {
		merged := make(map[string]string, len(call.Args)+1)
		maps.Copy(merged, call.Args)
		merged["method"] = method
		return httpRequestCall(ctx, env, &CallStep{
			Func:   "http.request",
			Args:   merged,
			Result: call.Result,
		}, opts)
	}
}

func isBlockedHost(host string) bool {
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}

	if hostname == "localhost" {
		return true
	}

	ip := net.ParseIP(hostname)
	if ip == nil {
		addrs, err := net.LookupIP(hostname)
		if err != nil || len(addrs) == 0 {
			return false
		}
		ip = addrs[0]
	}

	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func httpRequestCall(ctx context.Context, env *expr.Env, call *CallStep, opts CallOpts) (expr.Value, error) {
	args, err := evalCallArgs(env, call.Args)
	if err != nil {
		return expr.Null, err
	}

	method := "GET"
	if m, ok := args["method"]; ok {
		method = strings.ToUpper(m.AsString())
	}

	rawURL := ""
	if u, ok := args["url"]; ok {
		rawURL = u.AsString()
	}
	if rawURL == "" {
		return expr.Null, fmt.Errorf("http.request: url is required")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return expr.Null, fmt.Errorf("http.request: invalid url: %w", err)
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return expr.Null, fmt.Errorf("http.request: unsupported scheme %q", parsedURL.Scheme)
	}

	if !opts.AllowLocalNet && isBlockedHost(parsedURL.Host) {
		return expr.Null, fmt.Errorf("http.request: this URL is blocked")
	}

	if q, ok := args["query"]; ok && q.Type() == expr.TypeObject {
		qs := parsedURL.Query()
		for k, v := range q.AsObject() {
			qs.Set(k, v.ToString())
		}
		parsedURL.RawQuery = qs.Encode()
	}

	timeout := 60 * time.Second
	if t, ok := args["timeout"]; ok {
		sec := t.AsNumber()
		if sec >= 5 && sec <= 180 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	var bodyReader io.Reader
	if b, ok := args["body"]; ok && !b.IsNull() {
		e := expr.NewEnv()
		e.Set("b", b)
		encoded, err := expr.Eval("json.encode(b)", e)
		if err != nil {
			return expr.Null, fmt.Errorf("http.request: encode body: %w", err)
		}
		bodyReader = strings.NewReader(encoded.AsString())
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), bodyReader)
	if err != nil {
		return expr.Null, fmt.Errorf("http.request: %w", err)
	}

	if h, ok := args["headers"]; ok && h.Type() == expr.TypeObject {
		for k, v := range h.AsObject() {
			req.Header.Set(k, v.ToString())
		}
	}

	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return expr.Null, fmt.Errorf("http.request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
	if err != nil {
		return expr.Null, fmt.Errorf("http.request: read body: %w", err)
	}
	if len(respBody) > maxResponseBodySize {
		return expr.Null, fmt.Errorf("http.request: response body size exceeds limit %d bytes", maxResponseBodySize)
	}

	return expr.Object(map[string]expr.Value{
		"status": expr.Number(float64(resp.StatusCode)),
		"body":   expr.String(string(respBody)),
	}), nil
}

func sysSleep(ctx context.Context, env *expr.Env, call *CallStep, _ CallOpts) (expr.Value, error) {
	args, err := evalCallArgs(env, call.Args)
	if err != nil {
		return expr.Null, err
	}
	secVal, ok := args["seconds"]
	if !ok {
		return expr.Null, fmt.Errorf("sys.sleep: seconds is required")
	}
	dur := time.Duration(secVal.ToNumber() * float64(time.Second))

	select {
	case <-time.After(dur):
		return expr.Null, nil
	case <-ctx.Done():
		return expr.Null, ctx.Err()
	}
}

func sysSleepUntil(ctx context.Context, env *expr.Env, call *CallStep, _ CallOpts) (expr.Value, error) {
	args, err := evalCallArgs(env, call.Args)
	if err != nil {
		return expr.Null, err
	}
	dateVal, ok := args["date"]
	if !ok {
		return expr.Null, fmt.Errorf("sys.sleepUntil: date is required")
	}
	target, err := time.Parse(time.RFC3339, dateVal.AsString())
	if err != nil {
		return expr.Null, fmt.Errorf("sys.sleepUntil: invalid date: %w", err)
	}
	dur := time.Until(target)
	if dur <= 0 {
		return expr.Null, nil
	}

	select {
	case <-time.After(dur):
		return expr.Null, nil
	case <-ctx.Done():
		return expr.Null, ctx.Err()
	}
}
