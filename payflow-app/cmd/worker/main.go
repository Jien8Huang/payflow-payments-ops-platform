package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/payflow/payflow-app/internal/config"
	"github.com/payflow/payflow-app/internal/db"
	"github.com/payflow/payflow-app/internal/payment"
	"github.com/payflow/payflow-app/internal/queue"
	"github.com/payflow/payflow-app/internal/refund"
	"github.com/payflow/payflow-app/internal/webhook"
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

	redisURL := cfg.RedisURL
	if redisURL == "" {
		redisURL = "redis://127.0.0.1:6379/0"
	}
	rq, err := queue.NewRedis(redisURL)
	if err != nil {
		slog.Error("redis_connect_failed", "error", err.Error())
		os.Exit(1)
	}
	defer func() { _ = rq.Close() }()

	httpClient := &http.Client{Timeout: 10 * time.Second}
	maxAttempts := cfg.WebhookMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	slog.Info("worker_listening",
		"settlement", rq.SettlementKey(),
		"webhook", queue.DefaultWebhookQueueKey,
		"refund", queue.DefaultRefundQueueKey,
	)

	for {
		popCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		key, id, err := rq.BRPopJob(popCtx, 5*time.Second)
		cancel()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			slog.Error("queue_pop_failed", "error", err.Error())
			time.Sleep(time.Second)
			continue
		}

		switch key {
		case rq.SettlementKey():
			if err := payment.SettleMock(ctx, pool, id); err != nil {
				slog.Error("settle_failed", "payment_id", id.String(), "error", err.Error())
				continue
			}
			if err := webhook.EnqueuePaymentSettledIfNeeded(ctx, pool, rq, id); err != nil {
				slog.Error("webhook_enqueue_failed", "payment_id", id.String(), "error", err.Error())
				continue
			}
			slog.Info("payment_settled", "payment_id", id.String())

		case queue.DefaultWebhookQueueKey:
			if err := webhook.ProcessDelivery(ctx, pool, httpClient, id, maxAttempts); err != nil {
				slog.Error("webhook_delivery_failed", "delivery_id", id.String(), "error", err.Error())
				continue
			}
			slog.Info("webhook_delivery_done", "delivery_id", id.String())

		case queue.DefaultRefundQueueKey:
			if err := refund.SettleMock(ctx, pool, id); err != nil {
				slog.Error("refund_settle_failed", "refund_id", id.String(), "error", err.Error())
				continue
			}
			if err := webhook.EnqueueRefundSucceededIfNeeded(ctx, pool, rq, id); err != nil {
				slog.Error("refund_webhook_enqueue_failed", "refund_id", id.String(), "error", err.Error())
				continue
			}
			slog.Info("refund_settled", "refund_id", id.String())

		default:
			slog.Warn("unknown_queue_key", "key", key)
		}
	}
}
