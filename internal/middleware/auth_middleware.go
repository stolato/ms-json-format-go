package middleware

import (
	"api-go/internal/auth"
	"api-go/internal/repositories"
	"strings"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ErrInvalidToken é retornado por ValidateToken quando o token é inválido,
// expirado, não é um access token, a sessão foi encerrada, ou o usuário não
// existe/está inativo.
var ErrInvalidToken = auth.ErrInvalidToken

// Auth autentica requisições HTTP e conexões websocket.
type Auth struct {
	sessions repositories.SessionRepository
	users    repositories.UserRepository
}

func NewAuth(sessions repositories.SessionRepository, users repositories.UserRepository) *Auth {
	return &Auth{sessions: sessions, users: users}
}

// ValidateToken valida o access token e confere no banco se a sessão dele
// continua ativa (não houve logout/revogação) e se o usuário existe e está
// ativo. Usado pelo HTTP e pelo websocket.
func (a *Auth) ValidateToken(tokenString string) (*auth.Claims, error) {
	claims, err := auth.ParseAccess(tokenString)
	if err != nil {
		return nil, ErrInvalidToken
	}
	active, err := a.sessions.IsActive(claims.SessionID, claims.ID)
	if err != nil || !active {
		return nil, ErrInvalidToken
	}
	_id, err := primitive.ObjectIDFromHex(claims.ID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	user, err := a.users.FindOne(bson.D{{"_id", _id}})
	if err != nil || !user.Active {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// bearerToken extrai o token de "Bearer <token>"; retorna "" se o formato for inválido.
func bearerToken(header string) string {
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}

func setUserLocals(c fiber.Ctx, claims *auth.Claims) {
	c.Locals("user_id", claims.ID)
	c.Locals("name", claims.Name)
	c.Locals("email", claims.Email)
}

// Required exige um JWT válido de usuário ativo e coloca os dados do
// usuário no contexto local.
func (a *Auth) Required(c fiber.Ctx) error {
	tokenString := bearerToken(c.Get("Authorization"))
	if tokenString == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing or malformed JWT"})
	}
	claims, err := a.ValidateToken(tokenString)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired JWT"})
	}
	setUserLocals(c, claims)
	return c.Next()
}

// Optional identifica o usuário se houver um JWT válido de usuário ativo;
// caso contrário segue como anônimo, sem bloquear a rota.
func (a *Auth) Optional(c fiber.Ctx) error {
	if tokenString := bearerToken(c.Get("Authorization")); tokenString != "" {
		if claims, err := a.ValidateToken(tokenString); err == nil {
			setUserLocals(c, claims)
		}
	}
	return c.Next()
}
