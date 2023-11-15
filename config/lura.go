package config

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/luraproject/lura/v2/config"
)

const (
	// Namespace is the key under the Lura's "extra_config" root
	// section, for a valid config. See [config] documentation for
	// details.
	Namespace = "telemetry/opentelemetry"
)

var (
	// ErrNoConfig is used to signal no config was found
	ErrNoConfig = errors.New("No config found for opentelemetry")
)

// FromLura extracts the configuration from the Lura's ServiceConfig
// "extra_config" field.
//
// In case no "Layers" config is provided, a set of defaults with
// everything enabled will be used.
func FromLura(srvCfg config.ServiceConfig) (*Config, error) {
	cfg := new(Config)
	tmp, ok := srvCfg.ExtraConfig[Namespace]
	if !ok {
		return nil, ErrNoConfig
	}
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(tmp)
	if err := json.NewDecoder(buf).Decode(cfg); err != nil {
		return nil, err
	}

	if cfg.Layers == nil {
		cfg.Layers = &LayersOpts{
			Router: &RouterOpts{
				Metrics:            true,
				Traces:             true,
				DisablePropagation: false,
			},
			Pipe: &PipeOpts{
				Metrics: true,
				Traces:  true,
			},
			Backend: &BackendOpts{
				Metrics: &BackendMetricOpts{
					Stage:              true,
					RoundTrip:          true,
					ReadPayload:        true,
					DetailedConnection: true,
				},
				Traces: &BackendTraceOpts{
					Stage:              true,
					RoundTrip:          true,
					ReadPayload:        true,
					DetailedConnection: true,
				},
			},
		}
	}

	cfg.ServiceName = srvCfg.Name
	return cfg, nil
}
