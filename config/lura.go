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

// ErrNoConfig is used to signal no config was found
var ErrNoConfig = errors.New("no config found for opentelemetry")

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
		cfg.Layers = &LayersOpts{}
	}

	if cfg.Layers.Global == nil {
		cfg.Layers.Global = &GlobalOpts{
			DisableMetrics:     false,
			DisableTraces:      false,
			DisablePropagation: false,
		}
	}

	if cfg.Layers.Pipe == nil {
		cfg.Layers.Pipe = &PipeOpts{
			DisableMetrics: false,
			DisableTraces:  false,
		}
	}

	if cfg.Layers.Backend == nil {
		cfg.Layers.Backend = &BackendOpts{}
	}

	if cfg.Layers.Backend.Metrics == nil {
		cfg.Layers.Backend.Metrics = &BackendMetricOpts{
			DisableStage:       false,
			RoundTrip:          true,
			ReadPayload:        true,
			DetailedConnection: true,
		}
	}

	if cfg.Layers.Backend.Traces == nil {
		cfg.Layers.Backend.Traces = &BackendTraceOpts{
			DisableStage:       false,
			RoundTrip:          true,
			ReadPayload:        true,
			DetailedConnection: true,
		}
	}

	if len(cfg.SkipPaths) == 0 {
		// if there are no defined skip paths, we use the default ones:
		// to avoid using defaultSkipPaths, provide a list with an empty string
		cfg.SkipPaths = []string{
			"/health",
			"/__debug/",
			"/__echo/",
			"/__stats/",
		}
	}

	if cfg.ServiceName == "" {
		if srvCfg.Name != "" {
			cfg.ServiceName = srvCfg.Name
		} else {
			cfg.ServiceName = "KrakenD"
		}
	}
	return cfg, nil
}
