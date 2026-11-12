package handlers

import (
	"api-go/dtos"
	"api-go/internal/access"
	"api-go/internal/models"
	"api-go/internal/repositories"
	"errors"
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

// Show busca o item pelo slug (ou pelo _id em links antigos). A visualização
// só é contada depois de confirmada a permissão.
func (items *ItemsHandler) Show(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	_id, err := items.itemsRepository.ResolveRef(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	result, err := items.itemsRepository.FindOne(_id, false)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	if !access.CanAccessItem(items.orgsRepository, userID, result) {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	if result.ViewOnce {
		// Só o aviso: o conteúdo vem em POST /items/:slug/reveal. Assim prévias
		// de link (WhatsApp, Slack...) não consomem a visualização.
		result.Json = ""
		return c.JSON(result)
	}
	if err := items.itemsRepository.IncrementViews(_id); err != nil {
		slog.Warn("show: falha ao contar visualização", "error", err)
	}
	return c.JSON(result)
}

// Reveal entrega o conteúdo de um item de visualização única e o apaga. Se
// duas pessoas tentarem ao mesmo tempo, só uma recebe; a outra recebe 404.
func (items *ItemsHandler) Reveal(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	_id, err := items.itemsRepository.ResolveRef(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	current, err := items.itemsRepository.FindOne(_id, false)
	if err != nil || !access.CanAccessItem(items.orgsRepository, userID, current) {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	if !current.ViewOnce {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "NOT_VIEW_ONCE"})
	}
	revealed, err := items.itemsRepository.ConsumeViewOnce(_id)
	if errors.Is(err, repositories.ErrItemNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(revealed)
}

func (items *ItemsHandler) AllItems(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	page64, limit64 := parsePagination(c)
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
	results, err := items.itemsRepository.FindAll(find, page64, limit64)
	// Listagens nunca expõem o conteúdo de itens de visualização única
	// (senão membros da organização leriam sem consumir).
	for i := range results.Data {
		if results.Data[i].ViewOnce {
			results.Data[i].Json = ""
		}
	}
	if err != nil {
		slog.Error(err.Error())
		if results.Data == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(map[string]string{"message": "INTERNAL_ERROR"})
		}
	}
	return c.JSON(results)
}

func (items *ItemsHandler) Store(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	var item models.Item
	var dto dtos.ItemStoreDTO
	if err := c.Bind().JSON(&dto); err != nil {
		return invalidBody(c)
	}
	if errs := validator_fiber.Validator(dto); errs != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errs)
	}
	// Item privado precisa de dono; sem login ninguém conseguiria acessá-lo depois.
	if dto.Private && userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "PRIVATE_REQUIRES_LOGIN"})
	}
	// Visualização única só faz sentido quando outra pessoa vai abrir o link.
	if dto.ViewOnce && dto.Private {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "VIEW_ONCE_NOT_ALLOWED_FOR_PRIVATE"})
	}
	if dto.OrganizationID != "" {
		_id, errP := primitive.ObjectIDFromHex(dto.OrganizationID)
		if errP != nil {
			return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "Not object to mongoID"})
		}
		org, err := items.orgsRepository.FindOne(bson.D{{"_id", _id}})
		if err != nil {
			return respondError(c, err)
		}

		resultsOrgs, err := items.orgsRepository.FindAllMyOrgs(userID, 0, 100)
		if err != nil {
			return respondError(c, err)
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
	slug, err := repositories.NewSlug()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(map[string]string{"message": "INTERNAL_ERROR"})
	}
	item.Slug = slug
	item.UserId = userID
	item.Json = dto.Json
	item.Name = dto.Name
	item.Private = dto.Private
	item.ViewOnce = dto.ViewOnce
	item.OrganizationId = dto.OrganizationID
	item.CreatedAt = time.Now()
	item.UpdateAt = time.Now()
	item.ExpirateAt = time.Now().AddDate(0, 0, 7)
	saveItem, err := items.itemsRepository.AddItem(item)
	if err != nil {
		return respondError(c, err)
	}
	// "slug" é o que deve ir no link; "InsertedID" mantido por compatibilidade.
	return c.JSON(fiber.Map{"InsertedID": saveItem.InsertedID, "slug": slug})
}

func (items *ItemsHandler) UpdateItem(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	_id, err := items.itemsRepository.ResolveRef(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	var dto dtos.ItemUpdateDTO
	if err := c.Bind().JSON(&dto); err != nil {
		return invalidBody(c)
	}
	if errs := validator_fiber.Validator(dto); errs != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errs)
	}

	current, err := items.itemsRepository.FindOne(_id, false)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	// Sem permissão responde 404 (como no Show) para não revelar que o item existe.
	if !access.CanAccessItem(items.orgsRepository, userID, current) {
		return c.Status(fiber.StatusNotFound).JSON(map[string]string{"message": "NOT_FOUND"})
	}
	if current.ViewOnce {
		// A resposta do PUT traria o conteúdo; e sobrescrever seria sabotagem.
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "VIEW_ONCE_IMMUTABLE"})
	}

	saveItem, err := items.itemsRepository.UpdateItem(models.Item{Json: dto.Json}, _id)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(saveItem)
}

func (items *ItemsHandler) DeleteItem(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	_id, err := items.itemsRepository.ResolveRef(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(map[string]string{"message": "not possible delete item"})
	}
	result, err := items.itemsRepository.DeleteItem(_id, userID)
	if err != nil {
		return respondError(c, err)
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

const maxPageLimit = 100

// parsePagination lê page/limit da query garantindo page >= 0 e 1 <= limit <= 100.
func parsePagination(c fiber.Ctx) (page int64, limit int64) {
	page, err := strconv.ParseInt(c.Query("page", "0"), 10, 64)
	if err != nil || page < 0 {
		page = 0
	}
	limit, err = strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	if err != nil || limit < 1 {
		limit = 20
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	return page, limit
}
