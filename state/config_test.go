package state

import (
	"testing"

	"github.com/krakend/krakend-otel/config"
	luraconfig "github.com/luraproject/lura/v2/config"
)

func TestEndpointPipeConfigOverride(t *testing.T) {
	globalMetricAttrs := makeGlobalMetricAttr()
	overrideMetricAttrs := makeOverrideMetricAttr()
	expectedMetricAttrs := append(globalMetricAttrs, overrideMetricAttrs...) // skipcq: CRT-D0001

	globalTraceAttrs := makeGlobalTraceAttr()
	overrideTraceAttrs := makeOverrideTraceAttr()
	expectedTraceAttrs := append(globalTraceAttrs, overrideTraceAttrs...) // skipcq: CRT-D0001

	stateCfg := &StateConfig{
		cfgData: makePipeConf(globalMetricAttrs, globalTraceAttrs),
	}

	pipeCfg := &luraconfig.EndpointConfig{
		ExtraConfig: map[string]interface{}{
			"telemetry/opentelemetry": makePipeConf(overrideMetricAttrs, overrideTraceAttrs),
		},
	}

	pipeOpts := stateCfg.EndpointPipeOpts(pipeCfg)

	if len(pipeOpts.MetricsStaticAttributes) != len(expectedMetricAttrs) {
		t.Errorf(
			"Incorrect number of attributes for metrics. returned: %+v - expected: %+v",
			pipeOpts.MetricsStaticAttributes,
			expectedMetricAttrs,
		)
	}

	if len(pipeOpts.TracesStaticAttributes) != len(expectedTraceAttrs) {
		t.Errorf(
			"Incorrect number of attributes for traces. returned: %+v - expected: %+v",
			pipeOpts.TracesStaticAttributes,
			expectedTraceAttrs,
		)
	}
}

func TestEndpointPipeNoOverride(t *testing.T) {
	stateCfg := &StateConfig{
		cfgData: makePipeConf(makeGlobalMetricAttr(), makeGlobalTraceAttr()),
	}

	// Empty config
	pipeCfg := &luraconfig.EndpointConfig{
		ExtraConfig: map[string]interface{}{},
	}

	pipeOpts := stateCfg.EndpointPipeOpts(pipeCfg)

	if len(pipeOpts.MetricsStaticAttributes) > 1 {
		t.Errorf(
			"Incorrect number of attributes for metrics. returned: %+v",
			pipeOpts.MetricsStaticAttributes,
		)
	}
}

func TestEndpointPipeConfigOnlyOverride(t *testing.T) {
	stateCfg := &StateConfig{
		cfgData: makePipeConf([]config.KeyValue{}, []config.KeyValue{}),
	}

	pipeCfg := &luraconfig.EndpointConfig{
		ExtraConfig: map[string]interface{}{
			"telemetry/opentelemetry": makePipeConf(makeOverrideMetricAttr(), makeOverrideTraceAttr()),
		},
	}

	pipeOpts := stateCfg.EndpointPipeOpts(pipeCfg)

	if len(pipeOpts.MetricsStaticAttributes) > 1 {
		t.Errorf(
			"Incorrect number of attributes for metrics. returned: %+v",
			pipeOpts.MetricsStaticAttributes,
		)
	}
}

