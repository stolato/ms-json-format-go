package middleware

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

// Limites por janela fixa. O estado fica em memória do processo: com mais de
// uma instância da API, cada uma conta separadamente.
//
// O IP vem de c.IP(). Atrás de proxy, configure PROXY_HEADER e TRUSTED_PROXIES
// (ver cmd/main.go), senão todos os clientes compartilham o IP do proxy.

func tooManyRequests(c fiber.Ctx) error {
	return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"message": "TOO_MANY_REQUESTS"})
}

func newLimiter(prefix string, max int, window time.Duration, key func(fiber.Ctx) string) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   window,
		KeyGenerator: func(c fiber.Ctx) string { return prefix + ":" + key(c) },
		LimitReached: tooManyRequests,
	})
}

func byIP(c fiber.Ctx) string { return c.IP() }

// byUserOrIP usa o usuário autenticado quando houver (rodar depois do
// middleware de auth); senão o IP.
func byUserOrIP(c fiber.Ctx) string {
	if id, _ := c.Locals("user_id").(string); id != "" {
		return "u:" + id
	}
	return "ip:" + c.IP()
}

var (
	// GlobalRateLimit: teto geral por IP para qualquer rota.
	GlobalRateLimit = newLimiter("global", 300, time.Minute, byIP)

	// LoginRateLimit: tentativas de login por IP.
	LoginRateLimit = newLimiter("login-ip", 20, time.Minute, byIP)

	// LoginFailuresPerEmail: falhas de login por conta (protege contra força
	// bruta distribuída em vários IPs). Logins bem-sucedidos não contam.
	LoginFailuresPerEmail = limiter.New(limiter.Config{
		Max:                    10,
		Expiration:             15 * time.Minute,
		SkipSuccessfulRequests: true,
		KeyGenerator: func(c fiber.Ctx) string {
			var body struct {
				Email string `json:"email"`
			}
			if err := json.Unmarshal(c.Body(), &body); err == nil && body.Email != "" {
				return "login-email:" + strings.ToLower(strings.TrimSpace(body.Email))
			}
			return "login-email-ip:" + c.IP()
		},
		LimitReached: tooManyRequests,
	})

	RegisterRateLimit    = newLimiter("register", 5, time.Hour, byIP)
	RefreshRateLimit     = newLimiter("refresh", 30, time.Minute, byIP)
	VerifyEmailRateLimit = newLimiter("verify-email", 10, time.Minute, byIP)

	// Reset de senha: pedir o email é bem restrito (cada pedido dispara um
	// envio); usar o token segue o mesmo limite da verificação de email.
	ForgotPasswordRateLimit = newLimiter("forgot-password", 5, 15*time.Minute, byIP)
	ResetPasswordRateLimit  = newLimiter("reset-password", 10, time.Minute, byIP)

	// Itens: criar é mais restrito; editar é mais folgado por causa da edição ao vivo.
	CreateItemRateLimit = newLimiter("items-create", 30, time.Minute, byUserOrIP)
	UpdateItemRateLimit = newLimiter("items-update", 120, time.Minute, byUserOrIP)
	RevealItemRateLimit = newLimiter("items-reveal", 30, time.Minute, byUserOrIP)
)
