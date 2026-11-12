package repositories

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrVerificationInvalid = errors.New("invalid or expired verification token")

// EmailVerificationRepository guarda tokens de verificação de email. Só o
// SHA-256 do token é salvo; o token em si vai apenas no email.
type EmailVerificationRepository interface {
	EnsureIndexes() error
	Create(tokenHash, userID, email string, expiresAt time.Time) error
	// Consume apaga e devolve o token (uso único). ErrVerificationInvalid se não
	// existir ou tiver expirado.
	Consume(tokenHash string) (userID, email string, err error)
	LastCreatedAt(userID string) (time.Time, error)
	DeleteForUser(userID string) error
}

type emailVerificationDoc struct {
	ID        string    `bson:"_id"` // hash do token
	UserID    string    `bson:"user_id"`
	Email     string    `bson:"email"`
	CreatedAt time.Time `bson:"createdAt"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

type emailVerificationRepository struct {
	verifications *mongo.Collection
}

func NewEmailVerificationRepository(db *mongo.Database) EmailVerificationRepository {
	return &emailVerificationRepository{verifications: db.Collection(collEmailVerifications)}
}

func (r *emailVerificationRepository) EnsureIndexes() error {
	_, err := r.verifications.Indexes().CreateMany(context.TODO(), []mongo.IndexModel{
		{Keys: bson.D{{"user_id", 1}, {"createdAt", -1}}},
		{Keys: bson.D{{"expiresAt", 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	return err
}

func (r *emailVerificationRepository) Create(tokenHash, userID, email string, expiresAt time.Time) error {
	_, err := r.verifications.InsertOne(context.TODO(), emailVerificationDoc{
		ID:        tokenHash,
		UserID:    userID,
		Email:     email,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	})
	return err
}

func (r *emailVerificationRepository) Consume(tokenHash string) (string, string, error) {
	var doc emailVerificationDoc
	err := r.verifications.FindOneAndDelete(context.TODO(), bson.D{
		{"_id", tokenHash},
		{"expiresAt", bson.D{{"$gt", time.Now()}}},
	}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", "", ErrVerificationInvalid
	}
	if err != nil {
		return "", "", err
	}
	return doc.UserID, doc.Email, nil
}

func (r *emailVerificationRepository) LastCreatedAt(userID string) (time.Time, error) {
	var doc emailVerificationDoc
	err := r.verifications.FindOne(context.TODO(), bson.D{{"user_id", userID}},
		options.FindOne().SetSort(bson.D{{"createdAt", -1}}),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, nil
	}
	return doc.CreatedAt, err
}

func (r *emailVerificationRepository) DeleteForUser(userID string) error {
	_, err := r.verifications.DeleteMany(context.TODO(), bson.D{{"user_id", userID}})
	return err
}
