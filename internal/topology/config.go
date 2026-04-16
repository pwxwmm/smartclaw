package topology

import (
	"sync"
	"time"
)

type Config struct {
	SnapshotInterval time.Duration
	AutoSnapshot     bool
}

func DefaultConfig() Config {
	return Config{
		SnapshotInterval: 5 * time.Minute,
		AutoSnapshot:     true,
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
