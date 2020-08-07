package database

import (
	"context"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	//DB is a global mongo database instance.
	DB *mongo.Database
	//Client is a global mongo client instance
	Client *mongo.Client
)

func init() {
	connStr := os.Getenv("MONGODB_URL")
	if connStr == "" {
		log.Fatalln("MONGODB_URL env variable is not found.")
	}

	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	Client, err = mongo.Connect(ctx, options.Client().ApplyURI(connStr))
	if err != nil {
		log.Fatalln("Error connecting to Mongo DB", err)
	}

	DB = Client.Database("boe-tea")
}
