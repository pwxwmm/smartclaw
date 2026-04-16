package timetravel

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricTimetravelReplays = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smartclaw_timetravel_replays_total",
		Help: "Total replay sessions started",
	})
	metricTimetravelWhatIf = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smartclaw_timetravel_whatif_total",
		Help: "Total what-if scenarios run",
	})
)
