package server

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

type TrackingResponseWriter struct {
	track         *tracking
	recordHeaders bool
	rw            http.ResponseWriter
	flusher       http.Flusher
	hijacker      http.Hijacker
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
		w.track.responseHeaders[k] = v
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
	if w.hijacker != nil {
		return w.hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("not implemented")
}

func (w *TrackingResponseWriter) Flush() {
	if w.flusher != nil {
		w.flusher.Flush()
	}
}

func newTrackingResponseWriter(rw http.ResponseWriter, t *tracking, recordHeaders bool) *TrackingResponseWriter {
	return &TrackingResponseWriter{
		track:         t,
		recordHeaders: recordHeaders,
		rw:            rw,
		flusher:       rw.(http.Flusher),
		hijacker:      rw.(http.Hijacker),
	}
}
