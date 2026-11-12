package handlers

import (
	"api-go/dtos"
	"api-go/internal/auth"
	"api-go/internal/models"
	"api-go/internal/repositories"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stolato/validator_fiber"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	repo          repositories.UserRepository
	sessions      repositories.SessionRepository
	verifications repositories.EmailVerificationRepository
}

func NewAuthHandler(repo repositories.UserRepository, sessions repositories.SessionRepository, verifications repositories.EmailVerificationRepository) *AuthHandler {
	return &AuthHandler{
		repo:          repo,
		sessions:      sessions,
		verifications: verifications,
	}
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Hash bcrypt de uma senha qualquer, usado para gastar o mesmo tempo quando o
// email não existe (evita descobrir emails cadastrados pelo tempo de resposta).
var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

func (a *AuthHandler) Login(c fiber.Ctx) error {
	var req dtos.UserLoginDTO
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	if err := validator_fiber.Validator(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Validation failed", "errors": err})
	}

	user, err := a.repo.FindActiveByEmail(req.Email)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid email or password"})
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid email or password"})
	}

	tokens, err := a.newSession(user)
	if err != nil {
		slog.Error("login: falha ao criar sessão", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}
	return c.JSON(tokens)
}

func (a *AuthHandler) Register(c fiber.Ctx) error {
	var userDto dtos.UserRegisterDTO
	if err := c.Bind().JSON(&userDto); err != nil {
		return invalidBody(c)
	}
	if errs := validator_fiber.Validator(userDto); errs != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errs)
	}
	result, err := a.repo.Register(&userDto)
	if err != nil {
		return respondError(c, err)
	}
	if id, ok := result.InsertedID.(primitive.ObjectID); ok {
		if err := a.sendVerificationEmail(id.Hex(), repositories.NormalizeEmail(userDto.Email)); err != nil {
			// A conta foi criada; o usuário pode pedir o reenvio depois.
			slog.Error("register: falha ao criar verificação de email", "error", err)
		}
	}
	return c.JSON(fiber.Map{"message": "Success"})
}

// RefreshToken troca um refresh token válido por um novo par de tokens da mesma
// sessão. Cada refresh token só pode ser usado uma vez (rotação). Reapresentar
// um token antigo fora da janela de tolerância indica roubo: todas as sessões
// do usuário são revogadas.
func (a *AuthHandler) RefreshToken(c fiber.Ctx) error {
	var body refreshRequest
	if err := c.Bind().JSON(&body); err != nil {
		return invalidBody(c)
	}
	if err := validator_fiber.Validator(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(err)
	}

	sid, currentHash, err := auth.ParseRefreshToken(body.RefreshToken)
	if err != nil {
		return unauthorized(c)
	}
	newRefresh, newHash, err := auth.NewRefreshToken(sid)
	if err != nil {
		slog.Error("refresh: falha ao gerar token", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "INTERNAL_ERROR"})
	}

	result, userID, err := a.sessions.Rotate(sid, currentHash, newHash, time.Now().Add(auth.RefreshTTL))
	if err != nil {
		slog.Error("refresh: falha ao rotacionar sessão", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "INTERNAL_ERROR"})
	}
	switch result {
	case repositories.RotateOK:
	case repositories.RotateReused:
		slog.Warn("refresh token reutilizado; revogando todas as sessões", "user_id", userID)
		if err := a.sessions.RevokeAllForUser(userID); err != nil {
			slog.Error("refresh: falha ao revogar sessões", "error", err)
		}
		return unauthorized(c)
	default:
		return unauthorized(c)
	}

	_id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return unauthorized(c)
	}
	user, err := a.repo.FindOne(bson.D{{"_id", _id}, {"active", true}})
	if err != nil {
		_ = a.sessions.Revoke(sid, newHash)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "user not found or not active"})
	}

	access, err := auth.NewAccessToken(userID, user.Name, user.Email, sid)
	if err != nil {
		slog.Error("refresh: falha ao emitir access token", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to generate token"})
	}
	return c.JSON(fiber.Map{"token": access, "refresh_token": newRefresh})
}

// Logout encerra a sessão do refresh token informado. O access token dessa
// sessão deixa de valer imediatamente.
func (a *AuthHandler) Logout(c fiber.Ctx) error {
	var body refreshRequest
	if err := c.Bind().JSON(&body); err != nil {
		return invalidBody(c)
	}
	if err := validator_fiber.Validator(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(err)
	}
	if sid, hash, err := auth.ParseRefreshToken(body.RefreshToken); err == nil {
		if err := a.sessions.Revoke(sid, hash); err != nil {
			slog.Error("logout: falha ao revogar sessão", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "INTERNAL_ERROR"})
		}
	}
	// Responde sucesso mesmo com token inválido: o resultado é o mesmo (sessão encerrada).
	return c.JSON(fiber.Map{"message": "Success"})
}

// LogoutAll encerra todas as sessões do usuário autenticado.
func (a *AuthHandler) LogoutAll(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	if err := a.sessions.RevokeAllForUser(userID); err != nil {
		slog.Error("logout-all: falha ao revogar sessões", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "INTERNAL_ERROR"})
	}
	return c.JSON(fiber.Map{"message": "Success"})
}

// newSession cria uma sessão no banco e devolve o par de tokens dela.
func (a *AuthHandler) newSession(user models.User) (fiber.Map, error) {
	userID := user.Id.Hex()
	sid, err := auth.NewSessionID()
	if err != nil {
		return nil, err
	}
	refresh, hash, err := auth.NewRefreshToken(sid)
	if err != nil {
		return nil, err
	}
	if err := a.sessions.Create(sid, userID, hash, time.Now().Add(auth.RefreshTTL)); err != nil {
		return nil, err
	}
	access, err := auth.NewAccessToken(userID, user.Name, user.Email, sid)
	if err != nil {
		return nil, err
	}
	return fiber.Map{"token": access, "refresh_token": refresh}, nil
}

func unauthorized(c fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Invalid token"})
}
