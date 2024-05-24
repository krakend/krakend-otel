package state

import (
	"github.com/krakend/krakend-otel/config"
	luraconfig "github.com/luraproject/lura/v2/config"
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

func (s *StateConfig) EndpointPipeOpts(cfg *luraconfig.EndpointConfig) *config.PipeOpts {
	var sOpts *config.PipeOpts
	var extraPOpts *config.PipeOpts

	if s != nil && s.cfgData.Layers != nil {
		sOpts = s.cfgData.Layers.Pipe
	}

	cfgExtra, err := config.LuraExtraCfg(cfg.ExtraConfig)
	if err == nil && cfgExtra != nil && cfgExtra.Layers != nil {
		extraPOpts = cfgExtra.Layers.Pipe
	}

	if extraPOpts == nil {
		if sOpts == nil {
			return new(config.PipeOpts)
		}
		return sOpts
	} else if sOpts == nil {
		return extraPOpts
	}

	pOpts := new(config.PipeOpts)
	*pOpts = *sOpts

	pOpts.MetricsStaticAttributes = append(
		pOpts.MetricsStaticAttributes,
		cfgExtra.Layers.Pipe.MetricsStaticAttributes...,
	)

	pOpts.TracesStaticAttributes = append(
		pOpts.TracesStaticAttributes,
		cfgExtra.Layers.Pipe.TracesStaticAttributes...,
	)

	return pOpts
}

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
	var extraBOpts *config.BackendOpts
	var sOpts *config.BackendOpts

	if s != nil && s.cfgData.Layers != nil {
		sOpts = s.cfgData.Layers.Backend
	}

	cfgExtra, err := config.LuraExtraCfg(cfg.ExtraConfig)
	if err == nil && cfgExtra != nil && cfgExtra.Layers != nil {
		extraBOpts = cfgExtra.Layers.Backend
	}

	if extraBOpts == nil {
		if sOpts == nil {
			return new(config.BackendOpts)
		}
		return sOpts
	} else if sOpts == nil {
		return extraBOpts
	}

	bOpts := new(config.BackendOpts)
	*bOpts = *sOpts

	if extraBOpts.Metrics != nil {
		bOpts.Metrics.StaticAttributes = append(
			bOpts.Metrics.StaticAttributes,
			cfgExtra.Layers.Backend.Metrics.StaticAttributes...,
		)
	}

	if extraBOpts.Traces != nil {
		bOpts.Traces.StaticAttributes = append(
			bOpts.Traces.StaticAttributes,
			cfgExtra.Layers.Backend.Traces.StaticAttributes...,
		)
	}

	return bOpts
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
