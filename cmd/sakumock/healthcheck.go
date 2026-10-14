package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// HealthcheckCmd probes a running sakumock's health endpoint and exits non-zero
// unless it answers 200, so a shell-less container can use it as its
// HEALTHCHECK command.
type HealthcheckCmd struct {
	URL      string        `help:"Health endpoint URL" default:"http://127.0.0.1:18000/_sakumock/health" env:"SAKUMOCK_HEALTHCHECK_URL"`
	Timeout  time.Duration `help:"Request timeout" default:"3s"`
	Insecure bool          `help:"Skip TLS certificate verification (for a self-signed --tls-cert)"`
}

func (c *HealthcheckCmd) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return err
	}
	client := http.DefaultClient
	if c.Insecure {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		client = &http.Client{Transport: tr}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: unexpected status %s", c.URL, resp.Status)
	}
	_, err = os.Stdout.Write(body)
	return err
}
