// Package mailer envia emails via SMTP (funciona com qualquer provedor:
// Resend, SES, Mailgun, Gmail...).
//
// Variáveis de ambiente:
//
//	SMTP_HOST, SMTP_PORT (padrão 587; 465 = TLS implícito), SMTP_USER,
//	SMTP_PASSWORD, SMTP_FROM
//	MAIL_DEV_LOG=true  -> sem SMTP_HOST, registra o email no log em vez de enviar
//	                      (somente para desenvolvimento: o log conterá o link).
package mailer

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type config struct {
	host, port, user, password, from string
	devLog                           bool
}

var cfg config

var ErrNotConfigured = errors.New("envio de email não configurado (SMTP_HOST)")

// Init lê a configuração. Sem SMTP_HOST o servidor sobe, mas emails não são
// enviados (a menos que MAIL_DEV_LOG=true).
func Init() error {
	cfg = config{
		host:     os.Getenv("SMTP_HOST"),
		port:     os.Getenv("SMTP_PORT"),
		user:     os.Getenv("SMTP_USER"),
		password: os.Getenv("SMTP_PASSWORD"),
		from:     os.Getenv("SMTP_FROM"),
		devLog:   os.Getenv("MAIL_DEV_LOG") == "true",
	}
	if cfg.port == "" {
		cfg.port = "587"
	}
	if cfg.host == "" {
		if cfg.devLog {
			slog.Warn("mailer: MAIL_DEV_LOG=true, emails serão apenas registrados no log (não use em produção)")
		} else {
			slog.Warn("mailer: SMTP_HOST não definido, emails de verificação não serão enviados")
		}
		return nil
	}
	if _, err := mail.ParseAddress(cfg.from); err != nil {
		return fmt.Errorf("SMTP_FROM inválido: %w", err)
	}
	return nil
}

// Send envia um email de texto simples.
func Send(to, subject, body string) error {
	if _, err := mail.ParseAddress(to); err != nil || strings.ContainsAny(to, "\r\n") {
		return fmt.Errorf("destinatário inválido")
	}
	if cfg.host == "" {
		if cfg.devLog {
			slog.Info("mailer (dev): email não enviado", "to", to, "subject", subject, "body", body)
			return nil
		}
		return ErrNotConfigured
	}

	msg := strings.Join([]string{
		"From: " + cfg.from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
	}, "\r\n")

	fromAddr, _ := mail.ParseAddress(cfg.from)
	var auth smtp.Auth
	if cfg.user != "" {
		// PlainAuth recusa enviar credenciais sem TLS (exceto para localhost).
		auth = smtp.PlainAuth("", cfg.user, cfg.password, cfg.host)
	}
	addr := net.JoinHostPort(cfg.host, cfg.port)

	if cfg.port != "465" {
		// SendMail usa STARTTLS automaticamente quando o servidor oferece.
		return smtp.SendMail(addr, auth, fromAddr.Address, []string{to}, []byte(msg))
	}

	// Porta 465: TLS implícito.
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, cfg.host)
	if err != nil {
		return err
	}
	defer c.Close()
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return err
		}
	}
	if err := c.Mail(fromAddr.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
