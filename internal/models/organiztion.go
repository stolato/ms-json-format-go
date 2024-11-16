package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrganizationModel struct {
	Id        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name      string             `json:"name" bson:"name,omitempty" validate:"required"`
	CreatedAt time.Time          `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	UpdateAt  time.Time          `json:"updateAt,omitempty" bson:"updateAt,omitempty"`
	OwnerId   string             `json:"owner_id,omitempty" bson:"ownerId,omitempty"`
	Users     *[]User            `json:"users,omitempty" bson:"users,omitempty"`
}
