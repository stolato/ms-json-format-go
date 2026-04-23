package handlers

import (
	"api-go/internal/models"
	"api-go/internal/repositories"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
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
	return c.JSON(fiber.Map{
		"message": "Welcome!",
		"user_id": c.Locals("user_id"),
		"email":   c.Locals("email"),
		"name":    c.Locals("name"),
	})
}
