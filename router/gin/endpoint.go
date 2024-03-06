package gin

import (
	"github.com/gin-gonic/gin"
	luraconfig "github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	krakendgin "github.com/luraproject/lura/v2/router/gin"

	kotelconfig "github.com/krakend/krakend-otel/config"
	kotelserver "github.com/krakend/krakend-otel/http/server"
	otelstate "github.com/krakend/krakend-otel/state"
)

// New wraps a handler factory adding some simple instrumentation to the generated handlers
func New(hf krakendgin.HandlerFactory) krakendgin.HandlerFactory {
	return func(cfg *luraconfig.EndpointConfig, p proxy.Proxy) gin.HandlerFunc {
		otelCfg := otelstate.GlobalConfig()
		if otelCfg == nil {
			return hf(cfg, p)
		}
		if otelCfg.SkipEndpoint(cfg.Endpoint) {
			return hf(cfg, p)
		}
		urlPattern := kotelconfig.NormalizeURLPattern(cfg.Endpoint)
		next := hf(cfg, p)
		return func(c *gin.Context) {
			// we set the matched route to a data struct stored in the
			// context by the outer http layer, so it can be reported
			// in metrics and traces.
			kotelserver.SetEndpointPattern(c.Request.Context(), urlPattern)
			next(c)
		}
	}
}
