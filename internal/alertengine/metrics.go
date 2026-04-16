package alertengine

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricAlertsIngested = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smartclaw_alerts_ingested_total",
		Help: "Total alerts ingested",
	})
	metricAlertsDeduped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smartclaw_alerts_deduped_total",
		Help: "Alerts that were deduplicated",
	})
	metricAlertGroups = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smartclaw_alert_groups",
		Help: "Current number of alert groups",
	})
)
