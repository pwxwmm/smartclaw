package operator

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricOperatorChecks = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smartclaw_operator_checks_total",
		Help: "Health checks by type and pass/fail",
	}, []string{"type", "status"})
	metricOperatorEscalations = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smartclaw_operator_escalations_total",
		Help: "Total escalations triggered",
	})
	metricOperatorActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smartclaw_operator_active",
		Help: "Number of active operators",
	})
)