func TestBackendConfigOverride(t *testing.T) {
	globalMetricAttrs := makeGlobalMetricAttr()
	overrideMetricAttrs := makeOverrideMetricAttr()
	expectedMetricAttrs := append(globalMetricAttrs, overrideMetricAttrs...) // skipcq: CRT-D0001

	globalTraceAttrs := makeGlobalTraceAttr()
	overrideTraceAttrs := makeOverrideTraceAttr()
	expectedTraceAttrs := append(globalTraceAttrs, overrideTraceAttrs...) // skipcq: CRT-D0001

	stateCfg := &StateConfig{
		cfgData: makeBackendConf(globalMetricAttrs, globalTraceAttrs),
	}

	backendCfg := &luraconfig.Backend{
		ExtraConfig: map[string]interface{}{
			"telemetry/opentelemetry": makeBackendConf(overrideMetricAttrs, overrideTraceAttrs),
		},
	}

	backendOpts := stateCfg.BackendOpts(backendCfg)

	if len(backendOpts.Metrics.StaticAttributes) != len(expectedMetricAttrs) {
		t.Errorf(
			"Incorrect number of attributes for metrics. returned: %+v - expected: %+v",
			backendOpts.Metrics.StaticAttributes,
			expectedMetricAttrs,
		)
	}

	if len(backendOpts.Traces.StaticAttributes) != len(expectedTraceAttrs) {
		t.Errorf(
			"Incorrect number of attributes for traces. returned: %+v - expected: %+v",
			backendOpts.Traces.StaticAttributes,
			expectedTraceAttrs,
		)
	}
}

func TestBackendConfigNoOverride(t *testing.T) {
	stateCfg := &StateConfig{
		cfgData: makeBackendConf(makeGlobalMetricAttr(), makeGlobalTraceAttr()),
	}

	// Empty config
	backendCfg := &luraconfig.Backend{
		ExtraConfig: map[string]interface{}{},
	}

	backendOpts := stateCfg.BackendOpts(backendCfg)

	if len(backendOpts.Metrics.StaticAttributes) > 1 {
		t.Errorf(
			"Incorrect number of attributes for metrics. returned: %+v",
			backendOpts.Traces.StaticAttributes,
		)
	}
}

func TestBackendConfigOnlyOverride(t *testing.T) {
	stateCfg := &StateConfig{
		cfgData: makeBackendConf([]config.KeyValue{}, []config.KeyValue{}),
	}

	backendCfg := &luraconfig.Backend{
		ExtraConfig: map[string]interface{}{
			"telemetry/opentelemetry": makeBackendConf(makeOverrideMetricAttr(), makeOverrideTraceAttr()),
		},
	}

	backendOpts := stateCfg.BackendOpts(backendCfg)

	if len(backendOpts.Metrics.StaticAttributes) > 1 {
		t.Errorf(
			"Incorrect number of attributes for metrics. returned: %+v",
			backendOpts.Traces.StaticAttributes,
		)
	}
}

func makePipeConf(metricAttrs, traceAttrs []config.KeyValue) config.ConfigData {
	return config.ConfigData{
		Layers: &config.LayersOpts{
			Pipe: makePipeOpts(metricAttrs, traceAttrs),
		},
	}
}

func makePipeOpts(metricAttrs, traceAttrs []config.KeyValue) *config.PipeOpts {
	return &config.PipeOpts{
		MetricsStaticAttributes: metricAttrs,
		TracesStaticAttributes:  traceAttrs,
	}
}

func makeBackendConf(metricAttrs, traceAttrs []config.KeyValue) config.ConfigData {
	return config.ConfigData{
		Layers: &config.LayersOpts{
			Backend: makeBackendOpts(metricAttrs, traceAttrs),
		},
	}
}

func makeBackendOpts(metricAttrs, traceAttrs []config.KeyValue) *config.BackendOpts {
	return &config.BackendOpts{
		Metrics: &config.BackendMetricOpts{
			StaticAttributes: metricAttrs,
		},
		Traces: &config.BackendTraceOpts{
			StaticAttributes: traceAttrs,
		},
	}
}

func makeGlobalMetricAttr() []config.KeyValue {
	return makeStaticAttr("my_metric_key", "my_metric_value")
}

func makeOverrideMetricAttr() []config.KeyValue {
	return makeStaticAttr("my_metric_override_key", "my_metric_override_value")
}

func makeGlobalTraceAttr() []config.KeyValue {
	return makeStaticAttr("my_trace_key", "my_trace_value")
}

func makeOverrideTraceAttr() []config.KeyValue {
	return makeStaticAttr("my_trace_override_key", "my_trace_override_value")
}

func makeStaticAttr(key, value string) []config.KeyValue {
	return []config.KeyValue{
		config.KeyValue{
			Key:   key,
			Value: value,
		},
	}
}
