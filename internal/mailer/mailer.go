// Package mailer envia emails via Amazon SES (API v2).
//
// Configuração (lida em internal/config):
//
//	SES_FROM               remetente verificado no SES (ex.: "Json Format <no-reply@dominio.com>")
//	AWS_REGION             região do SES (ex.: us-east-1, sa-east-1)
//	SES_CONFIGURATION_SET  opcional: configuration set para eventos/métricas
//	MAIL_DEV_LOG=true      sem SES_FROM, registra o email no log em vez de enviar
//	                       (somente para desenvolvimento: o log conterá o link).
//
// Credenciais seguem a cadeia padrão da AWS: AWS_ACCESS_KEY_ID /
// AWS_SECRET_ACCESS_KEY, AWS_PROFILE, ou role da instância/task (IAM).
package mailer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	appconfig "api-go/internal/config"
)

const sendTimeout = 15 * time.Second

type config struct {
	from, configSet string
	devLog          bool
	client          *sesv2.Client
}

var cfg config

var ErrNotConfigured = errors.New("envio de email não configurado (SES_FROM)")

// Init aplica a configuração. Sem SES_FROM o servidor sobe, mas emails não
// são enviados (a menos que MAIL_DEV_LOG=true).
func Init(mc appconfig.Mail) error {
	cfg = config{
		from:      mc.From,
		configSet: mc.ConfigurationSet,
		devLog:    mc.DevLog,
	}
	if cfg.from == "" {
		if cfg.devLog {
			slog.Warn("mailer: MAIL_DEV_LOG=true, emails serão apenas registrados no log (não use em produção)")
		} else {
			slog.Warn("mailer: SES_FROM não definido, emails de verificação não serão enviados")
		}
		return nil
	}
	if _, err := mail.ParseAddress(cfg.from); err != nil {
		return fmt.Errorf("SES_FROM inválido: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("mailer: falha ao carregar configuração AWS: %w", err)
	}
	if awsCfg.Region == "" {
		return errors.New("mailer: AWS_REGION não definido")
	}
	cfg.client = sesv2.NewFromConfig(awsCfg)
	return nil
}

// Send envia um email de texto simples.
func Send(to, subject, body string) error {
	if _, err := mail.ParseAddress(to); err != nil || strings.ContainsAny(to, "\r\n") {
		return fmt.Errorf("destinatário inválido")
	}
	if cfg.client == nil {
		if cfg.devLog {
			slog.Info("mailer (dev): email não enviado", "to", to, "subject", subject, "body", body)
			return nil
		}
		return ErrNotConfigured
	}

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(cfg.from),
		Destination:      &types.Destination{ToAddresses: []string{to}},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(subject), Charset: aws.String("UTF-8")},
				Body: &types.Body{
					Text: &types.Content{Data: aws.String(body), Charset: aws.String("UTF-8")},
				},
			},
		},
	}
	if cfg.configSet != "" {
		input.ConfigurationSetName = aws.String(cfg.configSet)
	}

	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	_, err := cfg.client.SendEmail(ctx, input)
	return err
}

// inflight conta os envios assíncronos em andamento, para o shutdown esperar.
var inflight sync.WaitGroup

// SendAsync envia em segundo plano (para não atrasar a resposta HTTP). Falhas
// vão para onError. O shutdown espera os envios pendentes com Wait.
func SendAsync(to, subject, body string, onError func(error)) {
	inflight.Add(1)
	go func() {
		defer inflight.Done()
		if err := Send(to, subject, body); err != nil && onError != nil {
			onError(err)
		}
	}()
}

// Wait espera os envios de SendAsync terminarem ou ctx acabar. Chame depois
// que o servidor HTTP parou de atender, para não surgirem envios novos.
func Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("mailer: envios pendentes não terminaram: %w", ctx.Err())
	}
}
