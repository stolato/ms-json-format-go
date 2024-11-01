package repository

import (
	"api-go/internal/models"
	"api-go/internal/validation/dtos"
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collectionTime = "organization"

type OrganizationRepository struct {
	DB *mongo.Client
}

func (r *OrganizationRepository) Store(time models.OrganizationModel) (*mongo.InsertOneResult, error) {
	result, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionTime).InsertOne(context.TODO(), time)
	return result, err
}

func (r *OrganizationRepository) FindAll(filter bson.D, findOptions *options.FindOptions) ([]models.OrganizationModel, error) {
	cursor, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionTime).Find(context.TODO(), filter, findOptions)
	if err != nil {
		log.Fatal(err)
	}
	var results []models.OrganizationModel
	if err = cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}
	return results, err
}

func (r *OrganizationRepository) FindOne(filter bson.D) (models.OrganizationModel, error) {
	result := models.OrganizationModel{}
	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionTime).FindOne(context.TODO(), filter).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return result, errors.New("not find org")
	}
	return result, err
}

func (r *OrganizationRepository) FindAllMyOrgs(_id interface{}, page int64, limit int64) ([]models.OrganizationModel, error) {
	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSkip(limit * page)
	cursor, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionTime).Find(context.TODO(),
		bson.D{{
			"$or", bson.A{
				bson.D{{"ownerId", _id}}, bson.D{{"users._id", _id}},
			},
		}}, findOptions,
	)
	if err != nil {
		log.Fatal(err)
	}
	var results []models.OrganizationModel
	if err = cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}
	return results, err
}

func (r *OrganizationRepository) Delete(filter bson.D) (*mongo.DeleteResult, error) {
	result, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionTime).DeleteOne(context.TODO(), filter)
	if result.DeletedCount <= 0 {
		err = errors.New("not possible delete org")
	}
	return result, err
}

func (r *OrganizationRepository) FindByFilter(filter bson.D) (models.OrganizationModel, error) {
	result := models.OrganizationModel{}
	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionTime).FindOne(context.TODO(), filter).Decode(&result)
	return result, err
}

func (r *OrganizationRepository) AddUser(user dtos.AddUserOrganizationDTO, id primitive.ObjectID, ownerID interface{}) (models.OrganizationModel, error) {
	checkToOrg := models.OrganizationModel{}

	userCheck := models.User{}

	errUser := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionUser).FindOne(context.TODO(),
		bson.D{{"email", user.Email}, {"active", true}},
	).Decode(&userCheck)
	if errUser == mongo.ErrNoDocuments {
		return checkToOrg, errors.New("user not found or not active")
	}

	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionTime).FindOne(context.TODO(), bson.D{
		{"$or", bson.A{
			bson.D{{"users._id", userCheck.Id.Hex()}},
			bson.D{{"ownerId", userCheck.Id.Hex()}},
		}},
		{"_id", id},
	}).Decode(&checkToOrg)
	if err != mongo.ErrNoDocuments || checkToOrg.OwnerId == ownerID {
		return checkToOrg, errors.New("not possible add this user")
	}

	upsert := true
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}
	result := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionTime).FindOneAndUpdate(
		context.TODO(),
		bson.D{{"_id", id}, {"ownerId", ownerID}},
		bson.D{{"$push", bson.D{{"users", bson.D{{"_id", userCheck.Id.Hex()}, {"name", userCheck.Name}, {"email", userCheck.Email}}}}}},
		&opt,
	)
	doc := models.OrganizationModel{}
	err = result.Decode(&doc)
	return doc, err
}

func (r *OrganizationRepository) RemoveUser(userId string, id primitive.ObjectID, ownerID interface{}) (models.OrganizationModel, error) {
	doc := models.OrganizationModel{}
	ownerID = fmt.Sprintf("%s", ownerID)
	if userId == ownerID {
		return doc, errors.New("not possbile remove")
	}
	upsert := true
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}
	result := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionTime).FindOneAndUpdate(
		context.TODO(),
		bson.D{{"_id", id}, {"ownerId", ownerID}},
		bson.D{{"$pull", bson.D{{"users", bson.D{{"_id", userId}}}}}},
		&opt,
	)
	err := result.Decode(&doc)
	return doc, err
}
