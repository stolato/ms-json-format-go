package dtos

type UserLoginDTO struct {
	Email    string `json:"email" bson:"email" validate:"email,required"`
	Password string `json:"password" bson:"password" validate:"required,min=6"`
}
