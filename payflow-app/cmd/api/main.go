package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/payflow/payflow-app/internal/config"
	"github.com/payflow/payflow-app/internal/db"
	"github.com/payflow/payflow-app/internal/httpapi"
	"github.com/payflow/payflow-app/internal/migrate"
	"github.com/payflow/payflow-app/internal/payment"
	"github.com/payflow/payflow-app/internal/queue"
	"github.com/payflow/payflow-app/internal/refund"
	"github.com/payflow/payflow-app/internal/tenant"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db_connect_failed", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	if err := migrate.Up(ctx, pool); err != nil {
		slog.Error("migrate_failed", "error", err.Error())
		os.Exit(1)
	}

	var pub queue.Publisher = queue.NoOpPublisher{}
	if strings.TrimSpace(cfg.RedisURL) != "" {
		rq, err := queue.NewRedis(cfg.RedisURL)
		if err != nil {
			slog.Error("redis_connect_failed", "error", err.Error())
			os.Exit(1)
		}
		defer func() { _ = rq.Close() }()
		pub = rq
	}

	srv := &httpapi.Server{
		Pool:      pool,
		Tenants:   &tenant.Service{Pool: pool},
		Payments:  &payment.Service{Pool: pool, Q: pub},
		Refunds:   &refund.Service{Pool: pool, Q: pub},
		Pub:       pub,
		JWTSecret: []byte(cfg.JWTSecret),
	}
	handler := httpapi.NewRouter(srv)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("api_listen", "addr", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http_server_error", "error", err.Error())
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http_shutdown_error", "error", err.Error())
	}
}
