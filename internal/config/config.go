package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	OWMAPIKey            string
	OWMCities            []string
	OWMUnits             string
	DBDSN                string
	HTTPPort             string
	LogLevel             string
	FetchIntervalSeconds int
}

func Load() (*Config, error) {
	apiKey := os.Getenv("OWM_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OWM_API_KEY is required")
	}

	intervalStr := getEnv("FETCH_INTERVAL_SECONDS", "30")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil || interval < 1 {
		return nil, fmt.Errorf("FETCH_INTERVAL_SECONDS must be a positive integer")
	}

	cities := parseCities(getEnv("OWM_CITIES", "London,Madrid,Prague"))

	return &Config{
		OWMAPIKey:            apiKey,
		OWMCities:            cities,
		OWMUnits:             getEnv("OWM_UNITS", "metric"),
		DBDSN:                getEnv("DB_DSN", "postgres://etl:etl@postgres:5432/weather?sslmode=disable"),
		HTTPPort:             getEnv("HTTP_PORT", "8080"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		FetchIntervalSeconds: interval,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseCities(raw string) []string {
	parts := strings.Split(raw, ",")
	cities := make([]string, 0, len(parts))
	for _, p := range parts {
		if c := strings.TrimSpace(p); c != "" {
			cities = append(cities, c)
		}
	}
	return cities
}
