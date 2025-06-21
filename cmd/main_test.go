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
		expectedError string
	}{
		{
			name:         "valid human format",
			outputFormat: "human",
			alertSlack:   "",
		},
		{
			name:         "valid json format",
			outputFormat: "json",
			alertSlack:   "",
		},
		{
			name:         "valid yaml format",
			outputFormat: "yaml",
			alertSlack:   "",
		},
		{
			name:          "invalid output format",
			outputFormat:  "xml",
			alertSlack:    "",
			expectedError: "unsupported output format: xml (supported: human, json, yaml)",
		},
		{
			name:         "valid slack webhook",
			outputFormat: "json",
			alertSlack:   "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
		},
		{
			name:          "invalid slack webhook - wrong prefix",
			outputFormat:  "json",
			alertSlack:    "https://example.com/webhook",
			expectedError: "invalid Slack webhook URL: must start with https://hooks.slack.com/",
		},
		{
			name:          "invalid slack webhook - not https",
			outputFormat:  "json",
			alertSlack:    "http://hooks.slack.com/services/test",
			expectedError: "invalid Slack webhook URL: must start with https://hooks.slack.com/",
		},
		{
			name:          "combined invalid format and slack",
			outputFormat:  "xml",
			alertSlack:    "invalid-url",
			expectedError: "unsupported output format: xml (supported: human, json, yaml)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputFormat = tt.outputFormat
			alertSlack = tt.alertSlack

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
		expectedError string
	}{
		{
			name:          "empty output format is invalid",
			outputFormat:  "",
			alertSlack:    "",
			expectedError: "unsupported output format:  (supported: human, json, yaml)",
		},
		{
			name:         "case sensitive format validation",
			outputFormat: "JSON",
			expectedError: "unsupported output format: JSON (supported: human, json, yaml)",
		},
		{
			name:         "empty slack webhook is valid",
			outputFormat: "json",
			alertSlack:   "",
		},
		{
			name:         "slack webhook with path parameters",
			outputFormat: "json",
			alertSlack:   "https://hooks.slack.com/services/T123/B456/token123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputFormat = tt.outputFormat
			alertSlack = tt.alertSlack

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
			err := validateFlags()
			assert.NoError(t, err)
		})
	}
}
