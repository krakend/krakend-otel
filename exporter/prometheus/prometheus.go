// Package prometheus implements a Prometheus metrics exporter.
package prometheus

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/krakend/krakend-otel/config"
)

// PrometheusCollector implemnts the metrics exporter
type PrometheusCollector struct {
	registry          *prom.Registry
	exporter          *prometheus.Exporter
	disabledByDefault bool
}

// MetricReader implements the interface to exporte metrics.
func (c *PrometheusCollector) MetricReader(_ time.Duration) sdkmetric.Reader {
	return c.exporter
}

func (c *PrometheusCollector) MetricDefaultReporting() bool {
	return !c.disabledByDefault
}

// Exporter creates a Prometheus exporter instance.
func Exporter(ctx context.Context, cfg config.PrometheusExporter) (*PrometheusCollector, error) {
	if cfg.Port == 0 {
		cfg.Port = 9090
	}
	prometheusRegistry := prom.NewRegistry()

	if cfg.ProcessMetrics {
		err := prometheusRegistry.Register(promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{}))
		if err != nil {
			return nil, err
		}
	}

	if cfg.GoMetrics {
		err := prometheusRegistry.Register(promcollectors.NewGoCollector())
		if err != nil {
			return nil, err
		}
	}

	exporter, err := prometheus.New(prometheus.WithRegisterer(prometheusRegistry))
	if err != nil {
		return nil, err
	}

	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.HandlerFor(prometheusRegistry,
		promhttp.HandlerOpts{}))
	server := http.Server{
		Handler:           router,
		Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		ReadHeaderTimeout: 3 * time.Second,
	}

	go func() {
		if serverErr := server.ListenAndServe(); serverErr != http.ErrServerClosed {
			log.Printf("[SERVICE: kotel] The Prometheus exporter failed to listen and serve: %v", serverErr)
		}
	}()

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		server.Shutdown(ctx)
		cancel()
	}()

	return &PrometheusCollector{
		registry:          prometheusRegistry,
		exporter:          exporter,
		disabledByDefault: cfg.DisableMetrics,
	}, nil
}
