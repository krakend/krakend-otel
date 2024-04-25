// Package kotel adds opentelemetry instrumentation to a KrakenD instance
// (or some other [Lura](https://github.com/luraproject/lura) based softawre)
//
// In the KrakenD project, we can differentiate 3 main stages in the process
// of handling a request:
//   - the "router" stage: the part where the router plugins are run, and
//     is the part from receiving the request, up to the point where the request
//     enters the Lura's pipeline.
//   - the "proxy" stage: is for the processing endpoint part, up to the point
//   - the "backend" stage: is the part for each one of the backends that will
//     be used for a given endpoint.
package kotel

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	lconfig "github.com/luraproject/lura/v2/config"
	lcore "github.com/luraproject/lura/v2/core"
	"github.com/luraproject/lura/v2/logging"

	"github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/exporter"
	"github.com/krakend/krakend-otel/state"
)

// Register uses the ServiceConfig to instantiate the configured exporters.
// It also sets the global exporter instances, the global propagation method, and
// the global KrakenD otel state, so it can be used from anywhere.
func Register(ctx context.Context, l logging.Logger, srvCfg lconfig.ServiceConfig) (func(), error) {
	cfg, err := config.FromLura(srvCfg)
	if err != nil {
		if errors.Is(err, config.ErrNoConfig) {
			return func() {}, nil
		}
		// we do not log, we left it to the parent:
		return func() {}, err
	}
	return RegisterWithConfig(ctx, l, cfg)
}

// RegisterWithConfig instantiates the configured exporters from an already
// parsed config: sets the global exporter instances, the global propagation method, and
// the global KrakenD otel state, so it can be used from anywhere.
func RegisterWithConfig(ctx context.Context, l logging.Logger, cfg *config.ConfigData) (func(), error) {
	shutdownFn := func() {}
	if err := cfg.Validate(); err != nil {
		return shutdownFn, err
	}

	me, te, err := exporter.Instances(ctx, cfg)
	if err != nil {
		return shutdownFn, err
	}
	exporter.SetGlobalExporterInstances(me, te)
	shutdown, err := RegisterGlobalInstance(ctx, l, me, te, *cfg.MetricReportingPeriod, *cfg.TraceSampleRate, cfg.ServiceName, cfg.ServiceVersion)
	if err == nil {
		state.SetGlobalConfig(state.NewConfig(cfg))
	}
	return shutdown, err
}

// RegisterGlobalInstance creates the instance that will be used to report metrics and traces
func RegisterGlobalInstance(ctx context.Context, l logging.Logger,
	me map[string]exporter.MetricReader, te map[string]exporter.SpanExporter,
	metricReportingPeriod int, traceSampleRate float64, serviceName string, serviceVersion string,
) (func(), error) {
	shutdownFn := func() {}
	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(prop)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(e error) {
		// TODO: we might want to "throtle" the error reporting
		// when we have repeated messagese when a OTLP backend is
		// down.
		l.Error("[SERVICE: OpenTelemetry] " + e.Error())
	}))

	globalStateCfg := &state.OTELStateConfig{
		MetricReportingPeriod: metricReportingPeriod,
		TraceSampleRate:       traceSampleRate,
		MetricProviders:       make([]string, 0, len(me)),
		TraceProviders:        make([]string, 0, len(te)),
	}
	for k, v := range me {
		if v.MetricDefaultReporting() {
			globalStateCfg.MetricProviders = append(globalStateCfg.MetricProviders, k)
		}
	}
	for k, v := range te {
		if v.TraceDefaultReporting() {
			globalStateCfg.TraceProviders = append(globalStateCfg.TraceProviders, k)
		}
	}

	version := serviceVersion
	if version == "" {
		version = lcore.KrakendVersion
	}

	s, err := state.NewWithVersion(serviceName, globalStateCfg, version, me, te)
	if err != nil {
		return shutdownFn, err
	}
	shutdownFn = func() { s.Shutdown(ctx) }
	state.SetGlobalState(s)
	return shutdownFn, nil
}
