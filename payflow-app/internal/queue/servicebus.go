package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/google/uuid"
)

// Azure Service Bus queue names — must match
// payflow-terraform-modules/modules/azure_servicebus/main.tf.
const (
	azureQueueSettlement = "settlement"
	azureQueueWebhook    = "webhook"
	azureQueueRefund     = "refund"
)

// AzureServiceBus implements Publisher and JobQueue using a namespace connection string.
// For AKS, prefer mounting a secret or using workload identity with a future credential-based constructor.
type AzureServiceBus struct {
	client *azservicebus.Client

	settlementSender *azservicebus.Sender
	webhookSender    *azservicebus.Sender
	refundSender     *azservicebus.Sender

	settlementRecv *azservicebus.Receiver
	webhookRecv    *azservicebus.Receiver
	refundRecv     *azservicebus.Receiver
}

// NewAzureServiceBusFromConnectionString builds senders/receivers for the three PayFlow queues.
func NewAzureServiceBusFromConnectionString(connectionString string) (*AzureServiceBus, error) {
	if connectionString == "" {
		return nil, errors.New("queue: empty Azure Service Bus connection string")
	}
	client, err := azservicebus.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, err
	}
	s := &AzureServiceBus{client: client}

	s.settlementSender, err = client.NewSender(azureQueueSettlement, nil)
	if err != nil {
		_ = client.Close(context.Background())
		return nil, fmt.Errorf("queue: settlement sender: %w", err)
	}
	s.webhookSender, err = client.NewSender(azureQueueWebhook, nil)
	if err != nil {
		_ = s.closeSenders(context.Background())
		_ = client.Close(context.Background())
		return nil, fmt.Errorf("queue: webhook sender: %w", err)
	}
	s.refundSender, err = client.NewSender(azureQueueRefund, nil)
	if err != nil {
		_ = s.closeSenders(context.Background())
		_ = client.Close(context.Background())
		return nil, fmt.Errorf("queue: refund sender: %w", err)
	}

	s.settlementRecv, err = client.NewReceiverForQueue(azureQueueSettlement, nil)
	if err != nil {
		_ = s.closeSenders(context.Background())
		_ = client.Close(context.Background())
		return nil, fmt.Errorf("queue: settlement receiver: %w", err)
	}
	s.webhookRecv, err = client.NewReceiverForQueue(azureQueueWebhook, nil)
	if err != nil {
		_ = s.closeSendersReceivers(context.Background())
		_ = client.Close(context.Background())
		return nil, fmt.Errorf("queue: webhook receiver: %w", err)
	}
	s.refundRecv, err = client.NewReceiverForQueue(azureQueueRefund, nil)
	if err != nil {
		_ = s.closeSendersReceivers(context.Background())
		_ = client.Close(context.Background())
		return nil, fmt.Errorf("queue: refund receiver: %w", err)
	}

	return s, nil
}

// SettlementKey matches Redis settlement list key so cmd/worker can share switch logic.
func (*AzureServiceBus) SettlementKey() string { return DefaultSettlementQueueKey }

func (s *AzureServiceBus) PublishPaymentSettlement(ctx context.Context, paymentID uuid.UUID) error {
	return s.settlementSender.SendMessage(ctx, &azservicebus.Message{Body: []byte(paymentID.String())}, nil)
}

func (s *AzureServiceBus) PublishWebhookDelivery(ctx context.Context, deliveryID uuid.UUID) error {
	return s.webhookSender.SendMessage(ctx, &azservicebus.Message{Body: []byte(deliveryID.String())}, nil)
}

func (s *AzureServiceBus) PublishRefundSettlement(ctx context.Context, refundID uuid.UUID) error {
	return s.refundSender.SendMessage(ctx, &azservicebus.Message{Body: []byte(refundID.String())}, nil)
}

// BRPopJob receives one message from settlement, webhook, or refund queues (fair rotation).
func (s *AzureServiceBus) BRPopJob(ctx context.Context, timeout time.Duration) (listKey string, id uuid.UUID, err error) {
	deadline := time.Now().Add(timeout)
	receivers := []*azservicebus.Receiver{s.settlementRecv, s.webhookRecv, s.refundRecv}
	keys := []string{DefaultSettlementQueueKey, DefaultWebhookQueueKey, DefaultRefundQueueKey}
	start := 0

	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		perTry := 400 * time.Millisecond
		if remaining < perTry {
			perTry = remaining
		}

		for i := 0; i < len(receivers); i++ {
			idx := (start + i) % len(receivers)
			rec := receivers[idx]
			key := keys[idx]

			recvCtx, cancel := context.WithTimeout(ctx, perTry)
			msgs, err := rec.ReceiveMessages(recvCtx, 1, nil)
			cancel()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					continue
				}
				return "", uuid.Nil, err
			}
			if len(msgs) == 0 {
				continue
			}
			body := string(msgs[0].Body)
			parsed, err := uuid.Parse(body)
			if err != nil {
				_ = rec.AbandonMessage(ctx, msgs[0], nil)
				return "", uuid.Nil, fmt.Errorf("queue: service bus body not a UUID: %w", err)
			}
			if err := rec.CompleteMessage(ctx, msgs[0], nil); err != nil {
				return "", uuid.Nil, err
			}
			return key, parsed, nil
		}
		start = (start + 1) % len(receivers)
		time.Sleep(15 * time.Millisecond)
	}
	return "", uuid.Nil, ErrNoJob
}

func (s *AzureServiceBus) closeSenders(ctx context.Context) error {
	var first error
	for _, c := range []*azservicebus.Sender{s.settlementSender, s.webhookSender, s.refundSender} {
		if c == nil {
			continue
		}
		if err := c.Close(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *AzureServiceBus) closeSendersReceivers(ctx context.Context) error {
	var first error
	if err := s.closeSenders(ctx); err != nil && first == nil {
		first = err
	}
	for _, c := range []*azservicebus.Receiver{s.settlementRecv, s.webhookRecv, s.refundRecv} {
		if c == nil {
			continue
		}
		if err := c.Close(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Close shuts down senders, receivers, and the client.
func (s *AzureServiceBus) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = s.closeSendersReceivers(ctx)
	return s.client.Close(ctx)
}
