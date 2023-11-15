package lura

import (
	"sort"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/luraproject/lura/v2/config"

	kotelconfig "github.com/krakend/krakend-otel/config"
)

// backendConfigAttributes returnsa list of attributes
// that will be set for both traces and
// metrics, as those are expected to have low cardinality
//   - the method: one of the `GET`, `POST`, `PUT` .. etc
//   - the "path" , that is actually the path "template" to not have different values
//     for different params but the same endpoint.
//   - server address: the host for the request
func backendConfigAttributes(cfg *config.Backend) []attribute.KeyValue {
	urlPattern := kotelconfig.NormalizeURLPattern(cfg.URLPattern)
	parentEndpoint := kotelconfig.NormalizeURLPattern(cfg.ParentEndpoint)

	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(cfg.Method),
		semconv.HTTPRoute(urlPattern), // <- for traces we can use URLFull to not have the matched path
		attribute.String("krakend.endpoint", parentEndpoint),
		attribute.String("krakend.endpoint_method", cfg.ParentEndpointMethod),
	}
	numHosts := len(cfg.Host)
	if numHosts > 0 {
		if numHosts == 1 {
			attrs = append(attrs, semconv.ServerAddress(cfg.Host[0]))
		} else {
			hosts := make([]string, 0, numHosts)
			copy(hosts, cfg.Host)
			sort.StringSlice(hosts).Sort()
			strHosts := strings.Join(hosts, "_")
			attrs = append(attrs, semconv.ServerAddress(strHosts))
		}
	}

	return attrs
}
