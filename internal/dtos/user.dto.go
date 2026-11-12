package dtos

type UserLoginDTO struct {
	Email    string `json:"email" bson:"email" validate:"required,max=254,email"`
	Password string `json:"password" bson:"password" validate:"required,min=6,max=72"`
}
type UserRegisterDTO struct {
	UserLoginDTO
	Name    string `json:"name" bson:"name" validate:"required,max=100"`
	Active  bool   `json:"active" bson:"active"`
	Setting string `json:"settings" bson:"settings"`
}

type ForgotPasswordDTO struct {
	Email string `json:"email" validate:"required,max=254,email"`
}

type ResetPasswordDTO struct {
	Token    string `json:"token" validate:"required,max=128"`
	Password string `json:"password" validate:"required,min=6,max=72"`
}
