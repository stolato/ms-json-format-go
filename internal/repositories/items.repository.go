package repositories

import (
	"api-go/internal/encryption"
	"api-go/internal/models"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collection = "items"

type ItemsRepository interface {
	FindAll(filter bson.D, page int64, limit int64) (ResultItems, error)
	AddItem(item models.Item) (*mongo.InsertOneResult, error)
	UpdateItem(item models.Item, _id primitive.ObjectID) (models.Item, error)
	DeleteItem(_id primitive.ObjectID, user_id string) (*mongo.DeleteResult, error)
	FindOne(_id primitive.ObjectID, update bool) (models.Item, error)
	DeleteExpiredAnonymous(now time.Time) (int64, error)
	EnsureIndexes() error
	// ResolveRef converte a referência usada no link (slug, ou _id em links
	// antigos) no _id do item. ErrItemNotFound se não existir ou não for aceita.
	ResolveRef(ref string) (primitive.ObjectID, error)
	IncrementViews(_id primitive.ObjectID) error
	// ConsumeViewOnce apaga o item de visualização única e devolve seu
	// conteúdo, numa única operação atômica: só um chamador recebe o item.
	ConsumeViewOnce(_id primitive.ObjectID) (models.Item, error)
}

var (
	ErrItemNotFound          = newPublicError("NOT_FOUND")
	errNotPossibleDeleteItem = newPublicError("not possible delete item")
)

const slugBytes = 16 // 128 bits -> 22 caracteres em base64url

// NewSlug gera um identificador público aleatório para links.
func NewSlug() (string, error) {
	buf := make([]byte, slugBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// IsSlug valida o formato de um slug.
func IsSlug(s string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	return err == nil && len(raw) == slugBytes
}

// legacyIDLinksEnabled: links antigos pelo _id continuam funcionando (só para
// itens anteriores ao slug) até LEGACY_ID_LINKS=false.
func legacyIDLinksEnabled() bool {
	return os.Getenv("LEGACY_ID_LINKS") != "false"
}

type itemsRepository struct {
	DB *mongo.Client
}

type ResultItems struct {
	TotalItems int           `json:"total_items"`
	Page       int64         `json:"page"`
	Limit      int64         `json:"limit"`
	Data       []models.Item `json:"data"`
}

func NewItemsRepository(db *mongo.Client) ItemsRepository {
	return &itemsRepository{
		DB: db,
	}
}

func (r *itemsRepository) FindAll(filter bson.D, page int64, limit int64) (ResultItems, error) {
	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSkip(limit * page)
	cursor, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).Find(context.TODO(), filter, findOptions)
	if err != nil {
		return ResultItems{Page: page, Limit: limit}, err
	}
	var results []models.Item
	if err = cursor.All(context.TODO(), &results); err != nil {
		return ResultItems{Page: page, Limit: limit}, err
	}
	var decryptErrs []error
	for i := range results {
		if err := decryptItem(&results[i]); err != nil {
			decryptErrs = append(decryptErrs, err)
		}
	}
	count, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).CountDocuments(context.TODO(), filter)
	return ResultItems{Data: results, Page: page, Limit: limit, TotalItems: int(count)}, errors.Join(append(decryptErrs, err)...)
}

func (r *itemsRepository) FindOne(_id primitive.ObjectID, update bool) (models.Item, error) {
	result := models.Item{}
	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).FindOne(context.TODO(), bson.D{{"_id", _id}}).Decode(&result)
	if err == nil {
		err = decryptItem(&result)
	}
	if update {
		r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).FindOneAndUpdate(context.TODO(), bson.D{{Key: "_id", Value: result.Id}}, bson.D{{Key: "$set", Value: bson.D{
			{Key: "views", Value: result.Views + 1},
			{Key: "expirateAt", Value: time.Now().AddDate(0, 0, 7)},
			{Key: "updateAt", Value: time.Now()},
		}}})
	}
	return result, err
}

func (r *itemsRepository) AddItem(item models.Item) (*mongo.InsertOneResult, error) {
	// O id é gerado aqui (e não pelo Mongo) porque faz parte da criptografia.
	if item.Id.IsZero() {
		item.Id = primitive.NewObjectID()
	}
	encrypted, err := encryption.Encrypt(item.Json, item.Id.Hex())
	if err != nil {
		return nil, err
	}
	item.Json = encrypted
	result, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).InsertOne(context.TODO(), item)
	return result, err
}

func (r *itemsRepository) UpdateItem(item models.Item, _id primitive.ObjectID) (models.Item, error) {
	encrypted, err := encryption.Encrypt(item.Json, _id.Hex())
	if err != nil {
		return models.Item{}, err
	}
	update := bson.D{{"$set", bson.D{
		{"json", encrypted},
		{"expirateAt", time.Now().AddDate(0, 0, 7)},
		{"updateAt", time.Now()},
	}}}
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
	}
	result := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection("items").FindOneAndUpdate(context.TODO(), bson.D{{"_id", _id}}, update, &opt)
	doc := models.Item{}
	if err := result.Decode(&doc); err != nil {
		return doc, err
	}
	return doc, decryptItem(&doc)
}

