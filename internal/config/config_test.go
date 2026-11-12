package config

import (
	"slices"
	"testing"
)

func setRequired(t *testing.T) {
	t.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("MONGO_DATABASE", "test")
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)
	t.Setenv("PORT", "")
	t.Setenv("LEGACY_ID_LINKS", "")
	t.Setenv("SOCKET_CORS_ORIGINS", "*")
	t.Setenv("APP_URL", "https://app.exemplo.com/")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8000" {
		t.Errorf("Port = %q, quer 8000", cfg.Port)
	}
	if !cfg.LegacyIDLinks {
		t.Error("LegacyIDLinks deveria ser true por padrão")
	}
	if cfg.SocketCORSOrigins != nil {
		t.Errorf("\"*\" deveria virar lista vazia, veio %v", cfg.SocketCORSOrigins)
	}
	if cfg.AppURL != "https://app.exemplo.com" {
		t.Errorf("AppURL = %q, a barra final deveria ser removida", cfg.AppURL)
	}
}

func TestLoadParsesLists(t *testing.T) {
	setRequired(t)
	t.Setenv("PROXY_HEADER", "X-Real-IP")
	t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8 , ,1.2.3.4")
	t.Setenv("SOCKET_CORS_ORIGINS", "https://a.com,https://b.com")
	t.Setenv("LEGACY_ID_LINKS", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.TrustedProxies, []string{"10.0.0.0/8", "1.2.3.4"}) {
		t.Errorf("TrustedProxies = %v", cfg.TrustedProxies)
	}
	if !slices.Equal(cfg.SocketCORSOrigins, []string{"https://a.com", "https://b.com"}) {
		t.Errorf("SocketCORSOrigins = %v", cfg.SocketCORSOrigins)
	}
	if cfg.LegacyIDLinks {
		t.Error("LEGACY_ID_LINKS=false deveria desligar")
	}
}

func TestLoadAcceptsLegacyMongoURI(t *testing.T) {
	t.Setenv("MONGODB_URI", "")
	t.Setenv("MONGO_URI", "mongodb://legacy")
	t.Setenv("MONGO_DATABASE", "test")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MongoURI != "mongodb://legacy" {
		t.Errorf("MongoURI = %q", cfg.MongoURI)
	}
}

func TestLoadValidates(t *testing.T) {
	t.Setenv("MONGODB_URI", "")
	t.Setenv("MONGO_URI", "")
	t.Setenv("MONGO_DATABASE", "")
	if _, err := Load(); err == nil {
		t.Error("deveria falhar sem MONGODB_URI/MONGO_DATABASE")
	}

	setRequired(t)
	t.Setenv("PROXY_HEADER", "X-Real-IP")
	t.Setenv("TRUSTED_PROXIES", "")
	if _, err := Load(); err == nil {
		t.Error("deveria falhar com PROXY_HEADER sem TRUSTED_PROXIES")
	}
}
