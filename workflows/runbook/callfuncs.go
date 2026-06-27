package runbook

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sacloud/sakumock/workflows/expr"
)

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
	return func(ctx context.Context, env *expr.Env, call *CallStep) (expr.Value, error) {
		args, err := evalCallArgs(env, call.Args)
		if err != nil {
			return expr.Null, err
		}
		args["method"] = expr.String(method)

		fakeCall := &CallStep{
			Func: "http.request",
			Args: call.Args,
		}
		fakeCall.Args["method"] = method
		return httpRequestCall(ctx, env, fakeCall)
	}
}

func httpRequestCall(ctx context.Context, env *expr.Env, call *CallStep) (expr.Value, error) {
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
		encoded, err := expr.Eval("json.encode(b)", func() *expr.Env {
			e := expr.NewEnv()
			e.Set("b", b)
			return e
		}())
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return expr.Null, fmt.Errorf("http.request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return expr.Null, fmt.Errorf("http.request: read body: %w", err)
	}

	return expr.Object(map[string]expr.Value{
		"status": expr.Number(float64(resp.StatusCode)),
		"body":   expr.String(string(respBody)),
	}), nil
}

func sysSleep(ctx context.Context, _ *expr.Env, call *CallStep) (expr.Value, error) {
	args := call.Args
	secStr, ok := args["seconds"]
	if !ok {
		return expr.Null, fmt.Errorf("sys.sleep: seconds is required")
	}
	secVal, err := expr.Eval(secStr, expr.NewEnv())
	if err != nil {
		return expr.Null, fmt.Errorf("sys.sleep: %w", err)
	}
	dur := time.Duration(secVal.AsNumber()) * time.Second

	select {
	case <-time.After(dur):
		return expr.Null, nil
	case <-ctx.Done():
		return expr.Null, ctx.Err()
	}
}

func sysSleepUntil(ctx context.Context, _ *expr.Env, call *CallStep) (expr.Value, error) {
	args := call.Args
	dateStr, ok := args["date"]
	if !ok {
		return expr.Null, fmt.Errorf("sys.sleepUntil: date is required")
	}
	dateVal, err := expr.Eval(dateStr, expr.NewEnv())
	if err != nil {
		return expr.Null, fmt.Errorf("sys.sleepUntil: %w", err)
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
