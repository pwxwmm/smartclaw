package warroom

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricWarRoomSessionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smartclaw_warroom_sessions_active",
		Help: "Active war room sessions",
	})
	metricWarRoomFindings = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smartclaw_warroom_findings_total",
		Help: "Total findings submitted",
	})
)
