package warroom

type Config struct {
	ChannelBufferSize int
}

func DefaultConfig() Config {
	return Config{
		ChannelBufferSize: 16,
	}
}

var config = DefaultConfig()

func SetConfig(c Config) {
	config = c
}

func GetConfig() Config {
	return config
}
