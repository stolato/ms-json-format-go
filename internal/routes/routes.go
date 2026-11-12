package routes

import (
	"api-go/internal/handlers"
	"api-go/internal/middleware"
	"api-go/internal/repositories"

	"github.com/gofiber/fiber/v3"
)

// SetupRoutes registra as rotas HTTP. Repositórios e autenticação são criados
// pelo chamador (cmd/main.go) e compartilhados com o websocket.
func SetupRoutes(app *fiber.App, repos *repositories.Repositories, authMW *middleware.Auth, appURL string) {
	itemsHandler := handlers.NewItemsHandler(repos.Items, repos.Organizations)
	authHandler := handlers.NewAuthHandler(repos.Users, repos.Sessions, repos.EmailVerifications, appURL)
	userHandler := handlers.NewUserHandler(repos.Users)
	orgHandler := handlers.NewOrganizationHandler(repos.Organizations)

	// Routes
	app.Get("/items/:id", authMW.Optional, itemsHandler.Show)
	app.Put("/items/:id", authMW.Optional, middleware.UpdateItemRateLimit, itemsHandler.UpdateItem)
	app.Post("/items/:id/reveal", authMW.Optional, middleware.RevealItemRateLimit, itemsHandler.Reveal)
	app.Post("/items", authMW.Optional, middleware.CreateItemRateLimit, itemsHandler.Store)
	app.Post("/auth", middleware.LoginRateLimit, middleware.LoginFailuresPerEmail, authHandler.Login)
	app.Post("/register", middleware.RegisterRateLimit, authHandler.Register)
	app.Post("/refresh", middleware.RefreshRateLimit, authHandler.RefreshToken)
	app.Post("/logout", authHandler.Logout)
	app.Post("/logout-all", authMW.Required, authHandler.LogoutAll)
	app.Post("/verify-email", middleware.VerifyEmailRateLimit, authHandler.VerifyEmail)
	app.Post("/verify-email/resend", authMW.Required, authHandler.ResendVerification)

	userRouter := app.Group("/user", authMW.Required)
	userRouter.Get("/me", userHandler.Me)
	userRouter.Get("/settings", userHandler.GetSettings)
	userRouter.Put("/settings", userHandler.UpdateSettings)

	itemsRouter := app.Group("/items", authMW.Required)
	itemsRouter.Get("/", itemsHandler.AllItems)
	itemsRouter.Delete("/:id", itemsHandler.DeleteItem)

	organizationRouter := app.Group("/organization", authMW.Required)
	organizationRouter.Get("/", orgHandler.FindAllTimes)
	organizationRouter.Post("/", orgHandler.AddTime)
	organizationRouter.Delete("/:id", orgHandler.DeleteOrganization)
	organizationRouter.Post("/:id/users", orgHandler.AddUser)
	organizationRouter.Delete("/:id/users/:userId", orgHandler.RemoveUser)

	app.Get("/", func(c fiber.Ctx) error {
		return c.JSON(map[string]string{"name": "MS Items", "version": "1.2.0"})
	})
}
