package repository

import "go.mongodb.org/mongo-driver/mongo"

type RepositoryMain struct {
	DB *mongo.Client
}

type Respositorys struct {
	User         *UserRepository
	Items        *ItemsRepository
	Organization *OrganizationRepository
}

func (r *RepositoryMain) Repositorys() Respositorys {
	reps := Respositorys{
		User:         &UserRepository{DB: r.DB},
		Items:        &ItemsRepository{DB: r.DB},
		Organization: &OrganizationRepository{DB: r.DB},
	}
	return reps
}
