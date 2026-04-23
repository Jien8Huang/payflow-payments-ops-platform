package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ErrNoJob is returned when no queue message is available within the wait window
// (Redis BRPOP timeout or Service Bus short poll). cmd/worker treats this like an idle tick.
var ErrNoJob = errors.New("queue: no job available")

// Publisher enqueues async jobs (Redis locally; Service Bus in cloud — docs/contracts/async-plane.md).
type Publisher interface {
	PublishPaymentSettlement(ctx context.Context, paymentID uuid.UUID) error
	PublishWebhookDelivery(ctx context.Context, deliveryID uuid.UUID) error
	PublishRefundSettlement(ctx context.Context, refundID uuid.UUID) error
}

// JobQueue is the worker-facing broker: publish jobs and block until one is available.
// Implemented by *Redis and *AzureServiceBus.
type JobQueue interface {
	Publisher
	SettlementKey() string
	BRPopJob(ctx context.Context, timeout time.Duration) (listKey string, id uuid.UUID, err error)
	Close() error
}

// NoOpPublisher skips all async work (tests).
type NoOpPublisher struct{}

func (NoOpPublisher) PublishPaymentSettlement(ctx context.Context, paymentID uuid.UUID) error {
	_ = ctx
	_ = paymentID
	return nil
}

func (NoOpPublisher) PublishWebhookDelivery(ctx context.Context, deliveryID uuid.UUID) error {
	_ = ctx
	_ = deliveryID
	return nil
}

func (NoOpPublisher) PublishRefundSettlement(ctx context.Context, refundID uuid.UUID) error {
	_ = ctx
	_ = refundID
	return nil
}

const (
	DefaultSettlementQueueKey = "payflow:settlement_jobs"
	DefaultWebhookQueueKey    = "payflow:webhook_jobs"
	DefaultRefundQueueKey     = "payflow:refund_jobs"
)

// Redis implements Publisher and blocking pop for the worker.
type Redis struct {
	Client *redis.Client
	Key    string // settlement queue override (optional)
}

func (r *Redis) settlementKey() string {
	if r.Key != "" {
		return r.Key
	}
	return DefaultSettlementQueueKey
}

// SettlementKey returns the Redis list name used for payment settlement jobs.
func (r *Redis) SettlementKey() string {
	return r.settlementKey()
}

// NewRedis parses redisURL (e.g. redis://localhost:6379/0).
func NewRedis(redisURL string) (*Redis, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Redis{Client: redis.NewClient(opt)}, nil
}

func (r *Redis) PublishPaymentSettlement(ctx context.Context, paymentID uuid.UUID) error {
	return r.Client.LPush(ctx, r.settlementKey(), paymentID.String()).Err()
}

func (r *Redis) PublishWebhookDelivery(ctx context.Context, deliveryID uuid.UUID) error {
	return r.Client.LPush(ctx, DefaultWebhookQueueKey, deliveryID.String()).Err()
}

func (r *Redis) PublishRefundSettlement(ctx context.Context, refundID uuid.UUID) error {
	return r.Client.LPush(ctx, DefaultRefundQueueKey, refundID.String()).Err()
}

// BRPopJob blocks until a job is available on any of settlement, webhook, or refund queues.
// Returns the Redis list key that produced the job and the UUID payload.
func (r *Redis) BRPopJob(ctx context.Context, timeout time.Duration) (listKey string, id uuid.UUID, err error) {
	res, err := r.Client.BRPop(ctx, timeout, r.settlementKey(), DefaultWebhookQueueKey, DefaultRefundQueueKey).Result()
	if err == redis.Nil {
		return "", uuid.Nil, ErrNoJob
	}
	if err != nil {
		return "", uuid.Nil, err
	}
	if len(res) < 2 {
		return "", uuid.Nil, redis.Nil
	}
	parsed, err := uuid.Parse(res[1])
	if err != nil {
		return "", uuid.Nil, err
	}
	return res[0], parsed, nil
}

func (r *Redis) Close() error {
	return r.Client.Close()
}
