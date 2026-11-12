package repositories

import (
	"api-go/internal/dtos"
	"api-go/internal/models"
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type OrganizationRepository interface {
	Store(time models.OrganizationModel) (*mongo.InsertOneResult, error)
	FindAll(filter bson.D, findOptions *options.FindOptions) ([]models.OrganizationModel, error)
	FindOne(filter bson.D) (models.OrganizationModel, error)
	FindAllMyOrgs(_id string, page int64, limit int64) ([]models.OrganizationModel, error)
	Delete(filter bson.D) (*mongo.DeleteResult, error)
	FindByFilter(filter bson.D) (models.OrganizationModel, error)
	AddUser(user dtos.AddUserOrganizationDTO, id primitive.ObjectID, ownerID interface{}) (models.OrganizationModel, error)
	RemoveUser(userId string, id primitive.ObjectID, ownerID interface{}) (models.OrganizationModel, error)
	DeleteOwned(id primitive.ObjectID, ownerID string) (*mongo.DeleteResult, error)
}

func NewOrganizationRepository(db *mongo.Database) OrganizationRepository {
	return &organizationRepository{
		orgs:  db.Collection(collOrganizations),
		users: db.Collection(collUsers),
		items: db.Collection(collItems),
	}
}

type organizationRepository struct {
	orgs  *mongo.Collection
	users *mongo.Collection
	items *mongo.Collection
}

func (r *organizationRepository) Store(time models.OrganizationModel) (*mongo.InsertOneResult, error) {
	result, err := r.orgs.InsertOne(context.TODO(), time)
	return result, err
}

func (r *organizationRepository) FindAll(filter bson.D, findOptions *options.FindOptions) ([]models.OrganizationModel, error) {
	cursor, err := r.orgs.Find(context.TODO(), filter, findOptions)
	if err != nil {
		return nil, err
	}
	var results []models.OrganizationModel
	if err = cursor.All(context.TODO(), &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *organizationRepository) FindOne(filter bson.D) (models.OrganizationModel, error) {
	result := models.OrganizationModel{}
	err := r.orgs.FindOne(context.TODO(), filter).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return result, newPublicError("not find org")
	}
	return result, err
}

func (r *organizationRepository) FindAllMyOrgs(_id string, page int64, limit int64) ([]models.OrganizationModel, error) {
	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSkip(limit * page)
	cursor, err := r.orgs.Find(context.TODO(),
		bson.D{{
			"$or", bson.A{
				bson.D{{"ownerId", _id}}, bson.D{{"users._id", _id}},
			},
		}}, findOptions,
	)
	if err != nil {
		return nil, err
	}
	var results []models.OrganizationModel
	if err = cursor.All(context.TODO(), &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *organizationRepository) Delete(filter bson.D) (*mongo.DeleteResult, error) {
	result, err := r.orgs.DeleteOne(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	if result.DeletedCount <= 0 {
		err = newPublicError("not possible delete org")
	}
	return result, err
}

func (r *organizationRepository) FindByFilter(filter bson.D) (models.OrganizationModel, error) {
	result := models.OrganizationModel{}
	err := r.orgs.FindOne(context.TODO(), filter).Decode(&result)
	return result, err
}

// AddUser adiciona um usuário (por email) à organização id, somente se ownerID
// for o dono. Sem upsert: uma organização inexistente/apagada nunca é recriada.
func (r *organizationRepository) AddUser(user dtos.AddUserOrganizationDTO, id primitive.ObjectID, ownerID interface{}) (models.OrganizationModel, error) {
	doc := models.OrganizationModel{}

	userCheck := models.User{}
	errUser := r.users.FindOne(context.TODO(),
		// Só contas com email confirmado podem ser adicionadas: sem isso, quem
		// cadastrasse o email de outra pessoa entraria na organização no lugar dela.
		bson.D{{"email", NormalizeEmail(user.Email)}, {"active", true}, {"emailVerified", bson.D{{"$ne", false}}}},
		options.FindOne().SetCollation(emailCollation),
	).Decode(&userCheck)
	if errors.Is(errUser, mongo.ErrNoDocuments) {
		return doc, newPublicError("not possible add this user")
	}
	if errUser != nil {
		return doc, errUser
	}
	newID := userCheck.Id.Hex()
	if newID == ownerID {
		return doc, newPublicError("not possible add this user")
	}

	after := options.After
	err := r.orgs.FindOneAndUpdate(
		context.TODO(),
		bson.D{
			{"_id", id},
			{"ownerId", ownerID},
			{"users._id", bson.D{{"$ne", newID}}},
		},
		bson.D{{"$push", bson.D{{"users", bson.D{{"_id", newID}, {"name", userCheck.Name}, {"email", userCheck.Email}}}}}},
		&options.FindOneAndUpdateOptions{ReturnDocument: &after},
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		// Organização não existe, não é do ownerID, ou o usuário já é membro.
		return doc, newPublicError("not possible add this user")
	}
	return doc, err
}

// RemoveUser remove um membro da organização id, somente se ownerID for o dono.
// Sem upsert: uma organização inexistente/apagada nunca é recriada.
func (r *organizationRepository) RemoveUser(userId string, id primitive.ObjectID, ownerID interface{}) (models.OrganizationModel, error) {
	doc := models.OrganizationModel{}
	ownerID = fmt.Sprintf("%s", ownerID)
	if userId == ownerID {
		return doc, newPublicError("not possible remove")
	}
	after := options.After
	err := r.orgs.FindOneAndUpdate(
		context.TODO(),
		bson.D{{"_id", id}, {"ownerId", ownerID}},
		bson.D{{"$pull", bson.D{{"users", bson.D{{"_id", userId}}}}}},
		&options.FindOneAndUpdateOptions{ReturnDocument: &after},
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return doc, newPublicError("not possible remove")
	}
	return doc, err
}

// DeleteOwned apaga a organização id se ownerID for o dono. Os itens dela
// viram itens privados do autor: ninguém perde dados e ex-membros perdem acesso.
func (r *organizationRepository) DeleteOwned(id primitive.ObjectID, ownerID string) (*mongo.DeleteResult, error) {
	result, err := r.orgs.DeleteOne(context.TODO(), bson.D{{"_id", id}, {"ownerId", ownerID}})
	if err != nil {
		return nil, err
	}
	if result.DeletedCount == 0 {
		return result, newPublicError("not possible delete org")
	}
	_, err = r.items.UpdateMany(context.TODO(),
		bson.D{{"organizationId", id.Hex()}},
		bson.D{
			{"$set", bson.D{{"organizationId", ""}, {"private", true}}},
			{"$unset", bson.D{{"organization", ""}}},
		},
	)
	if err != nil {
		return result, fmt.Errorf("org apagada, mas falhou ao converter itens: %w", err)
	}
	return result, nil
}
