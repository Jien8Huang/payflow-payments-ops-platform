package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/payflow/payflow-app/internal/config"
	"github.com/payflow/payflow-app/internal/db"
	"github.com/payflow/payflow-app/internal/outbox"
	"github.com/payflow/payflow-app/internal/payment"
	"github.com/payflow/payflow-app/internal/queue"
	"github.com/payflow/payflow-app/internal/refund"
	"github.com/payflow/payflow-app/internal/tracing"
	"github.com/payflow/payflow-app/internal/webhook"
)

type outboxFollowup struct {
	paymentID *uuid.UUID
	refundID  *uuid.UUID
}

func tryProcessOutbox(ctx context.Context, pool *pgxpool.Pool) (outboxFollowup, error) {
	var fu outboxFollowup
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fu, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var obID uuid.UUID
	var kind string
	var payload []byte
	err = tx.QueryRow(ctx, `
		SELECT id, kind, payload::text
		FROM async_outbox
		WHERE processed_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(&obID, &kind, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return fu, nil
	}
	if err != nil {
		return fu, err
	}

	switch kind {
	case outbox.KindPaymentSettlement:
		var m struct {
			PaymentID string `json:"payment_id"`
		}
		if err := json.Unmarshal(payload, &m); err != nil {
			return fu, err
		}
		pid, err := uuid.Parse(m.PaymentID)
		if err != nil {
			return fu, err
		}
		if err := payment.SettleMockTx(ctx, tx, pid); err != nil {
			return fu, err
		}
		fu.paymentID = &pid
	case outbox.KindRefundSettlement:
		var m struct {
			RefundID string `json:"refund_id"`
		}
		if err := json.Unmarshal(payload, &m); err != nil {
			return fu, err
		}
		rid, err := uuid.Parse(m.RefundID)
		if err != nil {
			return fu, err
		}
		if err := refund.SettleMockTx(ctx, tx, rid); err != nil {
			return fu, err
		}
		fu.refundID = &rid
	default:
		return fu, fmt.Errorf("unknown outbox kind %q", kind)
	}

	if _, err := tx.Exec(ctx, `UPDATE async_outbox SET processed_at = now() WHERE id = $1`, obID); err != nil {
		return fu, err
	}
	if err := tx.Commit(ctx); err != nil {
		return fu, err
	}
	return fu, nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()
	ctx := context.Background()

	shutdownTrace, err := tracing.Init(ctx, "payflow-worker")
	if err != nil {
		slog.Warn("tracing_init_failed", "error", err.Error())
		shutdownTrace = func(context.Context) error { return nil }
	} else {
		defer func() {
			sdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := shutdownTrace(sdCtx); err != nil {
				slog.Warn("tracing_shutdown", "error", err.Error())
			}
		}()
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db_connect_failed", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	var jq queue.JobQueue
	switch cfg.QueueBackend {
	case "azservicebus":
		if cfg.AzureServiceBusConnectionString == "" {
			slog.Error("servicebus_config_missing", "hint", "set AZURE_SERVICEBUS_CONNECTION_STRING")
			os.Exit(1)
		}
		sb, err := queue.NewAzureServiceBusFromConnectionString(cfg.AzureServiceBusConnectionString)
		if err != nil {
			slog.Error("servicebus_connect_failed", "error", err.Error())
			os.Exit(1)
		}
		jq = sb
	default:
		redisURL := cfg.RedisURL
		if redisURL == "" {
			redisURL = "redis://127.0.0.1:6379/0"
		}
		rq, err := queue.NewRedis(redisURL)
		if err != nil {
			slog.Error("redis_connect_failed", "error", err.Error())
			os.Exit(1)
		}
		jq = rq
	}
	defer func() { _ = jq.Close() }()

	httpClient := &http.Client{Timeout: 10 * time.Second}
	maxAttempts := cfg.WebhookMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	slog.Info("worker_listening",
		"outbox", "payment_settlement+refund_settlement",
		"queue_backend", cfg.QueueBackend,
		"settlement_key", jq.SettlementKey(),
		"webhook_key", queue.DefaultWebhookQueueKey,
		"refund_key", queue.DefaultRefundQueueKey,
	)

	for {
		fu, err := tryProcessOutbox(ctx, pool)
		if err != nil {
			slog.Error("outbox_failed", "error", err.Error())
			time.Sleep(time.Second)
			continue
		}
		if fu.paymentID != nil {
			if err := webhook.EnqueuePaymentSettledIfNeeded(ctx, pool, jq, *fu.paymentID); err != nil {
				slog.Error("webhook_enqueue_failed", "payment_id", fu.paymentID.String(), "error", err.Error())
			} else {
				slog.Info("payment_settled", "payment_id", fu.paymentID.String())
			}
			continue
		}
		if fu.refundID != nil {
			if err := webhook.EnqueueRefundSucceededIfNeeded(ctx, pool, jq, *fu.refundID); err != nil {
				slog.Error("refund_webhook_enqueue_failed", "refund_id", fu.refundID.String(), "error", err.Error())
			} else {
				slog.Info("refund_settled", "refund_id", fu.refundID.String())
			}
			continue
		}

		popCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		key, id, err := jq.BRPopJob(popCtx, 5*time.Second)
		cancel()
		if errors.Is(err, queue.ErrNoJob) {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if err != nil {
			slog.Error("queue_pop_failed", "error", err.Error())
			time.Sleep(time.Second)
			continue
		}

		switch key {
		case jq.SettlementKey():
			if err := payment.SettleMock(ctx, pool, id); err != nil {
				slog.Error("settle_failed", "payment_id", id.String(), "error", err.Error())
				continue
			}
			if err := webhook.EnqueuePaymentSettledIfNeeded(ctx, pool, jq, id); err != nil {
				slog.Error("webhook_enqueue_failed", "payment_id", id.String(), "error", err.Error())
				continue
			}
			slog.Info("payment_settled_queue", "payment_id", id.String())

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
			if err := webhook.EnqueueRefundSucceededIfNeeded(ctx, pool, jq, id); err != nil {
				slog.Error("refund_webhook_enqueue_failed", "refund_id", id.String(), "error", err.Error())
				continue
			}
			slog.Info("refund_settled_queue", "refund_id", id.String())

		default:
			slog.Warn("unknown_queue_key", "key", key)
		}
	}
}
