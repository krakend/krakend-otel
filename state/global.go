package state

import (
	"sync"
)

var (
	otelMu    sync.RWMutex
	otelState *OTELState
)

var _ GetterFn = GlobalState

// GlobalState retrieves a configured global state
func GlobalState() OTEL {
	otelMu.RLock()
	s := otelState
	otelMu.RUnlock()
	return s
}

// SetGlobalState set the provided state as the global state.
func SetGlobalState(s *OTELState) {
	otelMu.Lock()
	otelState = s
	otelMu.Unlock()
}
