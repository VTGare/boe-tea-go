package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	err error
	//DB is a global database connection
	DB *Database
)

func init() {
	url := os.Getenv("MONGODB_URL")
	if url == "" {
		fmt.Println("MONGODB_URL is empty")
		os.Exit(1)
	}

	DB, err = NewDatabase(url, "boe-tea")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type Database struct {
	db            *mongo.Database
	client        *mongo.Client
	GuildSettings *mongo.Collection
	UserSettings  *mongo.Collection
	posts         *mongo.Collection
	stats         *mongo.Collection
}

func (d *Database) Close() {
	d.client.Disconnect(context.Background())
}

func NewDatabase(url, dbname string) (*Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(url))
	if err != nil {
		log.Fatalln("Error connecting to Mongo DB", err)
	}

	db := client.Database(dbname)

	d := &Database{db, client, db.Collection("guildsettings"), db.Collection("user_settings"), db.Collection("image_posts"), db.Collection("stats")}
	_, err = d.AllUsers()
	_, err = d.AllGuilds()

	if err != nil {
		return nil, err
	}

	return d, nil
}
