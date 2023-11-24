package repository

import (
	"api-go/internal/models"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
	"os"
)

var collectionUser = "users"

type UserRepository struct {
	DB *mongo.Client
}

func (r *UserRepository) Register(user *models.User) (*mongo.InsertOneResult, error) {
	password := md5Func(user.Password)
	fmt.Print(password)
	user.Password = password
	result, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionUser).InsertOne(context.TODO(), user)
	return result, err
}

func (r *UserRepository) FindOne(user *models.User) (models.User, error) {
	result := models.User{}
	password := md5Func(user.Password)
	err := r.DB.Database("dbitems").Collection(collectionUser).FindOne(context.TODO(), bson.D{{"email", user.Email}, {"password", password}}).Decode(&result)
	return result, err
}

func md5Func(pass string) string {
	hash := md5.New()
	defer hash.Reset()
	hash.Write([]byte(pass))
	return hex.EncodeToString(hash.Sum(nil))
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err
}
