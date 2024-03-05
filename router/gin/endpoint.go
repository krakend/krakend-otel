package gin

import (
	"github.com/gin-gonic/gin"
	luraconfig "github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	krakendgin "github.com/luraproject/lura/v2/router/gin"

	kotelconfig "github.com/krakend/krakend-otel/config"
	kotelserver "github.com/krakend/krakend-otel/http/server"
)

// New wraps a handler factory adding some simple instrumentation to the generated handlers
func New(hf krakendgin.HandlerFactory, srvCfg *luraconfig.ServiceConfig,
	otelCfgParser kotelconfig.ConfigParserFn,
) krakendgin.HandlerFactory {
	otelCfg, err := otelCfgParser(*srvCfg)
	if otelCfg == nil {
		if err != nil && err != kotelconfig.ErrNoConfig {
			// TODO: we might want to log the error using otel at this layer
		}
		return hf
	}

	// TODO: we might want to output some log info about using otel at this layer

	return func(cfg *luraconfig.EndpointConfig, p proxy.Proxy) gin.HandlerFunc {
		return HandlerFunc(cfg, otelCfg.SkipPaths, hf(cfg, p))
	}
}

// HandlerFunc task is to fill the "matched endpoint pattern" once we know it, so the
// global layer tracking can report it for metrics and traces.
func HandlerFunc(cfg *luraconfig.EndpointConfig, skipPaths []string, next gin.HandlerFunc,
) gin.HandlerFunc {
	// skip paths will not try to read the propagation header, because nothing
	// in the downstream pipeline will be instruemented. The header can be passed
	// using the regular `headers` feature.
	for _, sp := range skipPaths {
		if cfg.Endpoint == sp {
			return next
		}
	}

	urlPattern := kotelconfig.NormalizeURLPattern(cfg.Endpoint)
	return func(c *gin.Context) {
		kotelserver.SetEndpointPattern(c.Request.Context(), urlPattern)
		next(c)
	}
}
