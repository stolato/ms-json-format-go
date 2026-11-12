package handlers

import (
	"api-go/internal/auth"
	"api-go/internal/mailer"
	"api-go/internal/models"
	"api-go/internal/repositories"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stolato/validator_fiber"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	emailVerificationTTL = 24 * time.Hour
	resendCooldown       = time.Minute
)

type verifyEmailRequest struct {
	Token string `json:"token" validate:"required"`
}

// sendVerificationEmail cria um token de verificação e envia o link por email.
// O envio é assíncrono para não atrasar o cadastro (o shutdown espera ele
// terminar); falhas vão para o log.
func (a *AuthHandler) sendVerificationEmail(userID, email string) error {
	token, hash, err := auth.NewOpaqueToken()
	if err != nil {
		return err
	}
	if err := a.verifications.Create(hash, userID, email, time.Now().Add(emailVerificationTTL)); err != nil {
		return err
	}
	link := a.appURL + "/verify-email?token=" + token
	body := fmt.Sprintf("Olá!\r\n\r\nConfirme seu email acessando o link abaixo (válido por 24 horas):\r\n\r\n%s\r\n\r\nSe você não criou uma conta, ignore este email.\r\n", link)
	mailer.SendAsync(email, "Confirme seu email", body, func(err error) {
		slog.Error("falha ao enviar email de verificação", "user_id", userID, "error", err)
	})
	return nil
}

// VerifyEmail confirma o email a partir do token recebido por email.
func (a *AuthHandler) VerifyEmail(c fiber.Ctx) error {
	var body verifyEmailRequest
	if err := c.Bind().JSON(&body); err != nil {
		return invalidBody(c)
	}
	if err := validator_fiber.Validator(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(err)
	}

	userID, email, err := a.verifications.Consume(auth.HashSecret(body.Token))
	if err == nil {
		err = a.repo.MarkEmailVerified(userID, email)
	}
	if errors.Is(err, repositories.ErrVerificationInvalid) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "INVALID_OR_EXPIRED_TOKEN"})
	}
	if err != nil {
		slog.Error("verify-email", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "INTERNAL_ERROR"})
	}
	if err := a.verifications.DeleteForUser(userID); err != nil {
		slog.Warn("verify-email: falha ao limpar tokens antigos", "error", err)
	}
	return c.JSON(fiber.Map{"message": "Success"})
}

// ResendVerification reenvia o email de verificação do usuário autenticado.
func (a *AuthHandler) ResendVerification(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	user, err := a.findUser(userID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "user not found"})
	}
	if user.IsEmailVerified() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ALREADY_VERIFIED"})
	}
	last, err := a.verifications.LastCreatedAt(userID)
	if err != nil {
		slog.Error("resend-verification", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "INTERNAL_ERROR"})
	}
	if time.Since(last) < resendCooldown {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"message": "TRY_AGAIN_LATER"})
	}
	if err := a.sendVerificationEmail(userID, user.Email); err != nil {
		slog.Error("resend-verification", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "INTERNAL_ERROR"})
	}
	return c.JSON(fiber.Map{"message": "Success"})
}

func (a *AuthHandler) findUser(userID string) (models.User, error) {
	_id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return models.User{}, err
	}
	return a.repo.FindMe(&models.User{Id: _id})
}
