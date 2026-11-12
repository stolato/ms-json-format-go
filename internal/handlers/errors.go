package handlers

import (
	"api-go/internal/repositories"
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"
)

// respondError devolve ao cliente só mensagens de erros públicos (400). Erros
// internos (Mongo, criptografia...) vão para o log e viram 500 genérico, sem
// expor detalhes da infraestrutura.
func respondError(c fiber.Ctx, err error) error {
	var pub *repositories.PublicError
	if errors.As(err, &pub) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": pub.Error()})
	}
	slog.Error("erro interno", "method", c.Method(), "route", c.Route().Path, "error", err)
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "INTERNAL_ERROR"})
}

// invalidBody responde a corpos que não puderam ser lidos.
func invalidBody(c fiber.Ctx) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "INVALID_BODY"})
}
