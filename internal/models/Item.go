package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Item struct {
	Id             primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Json           string             `json:"json" bson:"json"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt"`
	UpdateAt       time.Time          `json:"updateAt" bson:"updateAt"`
	Ip             string             `json:"ip" bson:"ip"`
	UserId         interface{}        `json:"user_id" bson:"user_id"`
	OrganizationId string             `json:"organization_id,omitempty" bson:"organizationId"`
	Private        bool               `json:"private" bson:"private"`
	ExpirateAt     time.Time          `json:"expirateAt,omitempty" bson:"expirateAt,omitempty"`
	Name           string             `json:"name" bson:"name"`
	Views          int32              `json:"views" bson:"views"`
	Organization   *OrganizationModel `json:"organization,omitempty" bson:"organization,omitempty"`
}
