package midleware

import (
	"api-go/internal/models"
	"api-go/internal/repository"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
)

type CheckUserMidleware struct {
	Repository repository.UserRepository
}

func (c *CheckUserMidleware) CheckUser(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, claims, _ := jwtauth.FromContext(r.Context())
		id := claims["id"].(string)
		_id, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			render.Status(r, 401)
			render.JSON(w, r, map[string]string{"message": "user not active contact to support", "code": "USER_NOT_ACTIVE"})
			return
		}
		mUser := models.User{Id: _id}
		user, _ := c.Repository.FindMe(&mUser)
		if user.Active == false {
			render.Status(r, 401)
			render.JSON(w, r, map[string]string{"message": "user not active contact to support", "code": "USER_NOT_ACTIVE"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
