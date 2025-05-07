package http

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.21.0"
)

// TraceRequestAttrs returns a list of attributes to be set
// for a given http.Request (only useful for traces, as
// it reports the url string with any variable parameter
// that it might contain).
func TraceRequestAttrs(r *http.Request) []attribute.KeyValue {
	return TraceRequestAttrsWithTrustedProxies(r, nil)
}

func TraceRequestAttrsWithTrustedProxies(r *http.Request, trustedProxies map[string]bool) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 8)
	attrs = append(attrs,
		semconv.URLFull(r.URL.String()),
		semconv.ServerAddress(r.Host),
		semconv.HTTPRequestMethodKey.String(r.Method),
	)
	clientAddr := r.RemoteAddr
	if trustedProxies != nil {
		vals, ok := r.Header["X-Forwarded-For"]
		if !ok {
			vals, ok = r.Header["X-Real-Ip"]
		}
		if ok && len(vals) > 0 {
			s := strings.Split(vals[0], ",")
			for i := len(s) - 1; i > 0; i-- {
				ip := strings.TrimSpace(s[i])
				if ok := trustedProxies[ip]; !ok {
					// not trusted proxy ? might be client ip, we could
					// check if is an actual ip by parsing it , but if
					// it is spoofed by the client or some intermediate hop
					// we can check it by looking at the X-Forwarded-For,
					// or X-Real-Ip headers
					clientAddr = ip
					break
				}
			}
			if clientAddr == r.RemoteAddr {
				clientAddr = strings.TrimSpace(s[0])
			}
		}
	}
	attrs = append(attrs, semconv.ClientAddress(clientAddr))

	if r.ContentLength >= 0 {
		attrs = append(attrs, semconv.HTTPRequestBodySize(int(r.ContentLength)))
	}

	userAgent := r.UserAgent()
	if userAgent != "" {
		attrs = append(attrs, semconv.UserAgentOriginal(userAgent))
	}

	return attrs
}

// TraceResponseAttrs returns a list of attributes to be set
// for a given http.Response.
func TraceResponseAttrs(resp *http.Response) []attribute.KeyValue {
	if resp == nil {
		return []attribute.KeyValue{}
	}
	return []attribute.KeyValue{
		semconv.HTTPResponseStatusCode(int(resp.StatusCode)),
		semconv.HTTPResponseBodySize(int(resp.ContentLength)),
	}
}
