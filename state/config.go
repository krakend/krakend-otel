package state

import (
	"github.com/krakend/krakend-otel/config"
	luraconfig "github.com/luraproject/lura/v2/config"
)

type Config interface {
	OTEL() OTEL
	GlobalOpts() config.GlobalOpts

	// Gets the OTEL instance for a given endpoint
	EndpointOTEL(cfg *luraconfig.EndpointConfig) OTEL
	EndpointPipeOpts(cfg *luraconfig.EndpointConfig) config.PipeOpts
	EndpointBackendOpts(cfg *luraconfig.Backend) config.BackendOpts

	BackendOTEL(cfg *luraconfig.Backend) OTEL
	BackendOpts(cfg *luraconfig.Backend) config.BackendOpts

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

func (s *StateConfig) GlobalOpts() config.GlobalOpts {
	return *s.cfgData.Layers.Global
}

func (*StateConfig) EndpointOTEL(_ *luraconfig.EndpointConfig) OTEL {
	return GlobalState()
}

func (s *StateConfig) EndpointPipeOpts(cfg *luraconfig.EndpointConfig) config.PipeOpts {
	PipeOpts := *s.cfgData.Layers.Pipe
	cfgExtra, err := config.LuraExtraCfg(cfg.ExtraConfig)
	if err == nil && cfgExtra.Layers.Pipe != nil {
		PipeOpts.MetricsStaticAttributes = append(
			PipeOpts.MetricsStaticAttributes,
			cfgExtra.Layers.Pipe.MetricsStaticAttributes...,
		)

		PipeOpts.TracesStaticAttributes = append(
			PipeOpts.TracesStaticAttributes,
			cfgExtra.Layers.Pipe.TracesStaticAttributes...,
		)
	}

	return PipeOpts
}

func (s *StateConfig) EndpointBackendOpts(cfg *luraconfig.Backend) config.BackendOpts {
	return mergedBackendOpts(s, cfg)
}

func (*StateConfig) BackendOTEL(_ *luraconfig.Backend) OTEL {
	return GlobalState()
}

func (s *StateConfig) BackendOpts(cfg *luraconfig.Backend) config.BackendOpts {
	return mergedBackendOpts(s, cfg)
}

func mergedBackendOpts(s *StateConfig, cfg *luraconfig.Backend) config.BackendOpts {
	BackendOpts := *s.cfgData.Layers.Backend

	cfgExtra, err := config.LuraExtraCfg(cfg.ExtraConfig)
	if err == nil && cfgExtra.Layers.Backend != nil {
		if cfgExtra.Layers.Backend.Metrics != nil {
			BackendOpts.Metrics.StaticAttributes = append(
				BackendOpts.Metrics.StaticAttributes,
				cfgExtra.Layers.Backend.Metrics.StaticAttributes...,
			)
		}

		if cfgExtra.Layers.Backend.Traces != nil {
			BackendOpts.Traces.StaticAttributes = append(
				BackendOpts.Traces.StaticAttributes,
				cfgExtra.Layers.Backend.Traces.StaticAttributes...,
			)
		}
	}

	return BackendOpts
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
