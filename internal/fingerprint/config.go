package fingerprint

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

func SetConfig(c Config) {
	config = c
}

func GetConfig() Config {
	return config
}
