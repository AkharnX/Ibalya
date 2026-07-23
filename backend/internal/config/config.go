package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL        string
	APIAddr            string
	AdminToken         string
	LLMServiceURL      string
	PublicBaseURL      string
	GoogleClientID     string
	GoogleClientSecret string
	Channel            string // gmail | fixture
	FixturePath        string
	IngestInterval     int // minutes
	DetectInterval     int // minutes
	DigestHour         int
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func Load() Config {
	return Config{
		DatabaseURL:        env("DATABASE_URL", "postgres://agentops:agentops_dev@127.0.0.1:5435/agentops?sslmode=disable"),
		APIAddr:            env("API_ADDR", ":9999"),
		AdminToken:         env("ADMIN_TOKEN", ""),
		LLMServiceURL:      env("LLM_SERVICE_URL", "http://127.0.0.1:8092"),
		PublicBaseURL:      env("PUBLIC_BASE_URL", "http://localhost:9999"),
		GoogleClientID:     env("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: env("GOOGLE_CLIENT_SECRET", ""),
		Channel:            env("CHANNEL", "gmail"),
		FixturePath:        env("FIXTURE_PATH", ""),
		IngestInterval:     envInt("INGEST_INTERVAL_MINUTES", 15),
		DetectInterval:     envInt("DETECT_INTERVAL_MINUTES", 30),
		DigestHour:         envInt("DIGEST_HOUR", 7),
	}
}
