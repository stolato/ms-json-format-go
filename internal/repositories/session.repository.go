package repositories

import (
	"context"
	"errors"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collectionSessions = "sessions"

const (
	// Janela em que reapresentar o refresh token imediatamente anterior é
	// tratado como corrida do próprio cliente (ex.: duas abas renovando juntas)
	// e não como roubo.
	refreshReuseGrace = 30 * time.Second
	// Quantos hashes de refresh tokens antigos guardar para detectar reuso.
	maxPreviousHashes = 20
)

type RotateResult int

const (
	RotateOK          RotateResult = iota // token atual: rotacionado
	RotateInvalid                         // sessão inexistente/expirada ou token desconhecido
	RotateReusedGrace                     // token anterior reapresentado logo após a rotação
	RotateReused                          // token antigo reapresentado: provável roubo
)

type SessionRepository interface {
	EnsureIndexes() error
	Create(sid, userID, refreshHash string, expiresAt time.Time) error
	// Rotate troca, de forma atômica, o hash do refresh token atual pelo novo.
	// Retorna o user_id da sessão quando o resultado é RotateOK ou RotateReused.
	Rotate(sid, currentHash, newHash string, expiresAt time.Time) (RotateResult, string, error)
	IsActive(sid, userID string) (bool, error)
	// Revoke encerra a sessão se o hash for do refresh token atual ou de um anterior.
	Revoke(sid, refreshHash string) error
	RevokeAllForUser(userID string) error
}

type sessionDoc struct {
	ID             string    `bson:"_id"`
	UserID         string    `bson:"user_id"`
	RefreshHash    string    `bson:"refresh_hash"`
	PreviousHashes []string  `bson:"previous_hashes"`
	CreatedAt      time.Time `bson:"createdAt"`
	RotatedAt      time.Time `bson:"rotatedAt"`
	ExpiresAt      time.Time `bson:"expiresAt"`
}

type sessionRepository struct {
	DB *mongo.Client
}

func NewSessionRepository(db *mongo.Client) SessionRepository {
	return &sessionRepository{DB: db}
}

func (r *sessionRepository) coll() *mongo.Collection {
	return r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionSessions)
}

func (r *sessionRepository) EnsureIndexes() error {
	_, err := r.coll().Indexes().CreateMany(context.TODO(), []mongo.IndexModel{
		{Keys: bson.D{{"user_id", 1}}},
		// Remove automaticamente sessões expiradas.
		{Keys: bson.D{{"expiresAt", 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	return err
}

func (r *sessionRepository) Create(sid, userID, refreshHash string, expiresAt time.Time) error {
	now := time.Now()
	_, err := r.coll().InsertOne(context.TODO(), sessionDoc{
		ID:             sid,
		UserID:         userID,
		RefreshHash:    refreshHash,
		PreviousHashes: []string{},
		CreatedAt:      now,
		RotatedAt:      now,
		ExpiresAt:      expiresAt,
	})
	return err
}

func (r *sessionRepository) Rotate(sid, currentHash, newHash string, expiresAt time.Time) (RotateResult, string, error) {
	now := time.Now()
	var doc sessionDoc
	err := r.coll().FindOneAndUpdate(context.TODO(),
		bson.D{
			{"_id", sid},
			{"refresh_hash", currentHash},
			{"expiresAt", bson.D{{"$gt", now}}},
		},
		bson.D{
			{"$set", bson.D{
				{"refresh_hash", newHash},
				{"rotatedAt", now},
				{"expiresAt", expiresAt},
			}},
			{"$push", bson.D{{"previous_hashes", bson.D{
				{"$each", bson.A{currentHash}},
				{"$slice", -maxPreviousHashes},
			}}}},
		},
	).Decode(&doc)
	if err == nil {
		return RotateOK, doc.UserID, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return RotateInvalid, "", err
	}

	// Não rotacionou: descobrir se é um token antigo desta sessão (reuso).
	err = r.coll().FindOne(context.TODO(), bson.D{{"_id", sid}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return RotateInvalid, "", nil
	}
	if err != nil {
		return RotateInvalid, "", err
	}
	idx := -1
	for i, h := range doc.PreviousHashes {
		if h == currentHash {
			idx = i
		}
	}
	switch {
	case idx < 0:
		return RotateInvalid, "", nil
	case idx == len(doc.PreviousHashes)-1 && now.Sub(doc.RotatedAt) <= refreshReuseGrace:
		return RotateReusedGrace, doc.UserID, nil
	default:
		return RotateReused, doc.UserID, nil
	}
}

func (r *sessionRepository) IsActive(sid, userID string) (bool, error) {
	n, err := r.coll().CountDocuments(context.TODO(), bson.D{
		{"_id", sid},
		{"user_id", userID},
		{"expiresAt", bson.D{{"$gt", time.Now()}}},
	}, options.Count().SetLimit(1))
	return n > 0, err
}

func (r *sessionRepository) Revoke(sid, refreshHash string) error {
	_, err := r.coll().DeleteOne(context.TODO(), bson.D{
		{"_id", sid},
		{"$or", bson.A{
			bson.D{{"refresh_hash", refreshHash}},
			bson.D{{"previous_hashes", refreshHash}},
		}},
	})
	return err
}

func (r *sessionRepository) RevokeAllForUser(userID string) error {
	_, err := r.coll().DeleteMany(context.TODO(), bson.D{{"user_id", userID}})
	return err
}
