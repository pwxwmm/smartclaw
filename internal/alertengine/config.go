package alertengine

import (
	"sync"
	"time"
)

type Config struct {
	DedupWindow           time.Duration
	CorrelationWindow     time.Duration
	MaxRawAlerts          int
	AutoEscalateThreshold int
}

func DefaultConfig() Config {
	return Config{
		DedupWindow:           1 * time.Hour,
		CorrelationWindow:     5 * time.Minute,
		MaxRawAlerts:          10000,
		AutoEscalateThreshold: 5,
	}
}

var config = DefaultConfig()
var configMu sync.RWMutex

func SetConfig(c Config) {
	configMu.Lock()
	defer configMu.Unlock()
	config = c
}

func GetConfig() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return config
}
