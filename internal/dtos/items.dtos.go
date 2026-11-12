package dtos

type ItemStoreDTO struct {
	Json           string `json:"json" validate:"required"`
	Name           string `json:"name,omitempty" validate:"max=200"`
	OrganizationID string `json:"organization_id,omitempty" validate:"max=24"`
	Private        bool   `json:"private,omitempty"`
	ViewOnce       bool   `json:"view_once,omitempty"`
}

type ItemUpdateDTO struct {
	Json string `json:"json" validate:"required"`
}
