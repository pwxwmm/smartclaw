package fingerprint

import "sync"

type Config struct {
	VectorSize   int
	Threshold    float64
	MaxCacheSize int
}

func DefaultConfig() Config {
	return Config{
		VectorSize:   64,
		Threshold:    0.7,
		MaxCacheSize: 10000,
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
