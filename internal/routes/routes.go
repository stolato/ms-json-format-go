package routes

import (
	"api-go/internal/database"
	"api-go/internal/handlers"
	"api-go/internal/middleware"
	"api-go/internal/repositories"
	"log"

	"github.com/gofiber/fiber/v3"
)

func SetupRoutes(app *fiber.App) {
	db := database.DB

	//Init Repositories
	itemsRepo := repositories.NewItemsRepository(db)
	if err := itemsRepo.EnsureIndexes(); err != nil {
		log.Println("items: falha ao criar índice de slug:", err)
	}
	userRepo := repositories.NewUserRepository(db)
	orgRepo := repositories.NewOrganizationRepository(db)
	if err := userRepo.EnsureIndexes(); err != nil {
		log.Println("users: falha ao criar índice único de email (há emails duplicados?):", err)
	}
	verificationRepo := repositories.NewEmailVerificationRepository(db)
	if err := verificationRepo.EnsureIndexes(); err != nil {
		log.Println("email_verifications: falha ao criar índices:", err)
	}
	sessionRepo := repositories.NewSessionRepository(db)
	if err := sessionRepo.EnsureIndexes(); err != nil {
		log.Println("sessions: falha ao criar índices:", err)
	}

	//Init Handlers
	itemsHandler := handlers.NewItemsHandler(itemsRepo, orgRepo)
	authHandler := handlers.NewAuthHandler(userRepo, sessionRepo, verificationRepo)
	userHandler := handlers.NewUserHandler(userRepo)
	orgHandler := handlers.NewOrganizationHandler(orgRepo)

	// Routes
	app.Get("/items/:id", middleware.OptionalAuthMiddleware, itemsHandler.Show)
	app.Put("/items/:id", middleware.OptionalAuthMiddleware, middleware.UpdateItemRateLimit, itemsHandler.UpdateItem)
	app.Post("/items/:id/reveal", middleware.OptionalAuthMiddleware, middleware.RevealItemRateLimit, itemsHandler.Reveal)
	app.Post("/items", middleware.OptionalAuthMiddleware, middleware.CreateItemRateLimit, itemsHandler.Store)
	app.Post("/auth", middleware.LoginRateLimit, middleware.LoginFailuresPerEmail, authHandler.Login)
	app.Post("/register", middleware.RegisterRateLimit, authHandler.Register)
	app.Post("/refresh", middleware.RefreshRateLimit, authHandler.RefreshToken)
	app.Post("/logout", authHandler.Logout)
	app.Post("/logout-all", middleware.AuthMiddleware, authHandler.LogoutAll)
	app.Post("/verify-email", middleware.VerifyEmailRateLimit, authHandler.VerifyEmail)
	app.Post("/verify-email/resend", middleware.AuthMiddleware, authHandler.ResendVerification)

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
