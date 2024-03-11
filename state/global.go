package state

import (
	"sync"
)

var (
	otelState      *OTELState
	otelStateMutex sync.RWMutex

	globalConfig      Config
	globalConfigMutex sync.RWMutex
)

var _ GetterFn = GlobalState

// GlobalState retrieves a configured global state
func GlobalState() OTEL {
	otelStateMutex.RLock()
	s := otelState
	otelStateMutex.RUnlock()
	return s
}

// SetGlobalState set the provided state as the global state.
func SetGlobalState(s *OTELState) {
	otelStateMutex.Lock()
	otelState = s
	otelStateMutex.Unlock()
}

func SetGlobalConfig(cfg Config) {
	globalConfigMutex.Lock()
	globalConfig = cfg
	globalConfigMutex.Unlock()
}

func GlobalConfig() Config {
	globalConfigMutex.RLock()
	c := globalConfig
	globalConfigMutex.RUnlock()
	return c
}
