package config

import (
	"os"
	"strconv"
)

// Config holds process configuration from the environment (R9: injected, not hardcoded for prod).
type Config struct {
	DatabaseURL        string
	JWTSecret          string
	ListenAddr         string
	RedisURL           string
	WebhookMaxAttempts int
}

func Load() Config {
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		ListenAddr:  os.Getenv("LISTEN_ADDR"),
		RedisURL:    os.Getenv("REDIS_URL"),
	}
	if v := os.Getenv("WEBHOOK_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.WebhookMaxAttempts = n
		}
	}
	if c.DatabaseURL == "" {
		c.DatabaseURL = "postgres://payflow:payflow@localhost:5432/payflow?sslmode=disable"
	}
	if c.JWTSecret == "" {
		c.JWTSecret = "dev-insecure-change-me"
	}
	if c.ListenAddr == "" {
		c.ListenAddr = ":8080"
	}
	return c
}
