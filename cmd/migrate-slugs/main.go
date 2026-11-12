// Script TEMPORÁRIO: gera slug para os itens criados antes do slug existir e os
// marca como legados (slugLegacy), para que os links antigos pelo _id continuem
// funcionando enquanto LEGACY_ID_LINKS não for "false". Pode ser rodado mais de
// uma vez (só toca em itens sem slug). Remover depois que a migração terminar.
//
//	go run ./cmd/migrate-slugs --dry-run   # só conta
//	go run ./cmd/migrate-slugs             # migra
package main

import (
	"api-go/internal/database"
	"api-go/internal/repositories"
	"context"
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "apenas conta os itens que seriam migrados")
	flag.Parse()

	_ = godotenv.Overload(".env")
	database.Connect()
	ctx := context.Background()
	defer database.DB.Disconnect(ctx)

	coll := database.DB.Database(os.Getenv("MONGO_DATABASE")).Collection("items")
	pending := bson.D{{"slug", bson.D{{"$exists", false}}}}

	total, err := coll.CountDocuments(ctx, pending)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("itens sem slug: %d", total)
	if *dryRun || total == 0 {
		return
	}

	cursor, err := coll.Find(ctx, pending, options.Find().SetProjection(bson.D{{"_id", 1}}))
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	var migrated, failed int
	for cursor.Next(ctx) {
		var doc struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("decode: %v", err)
			failed++
			continue
		}
		slug, err := repositories.NewSlug()
		if err != nil {
			log.Fatal(err)
		}
		_, err = coll.UpdateOne(ctx,
			bson.D{{"_id", doc.Id}, {"slug", bson.D{{"$exists", false}}}},
			bson.D{{"$set", bson.D{{"slug", slug}, {"slugLegacy", true}}}},
		)
		if err != nil {
			log.Printf("item %s: %v", doc.Id.Hex(), err)
			failed++
			continue
		}
		migrated++
	}
	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}
	log.Printf("migrados: %d, falhas: %d", migrated, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
