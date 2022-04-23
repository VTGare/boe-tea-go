package mongo

import (
	"context"
	"errors"
	"fmt"

	"github.com/VTGare/boe-tea-go/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type bookmarkStore struct {
	client *mongo.Client
	db     *mongo.Database
	col    *mongo.Collection
}

func BookmarksStore(client *mongo.Client, database, collection string) store.BookmarkStore {
	db := client.Database(database)
	col := db.Collection(collection)

	return &bookmarkStore{
		client: client,
		db:     db,
		col:    col,
	}
}

func (b *bookmarkStore) ListBookmarks(ctx context.Context, userID string, order store.Order) ([]*store.Bookmark, error) {
	cur, err := b.col.Find(
		ctx,
		bson.M{"user_id": userID},
		options.Find().SetSort(bson.M{"created_at": order}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to find bookmarks: %w", err)
	}

	bookmarks := make([]*store.Bookmark, 0)
	if err := cur.All(ctx, &bookmarks); err != nil {
		return nil, fmt.Errorf("failed to decode to bookmarks: %w", err)
	}

	return bookmarks, nil
}

func (b *bookmarkStore) CountBookmarks(ctx context.Context, userID string) (int64, error) {
	count, err := b.col.CountDocuments(ctx, bson.M{"user_id": userID})
	if err != nil {
		return 0, fmt.Errorf("failed to count bookmarks: %w", err)
	}

	return count, nil
}

func (b *bookmarkStore) AddBookmark(ctx context.Context, bookmark *store.Bookmark) (bool, error) {
	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)

	session, err := b.client.StartSession()
	if err != nil {
		return false, fmt.Errorf("failed to start a session: %w", err)
	}

	defer session.EndSession(ctx)

	callback := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		err := b.col.FindOne(ctx, bson.M{"user_id": bookmark.UserID, "artwork_id": bookmark.ArtworkID}).Err()
		if err == nil {
			return false, nil
		}

		if !errors.Is(err, mongo.ErrNoDocuments) {
			return false, fmt.Errorf("failed to find an artwork: %w", err)
		}

		if _, err := b.col.InsertOne(ctx, bookmark); err != nil {
			return false, fmt.Errorf("failed to insert bookmark: %w", err)
		}

		_, err = b.artworks().UpdateOne(
			sessionCtx,
			bson.M{"artwork_id": bookmark.ArtworkID},
			bson.M{"$inc": bson.M{"favourites": 1}},
		)

		if err != nil {
			return false, fmt.Errorf("failed to increment artwork favourite count: %w", err)
		}

		return true, nil
	}

	added, err := session.WithTransaction(ctx, callback, txnOpts)
	if err != nil {
		return false, err
	}

	return added.(bool), nil
}

func (b *bookmarkStore) DeleteBookmark(ctx context.Context, bookmark *store.Bookmark) (bool, error) {
	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)

	session, err := b.client.StartSession()
	if err != nil {
		return false, fmt.Errorf("failed to start a session: %w", err)
	}

	defer session.EndSession(ctx)

	callback := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		res, err := b.col.DeleteOne(ctx, bson.M{"user_id": bookmark.UserID, "artwork_id": bookmark.ArtworkID})
		if err != nil {
			return false, fmt.Errorf("failed to delete bookmark: %w", err)
		}

		if res.DeletedCount == 0 {
			return false, nil
		}

		if err != nil {
			return false, fmt.Errorf("failed to update artwork favourite count: %w", err)
		}

		_, err = b.artworks().UpdateOne(
			sessionCtx,
			bson.M{"artwork_id": bookmark.ArtworkID},
			bson.M{"$inc": bson.M{"favourites": -1}},
		)

		if err != nil {
			return false, fmt.Errorf("failed to increment artwork favourite count: %w", err)
		}

		return true, nil
	}

	deleted, err := session.WithTransaction(ctx, callback, txnOpts)
	if err != nil {
		return false, err
	}

	return deleted.(bool), nil
}

func (b *bookmarkStore) artworks() *mongo.Collection {
	return b.db.Collection("artworks")
}
