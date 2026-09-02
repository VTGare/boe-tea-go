package mongo

import (
	"context"
	"errors"
	"fmt"

	"github.com/VTGare/boe-tea-go/store"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type mongoStore struct {
	client   *mongo.Client
	database *mongo.Database

	*artworkStore
	*userStore
	*guildStore
	*bookmarkStore
}

func New(ctx context.Context, uri string, db string) (store.Store, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongo: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping mongo: %w", err)
	}

	database := client.Database(db)
	return &mongoStore{
		client:        client,
		database:      database,
		artworkStore:  &artworkStore{client, database, database.Collection("artworks")},
		userStore:     &userStore{client, database, database.Collection("users")},
		guildStore:    &guildStore{client, database, database.Collection("guilds")},
		bookmarkStore: &bookmarkStore{client, database, database.Collection("bookmarks")},
	}, nil
}

func (m *mongoStore) Init(ctx context.Context) error {
	collections := []string{"artworks", "counters", "guilds", "users", "bookmarks"}
	for _, col := range collections {
		err := m.database.CreateCollection(ctx, col)
		if err != nil && !isNamespaceExistsError(err) {
			return fmt.Errorf("failed to create collection %q: %w", col, err)
		}
	}

	return ensureIndexes(ctx, m.database)
}

func isNamespaceExistsError(err error) bool {
	if cmdErr, ok := errors.AsType[mongo.CommandError](err); ok {
		return cmdErr.Code == 48
	}

	return false
}

// ensureIndexes creates the indexes backing the store's hot-path queries.
// Creating an index with an identical spec is a no-op, so it is safe to run
// on every boot.
func ensureIndexes(ctx context.Context, db *mongo.Database) error {
	indexes := map[string][]mongo.IndexModel{
		"guilds": {
			{
				Keys:    bson.D{{Key: "guild_id", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("guild_id"),
			},
		},
		"users": {
			{
				Keys:    bson.D{{Key: "user_id", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("user_id"),
			},
		},
		"artworks": {
			{
				Keys:    bson.D{{Key: "artwork_id", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("ID"),
			},
			{
				Keys:    bson.D{{Key: "url", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("URL"),
			},
			{
				Keys:    bson.D{{Key: "created_at", Value: -1}},
				Options: options.Index().SetName("created_at_idx"),
			},
			{
				Keys:    bson.D{{Key: "favourites", Value: -1}},
				Options: options.Index().SetName("favourites_idx"),
			},
		},
		"bookmarks": {
			{
				Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "artwork_id", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("user_artwork_unique"),
			},
			{
				Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: 1}},
				Options: options.Index().SetName("user_created_idx"),
			},
		},
	}

	for col, models := range indexes {
		if _, err := db.Collection(col).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("failed to create indexes for collection %q: %w", col, err)
		}
	}

	return nil
}

func (m *mongoStore) Close(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}
