// Package otelcollector implements the Open Telemetry exporter.
package otelcollector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	/*
		"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
		"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	*/
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	// ExporterKey is the name for the opentelemetry collector
	ExporterKey string = "opentelemetry"
)

// OtelCollectorConfig has the variables to configure
// the otel collector
type OtelCollectorConfig struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	UseHTTP bool   `json:"use_http"`
}

// OtelCollector implements the traces exporter.
type OtelCollector struct {
	exporter sdktrace.SpanExporter
}

// SpanExporter implements the interface to export traces.
func (c *OtelCollector) SpanExporter() sdktrace.SpanExporter {
	return c.exporter
}

// OtelCollectorConfigFromInterface creates an Open Telemetry configuration.
func OtelCollectorConfigFromInterface(in map[string]interface{}) (*OtelCollectorConfig, error) {
	cfg := OtelCollectorConfig{
		Port:    4317,
		UseHTTP: false,
	}
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(b, &cfg)
		if err != nil {
			return nil, err
		}
	}
	return &cfg, nil
}

// Exporter creates an Open Telemetry exporter instance.
func Exporter(ctx context.Context, cfg map[string]interface{}) (interface{}, error) {
	otelCfg, err := OtelCollectorConfigFromInterface(cfg)
	if err != nil {
		return nil, err
	}

	var exporter sdktrace.SpanExporter
	if otelCfg.UseHTTP {
		exporter, err = otlptracehttp.New(ctx)
		if err != nil {
			return nil, errors.New("cannot create http exporter")
		}
	} else {
		// by default, grpc conections has TLS enabled
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(fmt.Sprintf("%s:%d", otelCfg.Host, otelCfg.Port)))
		if err != nil {
			return nil, errors.New("cannot create grpc exporter")
		}
	}
	return &OtelCollector{
		exporter: exporter,
	}, nil
}
