package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

// LoggerMiddleware logs every HTTP request with method, path, status and latency.
func (m *Middleware) LoggerMiddleware(c fiber.Ctx) error {
	start := time.Now()
	err := c.Next()
	status := c.Response().StatusCode()

	fields := []zap.Field{
		zap.String("method", c.Method()),
		zap.String("path", c.Path()),
		zap.Int("status", status),
		zap.Duration("duration", time.Since(start)),
		zap.String("request_id", RequestIDFromCtx(c)),
		zap.String("remote", c.IP()),
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
	}

	switch {
	case status >= 500:
		m.log.Error("http", fields...)
	case status >= 400:
		m.log.Warn("http", fields...)
	default:
		m.log.Info("http", fields...)
	}

	return err
}
