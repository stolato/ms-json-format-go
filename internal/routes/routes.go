package routes

import (
	"api-go/internal/database"
	"api-go/internal/handlers"
	"api-go/internal/middleware"
	"api-go/internal/repositories"

	"github.com/gofiber/fiber/v3"
)

func SetupRoutes(app *fiber.App) {
	db := database.DB

	//Init Repositories
	itemsRepo := repositories.NewItemsRepository(db)
	userRepo := repositories.NewUserRepository(db)
	orgRepo := repositories.NewOrganizationRepository(db)

	//Init Handlers
	itemsHandler := handlers.NewItemsHandler(itemsRepo, orgRepo)
	authHandler := handlers.NewAuthHandler(userRepo)
	userHandler := handlers.NewUserHandler(userRepo)
	orgHandler := handlers.NewOrganizationHandler(orgRepo)

	// Routes
	app.Get("/items/:id", itemsHandler.Show)
	app.Put("/items/:id", itemsHandler.UpdateItem)
	app.Post("/items", middleware.OptionalAuthMiddleware, itemsHandler.Store)
	app.Post("/auth", authHandler.Login)
	app.Post("/register", authHandler.Register)
	app.Post("/refresh", authHandler.RefreshToken)

	userRouter := app.Group("/user", middleware.AuthMiddleware)
	userRouter.Get("/me", userHandler.Me)
	userRouter.Get("/settings", userHandler.GetSettings)
	userRouter.Put("/settings", userHandler.UpdateSettings)

	itemsRouter := app.Group("/items", middleware.AuthMiddleware)
	itemsRouter.Get("/", itemsHandler.AllItems)
	itemsRouter.Delete("/:id", itemsHandler.DeleteItem)

	organizationRouter := app.Group("/organization", middleware.AuthMiddleware)
	organizationRouter.Get("/", orgHandler.FindAllTimes)
	organizationRouter.Post("/", orgHandler.AddTime)
	organizationRouter.Delete("/:id", orgHandler.DeleteOrganization)
	organizationRouter.Post("/:id/users", orgHandler.AddUser)
	organizationRouter.Delete("/:id/users/:userId", orgHandler.RemoveUser)

	app.Get("/", func(c fiber.Ctx) error {
		return c.JSON(map[string]string{"name": "MS Items", "version": "1.1.0"})
	})
}
