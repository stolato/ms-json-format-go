// Script TEMPORÁRIO: criptografa o campo "json" dos itens que ainda estão em
// texto puro. Pode ser rodado mais de uma vez (só toca no que não tem o
// prefixo "v1:"). Remover depois que a migração terminar.
//
//	go run ./cmd/migrate-encrypt --dry-run   # só conta
//	go run ./cmd/migrate-encrypt             # migra
package main

import (
	"api-go/internal/database"
	"api-go/internal/encryption"
	"api-go/internal/models"
	"context"
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "apenas conta os itens que seriam migrados")
	flag.Parse()

	_ = godotenv.Overload(".env")
	if err := encryption.Init(); err != nil {
		log.Fatal(err)
	}
	database.Connect()
	ctx := context.Background()
	defer database.DB.Disconnect(ctx)

	coll := database.DB.Database(os.Getenv("MONGO_DATABASE")).Collection("items")
	pending := bson.D{{"json", bson.D{{"$not", bson.D{{"$regex", "^" + encryption.Prefix}}}}}}

	total, err := coll.CountDocuments(ctx, pending)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("itens em texto puro: %d", total)
	if *dryRun || total == 0 {
		return
	}

	cursor, err := coll.Find(ctx, pending)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	var migrated, skipped, failed int
	for cursor.Next(ctx) {
		var item models.Item
		if err := cursor.Decode(&item); err != nil {
			log.Printf("decode: %v", err)
			failed++
			continue
		}
		encrypted, err := encryption.Encrypt(item.Json, item.Id.Hex())
		if err != nil {
			log.Printf("item %s: %v", item.Id.Hex(), err)
			failed++
			continue
		}
		// Filtra também pelo valor antigo: se o item foi editado enquanto o
		// script rodava, não sobrescreve (a edição já gravou criptografado).
		res, err := coll.UpdateOne(ctx,
			bson.D{{"_id", item.Id}, {"json", item.Json}},
			bson.D{{"$set", bson.D{{"json", encrypted}}}},
		)
		if err != nil {
			log.Printf("item %s: %v", item.Id.Hex(), err)
			failed++
			continue
		}
		if res.ModifiedCount == 0 {
			skipped++
			continue
		}
		migrated++
	}
	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}
	log.Printf("migrados: %d, pulados (alterados durante a migração): %d, falhas: %d", migrated, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
