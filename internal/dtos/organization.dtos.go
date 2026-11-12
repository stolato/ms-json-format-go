package dtos

type OrganizationStoreDTO struct {
	Name string `json:"name" bson:"name" validate:"required,max=100"`
}

type AddUserOrganizationDTO struct {
	Email string `json:"email" validate:"required,max=254,email"`
}
