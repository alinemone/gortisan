package server

import (
	"context"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"net/http"
	"time"
)

// Health answers the liveness and readiness probes, /ping and /health. They
// are mounted outside BASE_PATH, so the probes do not change with it.
type Health struct {
	db    *bun.DB
	redis *redis.Client
}

func NewHealth(db *bun.DB, redis *redis.Client) *Health {
	return &Health{db: db, redis: redis}
}

// Ping answers {"ping":"pong"}: the process is up.
func (h *Health) Ping(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"ping": "pong"})
}

// HealthResponse is the body of Check.
type HealthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Check answers 200 when every dependency responds, 503 otherwise.
func (h *Health) Check(c *echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()

	response := HealthResponse{Status: "pass", Checks: map[string]string{}}
	response.Checks["postgres"] = "up"
	if err := h.db.PingContext(ctx); err != nil {
		response.Status, response.Checks["postgres"] = "fail", "down"
	}
	response.Checks["redis"] = "up"
	if err := h.redis.Ping(ctx).Err(); err != nil {
		response.Status, response.Checks["redis"] = "fail", "down"
	}

	status := http.StatusOK
	if response.Status != "pass" {
		status = http.StatusServiceUnavailable
	}
	return c.JSON(status, response)
}

// Register mounts /ping and /health on e.
func (h *Health) Register(e *echo.Echo) {
	e.GET("/ping", h.Ping)
	e.GET("/health", h.Check)
}
