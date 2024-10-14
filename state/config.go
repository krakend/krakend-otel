package state

import (
	"github.com/krakend/krakend-otel/config"
	luraconfig "github.com/luraproject/lura/v2/config"
)

type Config interface {
	OTEL() OTEL
	// GlobalOpts gets the configuration at the service level.
	GlobalOpts() *config.GlobalOpts

	// Gets the OTEL instance for a given endpoint
	EndpointOTEL(cfg *luraconfig.EndpointConfig) OTEL
	// EndpointPipeOpts retrieve "proxy" level configuration for a given
	// endpoint.
	EndpointPipeOpts(cfg *luraconfig.EndpointConfig) *config.PipeOpts
	// EndpointBackendOpts should return a config for all the child
	// backend of this endpoint.
	//
	// Deprecated: the interface should only need to fetch the BackendOpts
	// from a luraconfig.Backend when configuring at the Backend level:
	// the BackendOpts function must be used instead.
	EndpointBackendOpts(cfg *luraconfig.Backend) *config.BackendOpts

	BackendOTEL(cfg *luraconfig.Backend) OTEL
	BackendOpts(cfg *luraconfig.Backend) *config.BackendOpts

	// SkipEndpoint tells if an endpoint should not be instrumented
	SkipEndpoint(endpoint string) bool
}

var _ Config = (*StateConfig)(nil)

type StateConfig struct {
	cfgData config.ConfigData
}

func (*StateConfig) OTEL() OTEL {
	return GlobalState()
}

func (s *StateConfig) GlobalOpts() *config.GlobalOpts {
	return s.cfgData.Layers.Global
}

func (*StateConfig) EndpointOTEL(_ *luraconfig.EndpointConfig) OTEL {
	return GlobalState()
}

// EndpointPipeOpts checks if there is an override for pipe ("proxy")
// options at the endpoint levels a fully replaces (it DOES NOT MERGE
// attributes) the existing config from the service level configuration.
// If none of those configs are found, it falls back to the defaults.
func (s *StateConfig) EndpointPipeOpts(cfg *luraconfig.EndpointConfig) *config.PipeOpts {
	var opts *config.PipeOpts
	if s != nil && s.cfgData.Layers != nil {
		opts = s.cfgData.Layers.Pipe
	}

	cfgLayer, err := config.LuraLayerExtraCfg(cfg.ExtraConfig)
	if err == nil && cfgLayer != nil {
		opts = cfgLayer.Pipe
	}

	if opts == nil {
		return new(config.PipeOpts)
	}
	return opts
}

// EndpointBackendOpts is a bad interface function, as is should receive
// as a param a luraconfig.Endpoint .. but also makes no sense to have it
// because we only need the backend configuration at
func (s *StateConfig) EndpointBackendOpts(cfg *luraconfig.Backend) *config.BackendOpts {
	return s.mergedBackendOpts(cfg)
}

func (*StateConfig) BackendOTEL(_ *luraconfig.Backend) OTEL {
	return GlobalState()
}

func (s *StateConfig) BackendOpts(cfg *luraconfig.Backend) *config.BackendOpts {
	return s.mergedBackendOpts(cfg)
}

func (s *StateConfig) mergedBackendOpts(cfg *luraconfig.Backend) *config.BackendOpts {
	var opts *config.BackendOpts
	if s != nil && s.cfgData.Layers != nil {
		opts = s.cfgData.Layers.Backend
	}

	cfgLayer, err := config.LuraLayerExtraCfg(cfg.ExtraConfig)
	if err == nil && cfgLayer != nil {
		opts = cfgLayer.Backend
	}

	if opts == nil {
		return new(config.BackendOpts)
	}
	return opts
}

func (s *StateConfig) SkipEndpoint(endpoint string) bool {
	for _, toSkip := range s.cfgData.SkipPaths {
		if toSkip == endpoint {
			return true
		}
	}
	return false
}

func NewConfig(cfgData *config.ConfigData) *StateConfig {
	s := &StateConfig{
		cfgData: *cfgData,
	}
	s.cfgData.UnsetFieldsToDefaults()
	return s
}
