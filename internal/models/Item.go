package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Item struct {
	Id primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	// Slug é o identificador público e aleatório usado nos links (o _id do
	// Mongo é previsível e não serve como segredo do link).
	Slug string `json:"slug" bson:"slug,omitempty"`
	// SlugLegacy marca itens anteriores ao slug, que ainda aceitam o link
	// antigo pelo _id enquanto LEGACY_ID_LINKS não for "false".
	SlugLegacy bool `json:"-" bson:"slugLegacy,omitempty"`
	// ViewOnce: o conteúdo só é entregue uma vez (POST /items/:slug/reveal) e
	// o item é apagado em seguida. GET e listagens nunca trazem o JSON.
	ViewOnce       bool               `json:"view_once" bson:"viewOnce,omitempty"`
	Json           string             `json:"json" bson:"json"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt"`
	UpdateAt       time.Time          `json:"updateAt" bson:"updateAt"`
	Ip             string             `json:"ip" bson:"ip"`
	UserId         string             `json:"user_id" bson:"user_id"`
	OrganizationId string             `json:"organization_id,omitempty" bson:"organizationId"`
	Private        bool               `json:"private" bson:"private"`
	ExpirateAt     time.Time          `json:"expirateAt,omitempty" bson:"expirateAt,omitempty"`
	Name           string             `json:"name" bson:"name"`
	Views          int32              `json:"views" bson:"views"`
	Organization   *OrganizationModel `json:"organization,omitempty" bson:"organization,omitempty"`
}
