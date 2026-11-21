// Package app builds the service from its parts and runs it: config ->
// connections -> features -> routes. It is the only package that knows the
// concrete types; everything else receives what it needs through its
// constructor. Nothing is looked up from a context or a registry.
package app

import (
	"context"
	"log/slog"
	"os"

	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
	echoSwagger "github.com/swaggo/echo-swagger/v2"
	"github.com/uptrace/bun"

	"example.com/compat/internal/config"
	"example.com/compat/internal/platform"
	"example.com/compat/internal/server"

	_ "example.com/compat/docs" // the Swagger document, written by swag
	"example.com/compat/internal/user"
)

// Serve opens the connections, builds every feature and serves HTTP on
// cfg.HTTP.Port until ctx is done.
func Serve(ctx context.Context, cfg config.Config) error {
	logger := newLogger(cfg.Debug)
	slog.SetDefault(logger)

	db, err := platform.OpenPostgres(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer db.Close()

	rdb, err := platform.OpenRedis(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	defer rdb.Close()

	e := server.New(logger)
	server.NewHealth(db, rdb).Register(e)
	e.GET("/swagger/*", echoSwagger.WrapHandler)
	features(e.Group(cfg.HTTP.BasePath), db, rdb)

	logger.Info("listening", "addr", ":"+cfg.HTTP.Port, "base_path", cfg.HTTP.BasePath)
	return server.Run(ctx, e, cfg.HTTP)
}

// features builds every feature, bottom up, and mounts its routes on api,
// the BASE_PATH group. `./gortisan make:feature` adds the lines, e.g.
//
//	userService := user.NewService(user.NewPostgres(db))
//	user.RegisterRoutes(api, user.NewHandler(userService))
//
// A feature that needs another gets it through a small interface it declares
// itself; this function is the only place both are known.
func features(api *echo.Group, db *bun.DB, rdb *redis.Client) {
	userService := user.NewService(user.NewPostgres(db))
	user.RegisterRoutes(api, user.NewHandler(userService))
}

func newLogger(debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
