package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateFlags(t *testing.T) {
	tests := []struct {
		name          string
		outputFormat  string
		alertSlack    string
		logLevel      string
		logFormat     string
		expectedError string
	}{
		{
			name:         "valid human format",
			outputFormat: "human",
			alertSlack:   "",
			logLevel:     "info",
			logFormat:    "text",
		},
		{
			name:         "valid json format",
			outputFormat: "json",
			alertSlack:   "",
			logLevel:     "debug",
			logFormat:    "json",
		},
		{
			name:         "valid yaml format",
			outputFormat: "yaml",
			alertSlack:   "",
			logLevel:     "warn",
			logFormat:    "text",
		},
		{
			name:          "invalid output format",
			outputFormat:  "xml",
			alertSlack:    "",
			logLevel:      "info",
			logFormat:     "text",
			expectedError: "unsupported output format: xml (supported: human, json, yaml)",
		},
		{
			name:         "valid slack webhook",
			outputFormat: "json",
			alertSlack:   "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
			logLevel:     "info",
			logFormat:    "text",
		},
		{
			name:          "invalid slack webhook - wrong prefix",
			outputFormat:  "json",
			alertSlack:    "https://example.com/webhook",
			logLevel:      "info",
			logFormat:     "text",
			expectedError: "invalid Slack webhook URL: must start with https://hooks.slack.com/",
		},
		{
			name:          "invalid slack webhook - not https",
			outputFormat:  "json",
			alertSlack:    "http://hooks.slack.com/services/test",
			logLevel:      "info",
			logFormat:     "text",
			expectedError: "invalid Slack webhook URL: must start with https://hooks.slack.com/",
		},
		{
			name:          "invalid log level",
			outputFormat:  "json",
			alertSlack:    "",
			logLevel:      "invalid",
			logFormat:     "text",
			expectedError: "unsupported log level: invalid (supported: debug, info, warn, error)",
		},
		{
			name:          "invalid log format",
			outputFormat:  "json",
			alertSlack:    "",
			logLevel:      "info",
			logFormat:     "xml",
			expectedError: "unsupported log format: xml (supported: text, json)",
		},
		{
			name:          "combined invalid format and slack",
			outputFormat:  "xml",
			alertSlack:    "invalid-url",
			logLevel:      "info",
			logFormat:     "text",
			expectedError: "unsupported output format: xml (supported: human, json, yaml)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputFormat = tt.outputFormat
			alertSlack = tt.alertSlack
			logLevel = tt.logLevel
			logFormat = tt.logFormat

			err := validateFlags()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFlags_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		outputFormat  string
		alertSlack    string
		logLevel      string
		logFormat     string
		expectedError string
	}{
		{
			name:          "empty output format is invalid",
			outputFormat:  "",
			alertSlack:    "",
			logLevel:      "info",
			logFormat:     "text",
			expectedError: "unsupported output format:  (supported: human, json, yaml)",
		},
		{
			name:          "case sensitive format validation",
			outputFormat:  "JSON",
			logLevel:      "info",
			logFormat:     "text",
			expectedError: "unsupported output format: JSON (supported: human, json, yaml)",
		},
		{
			name:         "empty slack webhook is valid",
			outputFormat: "json",
			alertSlack:   "",
			logLevel:     "info",
			logFormat:    "text",
		},
		{
			name:         "slack webhook with path parameters",
			outputFormat: "json",
			alertSlack:   "https://hooks.slack.com/services/T123/B456/token123",
			logLevel:     "info",
			logFormat:    "text",
		},
		{
			name:          "case sensitive log level validation",
			outputFormat:  "json",
			alertSlack:    "",
			logLevel:      "INFO",
			logFormat:     "text",
			expectedError: "unsupported log level: INFO (supported: debug, info, warn, error)",
		},
		{
			name:          "case sensitive log format validation",
			outputFormat:  "json",
			alertSlack:    "",
			logLevel:      "info",
			logFormat:     "JSON",
			expectedError: "unsupported log format: JSON (supported: text, json)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputFormat = tt.outputFormat
			alertSlack = tt.alertSlack
			logLevel = tt.logLevel
			logFormat = tt.logFormat

			err := validateFlags()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOutputFormatMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"human", "human"},
		{"json", "json"},
		{"yaml", "yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			outputFormat = tt.input
			logLevel = "info"
			logFormat = "text"
			err := validateFlags()
			assert.NoError(t, err)
		})
	}
}
