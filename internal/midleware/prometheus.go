package midleware

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

// statusRecorder to record the status code from the ResponseWriter
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(statusCode int) {
	rec.statusCode = statusCode
	rec.ResponseWriter.WriteHeader(statusCode)
}

func MeasureResponseDuration(next http.Handler) http.Handler {
	// buckets := []float64{0.1, 0.15, 0.2, 0.25, 0.3}

	requestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "namespace",
			Name:      "client_request_count",
			Help:      "Total count router",
		}, []string{"route", "method", "status_code"},
	)

	// responseTimeHistogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
	// 	Namespace: "namespace",
	// 	Name:      "http_server_request_duration_seconds",
	// 	Help:      "Histogram of response time for handler in seconds",
	// 	Buckets:   buckets,
	// }, []string{"route", "method", "status_code"})

	// prometheus.MustRegister(responseTimeHistogram)
	prometheus.MustRegister(requestCounter)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// start := time.Now()
		rec := statusRecorder{w, 200}

		next.ServeHTTP(&rec, r)

		statusCode := strconv.Itoa(rec.statusCode)
		route := GetRoutePattern(r)
		// defer func() {
		// 	responseTimeHistogram.WithLabelValues(route, r.Method, statusCode).Observe(time.Since(start).Seconds())
		// }()
		requestCounter.WithLabelValues(route, r.Method, statusCode).Inc()
	})
}

// getRoutePattern returns the route pattern from the chi context there are 3 conditions
// a) static routes "/example" => "/example"
// b) dynamic routes "/example/:id" => "/example/{id}"
// c) if nothing matches the output is undefined
func GetRoutePattern(r *http.Request) string {
	reqContext := chi.RouteContext(r.Context())
	if pattern := reqContext.RoutePattern(); pattern != "" {
		return pattern
	}

	return "/"
}
