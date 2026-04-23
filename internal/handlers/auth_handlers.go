package handlers

import (
	"api-go/dtos"
	"api-go/internal/models"
	"api-go/internal/repositories"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stolato/validator_fiber"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type AuthHandler struct {
	repo repositories.UserRepository
}

func NewAuthHandler(repo repositories.UserRepository) *AuthHandler {
	return &AuthHandler{
		repo: repo,
	}
}

func (auth *AuthHandler) Login(c fiber.Ctx) error {
	var req dtos.UserLoginDTO
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	if err := validator_fiber.Validator(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Validation failed", "errors": err})
	}

	user, err := auth.repo.FindOne(bson.D{
		{"email", req.Email},
		{"active", true},
	})
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "user not found or not active"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid email or password"})
	}

	claims := &Claims{
		ID:    user.Id.Hex(),
		Name:  user.Name,
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	return c.JSON(fiber.Map{"token": tokenString})
}

func (auth *AuthHandler) Register(c fiber.Ctx) error {
	var userDto dtos.UserRegisterDTO
	if err := c.Bind().JSON(&userDto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
	}
	if errs := validator_fiber.Validator(userDto); errs != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errs)
	}
	_, err := auth.repo.Register(&userDto)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Success"})
}

func (auth *AuthHandler) RefreshToken(c fiber.Ctx) error {
	type tokenReqBody struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}
	var refresh tokenReqBody
	if err := c.Bind().JSON(&refresh); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
	}
	if err := validator_fiber.Validator(refresh); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(err)
	}
	token, _ := jwt.Parse(refresh.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		id := fmt.Sprintf("%v", claims["id"])
		_id, _ := primitive.ObjectIDFromHex(id)
		userModel := models.User{
			Id:     _id,
			Active: true,
		}
		user, err := auth.repo.FindMe(&userModel)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "user not found or not active"})
		}
		token, err := user.GenerateJWT()

		return c.JSON(token)
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Invalid token"})
}