// DeleteItem apaga o item se user_id for o autor do item ou o dono da
// organização do item. user_id vazio (uso interno, ex.: cron) apaga sem checar.
func (r *itemsRepository) DeleteItem(_id primitive.ObjectID, user_id string) (*mongo.DeleteResult, error) {
	db := r.DB.Database(os.Getenv("MONGO_DATABASE"))
	if user_id == "" {
		return r.delete(bson.D{{"_id", _id}})
	}

	// Lê só os campos de permissão (sem decifrar o conteúdo).
	var item models.Item
	err := db.Collection(collection).FindOne(context.TODO(), bson.D{{"_id", _id}},
		options.FindOne().SetProjection(bson.D{{"user_id", 1}, {"organizationId", 1}}),
	).Decode(&item)
	if err != nil {
		return nil, errNotPossibleDeleteItem
	}

	allowed := item.UserId == user_id
	if !allowed && item.OrganizationId != "" {
		orgID, errID := primitive.ObjectIDFromHex(item.OrganizationId)
		if errID == nil {
			count, errCount := db.Collection(collectionTime).CountDocuments(context.TODO(),
				bson.D{{"_id", orgID}, {"ownerId", user_id}})
			allowed = errCount == nil && count > 0
		}
	}
	if !allowed {
		return nil, errNotPossibleDeleteItem
	}

	result, err := r.delete(bson.D{{"_id", _id}})
	if err != nil {
		return nil, err
	}
	if result.DeletedCount <= 0 {
		return result, errNotPossibleDeleteItem
	}
	return result, nil
}

func (r *itemsRepository) EnsureIndexes() error {
	_, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).Indexes().CreateOne(context.TODO(), mongo.IndexModel{
		Keys: bson.D{{"slug", 1}},
		// Parcial: itens antigos ainda sem slug não entram no índice.
		Options: options.Index().SetUnique(true).SetName("slug_unique").
			SetPartialFilterExpression(bson.D{{"slug", bson.D{{"$type", "string"}}}}),
	})
	return err
}

func (r *itemsRepository) ResolveRef(ref string) (primitive.ObjectID, error) {
	var filter bson.D
	switch {
	case IsSlug(ref):
		filter = bson.D{{"slug", ref}}
	case legacyIDLinksEnabled():
		_id, err := primitive.ObjectIDFromHex(ref)
		if err != nil {
			return primitive.NilObjectID, ErrItemNotFound
		}
		// Itens criados já com slug NUNCA abrem pelo _id.
		filter = bson.D{
			{"_id", _id},
			{"$or", bson.A{
				bson.D{{"slug", bson.D{{"$exists", false}}}},
				bson.D{{"slugLegacy", true}},
			}},
		}
	default:
		return primitive.NilObjectID, ErrItemNotFound
	}
	var doc struct {
		Id primitive.ObjectID `bson:"_id"`
	}
	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).FindOne(context.TODO(), filter,
		options.FindOne().SetProjection(bson.D{{"_id", 1}}),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return primitive.NilObjectID, ErrItemNotFound
	}
	return doc.Id, err
}

// IncrementViews conta uma visualização e renova a expiração do item.
func (r *itemsRepository) IncrementViews(_id primitive.ObjectID) error {
	_, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).UpdateOne(context.TODO(),
		bson.D{{"_id", _id}},
		bson.D{
			{"$inc", bson.D{{"views", 1}}},
			{"$set", bson.D{
				{"expirateAt", time.Now().AddDate(0, 0, 7)},
				{"updateAt", time.Now()},
			}},
		},
	)
	return err
}

func (r *itemsRepository) ConsumeViewOnce(_id primitive.ObjectID) (models.Item, error) {
	var item models.Item
	err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).FindOneAndDelete(context.TODO(),
		bson.D{{"_id", _id}, {"viewOnce", true}},
	).Decode(&item)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return item, ErrItemNotFound
	}
	if err != nil {
		return item, err
	}
	return item, decryptItem(&item)
}

// anonymousOwner casa itens sem dono: user_id "" (gravado pelo Store), null ou ausente.
var anonymousOwner = bson.D{{"$in", bson.A{"", nil}}}

// DeleteExpiredAnonymous apaga itens expirados que são anônimos ou de
// visualização única (estes expiram mesmo tendo dono, se nunca forem revelados).
func (r *itemsRepository) DeleteExpiredAnonymous(now time.Time) (int64, error) {
	result, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).DeleteMany(context.TODO(), bson.D{
		{"$or", bson.A{
			bson.D{{"user_id", anonymousOwner}},
			bson.D{{"viewOnce", true}},
		}},
		{"expirateAt", bson.D{{"$lte", now}}},
	})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

func (r *itemsRepository) delete(filter bson.D) (*mongo.DeleteResult, error) {
	result, err := r.DB.Database(os.Getenv("MONGO_DATABASE")).Collection(collection).DeleteOne(context.TODO(), filter)
	return result, err
}

// decryptItem substitui item.Json pelo conteúdo decifrado. Valores legados em
// texto puro (ainda não migrados) passam sem alteração.
func decryptItem(item *models.Item) error {
	plain, err := encryption.Decrypt(item.Json, item.Id.Hex())
	if err != nil {
		item.Json = ""
		return fmt.Errorf("item %s: %w", item.Id.Hex(), err)
	}
	item.Json = plain
	return nil
}
