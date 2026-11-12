package middleware

import (
	"api-go/internal/auth"
	"api-go/internal/database"
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

// ValidateToken valida o access token e confere no banco se a sessão dele
// continua ativa (não houve logout/revogação) e se o usuário existe e está
// ativo. Usado pelo HTTP e pelo websocket.
func ValidateToken(tokenString string) (*auth.Claims, error) {
	claims, err := auth.ParseAccess(tokenString)
	if err != nil {
		return nil, ErrInvalidToken
	}
	active, err := repositories.NewSessionRepository(database.DB).IsActive(claims.SessionID, claims.ID)
	if err != nil || !active {
		return nil, ErrInvalidToken
	}
	_id, err := primitive.ObjectIDFromHex(claims.ID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	user, err := repositories.NewUserRepository(database.DB).FindOne(bson.D{{"_id", _id}})
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

// AuthMiddleware exige um JWT válido de usuário ativo e coloca os dados do
// usuário no contexto local.
func AuthMiddleware(c fiber.Ctx) error {
	tokenString := bearerToken(c.Get("Authorization"))
	if tokenString == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing or malformed JWT"})
	}
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired JWT"})
	}
	setUserLocals(c, claims)
	return c.Next()
}

// OptionalAuthMiddleware identifica o usuário se houver um JWT válido de
// usuário ativo; caso contrário segue como anônimo, sem bloquear a rota.
func OptionalAuthMiddleware(c fiber.Ctx) error {
	if tokenString := bearerToken(c.Get("Authorization")); tokenString != "" {
		if claims, err := ValidateToken(tokenString); err == nil {
			setUserLocals(c, claims)
		}
	}
	return c.Next()
}
