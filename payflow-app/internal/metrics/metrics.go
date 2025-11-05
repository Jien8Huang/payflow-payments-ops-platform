package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	PaymentsCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "payflow_payments_created_total",
		Help: "New payment rows created (not idempotent replays).",
	})
)
