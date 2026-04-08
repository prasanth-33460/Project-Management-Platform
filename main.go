package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/prasanth-33460/Project-Management-Platform/internal/api/handlers"
	wshandler "github.com/prasanth-33460/Project-Management-Platform/internal/api/websocket"
	"github.com/prasanth-33460/Project-Management-Platform/internal/config"
	"github.com/prasanth-33460/Project-Management-Platform/internal/repository"
	"github.com/prasanth-33460/Project-Management-Platform/internal/service"
)

func main() {
	// ── Structured logging ────────────────────────────────────────────────────
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(logHandler))

	cfg := config.Load()
	ctx := context.Background()

	slog.Info("starting project-management-platform",
		"env", cfg.Env,
		"port", cfg.Port,
	)

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := repository.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database connected")

	// Auto-migrate on startup — idempotent, skipped if schema already exists.
	if err := repository.RunMigrations(ctx, db, "migrations/000001_init.up.sql"); err != nil {
		slog.Warn("migration skipped or failed", "error", err)
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdb := repository.NewRedis(cfg.RedisURL)
	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Warn("redis close error", "error", err)
		}
	}()
	slog.Info("redis connected")

	// ── Repositories ──────────────────────────────────────────────────────────
	repos := repository.NewRepositories(db)

	// ── WebSocket Hub (Redis Pub/Sub fan-out) ─────────────────────────────────
	hub := wshandler.NewHub(rdb)
	go hub.Run(ctx)

	// ── Services ──────────────────────────────────────────────────────────────
	svcs := service.NewServices(repos, hub, cfg)

	// ── Fiber App ─────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		ErrorHandler:          handlers.ErrorHandler,
		DisableStartupMessage: true, // we log startup ourselves
	})

	app.Use(recover.New(recover.Config{EnableStackTrace: cfg.Env != "production"}))
	app.Use(logger.New(logger.Config{
		Format: `{"time":"${time}","status":${status},"latency":"${latency}","method":"${method}","path":"${path}"}` + "\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Health check — no auth required, used by Docker healthcheck + load balancers.
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "project-management-platform",
		})
	})

	// API v1
	api := app.Group("/api")
	handlers.RegisterRoutes(api, svcs, repos, hub)

	// ── Start listening ───────────────────────────────────────────────────────
	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("server listening", "addr", addr)

	go func() {
		if err := app.Listen(addr); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// ── Graceful shutdown on SIGINT / SIGTERM ─────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutdown signal received, draining connections")
	if err := app.Shutdown(); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped cleanly")
}
