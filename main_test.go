package main

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultVal   string
		envValue     string
		expectedVal  string
		shouldSetEnv bool
	}{
		{
			name:         "env exists",
			key:          "TEST_ENV_1",
			defaultVal:   "default",
			envValue:     "custom",
			expectedVal:  "custom",
			shouldSetEnv: true,
		},
		{
			name:        "env not exists",
			key:         "TEST_ENV_2",
			defaultVal:  "default",
			expectedVal: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldSetEnv {
				t.Setenv(tt.key, tt.envValue)
			}
			assert.Equal(t, tt.expectedVal, getEnv(tt.key, tt.defaultVal))
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultVal   bool
		envValue     string
		expectedVal  bool
		shouldSetEnv bool
	}{
		{
			name:         "env parses as true",
			key:          "TEST_BOOL_1",
			defaultVal:   false,
			envValue:     "true",
			expectedVal:  true,
			shouldSetEnv: true,
		},
		{
			name:         "env parses as false",
			key:          "TEST_BOOL_2",
			defaultVal:   true,
			envValue:     "false",
			expectedVal:  false,
			shouldSetEnv: true,
		},
		{
			name:         "unparsable env falls back to default",
			key:          "TEST_BOOL_3",
			defaultVal:   true,
			envValue:     "not-a-bool",
			expectedVal:  true,
			shouldSetEnv: true,
		},
		{
			name:        "env not exists",
			key:         "TEST_BOOL_4",
			defaultVal:  true,
			expectedVal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldSetEnv {
				t.Setenv(tt.key, tt.envValue)
			}
			assert.Equal(t, tt.expectedVal, getEnvBool(tt.key, tt.defaultVal))
		})
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		expected string
	}{
		{name: "empty secret", secret: "", expected: "<empty>"},
		{name: "short secret", secret: "1234", expected: "****"},
		{name: "long secret", secret: "1234567890", expected: "12****90"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, maskSecret(tt.secret))
		})
	}
}

// Bug: the startup banner passed printf-style arguments to slog, which renders
// them as !BADKEY pairs instead of structured fields.
func TestLogBuildInfoUsesStructuredAttributes(t *testing.T) {
	var buf bytes.Buffer
	logBuildInfo(slog.New(slog.NewTextHandler(&buf, nil)))

	out := buf.String()
	assert.NotContains(t, out, "!BADKEY")
	assert.Contains(t, out, "go_version=")
	assert.Contains(t, out, "version=")
}
