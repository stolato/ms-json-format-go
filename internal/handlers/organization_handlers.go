package handlers

import (
	"api-go/dtos"
	"api-go/internal/models"
	"api-go/internal/repositories"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stolato/validator_fiber"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrganizationHandler struct {
	organizationRepo repositories.OrganizationRepository
}

func NewOrganizationHandler(repo repositories.OrganizationRepository) *OrganizationHandler {
	return &OrganizationHandler{
		organizationRepo: repo,
	}
}

func (repo *OrganizationHandler) FindAllTimes(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	limit := c.Query("limit", "20")
	page := c.Query("page", "0")

	limit64, _ := strconv.ParseInt(limit, 10, 64)
	page64, _ := strconv.ParseInt(page, 10, 64)

	results, err := repo.organizationRepo.FindAllMyOrgs(userID, page64, limit64)
	if err != nil {
		slog.Error(err.Error())
	}
	return c.JSON(results)
}

func (repo *OrganizationHandler) AddTime(c fiber.Ctx) error {
	userID := c.Locals("user_id")
	var dto dtos.OrganizationStoreDTO
	var itemSave models.OrganizationModel
	if err := c.Bind().Body(&dto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
	}
	if errs := validator_fiber.Validator(dto); errs != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errs)
	}
	userOwner := []models.User{}
	id := fmt.Sprintf("%s", userID)
	_id, _ := primitive.ObjectIDFromHex(id)
	userOwner = append(userOwner, models.User{Id: _id})
	itemSave.Users = &userOwner
	itemSave.Name = dto.Name
	itemSave.OwnerId = id
	itemSave.CreatedAt = time.Now()
	itemSave.UpdateAt = time.Now()
	saveItem, err := repo.organizationRepo.Store(itemSave)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
	}
	return c.JSON(saveItem)
}

func (repo *OrganizationHandler) DeleteOrganization(c fiber.Ctx) error {
	idStr := c.Params("id")
	userID := c.Locals("user_id")
	_id, errP := primitive.ObjectIDFromHex(idStr)
	if errP != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "Not object to mongoID"})
	}

	result, err := repo.organizationRepo.Delete(bson.D{{"_id", _id}, {"ownerId", userID}})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(err)
	}
	return c.JSON(result)
}

func (repo *OrganizationHandler) AddUser(c fiber.Ctx) error {
	userID := c.Locals("user_id")
	idStr := c.Params("id")
	_id, errP := primitive.ObjectIDFromHex(idStr)
	if errP != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "Not object to mongoID"})
	}
	var addDto dtos.AddUserOrganizationDTO
	if err := c.Bind().Body(&addDto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
	}
	if errs := validator_fiber.Validator(addDto); errs != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errs)
	}
	saveItem, err := repo.organizationRepo.AddUser(addDto, _id, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(err)
	}
	return c.JSON(saveItem)
}

func (repo *OrganizationHandler) RemoveUser(c fiber.Ctx) error {
	userID := c.Locals("user_id")
	idStr := c.Params("id")
	userId := c.Params("userId")
	_id, errP := primitive.ObjectIDFromHex(idStr)
	if errP != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "Not object to mongoID"})
	}
	removeUser, err := repo.organizationRepo.RemoveUser(userId, _id, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(err)
	}
	return c.JSON(removeUser)
}
