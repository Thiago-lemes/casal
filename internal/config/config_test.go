package config

import (
	"testing"
	"time"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Error("sem DATABASE_URL deveria falhar")
	}
}

// O padrão precisa ser o seguro: quem esquece de configurar em produção não expõe o cookie.
func TestLoadDefaultsAreSafe(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("TRUST_PROXY", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CookieSecure {
		t.Error("COOKIE_SECURE deveria ser true por padrão")
	}
	if cfg.TrustProxy {
		t.Error("TRUST_PROXY deveria ser false por padrão")
	}
	if cfg.AdminEnabled {
		t.Error("o painel de suporte deve vir desligado por padrão")
	}
	if cfg.SessionTTL != 720*time.Hour || cfg.HTTPAddr != ":8080" {
		t.Errorf("padrões inesperados: %+v", cfg)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("TRUST_PROXY", "true")
	t.Setenv("SESSION_TTL_HOURS", "2")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieSecure || !cfg.TrustProxy || cfg.SessionTTL != 2*time.Hour {
		t.Errorf("valores não aplicados: %+v", cfg)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	for key, bad := range map[string]string{"COOKIE_SECURE": "talvez", "TRUST_PROXY": "2", "ADMIN_ENABLED": "sim", "SESSION_TTL_HOURS": "0", "SIGNUP_MODE": "fechado"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, bad)
			if _, err := Load(); err == nil {
				t.Errorf("%s=%q deveria ser rejeitado", key, bad)
			}
		})
	}
}
