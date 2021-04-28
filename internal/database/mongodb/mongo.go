package mongodb

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Mongo struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func New(uri, db string) (*Mongo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return nil, err
	}

	return &Mongo{client, client.Database(db)}, nil
}

func (m *Mongo) CreateCollections() error {
	err := m.Database.CreateCollection(
		context.Background(),
		"artworks",
	)

	if err != nil && !errors.As(err, &mongo.CommandError{}) {
		return err
	}

	err = m.Database.CreateCollection(
		context.Background(),
		"counters",
	)

	if err != nil && !errors.As(err, &mongo.CommandError{}) {
		return err
	}

	err = m.Database.CreateCollection(
		context.Background(),
		"guildsettings",
	)

	if err != nil && !errors.As(err, &mongo.CommandError{}) {
		return err
	}

	err = m.Database.CreateCollection(
		context.Background(),
		"user_settings",
	)

	if err != nil && !errors.As(err, &mongo.CommandError{}) {
		return err
	}

	return nil
}

func (m *Mongo) Close() error {
	return m.Client.Disconnect(context.Background())
}
