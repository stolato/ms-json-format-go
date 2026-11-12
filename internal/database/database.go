package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const connectTimeout = 10 * time.Second

// Connect abre a conexão com o Mongo e confirma com um ping. Quem chama é
// responsável por client.Disconnect.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo: falha ao conectar: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo: falha no ping: %w", err)
	}
	return client, nil
}
