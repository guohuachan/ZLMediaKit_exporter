// Package zlmapi is a thin client for the ZLMediaKit HTTP API.
//
// Every endpoint replies with the same envelope: a "code" field that is zero on
// success, a human readable "msg", and an endpoint specific "data" payload.
package zlmapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// SuccessCode is the value ZLMediaKit puts in "code" when a call succeeded.
const SuccessCode = 0

// Endpoints scraped by the exporter.
const (
	EndpointVersion            = "index/api/version"
	EndpointGetAPIList         = "index/api/getApiList"
	EndpointGetThreadsLoad     = "index/api/getThreadsLoad"
	EndpointGetWorkThreadsLoad = "index/api/getWorkThreadsLoad"
	EndpointGetStatistic       = "index/api/getStatistic"
	EndpointGetAllSession      = "index/api/getAllSession"
	EndpointGetMediaList       = "index/api/getMediaList"
	EndpointListRtpServer      = "index/api/listRtpServer"
)

// Options tunes the HTTP transport used to reach ZLMediaKit.
type Options struct {
	// Timeout bounds a single API request. Zero means no client-side limit,
	// leaving the caller's context as the only deadline.
	Timeout time.Duration

	// InsecureSkipVerify disables TLS certificate verification. It is off by
	// default: an exporter that silently accepts any certificate is worse than
	// one that refuses to start.
	InsecureSkipVerify bool
}

// Client talks to a single ZLMediaKit API server.
type Client struct {
	baseURL string
	secret  string
	http    http.Client
}

// NewClient returns a client for the ZLMediaKit API server at baseURL.
func NewClient(baseURL, secret string, opts Options) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("ZlMediaKit API uri is required")
	}
	if secret == "" {
		return nil, errors.New("ZlMediaKit API secret is required")
	}

	c := &Client{baseURL: baseURL, secret: secret}
	c.http.Timeout = opts.Timeout
	if opts.InsecureSkipVerify {
		// Clone rather than build a bare Transport, so proxy support, timeouts
		// and connection pooling keep their standard-library defaults.
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in via --zlm.tls-insecure-skip-verify
		c.http.Transport = transport
	}
	return c, nil
}

// Response is the envelope every ZLMediaKit API endpoint replies with.
type Response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

// APIError reports a ZLMediaKit response carrying a non-zero code.
type APIError struct {
	Endpoint string
	Code     int
	Msg      string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("unexpected API response code from %s: %d, reason: %s", e.Endpoint, e.Code, e.Msg)
}

// Get fetches endpoint and returns the decoded "data" payload.
func Get[T any](ctx context.Context, c *Client, endpoint string) (T, error) {
	var zero T

	uri := fmt.Sprintf("%s/%s", c.baseURL, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return zero, fmt.Errorf("error building request for %q: %w", uri, err)
	}
	req.Header.Set("secret", c.secret)

	res, err := c.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("error scraping ZLMediaKit: %w", err)
	}
	defer res.Body.Close()

	var resp Response[T]
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return zero, fmt.Errorf("error decoding JSON response from %s: %w", endpoint, err)
	}
	if resp.Code != SuccessCode {
		return zero, &APIError{Endpoint: endpoint, Code: resp.Code, Msg: resp.Msg}
	}
	return resp.Data, nil
}
