package state

import (
	luraconfig "github.com/luraproject/lura/v2/config"

	"github.com/krakend/krakend-otel/config"
)

type Config interface {
	OTEL() OTEL
	GlobalOpts() *config.GlobalOpts

	// Gets the OTEL instance for a given endpoint
	EndpointOTEL(cfg *luraconfig.EndpointConfig) OTEL
	EndpointPipeOpts(cfg *luraconfig.EndpointConfig) *config.PipeOpts
	EndpointBackendOpts(cfg *luraconfig.Backend) *config.BackendOpts

	BackendOTEL(cfg *luraconfig.Backend) OTEL
	BackendOpts(cfg *luraconfig.Backend) *config.BackendOpts

	// SkipEndpoint tells if an endpoint should not be instrumented
	SkipEndpoint(endpoint string) bool
}

type StateConfig struct {
	cfgData config.ConfigData
}

func (s *StateConfig) OTEL() OTEL {
	return GlobalState()
}

func (s *StateConfig) GlobalOpts() *config.GlobalOpts {
	return s.cfgData.Layers.Global
}

func (s *StateConfig) EndpointOTEL(cfg *luraconfig.EndpointConfig) OTEL {
	return GlobalState()
}

func (s *StateConfig) EndpointPipeOpts(cfg *luraconfig.EndpointConfig) *config.PipeOpts {
	return s.cfgData.Layers.Pipe
}

func (s *StateConfig) EndpointBackendOpts(cfg *luraconfig.Backend) *config.BackendOpts {
	return s.cfgData.Layers.Backend
}

func (s *StateConfig) BackendOTEL(cfg *luraconfig.Backend) OTEL {
	return GlobalState()
}

func (s *StateConfig) BackendOpts(cfg *luraconfig.Backend) *config.BackendOpts {
	return s.cfgData.Layers.Backend
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
