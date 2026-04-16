package changerisk

type Config struct {
	MaxHistorySize int
}

func DefaultConfig() Config {
	return Config{
		MaxHistorySize: 1000,
	}
}

var config = DefaultConfig()

func SetConfig(c Config) {
	config = c
}

func GetConfig() Config {
	return config
}
