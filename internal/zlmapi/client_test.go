package zlmapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret"

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		secret      string
		shouldError bool
	}{
		{name: "valid uri and secret", uri: "http://localhost:8080", secret: testSecret},
		{name: "empty uri", uri: "", secret: testSecret, shouldError: true},
		{name: "empty secret", uri: "http://localhost:8080", secret: "", shouldError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewClient(tt.uri, tt.secret, Options{})
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, c)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, c)
		})
	}
}

func TestGetSendsSecretHeader(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("secret")
		_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{"branchName":"master"}}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, testSecret, Options{})
	require.NoError(t, err)

	data, err := Get[Version](context.Background(), c, EndpointVersion)
	require.NoError(t, err)
	assert.Equal(t, testSecret, got)
	assert.Equal(t, "master", data.BranchName)
}

func TestGetErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantAPIErr bool
	}{
		{
			name: "success",
			body: `{"code":0,"msg":"success","data":{"branchName":"master"}}`,
		},
		{
			name:       "non-zero code",
			body:       `{"code":1,"msg":"error"}`,
			wantAPIErr: true,
		},
		{
			name: "invalid json",
			body: `invalid json`,
		},
		{
			name: "data type mismatch",
			body: `{"code":0,"msg":"success","data":{"branchName":42}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c, err := NewClient(srv.URL, testSecret, Options{})
			require.NoError(t, err)

			_, err = Get[Version](context.Background(), c, EndpointVersion)
			if tt.name == "success" {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)

			var apiErr *APIError
			assert.Equal(t, tt.wantAPIErr, errors.As(err, &apiErr))
		})
	}
}

func TestGetUnreachableServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	c, err := NewClient(url, testSecret, Options{})
	require.NoError(t, err)

	_, err = Get[Version](context.Background(), c, EndpointVersion)
	assert.Error(t, err)
}

// Bug: the request was built by hand and never carried the caller's context,
// so the scrape timeout had no effect on a hung ZLMediaKit.
func TestGetHonorsContextDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{}}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, testSecret, Options{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = Get[Version](ctx, c, EndpointVersion)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 400*time.Millisecond, "Get should abort at the deadline")
}

// Bug: --web.timeout was parsed but never applied to the HTTP client.
func TestClientTimeoutBoundsRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{}}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, testSecret, Options{Timeout: 20 * time.Millisecond})
	require.NoError(t, err)

	start := time.Now()
	_, err = Get[Version](context.Background(), c, EndpointVersion)

	assert.Error(t, err)
	assert.Less(t, time.Since(start), 400*time.Millisecond, "the client timeout should abort the request")
}

// Bug: the TLS option was inverted, so the default configuration silently
// accepted any certificate.
func TestTLSCertificateIsVerifiedByDefault(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{"branchName":"master"}}`))
	}))
	defer srv.Close()

	verifying, err := NewClient(srv.URL, testSecret, Options{})
	require.NoError(t, err)
	_, err = Get[Version](context.Background(), verifying, EndpointVersion)
	assert.Error(t, err, "a self-signed certificate must be rejected by default")

	skipping, err := NewClient(srv.URL, testSecret, Options{InsecureSkipVerify: true})
	require.NoError(t, err)
	_, err = Get[Version](context.Background(), skipping, EndpointVersion)
	assert.NoError(t, err, "InsecureSkipVerify must accept a self-signed certificate")
}

// FlexInt exists because ZLMediaKit has reported listRtpServer's port both as
// a JSON number and as a quoted string.
func TestFlexIntAcceptsNumbersAndStrings(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    FlexInt
		wantErr bool
	}{
		{name: "number", json: `10000`, want: 10000},
		{name: "quoted number", json: `"10000"`, want: 10000},
		{name: "negative number", json: `-1`, want: -1},
		{name: "quoted negative number", json: `"-1"`, want: -1},
		{name: "null", json: `null`, want: 0},
		{name: "empty string", json: `""`, want: 0},
		{name: "non-numeric string", json: `"abc"`, wantErr: true},
		{name: "float", json: `1.5`, wantErr: true},
		{name: "object", json: `{}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got FlexInt
			err := json.Unmarshal([]byte(tt.json), &got)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFlexIntString(t *testing.T) {
	assert.Equal(t, "10000", FlexInt(10000).String())
	assert.Equal(t, "0", FlexInt(0).String())
	assert.Equal(t, "-1", FlexInt(-1).String())
}

// The two RTP server shapes seen in the wild must both decode.
func TestRtpServerDecodesBothPortShapes(t *testing.T) {
	var current RtpServer
	require.NoError(t, json.Unmarshal([]byte(
		`{"vhost":"__defaultVhost__","app":"rtp","stream_id":"s","port":10000,"ssrc":42,"tcp_mode":1,"only_track":0}`), &current))
	assert.Equal(t, FlexInt(10000), current.Port)
	assert.Equal(t, FlexInt(42), current.SSRC)
	assert.Equal(t, "rtp", current.App)

	var legacy RtpServer
	require.NoError(t, json.Unmarshal([]byte(`{"port":"10000","stream_id":"s"}`), &legacy))
	assert.Equal(t, FlexInt(10000), legacy.Port)
	assert.Equal(t, "s", legacy.StreamID)
	assert.Empty(t, legacy.App, "fields absent from older responses stay zero")
}

func TestAPIErrorMessage(t *testing.T) {
	err := &APIError{Endpoint: EndpointVersion, Code: 1, Msg: "Incorrect secret"}
	assert.EqualError(t, err,
		"unexpected API response code from index/api/version: 1, reason: Incorrect secret")
}

func TestGetRejectsUnbuildableURL(t *testing.T) {
	c, err := NewClient("http://exam\nple.com", testSecret, Options{})
	require.NoError(t, err)

	_, err = Get[Version](context.Background(), c, EndpointVersion)
	assert.ErrorContains(t, err, "error building request")
}
