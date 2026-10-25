package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL     string
	HTTPAddr        string
	CookieSecure    bool
	SessionTTL      time.Duration
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:    getenv("VAPID_SUBJECT", "mailto:voce@exemplo.com"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL é obrigatória")
	}

	secure, err := strconv.ParseBool(getenv("COOKIE_SECURE", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("COOKIE_SECURE inválida: %w", err)
	}
	cfg.CookieSecure = secure

	hours, err := strconv.Atoi(getenv("SESSION_TTL_HOURS", "720"))
	if err != nil || hours <= 0 {
		return Config{}, fmt.Errorf("SESSION_TTL_HOURS inválida")
	}
	cfg.SessionTTL = time.Duration(hours) * time.Hour

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
