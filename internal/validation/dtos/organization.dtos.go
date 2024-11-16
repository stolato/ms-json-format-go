package dtos

type OrganizationStoreDTO struct {
	Name string `json:"name" bson:"name" validate:"required"`
}

type AddUserOrganizationDTO struct {
	Email string `json:"email" validate:"email,required"`
}
