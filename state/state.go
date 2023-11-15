// Package state provides the functionality to "pack" into a single
// structure a set of configured instances (exporters, meters, tracers...)
package state

import (
	"context"
	"fmt"
	"sync"
	// "time"

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

	"github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/exporter"
)

// OTEL defines the interface to obtain observability
// instruments for a state.
type OTEL interface {
	Tracer() trace.Tracer
	MeterProvider() metric.MeterProvider
	Propagator() propagation.TextMapPropagator
	Shutdown(ctx context.Context)
}

// GetterFn defines a function that will return an [OTEL] instance.
type GetterFn func() OTEL

// OTELState is the basic implementation of an [OTEL] intstance.
type OTELState struct {
	mu sync.RWMutex

	meterProvider  metric.MeterProvider
	tracerProvider trace.TracerProvider

	// we need not the interface, but the actual implementation
	// to be able to call shutown:
	sdkTracerProvider *sdktrace.TracerProvider

	tracer trace.Tracer
}

// New create a new OTELState based on the provided config, and the
// globally set up exporters.
func New(cfg config.Config) (*OTELState, error) {
	m, t := exporter.GetGlobalExporterInstances()
	return NewWithVersion(cfg, "undefined", m, t)
}

// NewWithVersion create a new OTELState with a version for
// the KrakenD service, with the provided metrics and traces exporters
func NewWithVersion(cfg config.Config, version string,
	me map[string]exporter.MetricReader, te map[string]exporter.SpanExporter) (*OTELState, error) {

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "KrakenD"
	}
	res := sdkresource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(version))

	// Configure the metrics parter
	// TODO:
	// reportingPeriod := time.Duration(cfg.MetricReportingPeriod) * time.Second
	metricOpts := make([]sdkmetric.Option, 0, len(cfg.MetricProviders)+2)
	for idx, prov := range cfg.MetricProviders {
		pm, ok := me[prov]
		if !ok {
			return nil, fmt.Errorf("not found exporter %s for metricprovider %d  (me %#v)", prov, idx, me)
		}
		// TODO: how do we pass the reporting period to the implementation ?
		/*
			metricOpts = append(metricOpts, sdkmetric.WithReader(
				sdkmetric.NewPeriodicReader(pm.MetricReader(),
					sdkmetric.WithInterval(reportingPeriod))))
		*/
		metricOpts = append(metricOpts, sdkmetric.WithReader(pm.MetricReader()))
	}

	var meterProvider metric.MeterProvider = noopmetric.NewMeterProvider()
	if len(metricOpts) > 0 {
		meterProvider = sdkmetric.NewMeterProvider(metricOpts...)
	}

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

	tracer := tracerProvider.Tracer("krakend")
	return &OTELState{
		meterProvider:     meterProvider,
		tracerProvider:    tracerProvider,
		sdkTracerProvider: sdkTracerProvider,
		tracer:            tracer,
	}, nil
}

// Tracer returns a tracer to start a span.
func (s *OTELState) Tracer() trace.Tracer {
	if s == nil || s.tracer == nil {
		p := nooptrace.NewTracerProvider()
		return p.Tracer("io.krakend.krakend-otel")
	}
	return s.tracer
}

// MeterProvider returns a MeterProvider from where we can
// create meters.
func (s *OTELState) MeterProvider() metric.MeterProvider {
	if s == nil || s.meterProvider == nil {
		return noopmetric.NewMeterProvider()
	}
	return s.meterProvider
}

// Propagator returns the configured propagator to use.
func (s *OTELState) Propagator() propagation.TextMapPropagator {
	// TODO: provide a way to configure the propagator
	return otel.GetTextMapPropagator()
}

// Shutdown performs the clean shutdown to be able to
// flush pending traces and / or metrics.
func (s *OTELState) Shutdown(ctx context.Context) {
	if s.sdkTracerProvider != nil {
		s.sdkTracerProvider.Shutdown(ctx)
	}
}
