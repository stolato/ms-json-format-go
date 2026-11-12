// Package access concentra as regras de quem pode ver/editar um item,
// usadas tanto pela API HTTP quanto pelo websocket.
package access

import (
	"api-go/internal/models"
	"api-go/internal/repositories"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CanAccessItem diz se userID ("" = anônimo) pode ver/editar o item:
//   - item de organização: usuário logado e dono/membro da organização;
//   - item privado: somente o dono;
//   - item público: qualquer pessoa.
func CanAccessItem(orgs repositories.OrganizationRepository, userID string, item models.Item) bool {
	if item.OrganizationId != "" {
		return userID != "" && IsOrgMember(orgs, userID, item.OrganizationId)
	}
	if item.Private {
		return userID != "" && userID == item.UserId
	}
	return true
}

// IsOrgMember diz se userID é dono ou membro da organização orgID.
func IsOrgMember(orgs repositories.OrganizationRepository, userID, orgID string) bool {
	_id, err := primitive.ObjectIDFromHex(orgID)
	if err != nil {
		return false
	}
	_, err = orgs.FindOne(bson.D{
		{"_id", _id},
		{"$or", bson.A{
			bson.D{{"ownerId", userID}},
			bson.D{{"users._id", userID}},
		}},
	})
	return err == nil
}
