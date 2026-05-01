package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBURL string
	Port  string
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DB_URL is required")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return &Config{DBURL: dbURL, Port: port}, nil
}
