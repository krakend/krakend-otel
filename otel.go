// Package kotel adds opentelemetry instrumentation to a KrakenD instance
// (or some other [Lura](https://github.com/luraproject/lura) based softawre)
//
// In the KrakenD project, we can differentiate 3 main stages in the process
// of handling a request:
//   - the "router" stage: the part where the router plugins are run, and
//     is the part from receiving the request, up to the point where the request
//     enters the Lura's pipeline.
//   - the "pipe" stage: is for the processing endpoint part, up to the point
//   - the "backend" stage: is the part for each one of the backends that will
//     be used for a given endpoint.
package kotel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	lconfig "github.com/luraproject/lura/v2/config"
	lcore "github.com/luraproject/lura/v2/core"

	"github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/exporter"
	"github.com/krakend/krakend-otel/state"
)

var (
	otelState state.OTELState
)

// Register all the known exporter factories (opentelemetry, prometheus, etc..)
// and uses the proviced ServiceConfig to instantiate the configured exporters.
// It also sets the global exporter instances, the global propagation method, and
// the global KrakenD otel state, so it can be used from anywhere.
func Register(ctx context.Context, srvCfg lconfig.ServiceConfig) error {
	exporter.RegisterKnownFactories()

	cfg, err := config.FromLura(srvCfg)
	if err != nil {
		return err
	}

	if len(cfg.Exporters) == 0 {
		return fmt.Errorf("no exporters declared")
	}

	me, te, errs := exporter.Instances(cfg)
	if len(errs) > 0 {
		// we will report a single error each time (even when there
		// might be multiple errors in the exporters config).
		return errs[0]
	}
	exporter.SetGlobalExporterInstances(me, te)

	// TODO: make the propagator configurable inside the state
	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	if cfg.Instance == nil {
		// if we do not have a selection of exporters to use, we default
		// to report to all configured exporters.
		cfg.Instance = &config.Instance{
			MetricReportingPeriod: 30,
			MetricProviders:       make([]string, 0, len(me)),
			TraceSampleRate:       1.0,
			TraceProviders:        make([]string, 0, len(te)),
		}
		for mk, _ := range me {
			cfg.Instance.MetricProviders = append(cfg.Instance.MetricProviders, mk)
		}
		for tk, _ := range te {
			cfg.Instance.TraceProviders = append(cfg.Instance.TraceProviders, tk)
		}
	}

	s, err := state.NewWithVersion(cfg.ServiceName, *cfg.Instance, lcore.KrakendVersion, me, te)
	if err != nil {
		return err
	}
	otel.SetTextMapPropagator(prop)
	state.SetGlobalState(s)
	return nil
}
