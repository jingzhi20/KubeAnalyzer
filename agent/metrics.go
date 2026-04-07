package main

import (
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	analyzerErrorsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aiops_analyzer_errors",
			Help: "Number of errors found by each analyzer kind",
		},
		[]string{"kind"},
	)

	analyzerDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aiops_analyzer_duration_seconds",
			Help:    "Duration of each analyzer execution in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"analyzer"},
	)

	analyzerRunsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aiops_analyzer_runs_total",
			Help: "Total number of analyzer runs",
		},
		[]string{"analyzer", "status"},
	)

	metricsOnce sync.Once
)

// InitMetrics registers Prometheus metrics and starts the HTTP server.
func InitMetrics() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(analyzerErrorsGauge)
		prometheus.MustRegister(analyzerDurationHistogram)
		prometheus.MustRegister(analyzerRunsCounter)

		metricsPort := os.Getenv("AIOPS_METRICS_PORT")
		if metricsPort == "" {
			metricsPort = "9090"
		}

		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			log.Printf("[Metrics] Starting Prometheus metrics server on :%s/metrics", metricsPort)
			if err := http.ListenAndServe(":"+metricsPort, mux); err != nil {
				log.Printf("[Metrics] Failed to start metrics server: %v", err)
			}
		}()
	})
}

// RecordAnalyzerErrors updates the analyzer_errors gauge based on results.
func RecordAnalyzerErrors(results []AnalyzeResult) {
	analyzerErrorsGauge.Reset()

	kindCounts := make(map[string]float64)
	for _, r := range results {
		kindCounts[r.Kind] += float64(len(r.Error))
	}

	for kind, count := range kindCounts {
		analyzerErrorsGauge.WithLabelValues(kind).Set(count)
	}
}

// RecordAnalyzerRun records a single analyzer execution.
func RecordAnalyzerRun(analyzerName string, durationSeconds float64, err error) {
	analyzerDurationHistogram.WithLabelValues(analyzerName).Observe(durationSeconds)
	status := "success"
	if err != nil {
		status = "error"
	}
	analyzerRunsCounter.WithLabelValues(analyzerName, status).Inc()
}
