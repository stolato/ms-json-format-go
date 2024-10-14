package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrganizationModel struct {
	Id        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name      string             `json:"name" bson:"name" validate:"required"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdateAt  time.Time          `json:"updateAt" bson:"updateAt"`
	OwnerId   string             `json:"owner_id" bson:"ownerId"`
	Users     []User             `json:"users" bson:"users"`
}
