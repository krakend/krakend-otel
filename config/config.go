// Package config defines the configuration to be used to setup
// the metrics and traces for each stage of a KrakenD instances
// as well as the level of detail we want for each stage.
package config

import (
	"fmt"
)

// ConfigData is the root configuration for the OTEL observability stack
type ConfigData struct {
	ServiceName           string      `json:"service_name"`
	ServiceVersion        string      `json:"service_version"`
	Layers                *LayersOpts `json:"layers"`
	Exporters             Exporters   `json:"exporters"`
	SkipPaths             []string    `json:"skip_paths"`
	MetricReportingPeriod *int        `json:"metric_reporting_period"`
	TraceSampleRate       *float64    `json:"trace_sample_rate"`
}

func (c *ConfigData) Validate() error {
	return c.Exporters.Validate()
}

func (c *ConfigData) UnsetFieldsToDefaults() {
	if c.MetricReportingPeriod == nil {
		reportingPeriod := 30
		c.MetricReportingPeriod = &reportingPeriod
	}
	if c.TraceSampleRate == nil {
		sampleRate := float64(1.0)
		c.TraceSampleRate = &sampleRate
	}

	if c.Layers == nil {
		c.Layers = &LayersOpts{}
	}

	if c.Layers.Global == nil {
		c.Layers.Global = &GlobalOpts{
			DisableMetrics:     false,
			DisableTraces:      false,
			DisablePropagation: false,
		}
	}

	if c.Layers.Pipe == nil {
		c.Layers.Pipe = &PipeOpts{
			DisableMetrics: false,
			DisableTraces:  false,
		}
	}

	if c.Layers.Backend == nil {
		c.Layers.Backend = &BackendOpts{}
	}

	if c.Layers.Backend.Metrics == nil {
		c.Layers.Backend.Metrics = &BackendMetricOpts{
			DisableStage:       false,
			RoundTrip:          true,
			ReadPayload:        true,
			DetailedConnection: true,
		}
	}

	if c.Layers.Backend.Traces == nil {
		c.Layers.Backend.Traces = &BackendTraceOpts{
			DisableStage:       false,
			RoundTrip:          true,
			ReadPayload:        true,
			DetailedConnection: true,
		}
	}

	if len(c.SkipPaths) == 0 {
		// if there are no defined skip paths, we use the default ones:
		// to avoid using defaultSkipPaths, provide a list with an empty string
		c.SkipPaths = []string{
			"/__health",
			"/__debug/",
			"/__echo/",
			"/__stats/",
		}
	}
}

type Exporters struct {
	OTLP       []OTLPExporter       `json:"otlp"`
	Prometheus []PrometheusExporter `json:"prometheus"`
}

func (e *Exporters) Validate() error {
	uniqueNames := make(map[string]bool, len(e.OTLP)+len(e.Prometheus))
	for idx, ecfg := range e.OTLP {
		if uniqueNames[ecfg.Name] {
			return fmt.Errorf("OTLP exporter with duplicate name: %s (at idx %d)", ecfg.Name, idx)
		}
		uniqueNames[ecfg.Name] = true
	}
	for idx, ecfg := range e.Prometheus {
		if uniqueNames[ecfg.Name] {
			return fmt.Errorf("prometheus with duplicate name: %s (at idx %d)", ecfg.Name, idx)
		}
		uniqueNames[ecfg.Name] = true
	}
	return nil
}

type OTLPExporter struct {
	Name                        string `json:"name"`
	Host                        string `json:"host"`
	Port                        int    `json:"port"`
	UseHTTP                     bool   `json:"use_http"`
	DisableMetrics              bool   `json:"disable_metrics"`
	DisableTraces               bool   `json:"disable_traces"`
	CustomMetricReportingPeriod uint   `json:"custom_reporting_period"`
}

type PrometheusExporter struct {
	Name           string `json:"name"`
	Port           int    `json:"port"`
	Host           string `json:"host"`
	ProcessMetrics bool   `json:"process_metrics"`
	GoMetrics      bool   `json:"go_metrics"`
	DisableMetrics bool   `json:"disable_metrics"`
}

// LayersOpts contains the level of telemetry detail
// that we want for each KrakenD stage
type LayersOpts struct {
	Global  *GlobalOpts  `json:"global"`
	Pipe    *PipeOpts    `json:"proxy"`
	Backend *BackendOpts `json:"backend"`
}

// GlobalOpts has the options for the KrakenD
// http handler stage.
// We can select if we want to disable the metrics,
// the traces, and / or the trace propagation.
type GlobalOpts struct {
	DisableMetrics          bool       `json:"disable_metrics"`
	DisableTraces           bool       `json:"disable_traces"`
	DisablePropagation      bool       `json:"disable_propagation"`
	ReportHeaders           bool       `json:"report_headers"`
	MetricsStaticAttributes Attributes `json:"metrics_static_attributes"`
	TracesStaticAttributes  Attributes `json:"traces_static_attributes"`
	SemConv                 string     `json:"semantic_convention"`
}

// PipeOpts has the options for the KrakenD pipe stage
// to disable metrics and traces.
type PipeOpts struct {
	DisableMetrics          bool       `json:"disable_metrics"`
	DisableTraces           bool       `json:"disable_traces"`
	ReportHeaders           bool       `json:"report_headers"`
	MetricsStaticAttributes Attributes `json:"metrics_static_attributes"`
	TracesStaticAttributes  Attributes `json:"traces_static_attributes"`
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
	Metrics *BackendMetricOpts `json:"metrics"`
	Traces  *BackendTraceOpts  `json:"traces"`
}

// Enabled returns if either metrics or traces enabled
// for the backend stage.
func (o *BackendOpts) Enabled() bool {
	if o == nil {
		return false
	}
	return o.Metrics.Enabled() || o.Traces.Enabled()
}

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Attributes []KeyValue

func (a Attributes) ToMap() (map[string]string, error) {
	var err error
	m := make(map[string]string, len(a))
	for _, attr := range a {
		if _, ok := m[attr.Key]; ok {
			err = fmt.Errorf("duplicate attribute keys")
		}
		m[attr.Key] = attr.Value
	}
	return m, err
}

// BackendMetricOpts provides the options for the metrics
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
	DisableStage       bool       `json:"disable_stage"`
	RoundTrip          bool       `json:"round_trip"`
	ReadPayload        bool       `json:"read_payload"`
	DetailedConnection bool       `json:"detailed_connection"`
	StaticAttributes   Attributes `json:"static_attributes"`
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
	DisableStage       bool       `json:"disable_stage"`
	RoundTrip          bool       `json:"round_trip"`
	ReadPayload        bool       `json:"read_payload"`
	DetailedConnection bool       `json:"detailed_connection"`
	StaticAttributes   Attributes `json:"static_attributes"`
	ReportHeaders      bool       `json:"report_headers"`
}

// Enabled tells if there are any traces to be reported.
func (o *BackendTraceOpts) Enabled() bool {
	if o == nil {
		return false
	}
	return !o.DisableStage || o.RoundTrip || o.ReadPayload || o.DetailedConnection
}
