package gin

import (
	"github.com/gin-gonic/gin"
	luraconfig "github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/proxy"
	krakendgin "github.com/luraproject/lura/v2/router/gin"
	"go.opentelemetry.io/otel/attribute"

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
		metricsAttrs := []attribute.KeyValue{}
		tracesAttrs := []attribute.KeyValue{}

		cfgExtra := kotelconfig.EndpointExtraCfg(cfg.ExtraConfig)
		if cfgExtra != nil && cfgExtra.Layers.Global != nil {
			for _, kv := range cfgExtra.Layers.Global.MetricsStaticAttributes {
				if len(kv.Key) > 0 && len(kv.Value) > 0 {
					metricsAttrs = append(metricsAttrs, attribute.String(kv.Key, kv.Value))
				}
			}

			for _, kv := range cfgExtra.Layers.Global.TracesStaticAttributes {
				if len(kv.Key) > 0 && len(kv.Value) > 0 {
					tracesAttrs = append(tracesAttrs, attribute.String(kv.Key, kv.Value))
				}
			}
		}

		return func(c *gin.Context) {
			// we set the matched route to a data struct stored in the
			// context by the outer http layer, so it can be reported
			// in metrics and traces.
			kotelserver.SetEndpointPattern(c.Request.Context(), urlPattern)
			kotelserver.SetMetricsStaticAttributtes(c.Request.Context(), metricsAttrs)
			kotelserver.SetTracesStaticAttributtes(c.Request.Context(), tracesAttrs)
			next(c)
		}
	}
}
