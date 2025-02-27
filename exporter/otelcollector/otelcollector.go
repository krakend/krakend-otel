// Package otelcollector implements the Open Telemetry exporter.
package otelcollector

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
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
	exporter                    sdktrace.SpanExporter
	metricExporter              sdkmetric.Exporter
	metricsDisabledByDefault    bool
	tracesDisabledByDefault     bool
	customMetricReportingPeriod time.Duration
}

// SpanExporter implements the interface to export traces.
func (c *OtelCollector) SpanExporter() sdktrace.SpanExporter {
	return c.exporter
}

func (c *OtelCollector) MetricReader(reportingPeriod time.Duration) sdkmetric.Reader {
	if c.customMetricReportingPeriod >= time.Second {
		reportingPeriod = c.customMetricReportingPeriod
	}
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
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, err
		}
		switch u.Scheme {
		case "http":
			tOpts = append(tOpts, otlptracehttp.WithInsecure())
			endpoint = u.Host
		case "https":
			endpoint = u.Host
		}
	} else {
		tOpts = append(tOpts, otlptracehttp.WithInsecure())
	}
	tOpts = append(tOpts, otlptracehttp.WithEndpoint(endpoint))

	exporter, err := otlptracehttp.New(ctx, tOpts...)
	if err != nil {
		return nil, errors.New("cannot create http trace exporter:" + err.Error())
	}

	mOpts = append(mOpts, otlpmetrichttp.WithEndpoint(endpoint))
	metricExporter, err := otlpmetrichttp.New(ctx, mOpts...)
	if err != nil {
		return nil, errors.New("cannot create http metric exporter:" + err.Error())
	}

	return &OtelCollector{
		exporter:                    exporter,
		metricExporter:              metricExporter,
		metricsDisabledByDefault:    cfg.DisableMetrics,
		tracesDisabledByDefault:     cfg.DisableTraces,
		customMetricReportingPeriod: time.Duration(cfg.CustomMetricReportingPeriod) * time.Second,
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
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, err
		}
		switch u.Scheme {
		case "http":
			tOpts = append(tOpts, otlptracegrpc.WithInsecure())
			endpoint = u.Host
		case "https":
			endpoint = u.Host
		}
	} else {
		tOpts = append(tOpts, otlptracegrpc.WithInsecure())
	}
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
		exporter:                    exporter,
		metricExporter:              metricExporter,
		metricsDisabledByDefault:    cfg.DisableMetrics,
		tracesDisabledByDefault:     cfg.DisableTraces,
		customMetricReportingPeriod: time.Duration(cfg.CustomMetricReportingPeriod) * time.Second,
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
	// by default, grpc conections have TLS enabled:
	options := []interface{}{otlptracegrpc.WithInsecure(), otlpmetricgrpc.WithInsecure()}
	return ExporterWithOptions(ctx, cfg, options)
}
