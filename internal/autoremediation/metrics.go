package autoremediation

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricRemediationActions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smartclaw_remediation_actions_total",
		Help: "Actions by status",
	}, []string{"status"})
	metricRemediationRunbooksLoaded = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smartclaw_remediation_runbooks_loaded",
		Help: "Number of loaded runbooks",
	})
)
