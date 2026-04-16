package topology

import "time"

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

func SetConfig(c Config) {
	config = c
}

func GetConfig() Config {
	return config
}
