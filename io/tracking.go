package otelio

import (
	"context"
	"io"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ioTracking contains the information to be reported
// as metrics or traces
type ioTracking struct {
	instr    *instruments
	ctx      context.Context
	span     trace.Span
	started  time.Time // when the first call to Read / Write happened
	finished bool      // to avoid several calls to close send repeated metrics
	gotError error     // in case an error happended when reading the data
	size     int64     // bytes
}

func (t *ioTracking) start() {
	if t.started.IsZero() {
		t.started = time.Now()
		t.ctx, t.span = t.instr.tracer.Start(t.ctx, t.instr.traceName)
		if len(t.instr.traceFixedAttrs) > 0 {
			t.span.SetAttributes(t.instr.traceFixedAttrs...)
		}
	}
}

func (t *ioTracking) incSize(size int64, err error) {
	t.size += size
	if err != nil {
		t.end(err)
		if err != io.EOF {
			t.gotError = err
		}
		t.end(err)
	}
}

func (t *ioTracking) end(err error) {
	if t.finished {
		return
	}
	t.finished = true
	secs := float64(time.Since(t.started)) / float64(time.Second)

	if err != nil && err != io.EOF {
		t.gotError = err
	}

	var metricAttrOpts metric.MeasurementOption // <- we can move this to the instr parte
	if t.gotError != nil {
		metricAttrOpts = t.instr.metricAttributeSetWithErrorOpt
		t.span.RecordError(t.gotError)
		t.span.SetStatus(codes.Error, t.gotError.Error())
	} else {
		t.span.SetStatus(codes.Ok, "")
		metricAttrOpts = t.instr.metricAttributeSetOpt
	}

	// There could be a difference between the received Content-Length
	// that might be tracked at the request level (not inside the io reader)
	// and the read / written  bytes (a client could read less than the content-length
	// bytes).
	t.instr.sizeCount.Add(t.ctx, t.size, metricAttrOpts)
	t.instr.sizeHistogram.Record(t.ctx, t.size, metricAttrOpts)

	t.instr.timeCount.Add(t.ctx, secs, metricAttrOpts)
	t.instr.timeHistogram.Record(t.ctx, secs, metricAttrOpts)

	t.span.SetAttributes(
		attribute.Int64(t.instr.traceSizeAttrName, t.size),
		attribute.Float64(t.instr.traceTimeAttrName, secs))
	t.span.End()
}
