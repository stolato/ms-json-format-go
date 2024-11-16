package api

import (
	"api-go/internal/controllers"
	"api-go/internal/metrics"
	"api-go/internal/midleware"
	"api-go/internal/repository"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/mongo"
)

type RouterAPI struct {
	DB *mongo.Client
}

var tokenAuth *jwtauth.JWTAuth

func (router *RouterAPI) InitRouter() {
	tokenAuth = jwtauth.New("HS256", []byte(os.Getenv("JWT_SECRET")), nil, jwt.WithAcceptableSkew(30*time.Second))
	r := chi.NewRouter()
	Cors := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"*"},
	})
	r.Use(Cors.Handler)
	r.Use(midleware.MeasureResponseDuration)

	responseTimeHistogram := metrics.NewMetricsHistorigram()

	mainRepository := repository.RepositoryMain{DB: router.DB}
	metricsHello := controllers.MetricsHello{
		Duration: responseTimeHistogram.Duration,
		Summary:  responseTimeHistogram.Summary,
	}

	itemsHandle := controllers.Repository{
		MainResp: mainRepository.Repositorys(),
		Duration: responseTimeHistogram.Duration,
		Summary:  responseTimeHistogram.Summary,
	}
	userHandle := controllers.RepositoryUser{User: *mainRepository.Repositorys().User}
	organizationHandle := controllers.OrganizationController{Repo: mainRepository.Repositorys()}
	checkUserMiddleware := midleware.CheckUserMidleware{Repository: *mainRepository.Repositorys().User}

	//r.Get("/items", itemsHandle.FindAllItems)
	r.Use(jwtauth.Verifier(tokenAuth))
	r.Post("/register", userHandle.RegisterController)
	r.Post("/refresh", userHandle.RefreshToken)
	r.Post("/auth", userHandle.AuthController)
	r.Get("/items/{id}", itemsHandle.FindOneItem)
	r.Post("/items", itemsHandle.AddItem)
	r.Put("/items/{id}", itemsHandle.UpdateItem)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(checkUserMiddleware.CheckUser)
		r.Get("/items", itemsHandle.FindAllItems)
		r.Delete("/items/{id}", itemsHandle.DeleteItem)
		r.Get("/user/me", userHandle.Me)
		r.Put("/user/settings", userHandle.UpdateSettings)
		r.Get("/user/settings", userHandle.GetSettings)
		r.Get("/organization", organizationHandle.FindAllTimes)
		r.Post("/organization", organizationHandle.AddTime)
		r.Delete("/organization/{id}", organizationHandle.DeleteOrganization)
		r.Post("/organization/{id}/users", organizationHandle.AddUser)
		r.Delete("/organization/{id}/users/{userId}", organizationHandle.RemoveUser)
	})

	r.Get("/", metricsHello.HelloWord)

	port := os.Getenv("PORT")

	slog.Info("Server started on port " + port)
	err := http.ListenAndServe(":"+port, r)
	if err != nil {
		log.Fatalln(err.Error())
	}
}
