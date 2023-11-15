package otelio

import (
	"context"
	"io"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var _ io.WriteCloser = (*instrumentedWriter)(nil)

// InstrumentedWriter keeps track of the number of bytes and
// and time it takes to write ot an io.Writer.
//
// We want the start of the span to happen not when the instrumentedWriter
// is created, but on the first call to Write.
//
// we can finish the writing by receiving an `io.EOF` error
// (the full buffer was written), or by a `Close` call, when
// no more data will be written by the consumer
type instrumentedWriter struct {
	writer io.Writer
	track  ioTracking
}

// NewInstrumentedWriterFactory creates a function that can wrap a writer with
// an instrumented writer. Is better than the [NewIntrumentedWriter] call because
// the instruments here are only created once.
func NewInstrumentedWriterFactory(prefix string, attrT []attribute.KeyValue, attrM []attribute.KeyValue,
	tracer trace.Tracer, meter metric.Meter,
) func(io.Writer, context.Context) *instrumentedWriter {
	if prefix == "" {
		prefix = "written."
	}
	instr := newInstruments(prefix, attrT, attrM, tracer, meter)

	return func(w io.Writer, ctx context.Context) *instrumentedWriter {
		return &instrumentedWriter{
			writer: w,
			track: ioTracking{
				instr: instr,
				ctx:   ctx,
			},
		}
	}
}

// NewInstrumentedWriter wraps a writer with an instrumented writer.
// Is better to use [NewInstrumentedWriterFactory].
func NewInstrumentedWriter(prefix string, w io.Writer, ctx context.Context,
	attrT []attribute.KeyValue, attrM []attribute.KeyValue,
	tracer trace.Tracer, meter metric.Meter,
) *instrumentedWriter {
	return NewInstrumentedWriterFactory(prefix, attrT, attrM, tracer, meter)(w, ctx)
}

// Write wraps the writer Write and keeps track of the
// written bytes. In case an error happens it will automatically
// end the span and report the metrics.
func (t *instrumentedWriter) Write(b []byte) (int, error) {
	t.track.start()
	n, err := t.writer.Write(b)
	t.track.incSize(int64(n), err)
	return n, err
}

// Close wraps the writer Close and ends the trace and reports
// the metrics.
func (t *instrumentedWriter) Close() error {
	var err error
	if cl, ok := t.writer.(io.Closer); ok {
		err = cl.Close()
	}
	t.track.end(err)
	return err
}
