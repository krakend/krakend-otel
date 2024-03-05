package otelio

import (
	"context"
	"io"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var _ io.ReadCloser = (*instrumentedReader)(nil)

// we can finish the reading by receiving an `io.EOF` error
// (the full buffer was read), or by a `Close` call, when
// no more data will be read by the consumer
type instrumentedReader struct {
	reader io.ReadCloser
	track  ioTracking
}

// NewInstrumentedReaderFactory creates a function that can wrap a reader with
// an instrumented reader. Is better than the [NewIntrumentedReader] call because
// the instruments here are only created once.
func NewInstrumentedReaderFactory(prefix string, attrT []attribute.KeyValue, attrM []attribute.KeyValue,
	tracer trace.Tracer, meter metric.Meter,
) func(io.Reader, context.Context) *instrumentedReader {
	if prefix == "" {
		prefix = "read."
	}
	instr := newInstruments(prefix, attrT, attrM, tracer, meter)

	return func(r io.Reader, ctx context.Context) *instrumentedReader {
		rc, ok := r.(io.ReadCloser)
		if !ok {
			rc = io.NopCloser(r)
		}
		return &instrumentedReader{
			reader: rc,
			track: ioTracking{
				instr: instr,
				ctx:   ctx,
			},
		}
	}
}

// NewInstrumentedReader wraps a reader with an instrumented reader.
// Is better to use [NewInstrumentedReaderFactory].
func NewInstrumentedReader(prefix string, r io.Reader, ctx context.Context,
	attrT []attribute.KeyValue, attrM []attribute.KeyValue,
	tracer trace.Tracer, meter metric.Meter,
) *instrumentedReader {
	return NewInstrumentedReaderFactory(prefix, attrT, attrM, tracer, meter)(r, ctx)
}

// Read wraps the reader's Read and keeps track of the
// read bytes. In case an error happens it will automatically
// end the span and report the metrics.
func (t *instrumentedReader) Read(b []byte) (int, error) {
	t.track.start()
	n, err := t.reader.Read(b)
	t.track.incSize(int64(n), err)
	return n, err
}

// Close wraps the reader Close and ends the trace and reports
// the metrics.
func (t *instrumentedReader) Close() error {
	err := t.reader.Close()
	t.track.end(err)
	return err
}
