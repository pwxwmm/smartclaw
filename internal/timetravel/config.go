package timetravel

import "sync"

type Config struct {
	MaxSessions int
}

func DefaultConfig() Config {
	return Config{
		MaxSessions: 100,
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
