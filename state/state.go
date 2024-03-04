// Package state provides the functionality to "pack" into a single
// structure a set of configured instances (exporters, meters, tracers...)
package state

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/krakend/krakend-otel/exporter"
)

const (
	providerName string = "io.krakend.krakend-otel"
)

// OTEL defines the interface to obtain observability
// instruments for a state.
type OTEL interface {
	Tracer() trace.Tracer
	Meter() metric.Meter
	Propagator() propagation.TextMapPropagator
	Shutdown(ctx context.Context)
	MeterProvider() metric.MeterProvider
	TracerProvider() trace.TracerProvider
}

// GetterFn defines a function that will return an [OTEL] instance.
type GetterFn func() OTEL

type OTELStateConfig struct {
	MetricProviders       []string `json:"metric_providers"`
	TraceProviders        []string `json:"trace_providers"`
	MetricReportingPeriod int      `json:"metric_reporting_period"`
	TraceSampleRate       float64  `json:"trace_sample_rate"`
}

// OTELState is the basic implementation of an [OTEL] intstance.
type OTELState struct {
	meterProvider  metric.MeterProvider
	tracerProvider trace.TracerProvider

	// we need not the interface, but the actual implementation
	// to be able to call shutown:
	sdkMeterProvider  *sdkmetric.MeterProvider
	sdkTracerProvider *sdktrace.TracerProvider
	tracer            trace.Tracer
	meter             metric.Meter
}

// NewWithVersion create a new OTELState with a version for
// the KrakenD service, with the provided metrics and traces exporters
func NewWithVersion(serviceName string, cfg *OTELStateConfig, version string,
	me map[string]exporter.MetricReader, te map[string]exporter.SpanExporter,
) (*OTELState, error) {
	res := sdkresource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(version))

	reportingPeriod := time.Duration(cfg.MetricReportingPeriod) * time.Second
	metricOpts := make([]sdkmetric.Option, 0, len(cfg.MetricProviders)+2)
	for idx, prov := range cfg.MetricProviders {
		pm, ok := me[prov]
		if !ok {
			return nil, fmt.Errorf("not found exporter %s for metricprovider %d  (me %#v)", prov, idx, me)
		}
		reader := pm.MetricReader(reportingPeriod)
		metricOpts = append(metricOpts, sdkmetric.WithReader(reader))
	}

	var meterProvider metric.MeterProvider = noopmetric.NewMeterProvider()
	var sdkMeterProvider *sdkmetric.MeterProvider
	if len(metricOpts) > 0 {
		sdkMeterProvider = sdkmetric.NewMeterProvider(metricOpts...)
		meterProvider = sdkMeterProvider
	}
	meter := meterProvider.Meter(providerName)

	// Configure the tracing part
	traceOpts := make([]sdktrace.TracerProviderOption, 0, len(cfg.TraceProviders)+2)
	for idx, prov := range cfg.TraceProviders {
		pt, ok := te[prov]
		if !ok {
			return nil, fmt.Errorf("not found exporter %s for provider %d for tracing", prov, idx)
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(pt.SpanExporter()))
	}

	var tracerProvider trace.TracerProvider = nooptrace.NewTracerProvider()
	var sdkTracerProvider *sdktrace.TracerProvider
	if len(traceOpts) > 0 {
		samplerOpt := sdktrace.WithSampler(sdktrace.AlwaysSample())
		if cfg.TraceSampleRate > 0.0 && cfg.TraceSampleRate < 1.0 {
			samplerOpt = sdktrace.WithSampler(sdktrace.ParentBased(
				sdktrace.TraceIDRatioBased(cfg.TraceSampleRate)))
		}
		traceOpts = append(traceOpts, samplerOpt)
		traceOpts = append(traceOpts, sdktrace.WithResource(res))
		sdkTracerProvider = sdktrace.NewTracerProvider(traceOpts...)
		tracerProvider = sdkTracerProvider
	}
	tracer := tracerProvider.Tracer(providerName)

	return &OTELState{
		meterProvider:     meterProvider,
		tracerProvider:    tracerProvider,
		sdkMeterProvider:  sdkMeterProvider,
		sdkTracerProvider: sdkTracerProvider,
		tracer:            tracer,
		meter:             meter,
	}, nil
}

// Tracer returns a tracer to start a span.
func (s *OTELState) Tracer() trace.Tracer {
	return s.tracer
}

// Meter returns a meter to create metric instruments.
func (s *OTELState) Meter() metric.Meter {
	return s.meter
}

func (s *OTELState) MeterProvider() metric.MeterProvider {
	return s.meterProvider
}

func (s *OTELState) TracerProvider() trace.TracerProvider {
	return s.tracerProvider
}

// Propagator returns the configured propagator to use.
func (s *OTELState) Propagator() propagation.TextMapPropagator {
	if s == nil {
		return nil
	}
	return otel.GetTextMapPropagator()
}

// Shutdown performs the clean shutdown to be able to
// flush pending traces and / or metrics.
func (s *OTELState) Shutdown(ctx context.Context) {
	if s.sdkTracerProvider != nil {
		s.sdkTracerProvider.Shutdown(ctx)
	}
	if s.sdkMeterProvider != nil {
		s.sdkMeterProvider.Shutdown(ctx)
	}
}
