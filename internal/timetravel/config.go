package timetravel

type Config struct {
	MaxSessions int
}

func DefaultConfig() Config {
	return Config{
		MaxSessions: 100,
	}
}

var config = DefaultConfig()

func SetConfig(c Config) {
	config = c
}

func GetConfig() Config {
	return config
}
