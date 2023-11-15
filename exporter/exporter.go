// Package exporter defines the interfaces required to implement in
// order to add additional exporters.
package exporter

import (
	"context"
	"fmt"
	"sync"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/exporter/otelcollector"
	"github.com/krakend/krakend-otel/exporter/prometheus"
)

// MetricReader is the interface required in order to
// export metrics.
type MetricReader interface {
	MetricReader() sdkmetric.Reader
}

// SpanExporter is the interface required in order to
// export traces.
type SpanExporter interface {
	SpanExporter() sdktrace.SpanExporter
}

// ExporterFactory is the function type to obtain exporters, that can
// implement [MetricReader], [SpanExporter] or both.
type ExporterFactory func(context.Context, map[string]interface{}) (interface{}, error)

var (
	exporterFactories map[string]ExporterFactory

	metricsInstances map[string]MetricReader
	tracesInstances  map[string]SpanExporter

	mu           = new(sync.RWMutex)
	registerOnce = new(sync.Once)
)

// RegisterExporterFactory adds an ExporterFactory associated
// with a name, to the global exporterFactories varialbe.
func RegisterExporterFactory(name string, ef ExporterFactory) {
	mu.Lock()
	exporterFactories[name] = ef
	mu.Unlock()
}

// RegisterKnownFactories registers all known exporter factories
func RegisterKnownFactories() {
	registerOnce.Do(func() {
		mu.Lock()
		exporterFactories = map[string]ExporterFactory{
			prometheus.ExporterKey:    prometheus.Exporter,
			otelcollector.ExporterKey: otelcollector.Exporter,
		}
		metricsInstances = make(map[string]MetricReader)
		tracesInstances = make(map[string]SpanExporter)
		mu.Unlock()
	})
}

// Instances create instances for a given configuration.
func Instances(cfg *config.Config) (map[string]MetricReader, map[string]SpanExporter, []error) {
	m := make(map[string]MetricReader)
	s := make(map[string]SpanExporter)
	var errList []error

	ctx := context.Background()
	mu.RLock()

	for name, ecfg := range cfg.Exporters {
		f, ok := exporterFactories[ecfg.Kind]
		if !ok {
			err := fmt.Errorf("Exporter of kind: %s not found", ecfg.Kind)
			errList = append(errList, err)
			continue
		}
		i, err := f(ctx, ecfg.Config)
		if err != nil {
			errList = append(errList, err)
			continue
		}
		if i == nil {
			err := fmt.Errorf("Implementation of kind: %s creates nil instance", ecfg.Kind)
			errList = append(errList, err)
		}
		if ss, ok := i.(SpanExporter); ok && ss != nil {
			s[name] = ss
		} else if mm, ok := i.(MetricReader); ok {
			m[name] = mm
		} else {
			errList = append(errList, fmt.Errorf("Kind %s is not a exporter", ecfg.Kind))
		}
	}
	mu.RUnlock()
	return m, s, errList
}

// SetupGlobalExporterInstances creates the exporter instances
// according to the provided configuration, and sets those
// as the global defaults.
func SetupGlobalExporterInstances(cfg *config.Config) []error {
	m, s, errs := Instances(cfg)
	if len(errs) > 0 {
		return errs
	}
	SetGlobalExporterInstances(m, s)
	return nil
}

// SetGlobalExporterInstances sets the provided metric and traces
// as global defaults.
func SetGlobalExporterInstances(m map[string]MetricReader, t map[string]SpanExporter) {
	mu.Lock()
	metricsInstances = make(map[string]MetricReader, len(m))
	tracesInstances = make(map[string]SpanExporter, len(t))
	for k, v := range m {
		metricsInstances[k] = v
	}
	for k, v := range t {
		tracesInstances[k] = v
	}
	mu.Unlock()
}

// GetGlobalExporterInstances gets the global metrics and traces exporters
func GetGlobalExporterInstances() (map[string]MetricReader, map[string]SpanExporter) {
	mu.RLock()
	m := make(map[string]MetricReader, len(metricsInstances))
	t := make(map[string]SpanExporter, len(tracesInstances))
	for k, v := range metricsInstances {
		m[k] = v
	}
	for k, v := range t {
		t[k] = v
	}
	mu.RUnlock()

	return m, t
}

// GlobalTraceInstance gets a global trace exporter by name
func GlobalTraceInstance(name string) (SpanExporter, error) {
	mu.RLock()
	i, ok := tracesInstances[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return i, nil
}

// GlobalMetricInstance get a global metrics exporter by name
func GlobalMetricInstance(name string) (MetricReader, error) {
	mu.RLock()
	i, ok := metricsInstances[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return i, nil
}
