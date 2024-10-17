package metrics

import "github.com/prometheus/client_golang/prometheus"

type NewMetrics struct {
	Duration prometheus.HistogramVec
	Summary  prometheus.Summary
}

func NewMetricsHistorigram() NewMetrics {
	buckets := []float64{0.1, 0.15, 0.2, 0.25, 0.3}
	m := NewMetrics{
		Duration: *prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "namespace",
			Name:      "http_server_request_duration_seconds",
			Help:      "Histogram of response time for handler in seconds",
			Buckets:   buckets,
		}, []string{"route", "method", "status_code"}),
		Summary: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace:  "myapp",
			Name:       "login_request_duration_seconds",
			Help:       "Duration of the login request.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}),
	}

	prometheus.MustRegister(m.Duration, m.Summary)

	return m
}
