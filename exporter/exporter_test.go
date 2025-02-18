package exporter

import (
	"context"
	"testing"

	"github.com/krakend/krakend-otel/config"
)

type otelExpectedExporter struct {
	metricReporting bool
	traceReporting  bool
}

func TestCreateOTLPExporters(t *testing.T) {
	testCases := []struct {
		name              string
		cfg               []config.OTLPExporter
		expectedExporters map[string]otelExpectedExporter
	}{
		{
			name: "minimal config",
			cfg: []config.OTLPExporter{
				{
					Name: "minimal",
				},
			},
			expectedExporters: map[string]otelExpectedExporter{
				"minimal": {
					metricReporting: true,
					traceReporting:  true,
				},
			},
		},
		{
			name: "using http",
			cfg: []config.OTLPExporter{
				{
					Name:    "http",
					UseHTTP: true,
				},
			},
			expectedExporters: map[string]otelExpectedExporter{
				"http": {
					metricReporting: true,
					traceReporting:  true,
				},
			},
		},
		{
			name: "disable reporting",
			cfg: []config.OTLPExporter{
				{
					Name:           "disable_reporting",
					DisableMetrics: true,
					DisableTraces:  true,
				},
			},
			expectedExporters: map[string]otelExpectedExporter{
				"disable_reporting": {
					metricReporting: false,
					traceReporting:  false,
				},
			},
		},
		{
			name: "multiple exporters",
			cfg: []config.OTLPExporter{
				{
					Name:    "http_exporter",
					Host:    "localhost",
					Port:    1234,
					UseHTTP: true,
				},
				{
					Name: "normal_exporter",
					Host: "somewhere",
					Port: 5467,
				},
			},
			expectedExporters: map[string]otelExpectedExporter{
				"http_exporter": {
					metricReporting: true,
					traceReporting:  true,
				},
				"normal_exporter": {
					metricReporting: true,
					traceReporting:  true,
				},
			},
		},
		{
			name: "with IP address",
			cfg: []config.OTLPExporter{
				{
					Name: "ip_address",
					Host: "1.2.3.4",
				},
			},
			expectedExporters: map[string]otelExpectedExporter{
				"ip_address": {
					metricReporting: true,
					traceReporting:  true,
				},
			},
		},
		{
			name: "insecure",
			cfg: []config.OTLPExporter{
				{
					Name: "insecure1",
					Host: "http://1.2.3.4",
				},
				{
					Name: "insecure2",
					Host: "http://1.2.3.4",
					Port: 1234,
				},
				{
					Name:    "insecure3",
					Host:    "http://1.2.3.4",
					Port:    1234,
					UseHTTP: true,
				},
			},
			expectedExporters: map[string]otelExpectedExporter{
				"insecure1": {
					metricReporting: true,
					traceReporting:  true,
				},
				"insecure2": {
					metricReporting: true,
					traceReporting:  true,
				},
				"insecure3": {
					metricReporting: true,
					traceReporting:  true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m, s, err := CreateOTLPExporters(context.Background(), tc.cfg)
			if err != nil {
				t.Errorf("unexpected error creating the exporters: %s", err.Error())
			}
			if len(m) != len(tc.expectedExporters) {
				t.Errorf("unexpected number of metric readers: %d", len(m))
			}
			if len(s) != len(tc.expectedExporters) {
				t.Errorf("unexpected number of span exporters: %d", len(s))
			}

			for name, expected := range tc.expectedExporters {
				if _, ok := m[name]; !ok {
					t.Errorf("missing metric reader for %s", name)
				}
				if _, ok := s[name]; !ok {
					t.Errorf("missing span exporter for %s", name)
				}

				if m[name].MetricDefaultReporting() != expected.metricReporting {
					t.Errorf("unexpected metric reporting for %s", name)
				}
				if s[name].TraceDefaultReporting() != expected.traceReporting {
					t.Errorf("unexpected trace reporting for %s", name)
				}
			}
		})
	}
}
