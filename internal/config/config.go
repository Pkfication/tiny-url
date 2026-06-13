package config

import "os"

type Config struct {
	ZKServers string
	Port      string
}

func Load() *Config {
	zkServers := os.Getenv("ZK_SERVERS")
	if zkServers == "" {
		zkServers = "localhost:2181"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		ZKServers: zkServers,
		Port:      port,
	}
}
