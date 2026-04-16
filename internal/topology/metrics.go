package topology

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricNodesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smartclaw_topology_nodes",
		Help: "Total number of nodes in the topology graph",
	})
	metricEdgesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smartclaw_topology_edges",
		Help: "Total number of edges in the topology graph",
	})
	metricSnapshotDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "smartclaw_topology_snapshot_duration_seconds",
		Help:    "Time taken to save topology snapshot",
		Buckets: prometheus.DefBuckets,
	})
)
