package handlers

import (
	"api-go/internal/auth"
	"api-go/internal/dtos"
	"api-go/internal/mailer"
	"api-go/internal/repositories"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stolato/validator_fiber"
	"golang.org/x/crypto/bcrypt"
)

const (
	passwordResetTTL      = time.Hour
	passwordResetCooldown = time.Minute
)

// ForgotPassword envia um link de redefinição de senha para o email, se ele
// pertencer a uma conta ativa. A resposta é sempre a mesma, exista a conta ou
// não, para não revelar quais emails estão cadastrados.
func (a *AuthHandler) ForgotPassword(c fiber.Ctx) error {
	var body dtos.ForgotPasswordDTO
	if err := c.Bind().JSON(&body); err != nil {
		return invalidBody(c)
	}
	if err := validator_fiber.Validator(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(err)
	}

	if user, err := a.repo.FindActiveByEmail(body.Email); err == nil {
		if err := a.sendPasswordResetEmail(user.Id.Hex(), user.Email); err != nil {
			slog.Error("forgot-password", "error", err)
		}
	}
	return c.JSON(fiber.Map{"message": "Success"})
}

// sendPasswordResetEmail cria um token de reset e envia o link por email. Se
// já houve um pedido há menos de passwordResetCooldown, não faz nada (evita
// usar a rota para lotar a caixa de entrada de alguém).
func (a *AuthHandler) sendPasswordResetEmail(userID, email string) error {
	last, err := a.resets.LastCreatedAt(userID)
	if err != nil {
		return err
	}
	if time.Since(last) < passwordResetCooldown {
		return nil
	}
	token, hash, err := auth.NewOpaqueToken()
	if err != nil {
		return err
	}
	if err := a.resets.Create(hash, userID, email, time.Now().Add(passwordResetTTL)); err != nil {
		return err
	}
	link := a.appURL + "/reset-password?token=" + token
	body := fmt.Sprintf("Olá!\r\n\r\nRecebemos um pedido para redefinir a senha da sua conta. Acesse o link abaixo (válido por 1 hora):\r\n\r\n%s\r\n\r\nSe você não fez esse pedido, ignore este email: sua senha continua a mesma.\r\n", link)
	mailer.SendAsync(email, "Redefinição de senha", body, func(err error) {
		slog.Error("falha ao enviar email de redefinição de senha", "user_id", userID, "error", err)
	})
	return nil
}

// ResetPassword troca a senha a partir do token recebido por email. O token é
// de uso único; ao concluir, os demais tokens de reset do usuário são apagados
// e todas as sessões abertas são encerradas.
func (a *AuthHandler) ResetPassword(c fiber.Ctx) error {
	var body dtos.ResetPasswordDTO
	if err := c.Bind().JSON(&body); err != nil {
		return invalidBody(c)
	}
	if err := validator_fiber.Validator(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(err)
	}

	// Gera o hash antes de consumir o token, para uma falha aqui não queimá-lo.
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("reset-password: falha ao gerar hash", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "INTERNAL_ERROR"})
	}

	userID, email, err := a.resets.Consume(auth.HashSecret(body.Token))
	if err == nil {
		err = a.repo.ResetPassword(userID, email, string(passwordHash))
	}
	if errors.Is(err, repositories.ErrTokenInvalid) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "INVALID_OR_EXPIRED_TOKEN"})
	}
	if err != nil {
		slog.Error("reset-password", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "INTERNAL_ERROR"})
	}

	if err := a.resets.DeleteForUser(userID); err != nil {
		slog.Warn("reset-password: falha ao limpar tokens antigos", "error", err)
	}
	if err := a.sessions.RevokeAllForUser(userID); err != nil {
		slog.Error("reset-password: falha ao revogar sessões", "user_id", userID, "error", err)
	}
	return c.JSON(fiber.Map{"message": "Success"})
}
