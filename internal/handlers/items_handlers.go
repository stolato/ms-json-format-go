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

type ItemsHandler struct {
	itemsRepository repositories.ItemsRepository
	orgsRepository  repositories.OrganizationRepository
}

func NewItemsHandler(items repositories.ItemsRepository, orgs repositories.OrganizationRepository) *ItemsHandler {
	return &ItemsHandler{
		itemsRepository: items,
		orgsRepository:  orgs,
	}
}

func (items *ItemsHandler) Show(c fiber.Ctx) error {
	idStr := c.Params("id")
	userID, _ := c.Locals("user_id").(string)
	_id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "Not object to mongoID"})
	}

	result, err := items.itemsRepository.FindOne(_id, true)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	resultsOrgs, _ := items.orgsRepository.FindAllMyOrgs(userID, 0, 100)
	var orgsId []string
	for _, value := range resultsOrgs {
		orgsId = append(orgsId, value.Id.Hex())
	}

	if result.OrganizationId != "" && !contains(orgsId, result.OrganizationId) {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}

	if result.Private && userID != result.UserId {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	return c.JSON(result)
}

func (items *ItemsHandler) AllItems(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	limit := c.Query("limit", "20")
	page := c.Query("page", "0")
	resultsOrgs, _ := items.orgsRepository.FindAllMyOrgs(userID, 0, 100)
	var orgsId []string
	for _, value := range resultsOrgs {
		orgsId = append(orgsId, value.Id.Hex())
	}
	if len(orgsId) == 0 {
		orgsId = append(orgsId, " ")
	}
	find := bson.D{{
		"$or", bson.A{
			bson.D{{"user_id", userID}},
			bson.D{{"organizationId", bson.D{{"$in", orgsId}}}},
		},
	}}
	limit64, _ := strconv.ParseInt(limit, 10, 64)
	page64, _ := strconv.ParseInt(page, 10, 64)
	fmt.Println(page64)
	results, err := items.itemsRepository.FindAll(find, page64, limit64)
	if err != nil {
		slog.Error(err.Error())
	}
	return c.JSON(results)
}

func (items *ItemsHandler) Store(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	var item models.Item
	var dto dtos.ItemStoreDTO
	if err := c.Bind().JSON(&dto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
	}
	if errs := validator_fiber.Validator(dto); errs != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errs)
	}
	if dto.OrganizationID != "" {
		_id, errP := primitive.ObjectIDFromHex(dto.OrganizationID)
		if errP != nil {
			return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "Not object to mongoID"})
		}
		org, err := items.orgsRepository.FindOne(bson.D{{"_id", _id}})
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
		}

		resultsOrgs, err := items.orgsRepository.FindAllMyOrgs(userID, 0, 100)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
		}
		var orgsId []string
		for _, value := range resultsOrgs {
			orgsId = append(orgsId, value.Id.Hex())
		}

		if !contains(orgsId, dto.OrganizationID) {
			return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "NOT_FOUND_ORG"})
		}
		item.Organization = &models.OrganizationModel{
			Name: org.Name,
			Id:   org.Id,
		}
	}
	item.UserId = userID
	item.Json = dto.Json
	item.Name = dto.Name
	item.OrganizationId = dto.OrganizationID
	item.CreatedAt = time.Now()
	item.UpdateAt = time.Now()
	item.ExpirateAt = time.Now().AddDate(0, 0, 7)
	saveItem, err := items.itemsRepository.AddItem(item)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
	}
	return c.JSON(saveItem)
}

func (items *ItemsHandler) UpdateItem(c fiber.Ctx) error {
	idStr := c.Params("id")
	_id, errP := primitive.ObjectIDFromHex(idStr)
	if errP != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "Not object to mongoID"})
	}
	var item models.Item
	if err := c.Bind().Body(&item); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
	}
	saveItem, err := items.itemsRepository.UpdateItem(item, _id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
	}
	return c.JSON(saveItem)
}

func (items *ItemsHandler) DeleteItem(c fiber.Ctx) error {
	idStr := c.Params("id")
	userID, _ := c.Locals("user_id").(string)
	_id, errP := primitive.ObjectIDFromHex(idStr)
	if errP != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "Not object to mongoID"})
	}
	result, err := items.itemsRepository.DeleteItem(_id, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": err.Error()})
	}
	return c.JSON(result)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
