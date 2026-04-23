package middleware

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	otelpropagation "go.opentelemetry.io/otel/propagation"
)

var (
	httpDuration metric.Float64Histogram
	durationOnce sync.Once
)

func getHistogram() metric.Float64Histogram {
	durationOnce.Do(func() {
		var err error
		httpDuration, err = otel.Meter("fiber").Float64Histogram(
			"http.server.request.duration",
			metric.WithDescription("HTTP server request duration"),
			metric.WithUnit("s"),
		)
		if err != nil {
			panic(err)
		}
	})
	return httpDuration
}

// fiberHeaderCarrier adapts fasthttp headers to the OTel TextMapCarrier interface.
type fiberHeaderCarrier struct {
	c fiber.Ctx
}

func (f fiberHeaderCarrier) Get(key string) string {
	return string(f.c.Request().Header.Peek(key))
}

func (f fiberHeaderCarrier) Set(key, val string) {
	f.c.Request().Header.Set(key, val)
}

func (f fiberHeaderCarrier) Keys() []string {
	var keys []string
	f.c.Request().Header.VisitAll(func(key, _ []byte) {
		keys = append(keys, string(key))
	})
	return keys
}

func TracingMiddleware(c fiber.Ctx) error {
	start := time.Now()

	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(context.Background(), otelpropagation.TextMapCarrier(fiberHeaderCarrier{c}))

	tracer := otel.Tracer("fiber")
	spanName := c.Method() + " " + c.Route().Path
	ctx, span := tracer.Start(ctx, spanName)
	defer span.End()

	c.Locals("otel_ctx", ctx)

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.url", c.OriginalURL()),
		attribute.String("http.route", c.Route().Path),
	)

	err := c.Next()

	status := c.Response().StatusCode()
	duration := time.Since(start).Seconds()

	attrs := metric.WithAttributes(
		attribute.String("http.request.method", c.Method()),
		attribute.String("http.route", c.Route().Path),
		attribute.String("http.response.status_code", strconv.Itoa(status)),
	)
	getHistogram().Record(ctx, duration, attrs)

	span.SetAttributes(attribute.Int("http.status_code", status))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	} else if status >= 500 {
		span.SetStatus(codes.Error, "server error")
	}

	return err
}

// OtelCtx retorna o context com span ativo, para uso nos handlers/repositórios.
func OtelCtx(c fiber.Ctx) context.Context {
	if ctx, ok := c.Locals("otel_ctx").(context.Context); ok {
		return ctx
	}
	return context.Background()
}
