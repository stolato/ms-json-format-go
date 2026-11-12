package repositories

import (
	"api-go/dtos"
	"api-go/internal/models"
	"context"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

var collectionUser = "users"

// emailCollation compara emails sem diferenciar maiúsculas/minúsculas. É a
// mesma collation do índice único, então consultas com ela usam o índice.
var emailCollation = &options.Collation{Locale: "en", Strength: 2}

// NormalizeEmail padroniza o email antes de salvar/consultar.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

var ErrEmailTaken = newPublicError("email ja cadastrado")

type UserRepository interface {
	EnsureIndexes() error
	Register(user *dtos.UserRegisterDTO) (*mongo.InsertOneResult, error)
	FindOne(filter bson.D) (models.User, error)
	// FindActiveByEmail busca usuário ativo pelo email, sem diferenciar maiúsculas.
	FindActiveByEmail(email string) (models.User, error)
	// MarkEmailVerified confirma o email do usuário, se ainda for o mesmo.
	MarkEmailVerified(userID, email string) error
	FindMe(user *models.User) (models.User, error)
	UpdateSettings(userId string, settings string)
}

type userRepository struct {
	DB *mongo.Client
}

func NewUserRepository(db *mongo.Client) UserRepository {
	return &userRepository{DB: db}
}

// EnsureIndexes cria o índice único (case-insensitive) de email. Falha se já
// existirem emails duplicados no banco; nesse caso resolva-os e reinicie.
func (r *userRepository) EnsureIndexes() error {
	_, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionUser).Indexes().CreateOne(context.TODO(), mongo.IndexModel{
		Keys:    bson.D{{"email", 1}},
		Options: options.Index().SetUnique(true).SetCollation(emailCollation).SetName("email_unique_ci"),
	})
	return err
}

func (r *userRepository) Register(userDto *dtos.UserRegisterDTO) (*mongo.InsertOneResult, error) {
	email := NormalizeEmail(userDto.Email)
	coll := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionUser)

	// Checagem prévia (amigável); o índice único é quem garante contra corrida.
	n, err := coll.CountDocuments(context.TODO(), bson.D{{"email", email}},
		options.Count().SetCollation(emailCollation).SetLimit(1))
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrEmailTaken
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userDto.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	notVerified := false
	user := models.User{
		Email:         email,
		EmailVerified: &notVerified,
		Password:      string(hashedPassword),
		Active:        true,
		Setting:       "{\"dark_mode\":true,\"preview\":true}",
	}
	result, err := coll.InsertOne(context.TODO(), user)
	if mongo.IsDuplicateKeyError(err) {
		return nil, ErrEmailTaken
	}
	return result, err
}

func (r *userRepository) MarkEmailVerified(userID, email string) error {
	_id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}
	res, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionUser).UpdateOne(context.TODO(),
		bson.D{{"_id", _id}, {"email", NormalizeEmail(email)}},
		bson.D{{"$set", bson.D{{"emailVerified", true}}}},
		options.Update().SetCollation(emailCollation),
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrVerificationInvalid
	}
	return nil
}

func (r *userRepository) FindActiveByEmail(email string) (models.User, error) {
	result := models.User{}
	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionUser).FindOne(context.TODO(),
		bson.D{{"email", NormalizeEmail(email)}, {"active", true}},
		options.FindOne().SetCollation(emailCollation),
	).Decode(&result)
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
