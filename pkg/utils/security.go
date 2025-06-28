package utils

import "strings"

func RedactWebhookURL(webhookURL string) string {
	if webhookURL == "" {
		return ""
	}
	if lastSlash := strings.LastIndex(webhookURL, "/"); lastSlash != -1 && lastSlash < len(webhookURL)-1 {
		return webhookURL[:lastSlash+1] + "***"
	}
	return "***"
}
