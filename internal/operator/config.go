package operator

import "time"

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

func SetConfig(c Config) {
	config = c
}

func GetConfig() Config {
	return config
}
