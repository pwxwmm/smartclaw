package changerisk

import "sync"

type Config struct {
	MaxHistorySize int
}

func DefaultConfig() Config {
	return Config{
		MaxHistorySize: 1000,
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
