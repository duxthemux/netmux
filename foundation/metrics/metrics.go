// Package metrics will allow us to publish/report metrics to measure and manage netmux operations.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Observer implements the last mile collector to a metric. Ideally all preparation to report metrics should be chached
// by this entity.
type Observer interface {
	ObserveFloat64(value float64)
}

// Metric represents one data point to be collected. Since it may have flags/labels, we use it as an intermediary
// entity. An observer should be created from the metric for further reporting.
type Metric interface {
	Observer(labels map[string]string) Observer
}

// Factory allows creation of metrics.
type Factory interface {
	New(m string, params ...string) Metric
}

//----------------------------------------------------------------------------------------------------------------------

// Stdout metrics family is a simple implementation of our metrics stack, sending them to stdout.

type StdoutObserver struct {
	attrs []any
}

func (s *StdoutObserver) ObserveFloat64(value float64) {
	s.attrs[3] = value
	slog.Info("Metric", s.attrs...)
}

type StdoutMetric struct {
	name string
}

func (s *StdoutMetric) Observer(labels map[string]string) *StdoutObserver {
	attrs := make([]any, len(labels)+4) //nolint:gomnd
	attrs[0] = "name"
	attrs[1] = s.name
	attrs[2] = "value"
	attrs[3] = 0
	counter := 4

	for k, v := range labels {
		attrs[counter] = k
		attrs[counter+1] = v
		counter += 2
	}

	return &StdoutObserver{}
}

type StdoutFactory struct{}

func (s *StdoutFactory) New(m string, _ ...string) *StdoutMetric {
	return &StdoutMetric{
		name: m,
	}
}

func NewStdoutFactory() *StdoutFactory {
	return &StdoutFactory{}
}

//----------------------------------------------------------------------------------------------------------------------

// Prom family is the implementation of our stack to work on top of prometheus.

type PromObserver struct {
	promObserver prometheus.Observer
}

func (s *PromObserver) ObserveFloat64(value float64) {
	s.promObserver.Observe(value)
}

type PromMetric struct {
	name       string
	promMetric *prometheus.HistogramVec
}

func (p *PromMetric) Observer(labels map[string]string) Observer { //nolint:ireturn,nolintlint
	ret := &PromObserver{promObserver: p.promMetric.With(labels)}

	return ret
}

type PromFactory struct {
	metrics map[string]*PromMetric
}

func (p *PromFactory) Start(ctx context.Context, addr string) error {
	server := http.Server{
		Addr:              addr,
		Handler:           promhttp.Handler(),
		ReadHeaderTimeout: time.Second * 5, //nolint:gomnd
	}

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*15) //nolint:gomnd

		defer cancel()

		_ = server.Shutdown(ctx) //nolint:contextcheck
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("error starting prom server: %w", err)
	}

	return nil
}

func (p *PromFactory) New(metric string, labels ...string) Metric { //nolint:ireturn,nolintlint
	ret, ok := p.metrics[metric]
	if ok {
		return ret
	}

	ret = &PromMetric{
		name: metric,
		promMetric: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "netmux",
			Subsystem: "nexmut",
			Name:      metric,
		},
			labels,
		),
	}

	p.metrics[metric] = ret

	return ret
}

func NewPromFactory() *PromFactory {
	return &PromFactory{
		metrics: make(map[string]*PromMetric),
	}
}
