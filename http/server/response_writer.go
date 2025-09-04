package server

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

type TrackingResponseWriter struct {
	track          *tracking
	recordHeaders  bool
	skipHeaders    map[string]bool
	rw             http.ResponseWriter
	flusher        http.Flusher
	hijacker       http.Hijacker
	hijackCallback func(net.Conn, error) (net.Conn, error)
}

func (w *TrackingResponseWriter) gatherHeaders() {
	if !w.recordHeaders {
		return
	}
	if w.track.responseHeaders != nil {
		// we already recorded the response headers
		return
	}
	h := w.rw.Header()
	w.track.responseHeaders = make(map[string][]string, len(h))
	for k, v := range h {
		if w.skipHeaders == nil || !w.skipHeaders[k] {
			w.track.responseHeaders[k] = v
		}
	}
}

func (w *TrackingResponseWriter) Header() http.Header {
	return w.rw.Header()
}

func (w *TrackingResponseWriter) Write(b []byte) (int, error) {
	w.gatherHeaders()
	nBytes, e := w.rw.Write(b)
	if e != nil {
		w.track.writeErrs = append(w.track.writeErrs, e)
	}
	w.track.responseSize += nBytes
	return nBytes, e
}

func (w *TrackingResponseWriter) WriteHeader(statusCode int) {
	w.gatherHeaders()
	w.track.responseStatus = statusCode
	w.rw.WriteHeader(statusCode)
}

func (w *TrackingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	var c net.Conn
	var rw *bufio.ReadWriter
	if w.hijacker != nil {
		c, rw, w.track.hijackedErr = w.hijacker.Hijack()
	} else {
		w.track.hijackedErr = fmt.Errorf("not implements Hijacker interface")
	}
	w.track.isHijacked = true
	if w.hijackCallback != nil {
		wrapConn, avoidHijackErr := w.hijackCallback(c, w.track.hijackedErr)
		if avoidHijackErr != nil {
			if w.track.hijackedErr == nil {
				w.track.hijackedErr = avoidHijackErr
			}
		} else {
			c = wrapConn
		}
	}
	return c, rw, w.track.hijackedErr
}

func (w *TrackingResponseWriter) Flush() {
	if w.flusher != nil {
		w.flusher.Flush()
	}
}

func newTrackingResponseWriter(rw http.ResponseWriter, t *tracking, recordHeaders bool,
	skipHeaders map[string]bool, hijackCallback func(net.Conn, error) (net.Conn, error),
) *TrackingResponseWriter {
	flusher, _ := rw.(http.Flusher)
	hijacker, _ := rw.(http.Hijacker)
	return &TrackingResponseWriter{
		track:          t,
		recordHeaders:  recordHeaders,
		skipHeaders:    skipHeaders,
		rw:             rw,
		flusher:        flusher,
		hijacker:       hijacker,
		hijackCallback: hijackCallback,
	}
}
