package config

import (
	"bytes"
	"encoding/json"
	"errors"

	luraconfig "github.com/luraproject/lura/v2/config"
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
func FromLura(srvCfg luraconfig.ServiceConfig) (*ConfigData, error) {
	cfg, err := LuraExtraCfg(srvCfg.ExtraConfig)
	if err != nil {
		return nil, err
	}

	if cfg.ServiceName == "" {
		if srvCfg.Name != "" {
			cfg.ServiceName = srvCfg.Name
		} else {
			cfg.ServiceName = "KrakenD"
		}
	}

	cfg.UnsetFieldsToDefaults()
	return cfg, nil
}

// LuraExtraCfg extracts the extra config field for the namespace if
// provided
func LuraExtraCfg(extraCfg luraconfig.ExtraConfig) (*ConfigData, error) {
	tmp, ok := extraCfg[Namespace]
	if !ok {
		return nil, ErrNoConfig
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(tmp); err != nil {
		return nil, err
	}

	cfg := new(ConfigData)
	if err := json.NewDecoder(buf).Decode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func LuraLayerExtraCfg(extraCfg luraconfig.ExtraConfig) (*LayersOpts, error) {
	cfg, err := LuraExtraCfg(extraCfg)

	if err != nil {
		return nil, err
	}

	return cfg.Layers, nil
}
