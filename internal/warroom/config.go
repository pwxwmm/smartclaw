package warroom

import "sync"

type Config struct {
	ChannelBufferSize int
}

func DefaultConfig() Config {
	return Config{
		ChannelBufferSize: 16,
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
