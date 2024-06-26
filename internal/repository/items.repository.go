package repository

import (
	"api-go/internal/models"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"time"
)

var collection = "items"

type ItemsRepository struct {
	DB *mongo.Client
}

func NewItemsRepository(db *mongo.Client) *ItemsRepository {
	return &ItemsRepository{
		DB: db,
	}
}

func (r *ItemsRepository) FindAll(filter bson.D, findOptions *options.FindOptions) ([]models.Item, error) {
	cursor, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).Find(context.TODO(), filter, findOptions)
	if err != nil {
		log.Fatal(err)
	}
	var results []models.Item
	if err = cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}
	return results, err
}

func (r *ItemsRepository) FindOne(_id primitive.ObjectID) (models.Item, error) {
	result := models.Item{}
	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).FindOne(context.TODO(), bson.D{{"_id", _id}}).Decode(&result)
	r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).FindOneAndUpdate(context.TODO(), bson.D{{"_id", result.Id}}, bson.D{{"$set", bson.D{
		{"views", result.Views + 1},
		{"expirateAt", time.Now().AddDate(0, 0, 7)},
		{"updateAt", time.Now()},
	}}})
	return result, err
}

func (r *ItemsRepository) AddItem(item models.Item) (*mongo.InsertOneResult, error) {
	result, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).InsertOne(context.TODO(), item)
	return result, err
}

func (r *ItemsRepository) UpdateItem(item models.Item, _id primitive.ObjectID) (models.Item, error) {
	update := bson.D{{"$set", bson.D{
		{"json", item.Json},
		{"expirateAt", time.Now().AddDate(0, 0, 7)},
		{"updateAt", time.Now()},
	}}}
	upsert := true
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}
	result := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection("items").FindOneAndUpdate(context.TODO(), bson.D{{"_id", _id}}, update, &opt)
	doc := models.Item{}
	err := result.Decode(&doc)
	return doc, err
}

func (r *ItemsRepository) DeleteItem(_id primitive.ObjectID, user_id interface{}) (*mongo.DeleteResult, error) {
	result, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).DeleteOne(context.TODO(), bson.D{{"_id", _id}, {"user_id", user_id}})
	return result, err
}
