package alertengine

import "time"

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

func SetConfig(c Config) {
	config = c
}

func GetConfig() Config {
	return config
}
