// Package config concentra a leitura das variáveis de ambiente. Nenhum outro
// pacote deve chamar os.Getenv: tudo é lido aqui uma única vez e repassado.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port       string // PORT (padrão 8000)
	SocketPort string // SOCKET_PORT (padrão 8001)

	MongoURI      string // MONGODB_URI (aceita MONGO_URI por compatibilidade)
	MongoDatabase string // MONGO_DATABASE

	JWTSecret     string // JWT_SECRET (validado por auth.Init)
	EncryptionKey string // JSON_ENCRYPTION_KEY (validado por encryption.Init)

	// AppURL é a URL do front, usada nos links enviados por email.
	AppURL string // APP_URL

	Mail Mail

	// ProxyHeader/TrustedProxies definem de onde vem o IP do cliente (ver cmd/main.go).
	ProxyHeader    string   // PROXY_HEADER
	TrustedProxies []string // TRUSTED_PROXIES (lista separada por vírgula)

	// LegacyIDLinks mantém os links antigos pelo _id; LEGACY_ID_LINKS=false desliga.
	LegacyIDLinks bool

	// SocketCORSOrigins: origens aceitas pelo websocket. Vazio = qualquer origem.
	SocketCORSOrigins []string // SOCKET_CORS_ORIGINS
}

type Mail struct {
	From             string // SES_FROM
	ConfigurationSet string // SES_CONFIGURATION_SET
	DevLog           bool   // MAIL_DEV_LOG=true
}

// LoadDotEnv carrega o .env do diretório atual ou, se não houver, o do
// diretório acima do executável. Variáveis do .env sobrescrevem as do ambiente.
func LoadDotEnv() {
	exe, _ := os.Executable()
	for _, path := range []string{".env", filepath.Join(filepath.Dir(exe), "../.env")} {
		if err := godotenv.Overload(path); err == nil {
			return
		}
	}
}

// Load lê a configuração do ambiente e valida o que é obrigatório para
// conectar ao banco. Segredos (JWT, chave de criptografia) são validados pelos
// pacotes que os usam, para que scripts que não precisam deles possam rodar.
func Load() (Config, error) {
	cfg := Config{
		Port:          env("PORT", "8000"),
		SocketPort:    env("SOCKET_PORT", "8001"),
		MongoURI:      firstNonEmpty(os.Getenv("MONGODB_URI"), os.Getenv("MONGO_URI")),
		MongoDatabase: strings.TrimSpace(os.Getenv("MONGO_DATABASE")),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		EncryptionKey: os.Getenv("JSON_ENCRYPTION_KEY"),
		AppURL:        strings.TrimRight(strings.TrimSpace(os.Getenv("APP_URL")), "/"),
		Mail: Mail{
			From:             strings.TrimSpace(os.Getenv("SES_FROM")),
			ConfigurationSet: strings.TrimSpace(os.Getenv("SES_CONFIGURATION_SET")),
			DevLog:           os.Getenv("MAIL_DEV_LOG") == "true",
		},
		ProxyHeader:       strings.TrimSpace(os.Getenv("PROXY_HEADER")),
		TrustedProxies:    splitList(os.Getenv("TRUSTED_PROXIES")),
		LegacyIDLinks:     os.Getenv("LEGACY_ID_LINKS") != "false",
		SocketCORSOrigins: splitList(os.Getenv("SOCKET_CORS_ORIGINS")),
	}
	if len(cfg.SocketCORSOrigins) == 1 && cfg.SocketCORSOrigins[0] == "*" {
		cfg.SocketCORSOrigins = nil
	}

	var errs []error
	if cfg.MongoURI == "" {
		errs = append(errs, errors.New("MONGODB_URI não definida"))
	}
	if cfg.MongoDatabase == "" {
		errs = append(errs, errors.New("MONGO_DATABASE não definida"))
	}
	if cfg.ProxyHeader != "" && len(cfg.TrustedProxies) == 0 {
		errs = append(errs, errors.New("PROXY_HEADER definido sem TRUSTED_PROXIES: o header poderia ser forjado por qualquer cliente"))
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("configuração inválida: %w", err)
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// splitList separa uma lista por vírgulas, ignorando itens vazios.
func splitList(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
