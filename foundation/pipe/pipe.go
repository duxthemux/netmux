// Package iopipe simplifies the process of pumping data from one end to another
// while allowing us to measure throughput.
package pipe

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"time"

	"golang.org/x/sync/errgroup"
)

// helperIoClose reduces repetitive error handling when closing io.Closer instances.
func helperIoClose(closer io.Closer) {
	if err := closer.Close(); err != nil {
		_, file, line, ok := runtime.Caller(1)
		if ok {
			slog.Warn("error closing", "err", err, "file", file, "line", line)

			return
		}

		slog.Warn("error closing - could not find caller", "err", err)
	}
}

// ---------------------------------------------------------------------------------------------------------------------

// countWriter is a wrapper to a io.Writer that allows tracking the amount of data written to it.
type countWriter struct {
	w       io.Writer
	counter int64
}

// Write - same as io.Writer.Write. Every call will add the len of p to the counter.
func (c *countWriter) Write(p []byte) (int, error) {
	c.counter += int64(len(p))

	ret, err := c.w.Write(p)
	if err != nil {
		return 0, fmt.Errorf("error writing to inner writer: %w", err)
	}

	return ret, nil
}

// Total will return the total number of bytes written to this countWriter so far.
func (c *countWriter) Total() int64 {
	return c.counter
}

// NewCountWriter creates a new countWriter on top of a pre-existing writer.
func newCountWriter(w io.Writer) *countWriter {
	return &countWriter{
		w: w,
	}
}

// Int64Metric defines what we will use to produce/inform int64 metrics.
type Float64Metric func(i float64)

// ---------------------------------------------------------------------------------------------------------------------

// Pipe will ensure that data flows from A to B and vice-versa - being A and B instances of io.ReadWriteCloser.
type Pipe struct {
	AMetric Float64Metric
	BMetric Float64Metric
	a       io.ReadWriteCloser
	b       io.ReadWriteCloser
}

// Run will block and to piping until one of the three conditions is met:
//   - 1: context is canceled
//   - 2: A is closed
//   - 3: B is closed
//
//nolint:funlen
func (p *Pipe) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(fmt.Errorf("pipe ended"))

	group, ctx := errgroup.WithContext(ctx)

	aCountWriter := newCountWriter(p.a)
	bCountWriter := newCountWriter(p.b)

	if p.AMetric != nil || p.BMetric != nil {
		group.Go(func() error {
			defer cancel(fmt.Errorf("closed metrics collecting function"))
			for {
				if ctx.Err() != nil {
					return fmt.Errorf("context cancelled when capturing metrics: %w", ctx.Err())
				}

				if p.AMetric != nil {
					p.AMetric(float64(aCountWriter.Total()))
				}

				if p.BMetric != nil {
					p.BMetric(float64(bCountWriter.Total()))
				}

				time.Sleep(time.Second)
			}
		})
	}

	group.Go(func() error {
		<-ctx.Done()
		helperIoClose(p.a)
		helperIoClose(p.b)

		return nil
	})

	group.Go(func() error {
		defer cancel(fmt.Errorf("pipe b->a closed"))

		_, err := io.Copy(aCountWriter, p.b)
		if err != nil {
			return fmt.Errorf("error copying b-a: %w", err)
		}

		return nil
	})

	group.Go(func() error {
		defer cancel(fmt.Errorf("pipe a->b closed"))

		_, err := io.Copy(bCountWriter, p.a)
		if err != nil {
			return fmt.Errorf("error copying a-b: %w", err)
		}

		return nil
	})

	if err := group.Wait(); err != nil {
		return fmt.Errorf("error processing pipe: %w", err)
	}

	return nil
}

// New will create a new Pipe, copying data from A to B and vice-versa.
func New(a io.ReadWriteCloser, b io.ReadWriteCloser) *Pipe {
	return &Pipe{
		a: a,
		b: b,
	}
}
