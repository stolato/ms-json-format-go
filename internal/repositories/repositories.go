package repositories

import (
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
)

// Nomes das coleções no Mongo.
const (
	collItems              = "items"
	collUsers              = "users"
	collOrganizations      = "organization"
	collSessions           = "sessions"
	collEmailVerifications = "email_verifications"
	collPasswordResets     = "password_resets"
)

// Repositories agrupa todos os repositórios da aplicação, criados sobre o
// mesmo banco.
type Repositories struct {
	Items              ItemsRepository
	Users              UserRepository
	Organizations      OrganizationRepository
	Sessions           SessionRepository
	EmailVerifications EmailVerificationRepository
	PasswordResets     PasswordResetRepository
}

type Options struct {
	// LegacyIDLinks: links antigos pelo _id continuam funcionando (só para
	// itens anteriores ao slug).
	LegacyIDLinks bool
}

func New(db *mongo.Database, opts Options) *Repositories {
	return &Repositories{
		Items:              NewItemsRepository(db, opts.LegacyIDLinks),
		Users:              NewUserRepository(db),
		Organizations:      NewOrganizationRepository(db),
		Sessions:           NewSessionRepository(db),
		EmailVerifications: NewEmailVerificationRepository(db),
		PasswordResets:     NewPasswordResetRepository(db),
	}
}

// EnsureIndexes cria os índices de todas as coleções. Continua nas demais
// mesmo que uma falhe e devolve todos os erros juntos.
func (r *Repositories) EnsureIndexes() error {
	steps := []struct {
		name string
		fn   func() error
	}{
		{"items (slug único)", r.Items.EnsureIndexes},
		{"users (email único; há emails duplicados?)", r.Users.EnsureIndexes},
		{"email_verifications", r.EmailVerifications.EnsureIndexes},
		{"password_resets", r.PasswordResets.EnsureIndexes},
		{"sessions", r.Sessions.EnsureIndexes},
	}
	var errs []error
	for _, s := range steps {
		if err := s.fn(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.name, err))
		}
	}
	return errors.Join(errs...)
}
