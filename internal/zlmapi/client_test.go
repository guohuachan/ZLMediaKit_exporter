package zlmapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
