package fingerprint

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricFingerprintStored = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smartclaw_fingerprint_stored_total",
		Help: "Total fingerprints stored",
	})
	metricFingerprintSearches = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smartclaw_fingerprint_searches_total",
		Help: "Total similarity searches",
	})
	metricFingerprintCacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smartclaw_fingerprint_cache_size",
		Help: "Current cache size",
	})
)
