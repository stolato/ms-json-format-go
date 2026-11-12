package repositories

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ErrTokenInvalid indica token de uso único (verificação de email, reset de
// senha) inexistente, já usado ou expirado.
var ErrTokenInvalid = errors.New("invalid or expired token")

// ErrVerificationInvalid é mantido por compatibilidade; é o mesmo ErrTokenInvalid.
var ErrVerificationInvalid = ErrTokenInvalid

// OneTimeTokenRepository guarda tokens de uso único enviados por email. Só o
// SHA-256 do token é salvo; o token em si vai apenas no email.
type OneTimeTokenRepository interface {
	EnsureIndexes() error
	Create(tokenHash, userID, email string, expiresAt time.Time) error
	// Consume apaga e devolve o token (uso único). ErrTokenInvalid se não
	// existir ou tiver expirado.
	Consume(tokenHash string) (userID, email string, err error)
	LastCreatedAt(userID string) (time.Time, error)
	DeleteForUser(userID string) error
}

// EmailVerificationRepository guarda tokens de verificação de email.
type EmailVerificationRepository interface{ OneTimeTokenRepository }

// PasswordResetRepository guarda tokens de redefinição de senha.
type PasswordResetRepository interface{ OneTimeTokenRepository }

type oneTimeTokenDoc struct {
	ID        string    `bson:"_id"` // hash do token
	UserID    string    `bson:"user_id"`
	Email     string    `bson:"email"`
	CreatedAt time.Time `bson:"createdAt"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

type oneTimeTokenRepository struct {
	tokens *mongo.Collection
}

func NewEmailVerificationRepository(db *mongo.Database) EmailVerificationRepository {
	return &oneTimeTokenRepository{tokens: db.Collection(collEmailVerifications)}
}

func NewPasswordResetRepository(db *mongo.Database) PasswordResetRepository {
	return &oneTimeTokenRepository{tokens: db.Collection(collPasswordResets)}
}

func (r *oneTimeTokenRepository) EnsureIndexes() error {
	_, err := r.tokens.Indexes().CreateMany(context.TODO(), []mongo.IndexModel{
		{Keys: bson.D{{"user_id", 1}, {"createdAt", -1}}},
		{Keys: bson.D{{"expiresAt", 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	return err
}

func (r *oneTimeTokenRepository) Create(tokenHash, userID, email string, expiresAt time.Time) error {
	_, err := r.tokens.InsertOne(context.TODO(), oneTimeTokenDoc{
		ID:        tokenHash,
		UserID:    userID,
		Email:     email,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	})
	return err
}

func (r *oneTimeTokenRepository) Consume(tokenHash string) (string, string, error) {
	var doc oneTimeTokenDoc
	err := r.tokens.FindOneAndDelete(context.TODO(), bson.D{
		{"_id", tokenHash},
		{"expiresAt", bson.D{{"$gt", time.Now()}}},
	}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", "", ErrTokenInvalid
	}
	if err != nil {
		return "", "", err
	}
	return doc.UserID, doc.Email, nil
}

func (r *oneTimeTokenRepository) LastCreatedAt(userID string) (time.Time, error) {
	var doc oneTimeTokenDoc
	err := r.tokens.FindOne(context.TODO(), bson.D{{"user_id", userID}},
		options.FindOne().SetSort(bson.D{{"createdAt", -1}}),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, nil
	}
	return doc.CreatedAt, err
}

func (r *oneTimeTokenRepository) DeleteForUser(userID string) error {
	_, err := r.tokens.DeleteMany(context.TODO(), bson.D{{"user_id", userID}})
	return err
}
