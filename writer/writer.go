package writer

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/adasari/telemetry-log-extension/utils"
	"github.com/fluent/fluent-logger-golang/fluent"
)

type Writer interface {
	Write(ctx context.Context, record map[string]interface{}) error
	Flush(ctx context.Context) error
}

type FluentDWriter struct {
	tag          string
	fluentLogger *fluent.Fluent
}

func NewFluentDWriter() (*FluentDWriter, error) {
	host := os.Getenv("TELEMETRY_EXTENSION_FLUENTD_HOST")
	if host == "" {
		return nil, fmt.Errorf("host is missing")
	}

	var err error
	port := 24224
	portStr := os.Getenv("TELEMETRY_EXTENSION_FLUENTD_PORT")
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %v", portStr)
		}
	}

	fluentLogger, err := fluent.New(fluent.Config{
		FluentHost:            host,
		FluentPort:            port,
		TlsInsecureSkipVerify: true,
		FluentNetwork:         "tls", // TODO: replace with env variable.
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create fluent logger client: %v", err)
	}
	return &FluentDWriter{
		tag:          utils.GetEnvOrDefault("TELEMETRY_EXTENSION_FLUENTD_TAG_NAME", "lambda"),
		fluentLogger: fluentLogger,
	}, nil
}

func (f *FluentDWriter) Write(ctx context.Context, record map[string]interface{}) error {
	if err := f.fluentLogger.Post(f.tag, record); err != nil {
		return fmt.Errorf("failed to send record: %v", err)
	}

	return nil
}

func (f *FluentDWriter) Flush(ctx context.Context) error {
	return nil
}
