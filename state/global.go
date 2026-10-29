package state

import (
	"sync"
)

var (
	otelState      OTEL
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

// SetGlobalOTELState set the provided state as the global state.
func SetGlobalOTELState(s OTEL) {
	otelStateMutex.Lock()
	otelState = s
	otelStateMutex.Unlock()
}

// SetGlobalState set the provided state as the global state.
// Deprecated: use SetGlobalOTELState
func SetGlobalState(s *OTELState) {
	if s == nil {
		SetGlobalOTELState(nil)
		return
	}
	SetGlobalOTELState(s)
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
