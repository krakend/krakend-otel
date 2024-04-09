package kotel

import (
	"context"
	"testing"

	lconfig "github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
)

func TestGlobalConfig(t *testing.T) {
	cfg := lconfig.ServiceConfig{
		ExtraConfig: map[string]interface{}{
			"telemetry/otel": map[string]interface{}{
				"sample_rate":      2,
				"reporting_period": 4,
				"enabled_layers": map[string]interface{}{
					"router":  true,
					"proxy":   true,
					"backend": true,
				},
				"exporters": map[string]interface{}{},
			},
		},
	}

	shuftdownFn, err := Register(context.Background(), logging.NoOp, cfg)
	if err != nil {
		t.Errorf("unexpected error %s", err.Error())
	}
	shuftdownFn()
}

/*
func TestGetAggregatedPathForMetrics(t *testing.T) {
	for i, tc := range []struct {
		cfg      *config.EndpointConfig
		expected string
	}{
		{
			cfg:      &config.EndpointConfig{Endpoint: "/api/:foo/:bar"},
			expected: "/api/{foo}/{bar}",
		},
		{
			cfg: &config.EndpointConfig{
				Endpoint: "/api/:foo/:bar",
				ExtraConfig: config.ExtraConfig{
					Namespace: map[string]interface{}{"path_aggregation": "pattern"},
				},
			},
			expected: "/api/{foo}/{bar}",
		},
		{
			cfg: &config.EndpointConfig{
				Endpoint: "/api/:foo/:bar",
				ExtraConfig: config.ExtraConfig{
					Namespace: map[string]interface{}{"path_aggregation": "lastparam"},
				},
			},
			expected: "/api/foo/{bar}",
		},
		{
			cfg: &config.EndpointConfig{
				Endpoint: "/api/:foo/:bar",
				ExtraConfig: config.ExtraConfig{
					Namespace: map[string]interface{}{"path_aggregation": "off"},
				},
			},
			expected: "/api/foo/bar",
		},
		{
			expected: "/api/foo/bar",
		},
	} {
		extractor := GetAggregatedPathForMetrics(tc.cfg)
		r, _ := http.NewRequest("GET", "http://example.tld/api/foo/bar", nil)
		if tag := extractor(r); tag != tc.expected {
			t.Errorf("tc-%d: unexpected result: %s", i, tag)
		}
	}
}

func TestGetAggregatedPathForBackendMetrics(t *testing.T) {
	for i, tc := range []struct {
		cfg      *config.Backend
		expected string
	}{
		{
			cfg:      &config.Backend{URLPattern: "/api/{{.Foo}}/{{.Bar}}"},
			expected: "/api/{foo}/{bar}",
		},
		{
			cfg: &config.Backend{
				URLPattern: "/api/{{.Foo}}/{{.Bar}}",
				ExtraConfig: config.ExtraConfig{
					Namespace: map[string]interface{}{"path_aggregation": "pattern"},
				},
			},
			expected: "/api/{foo}/{bar}",
		},
		{
			cfg: &config.Backend{
				URLPattern: "/api/{{.Foo}}/{{.Bar}}",
				ExtraConfig: config.ExtraConfig{
					Namespace: map[string]interface{}{"path_aggregation": "lastparam"},
				},
			},
			expected: "/api/foo/{bar}",
		},
		{
			cfg: &config.Backend{
				URLPattern: "/api/{{.Foo}}/{{.Bar}}",
				ExtraConfig: config.ExtraConfig{
					Namespace: map[string]interface{}{"path_aggregation": "off"},
				},
			},
			expected: "/api/foo/bar",
		},
		{
			expected: "/api/foo/bar",
		},
	} {
		extractor := GetAggregatedPathForBackendMetrics(tc.cfg)
		r, _ := http.NewRequest("GET", "http://example.tld/api/foo/bar", nil)
		if tag := extractor(r); tag != tc.expected {
			t.Errorf("tc-%d: unexpected result: %s", i, tag)
		}
	}
}

*/
