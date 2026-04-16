package operator

import (
	"sync"
	"time"
)

type Config struct {
	CheckInterval    time.Duration
	MaxRecentResults int
	MaxEvents        int
}

func DefaultConfig() Config {
	return Config{
		CheckInterval:    1 * time.Minute,
		MaxRecentResults: 100,
		MaxEvents:        500,
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
