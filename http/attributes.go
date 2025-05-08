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
	// we fill a max of 5 attributes, but 8 is a power of 2,
	// and leaves room in the array to fill some extra args
	// from the calling function.
	attrs := make([]attribute.KeyValue, 0, 8)
	attrs = append(attrs,
		semconv.URLFull(r.URL.String()),
		semconv.ServerAddress(r.Host),
		semconv.HTTPRequestMethodKey.String(r.Method),
	)
	if r.ContentLength >= 0 {
		attrs = append(attrs, semconv.HTTPRequestBodySize(int(r.ContentLength)))
	}
	userAgent := r.UserAgent()
	if userAgent != "" {
		attrs = append(attrs, semconv.UserAgentOriginal(userAgent))
	}
	return attrs
}

func clientAddr(r *http.Request, trustedProxies map[string]bool) string {
	if r.RemoteAddr == "" {
		return ""
	}
	if trustedProxies == nil || len(trustedProxies) == 0 {
		return r.RemoteAddr
	}

	vals, ok := r.Header["X-Forwarded-For"]
	if !ok {
		vals, ok = r.Header["X-Real-Ip"]
	}
	if !ok || len(vals) == 0 {
		return r.RemoteAddr
	}

	s := strings.Split(vals[0], ",")
	for i := len(s) - 1; i > 0; i-- {
		ip := strings.TrimSpace(s[i])
		if ok := trustedProxies[ip]; !ok {
			// not trusted proxy ? might be client ip, we could
			// check if is an actual ip by parsing it , but if
			// it is spoofed by the client or some intermediate hop
			// we can check it by looking at the X-Forwarded-For,
			// or X-Real-Ip headers
			return ip
		}
	}
	return strings.TrimSpace(s[0])
}

func TraceIncomingRequestAttrs(r *http.Request, trustedProxies map[string]bool) []attribute.KeyValue {
	attrs := TraceRequestAttrs(r)
	if cAddr := clientAddr(r, trustedProxies); cAddr != "" {
		attrs = append(attrs, semconv.ClientAddress(cAddr))
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
