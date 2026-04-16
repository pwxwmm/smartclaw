package changerisk

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricRiskAssessments = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smartclaw_risk_assessments_total",
		Help: "Risk assessments by level",
	}, []string{"level"})
	metricRiskHistorySize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smartclaw_risk_history_size",
		Help: "Number of historical change records",
	})
)
