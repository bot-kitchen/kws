package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LoggerMiddleware creates a logging middleware using zap
func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		fields := []zap.Field{
			zap.Int("status", statusCode),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Duration("latency", latency),
			zap.Int("body_size", c.Writer.Size()),
		}

		// Add request ID if present
		if requestID := GetRequestID(c); requestID != "" {
			fields = append(fields, zap.String("request_id", requestID))
		}

		// Add tenant ID if present
		if tenantID := GetTenantID(c); tenantID != "" {
			fields = append(fields, zap.String("tenant_id", tenantID))
		}

		// Add KOS ID if present
		if kosID := GetKOSID(c); kosID != "" {
			fields = append(fields, zap.String("kos_id", kosID))
		}

		// Add error if present
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		switch {
		case statusCode >= 500:
			logger.Error("Server error", fields...)
		case statusCode >= 400:
			logger.Warn("Client error", fields...)
		case statusCode >= 300:
			logger.Info("Redirect", fields...)
		default:
			logger.Info("Request completed", fields...)
		}
	}
}

// RecoveryMiddleware creates a panic recovery middleware
func RecoveryMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					zap.String("request_id", GetRequestID(c)),
				)

				c.AbortWithStatusJSON(500, gin.H{
					"error":      "internal_error",
					"message":    "An internal error occurred",
					"request_id": GetRequestID(c),
				})
			}
		}()

		c.Next()
	}
}
