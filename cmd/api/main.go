package main

import (
	"api-go/config"
	"api-go/internal/controllers"
	"api-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/joho/godotenv"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"
)

var tokenAuth *jwtauth.JWTAuth

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal(err)
	}
	tokenAuth = jwtauth.New("HS256", []byte(os.Getenv("JWT_SECRET")), nil, jwt.WithAcceptableSkew(30*time.Second))
	r := chi.NewRouter()
	Cors := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"*"},
	})
	r.Use(Cors.Handler)

	db, err := config.DB()
	repo := repository.ItemsRepository{DB: db}
	repoUser := repository.UserRepository{DB: db}

	itemsHandle := controllers.Repository{Items: repo}
	userHandle := controllers.RepositoryUser{User: repoUser}

	//r.Get("/items", itemsHandle.FindAllItems)
	r.Use(jwtauth.Verifier(tokenAuth))
	r.Post("/register", userHandle.RegisterController)
	r.Post("/auth", userHandle.AuthController)
	r.Get("/items/{id}", itemsHandle.FindOneItem)
	r.Post("/items", itemsHandle.AddItem)
	r.Put("/items/{id}", itemsHandle.UpdateItem)

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Get("/items", itemsHandle.FindAllItems)
		r.Delete("/items/{id}", itemsHandle.DeleteItem)
		r.Get("/user/me", userHandle.Me)
	})

	r.Get("/", helloWord)

	port := os.Getenv("PORT")

	slog.Info("Server started on port " + port)
	err = http.ListenAndServe(":"+port, r)
	if err != nil {
		return
	}
}

func helloWord(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, "Hi!! :D")
}
