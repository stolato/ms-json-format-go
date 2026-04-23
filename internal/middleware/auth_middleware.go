package middleware

import (
	"api-go/internal/database"
	"api-go/internal/handlers"
	"api-go/internal/repositories"
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AuthMiddleware decodifica e valida o token JWT, consulta o banco de dados para verificar o status do usuário
// e coloca as informações do usuário no contexto local.
func AuthMiddleware(c fiber.Ctx) error {
	db := database.DB
	userRepo := repositories.NewUserRepository(db)
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing or malformed JWT"})
	}

	headerParts := strings.Split(authHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing or malformed JWT"})
	}

	tokenString := headerParts[1]

	claims := &handlers.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Unexpected signing method")
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired JWT"})
	}

	_id, _ := primitive.ObjectIDFromHex(claims.ID)
	user, err := userRepo.FindOne(bson.D{{"_id", _id}})
	if err != nil {
		fmt.Println(err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	if !user.Active {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User is inactive"})
	}

	c.Locals("user_id", claims.ID)
	c.Locals("name", claims.Name)
	c.Locals("email", claims.Email)
	return c.Next()
}

// OptionalAuthMiddleware tenta extrair o JWT se presente, mas não bloqueia a rota se ausente ou inválido.
func OptionalAuthMiddleware(c fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Next()
	}

	headerParts := strings.Split(authHeader, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		return c.Next()
	}

	claims := &handlers.Claims{}
	token, err := jwt.ParseWithClaims(headerParts[1], claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Unexpected signing method")
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err == nil && token.Valid {
		c.Locals("user_id", claims.ID)
		c.Locals("name", claims.Name)
		c.Locals("email", claims.Email)
	}

	return c.Next()
}
