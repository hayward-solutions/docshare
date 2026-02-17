package middleware

import (
	"time"

	"github.com/docshare/api/pkg/logger"
	"github.com/gofiber/fiber/v2"
)

func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		requestID := logger.GenerateRequestID()
		c.Locals("requestID", requestID)

		err := c.Next()

		latency := time.Since(start)
		statusCode := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
		userAgent := c.Get("User-Agent")
		ip := c.IP()

		userID := logger.GetUserIDFromContext(c)
		requestBody := logger.GetRequestBodySummary(c)
		responseBody := logger.GetResponseSizeSummary(c)

		details := map[string]interface{}{
			"method":        method,
			"path":          path,
			"status_code":   statusCode,
			"latency_ms":    latency.Milliseconds(),
			"user_agent":    userAgent,
			"ip":            ip,
			"request_body":  requestBody,
			"response_body": responseBody,
			"request_id":    requestID,
		}

		if userID != nil {
			if statusCode >= 400 {
				logger.ErrorWithUser(*userID, "http_request", err, details)
			} else {
				logger.InfoWithUser(*userID, "http_request", details)
			}
		} else {
			if statusCode >= 400 {
				logger.Error("http_request", err, details)
			} else {
				logger.Info("http_request", details)
			}
		}

		return err
	}
}

func SecurityLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		statusCode := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
		userID := logger.GetUserIDFromContext(c)
		ip := c.IP()

		if statusCode == 403 {
			details := map[string]interface{}{
				"method":  method,
				"path":    path,
				"ip":      ip,
				"user_id": userID,
				"reason":  "access_denied",
			}

			if userID != nil {
				logger.WarnWithUser(*userID, "access_denied", details)
			} else {
				logger.Warn("access_denied_unauthenticated", details)
			}
		}

		if statusCode == 404 {
			details := map[string]interface{}{
				"method":  method,
				"path":    path,
				"ip":      ip,
				"user_id": userID,
				"reason":  "not_found",
			}

			if userID != nil {
				logger.WarnWithUser(*userID, "not_found", details)
			} else {
				logger.Warn("not_found_unauthenticated", details)
			}
		}

		return err
	}
}
