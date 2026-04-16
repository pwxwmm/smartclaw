package autoremediation

type Config struct {
	MaxHistorySize int
	RunbookDir     string
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
