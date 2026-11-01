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
	CookieSecure    bool // cookie só por HTTPS; desligue apenas em desenvolvimento local
	TrustProxy      bool // confia no X-Forwarded-For (só atrás de um proxy seu)
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

	var err error
	// o padrão é o seguro: quem esquece de configurar em produção não expõe o cookie
	if cfg.CookieSecure, err = parseBool("COOKIE_SECURE", true); err != nil {
		return Config{}, err
	}
	if cfg.TrustProxy, err = parseBool("TRUST_PROXY", false); err != nil {
		return Config{}, err
	}

	hours, err := strconv.Atoi(getenv("SESSION_TTL_HOURS", "720"))
	if err != nil || hours <= 0 {
		return Config{}, fmt.Errorf("SESSION_TTL_HOURS inválida")
	}
	cfg.SessionTTL = time.Duration(hours) * time.Hour

	return cfg, nil
}

func parseBool(key string, fallback bool) (bool, error) {
	v, err := strconv.ParseBool(getenv(key, strconv.FormatBool(fallback)))
	if err != nil {
		return false, fmt.Errorf("%s inválida: %w", key, err)
	}
	return v, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
