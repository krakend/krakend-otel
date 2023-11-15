// Package otelcollector implements the Open Telemetry exporter.
package otelcollector

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/krakend/krakend-otel/config"
)

// OtelCollector implements the traces exporter.
type OtelCollector struct {
	exporter                 sdktrace.SpanExporter
	metricExporter           sdkmetric.Exporter
	metricsDisabledByDefault bool
	tracesDisabledByDefault  bool
}

// SpanExporter implements the interface to export traces.
func (c *OtelCollector) SpanExporter() sdktrace.SpanExporter {
	return c.exporter
}

func (c *OtelCollector) MetricReader(reportingPeriod time.Duration) sdkmetric.Reader {
	return sdkmetric.NewPeriodicReader(c.metricExporter,
		sdkmetric.WithInterval(reportingPeriod))
}

func (c *OtelCollector) MetricDefaultReporting() bool {
	return !c.metricsDisabledByDefault
}

func (c *OtelCollector) TraceDefaultReporting() bool {
	return !c.tracesDisabledByDefault
}

func httpExporterWithOptions(ctx context.Context, cfg config.OTLPExporter,
	options []interface{},
) (*OtelCollector, error) {
	tOpts := make([]otlptracehttp.Option, 0, len(options)+1)
	mOpts := make([]otlpmetrichttp.Option, 0, len(options)+1)
	for _, iopt := range options {
		if to, ok := iopt.(otlptracehttp.Option); ok {
			tOpts = append(tOpts, to)
		}
		if mo, ok := iopt.(otlpmetrichttp.Option); ok {
			mOpts = append(mOpts, mo)
		}
	}

	endpoint := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	tOpts = append(tOpts, otlptracehttp.WithEndpoint(endpoint))
	exporter, err := otlptracehttp.New(ctx, tOpts...)
	if err != nil {
		return nil, errors.New("cannot create http trace exporter")
	}

	mOpts = append(mOpts, otlpmetrichttp.WithEndpoint(endpoint))
	metricExporter, err := otlpmetrichttp.New(ctx, mOpts...)
	if err != nil {
		return nil, errors.New("cannot create http metric exporter")
	}

	return &OtelCollector{
		exporter:                 exporter,
		metricExporter:           metricExporter,
		metricsDisabledByDefault: cfg.DisableMetrics,
		tracesDisabledByDefault:  cfg.DisableTraces,
	}, nil
}

func grpcExporterWithOptions(ctx context.Context, cfg config.OTLPExporter,
	options []interface{},
) (*OtelCollector, error) {
	tOpts := make([]otlptracegrpc.Option, 0, len(options)+1)
	mOpts := make([]otlpmetricgrpc.Option, 0, len(options)+1)
	for _, iopt := range options {
		if to, ok := iopt.(otlptracegrpc.Option); ok {
			tOpts = append(tOpts, to)
		}
		if mo, ok := iopt.(otlpmetricgrpc.Option); ok {
			mOpts = append(mOpts, mo)
		}
	}

	endpoint := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	tOpts = append(tOpts, otlptracegrpc.WithEndpoint(endpoint))
	exporter, err := otlptracegrpc.New(ctx, tOpts...)
	if err != nil {
		return nil, errors.New("cannot create grpc traces exporter")
	}
	mOpts = append(mOpts, otlpmetricgrpc.WithEndpoint(endpoint))
	metricExporter, err := otlpmetricgrpc.New(ctx, mOpts...)
	if err != nil {
		return nil, errors.New("cannot create grpc metric exporter")
	}

	return &OtelCollector{
		exporter:                 exporter,
		metricExporter:           metricExporter,
		metricsDisabledByDefault: cfg.DisableMetrics,
		tracesDisabledByDefault:  cfg.DisableTraces,
	}, nil
}

func ExporterWithOptions(ctx context.Context, cfg config.OTLPExporter, options []interface{}) (*OtelCollector, error) {
	if cfg.Port == 0 {
		cfg.Port = 4317
	}
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}

	if cfg.UseHTTP {
		return httpExporterWithOptions(ctx, cfg, options)
	}
	return grpcExporterWithOptions(ctx, cfg, options)
}

// Exporter creates an Open Telemetry exporter instance.
func Exporter(ctx context.Context, cfg config.OTLPExporter) (*OtelCollector, error) {
	options := make([]interface{}, 0, 2)
	// by default, grpc conections have TLS enabled:
	options = append(options, otlptracegrpc.WithInsecure())
	options = append(options, otlpmetricgrpc.WithInsecure())
	return ExporterWithOptions(ctx, cfg, options)
}
