// Package prometheus implements a Prometheus metrics exporter.
package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const (
	// ExporterKey is the name for the prometheus exporter
	ExporterKey string = "prometheus"
)

// PrometheusConfig has the variables to configure the
// prometheus exporter.
type PrometheusConfig struct {
	Port           int  `json:"port"`
	ProcessMetrics bool `json:"process_metrics"`
	GoMetrics      bool `json:"go_metrics"`
}

// PrometheusCollector implemnts the metrics exporter
type PrometheusCollector struct {
	registry *prom.Registry
	exporter *prometheus.Exporter
}

// MetricReader implements the interface to exporte metrics.
func (c *PrometheusCollector) MetricReader() sdkmetric.Reader {
	return c.exporter
}

// PrometheusConfigFromInterface creates a Prometheus configuration.
func PrometheusConfigFromInterface(in map[string]interface{}) (*PrometheusConfig, error) {
	cfg := PrometheusConfig{
		Port:           9091,
		ProcessMetrics: true,
		GoMetrics:      true,
	}

	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(b, &cfg)
		if err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

// Exporter creates a Prometheus exporter instance.
func Exporter(ctx context.Context, cfg map[string]interface{}) (interface{}, error) {
	promCfg, err := PrometheusConfigFromInterface(cfg)
	if err != nil {
		return nil, err
	}

	prometheusRegistry := prom.NewRegistry()

	if promCfg.ProcessMetrics {
		err := prometheusRegistry.Register(prom.NewProcessCollector(prom.ProcessCollectorOpts{}))
		if err != nil {
			return nil, err
		}
	}

	if promCfg.GoMetrics {
		err = prometheusRegistry.Register(prom.NewGoCollector())
		if err != nil {
			return nil, err
		}
	}

	// TODO: should we put a WithNamespace option here ?
	exporter, err := prometheus.New(prometheus.WithRegisterer(prometheusRegistry))
	if err != nil {
		return nil, err
	}

	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.HandlerFor(prometheusRegistry,
		promhttp.HandlerOpts{}))
	server := http.Server{
		Handler: router,
		Addr:    fmt.Sprintf(":%d", promCfg.Port),
	}

	go func() {
		if serverErr := server.ListenAndServe(); serverErr != http.ErrServerClosed {
			log.Fatalf("[SERVICE: kotel] The Prometheus exporter failed to listen and serve: %v", serverErr)
		}
	}()

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		server.Shutdown(ctx)
		cancel()
	}()

	return &PrometheusCollector{
		registry: prometheusRegistry,
		exporter: exporter,
	}, nil
}
