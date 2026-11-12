package handlers

import (
	"api-go/internal/models"
	"api-go/internal/repositories"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// maxSettingsSize limita as preferências do usuário (string JSON livre).
const maxSettingsSize = 10 * 1024

type UserHandler struct {
	userRepository repositories.UserRepository
}

func NewUserHandler(repo repositories.UserRepository) *UserHandler {
	return &UserHandler{
		userRepository: repo,
	}
}

func (u *UserHandler) UpdateSettings(c fiber.Ctx) error {
	id := c.Locals("user_id")
	var settings models.User

	if err := c.Bind().JSON(&settings); err != nil {
		return invalidBody(c)
	}
	if len(settings.Setting) > maxSettingsSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "SETTINGS_TOO_LARGE"})
	}
	u.userRepository.UpdateSettings(id.(string), settings.Setting)
	return c.JSON(settings)
}

func (u *UserHandler) GetSettings(c fiber.Ctx) error {
	id := c.Locals("user_id")
	_id, _ := primitive.ObjectIDFromHex(id.(string))
	mUser := models.User{Id: _id}
	user, _ := u.userRepository.FindMe(&mUser)
	return c.JSON(map[string]string{"settings": user.Setting})
}

func (u *UserHandler) Me(c fiber.Ctx) error {
	id, _ := c.Locals("user_id").(string)
	_id, _ := primitive.ObjectIDFromHex(id)
	user, err := u.userRepository.FindMe(&models.User{Id: _id})
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "user not found"})
	}
	return c.JSON(fiber.Map{
		"message":        "Welcome!",
		"user_id":        id,
		"email":          user.Email,
		"name":           user.Name,
		"email_verified": user.IsEmailVerified(),
	})
}
