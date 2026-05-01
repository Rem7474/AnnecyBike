package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DBURL               string
	GBFSBaseURL         string
	PollInterval        time.Duration
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DB_URL is required")
	}

	gbfsBase := os.Getenv("GBFS_BASE_URL")
	if gbfsBase == "" {
		gbfsBase = "https://gbfs.partners.fifteen.eu/gbfs/2.2/annecy/en"
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
		GBFSBaseURL:  gbfsBase,
		PollInterval: time.Duration(intervalSec) * time.Second,
	}, nil
}
