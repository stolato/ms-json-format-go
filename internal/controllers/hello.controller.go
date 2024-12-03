package controllers

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsHello struct {
	Duration prometheus.HistogramVec
	Summary  prometheus.Summary
}

func (m *MetricsHello) HelloWord(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	render.JSON(w, r, map[string]string{"name": "MS Items", "version": "1.0.5-beta-ecs"})
	m.Duration.WithLabelValues("/", "GET", "200").Observe(time.Since(start).Seconds())
	m.Summary.Observe(time.Since(start).Seconds())
	return
}
