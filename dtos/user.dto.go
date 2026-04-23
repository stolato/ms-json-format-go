package dtos

type UserLoginDTO struct {
	Email    string `json:"email" bson:"email" validate:"email,required"`
	Password string `json:"password" bson:"password" validate:"required,min=6"`
}
type UserRegisterDTO struct {
	UserLoginDTO
	Name    string `json:"name" bson:"name" validate:"required"`
	Active  bool   `json:"active" bson:"active"`
	Setting string `json:"settings" bson:"settings"`
}
