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

	db, err := repository.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database connected")

	// auto-migrate on startup — skipped if schema already exists
	if err := repository.RunMigrations(ctx, db, "migrations/000001_init.up.sql"); err != nil {
		slog.Warn("migration skipped or failed", "error", err)
	}

	rdb := repository.NewRedis(cfg.RedisURL)
	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Warn("redis close error", "error", err)
		}
	}()
	slog.Info("redis connected")

	repos := repository.NewRepositories(db)

	hub := wshandler.NewHub(rdb)
	go hub.Run(ctx)

	svcs := service.NewServices(repos, hub, cfg)

	app := fiber.New(fiber.Config{
		ErrorHandler:          handlers.ErrorHandler,
		DisableStartupMessage: true,
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

	// health check — no auth, used by Docker healthcheck and load balancers
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "project-management-platform",
		})
	})

	api := app.Group("/api")
	handlers.RegisterRoutes(api, svcs, repos, hub)

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("server listening", "addr", addr)

	go func() {
		if err := app.Listen(addr); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

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
