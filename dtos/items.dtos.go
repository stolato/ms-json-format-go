package dtos

type ItemStoreDTO struct {
	Json           string `json:"json" validate:"required"`
	Name           string `json:"name,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
}
