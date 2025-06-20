package internal

import (
	"context"
	"io"
)

type OutputFormat string

const (
	OutputFormatHuman OutputFormat = "human"
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatYAML  OutputFormat = "yaml"
)

type Reporter struct {
	writer io.Writer
	format OutputFormat
}

func NewReporter(writer io.Writer, format OutputFormat) *Reporter {
	return &Reporter{
		writer: writer,
		format: format,
	}
}

func (r *Reporter) GenerateReport(ctx context.Context) error {
	return nil
}

func (r *Reporter) SendSlackNotification(ctx context.Context, webhookURL string) error {
	return nil
}

func (r *Reporter) SendPrometheusMetrics(ctx context.Context, pushGatewayURL string) error {
	return nil
}
