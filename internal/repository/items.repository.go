package repository

import (
	"api-go/internal/models"
	"context"
	"errors"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collection = "items"

type ItemsRepository struct {
	DB *mongo.Client
}

type ResultItems struct {
	TotalItems int           `json:"total_items"`
	Page       int64         `json:"page"`
	Limit      int64         `json:"limit"`
	Data       []models.Item `json:"data"`
}

func NewItemsRepository(db *mongo.Client) *ItemsRepository {
	return &ItemsRepository{
		DB: db,
	}
}

func (r *ItemsRepository) FindAll(filter bson.D, page int64, limit int64) (ResultItems, error) {
	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSkip(limit * page)
	cursor, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).Find(context.TODO(), filter, findOptions)
	if err != nil {
		log.Println(err.Error())
	}
	var results []models.Item
	if err = cursor.All(context.TODO(), &results); err != nil {
		log.Println(err.Error())
	}
	count, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).CountDocuments(context.TODO(), filter)
	return ResultItems{Data: results, Page: page, Limit: limit, TotalItems: int(count)}, err
}

func (r *ItemsRepository) FindOne(_id primitive.ObjectID, update bool) (models.Item, error) {
	result := models.Item{}
	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).FindOne(context.TODO(), bson.D{{"_id", _id}}).Decode(&result)
	if update {
		r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).FindOneAndUpdate(context.TODO(), bson.D{{"_id", result.Id}}, bson.D{{"$set", bson.D{
			{"views", result.Views + 1},
			{"expirateAt", time.Now().AddDate(0, 0, 7)},
			{"updateAt", time.Now()},
		}}})
	}
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
	item, err := r.FindOne(_id, false)
	if item.OrganizationId != "" {
		org := models.OrganizationModel{}
		err = r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionTime).FindOne(context.TODO(), bson.D{
			{},
		}).Decode(&org)
		if err == mongo.ErrNoDocuments {
			result, err := r.delete(bson.D{{"_id", _id}, {"user_id", user_id}})
			return result, err
		}
		if org.OwnerId == user_id {
			result, err := r.delete(bson.D{{"_id", _id}})
			return result, err
		}
	}
	find := bson.D{{"_id", _id}, {"user_id", user_id}}
	if user_id == nil {
		find = bson.D{{"_id", _id}}
	}
	result, err := r.delete(find)
	if result.DeletedCount <= 0 {
		return result, errors.New("not possible delete item")
	}
	return result, err
}

func (r *ItemsRepository) delete(filter bson.D) (*mongo.DeleteResult, error) {
	result, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).DeleteOne(context.TODO(), filter)
	return result, err
}
