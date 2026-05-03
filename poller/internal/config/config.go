package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DBURL        string
	GBFSURL      string
	PollInterval time.Duration
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DB_URL is required")
	}

	gbfsURL := os.Getenv("GBFS_URL")
	if gbfsURL == "" {
		gbfsURL = "https://gbfs.partners.fifteen.eu/gbfs/annecy/gbfs.json"
	}

	intervalSec := 60
	if s := os.Getenv("POLL_INTERVAL_SECONDS"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 10 {
			return nil, fmt.Errorf("invalid POLL_INTERVAL_SECONDS: %s", s)
		}
		intervalSec = v
	}

	return &Config{
		DBURL:        dbURL,
		GBFSURL:      gbfsURL,
		PollInterval: time.Duration(intervalSec) * time.Second,
	}, nil
}
