package repositories

import (
	"api-go/dtos"
	"api-go/internal/models"
	"context"
	"errors"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

var collectionUser = "users"

type UserRepository interface {
	Register(user *dtos.UserRegisterDTO) (*mongo.InsertOneResult, error)
	FindOne(filter bson.D) (models.User, error)
	FindMe(user *models.User) (models.User, error)
	UpdateSettings(userId string, settings string)
}

type userRepository struct {
	DB *mongo.Client
}

func NewUserRepository(db *mongo.Client) UserRepository {
	return &userRepository{DB: db}
}

func (r *userRepository) Register(userDto *dtos.UserRegisterDTO) (*mongo.InsertOneResult, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userDto.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := models.User{
		Email:    userDto.Email,
		Password: string(hashedPassword),
		Active:   true,
		Setting:  "{\"dark_mode\":true,\"preview\":true}",
	}
	check, err := r.FindOne(bson.D{{"email", user.Email}})
	if check.Email != "" {
		return nil, errors.New("email ja cadastrado")
	}
	result, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionUser).InsertOne(context.TODO(), user)
	return result, err
}

func (r *userRepository) FindOne(filter bson.D) (models.User, error) {
	result := models.User{}
	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionUser).FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (r *userRepository) UpdateSettings(userId string, settings string) {
	update := bson.D{{"$set", bson.D{
		{"settings", settings},
		{"updateAt", time.Now()},
	}}}
	upsert := true
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}
	_id, _ := primitive.ObjectIDFromHex(userId)
	r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionUser).FindOneAndUpdate(context.TODO(), bson.D{{"_id", _id}}, update, &opt)
}

func (r *userRepository) FindMe(user *models.User) (models.User, error) {
	result := models.User{}
	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionUser).FindOne(context.TODO(), bson.D{{"_id", user.Id}}).Decode(&result)
	return result, err
}

func CheckPasswordHash(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err
}
