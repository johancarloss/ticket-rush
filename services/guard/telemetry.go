package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// metrics are what Prometheus scrapes from GET /metrics (docs/guides/INSTRUMENTACAO.md).
type metrics struct {
	registry  *prometheus.Registry
	decisions *prometheus.CounterVec
	duration  prometheus.Histogram
}

func newMetrics() *metrics {
	m := &metrics{
		registry: prometheus.NewRegistry(),
		decisions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "guard_decisions_total",
			Help: "Answers given to Traefik, by result (allowed or the reason for the denial).",
		}, []string{"result"}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "guard_verify_duration_seconds",
			Help:    "Time to answer one forwardAuth request.",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25},
		}),
	}
	m.registry.MustRegister(m.decisions, m.duration,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

func (m *metrics) record(result string, started time.Time) {
	m.decisions.WithLabelValues(result).Inc()
	m.duration.Observe(time.Since(started).Seconds())
}

func (m *metrics) handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// setupTracing sends spans to the endpoint in OTEL_EXPORTER_OTLP_ENDPOINT. Without it, tracing stays off,
// so the tests and a run outside Compose need nothing extra.
func setupTracing(ctx context.Context) (func(context.Context) error, error) {
	// The incoming traceparent header continues the trace Traefik started for the purchase.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	// The service name comes from OTEL_SERVICE_NAME, set in compose.yaml.
	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}

// traced wraps the handler in a span per request, skipping the health and metrics probes that would only add noise.
func traced(handler http.Handler) http.Handler {
	return otelhttp.NewHandler(handler, "guard",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/health" && r.URL.Path != "/metrics"
		}),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
}
