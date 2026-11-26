package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metrics are what Prometheus scrapes from GET /metrics (docs/guides/INSTRUMENTACAO.md). The skeleton exposes the
// Go runtime and process metrics; the consumer metrics and the traces come with issue #43.
type metrics struct {
	registry *prometheus.Registry
}

func newMetrics() *metrics {
	m := &metrics{registry: prometheus.NewRegistry()}
	m.registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

func (m *metrics) handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
