package state

const (
	// KrakenDContextOTELStrKey is a special key to be used when there
	// is no way to obtain the span context from an inner context
	// (like when gin has not the fallback option enabled in the engine).
	KrakenDContextOTELStrKey string = "KrakendD-Context-OTEL"
)
