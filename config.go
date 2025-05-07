package main

import (
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

const configPath = ".env"

type FileConfig struct {
	Result string `env:"RESULT_FILE"`
	Users  string `env:"USERS_FILE"`
	Proxy  string `env:"PROXY_FILE"`
}
type DirConfig struct {
	Sessions string `env:"SESSIONS_DIR"`
}
type Config struct {
	Dir        DirConfig
	File       FileConfig
	NumWorkers int `env:"NUM_WORKERS"`
}

func MustLoadConfig() *Config {
	var cfg Config
	if _, err := os.Stat(configPath); err != nil {
		log.Fatalf("error opening config file: %s", err)
	}

	err := cleanenv.ReadConfig(configPath, &cfg)
	if err != nil {
		log.Fatalf("error reading config file: %s", err)
	}

	return &cfg
}
