// Package config defines the configuration to be used to setup
// the metrics and traces for each stage of a KrakenD instances
// as well as the level of detail we want for each stage.
package config

// Config is the root configuration for the OTEL observability stack
type Config struct {
	ServiceName           string                 `json:"service_name"`
	Layers                *LayersOpts            `json:"layers"`
	Exporters             map[string]Exporter    `json:"exporters"`
	MetricProviders       []string               `json:"metric_providers"`
	MetricReportingPeriod int                    `json:"metric_reporting_period"`
	TraceProviders        []string               `json:"trace_providers"`
	TraceSampleRate       float64                `json:"trace_sample_rate"`
	Extra                 map[string]interface{} `json:"extra"`
}

// Exporter has the inforamtion to configure an exporter
// instance.
//
// The Kind is the name of the kind of exporter we want:
// OTEL, Prometheus, ...
//
// The Config is the configuration for this provider
type Exporter struct {
	Kind   string                 `json:"kind"`
	Config map[string]interface{} `json:"config"`
}

// LayersOpts contains the level of telemetry detail
// that we want for each KrakenD stage
type LayersOpts struct {
	Router  *RouterOpts  `json:"router"`
	Pipe    *PipeOpts    `json:"pipe"`
	Backend *BackendOpts `json:"backend"`
}

// RouterOpts has the options for the KrakenD
// router stage.
// We can select if we want to disable the metrics,
// the traces, and / or the trace propagation.
type RouterOpts struct {
	DisableMetrics     bool `json:"disable_metrics"`
	DisableTraces      bool `json:"disable_traces"`
	DisablePropagation bool `json:"disable_propagation"`
}

// PipeOpts has the options for the KrakenD pipe stage
// to disable metrics and traces.
type PipeOpts struct {
	DisableMetrics bool `json:"disable_metrics"`
	DisableTraces  bool `json:"disable_traces"`
}

// Enabled returns if either metrics or traces are enabled
// for the pipe stage.
func (o *PipeOpts) Enabled() bool {
	if o == nil {
		return false
	}
	return !o.DisableMetrics || !o.DisableTraces
}

// BackendOpts defines the instrumentation detail level for
// backend requests.
// SkipInstrumentationPaths allows us to provide a list of path
// that we do not want to have instrumentation for: those could
// be the __debug , __health, or __echo endpoint, for example.
type BackendOpts struct {
	Metrics   *BackendMetricOpts `json:"metrics"`
	Traces    *BackendTraceOpts  `json:"traces"`
	SkipPaths []string           `json:"skip_paths"`
}

// Enabled returns if either metrics or traces enabled
// for the backend stage.
func (o *BackendOpts) Enabled() bool {
	if o == nil {
		return false
	}
	return o.Metrics.Enabled() || o.Traces.Enabled()
}

// BackendMetricsOpts provides the options for the metrics
// to be reported at the backend level.
//
// DisableStage option means it will perevent to report metrics
// for ALLL the full backend part (request + manipulations at the backend
// level), so other fields will have no effect.
//
// RoundTrip options will report metrics on the actual request
// made for this backend: latency, body size, response code...
//
// ReadPayload will report the metrics about the reading the
// body content of the request (from first time to read, until
// all the body has been read). This last options gives extra
// fined grained times, that might not be always useful.
type BackendMetricOpts struct {
	DisableStage       bool              `json:"disable_stage"`
	RoundTrip          bool              `json:"round_trip"`
	ReadPayload        bool              `json:"read_payload"`
	DetailedConnection bool              `json:"detailed_connection"`
	StaticAttributes   map[string]string `json:"static_attributes"`
}

// Enabled tells if there are any metrics to be reported.
func (o *BackendMetricOpts) Enabled() bool {
	if o == nil {
		return false
	}
	return !o.DisableStage || o.RoundTrip || o.ReadPayload || o.DetailedConnection
}

// BackendTraceOpts provides the options for the tracing
// to be reported at the backend level.
//
// DisableStage means it will avoid creating a Span for ALL
// the full backend part (request + manipulations at the backend
// level), so other fields will have no effect.
//
// RoundTrip options will create an span for the actual request
// made for this backend.
//
// ReadPayload will create an additional span just for the reading
// the response body part.
type BackendTraceOpts struct {
	DisableStage       bool              `json:"disable_stage"`
	RoundTrip          bool              `json:"round_trip"`
	ReadPayload        bool              `json:"read_payload"`
	DetailedConnection bool              `json:"detailed_connection"`
	StaticAttributes   map[string]string `json:"static_attributes"`
}

// Enabled tells if there are any traces to be reported.
func (o *BackendTraceOpts) Enabled() bool {
	if o == nil {
		return false
	}
	return !o.DisableStage || o.RoundTrip || o.ReadPayload || o.DetailedConnection
}
