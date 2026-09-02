package mongo

import (
	"context"
	"fmt"

	"github.com/VTGare/boe-tea-go/store"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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

func (b *bookmarkStore) ListBookmarks(ctx context.Context, userID string, filter store.BookmarkFilter, order store.Order, opts ...store.BookmarkListOptions) ([]*store.Bookmark, error) {
	f := bson.M{"user_id": userID}
	if filter != store.BookmarkFilterAll {
		f["nsfw"] = filter == store.BookmarkFilterUnsafe
	}

	findOpts := options.Find().SetSort(bson.M{"created_at": order})
	if len(opts) != 0 {
		if opts[0].Limit > 0 {
			findOpts.SetLimit(opts[0].Limit)
		}

		if opts[0].Skip > 0 {
			findOpts.SetSkip(opts[0].Skip)
		}
	}

	cur, err := b.col.Find(ctx, f, findOpts)
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
	if _, err := b.col.InsertOne(ctx, bookmark); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to insert bookmark: %w", err)
	}

	if _, err := b.artworks().UpdateOne(
		ctx,
		bson.M{"artwork_id": bookmark.ArtworkID},
		bson.M{"$inc": bson.M{"favourites": 1}},
	); err != nil {
		// Best-effort rollback so the favourites counter doesn't drift.
		_, _ = b.col.DeleteOne(ctx, bson.M{"user_id": bookmark.UserID, "artwork_id": bookmark.ArtworkID})

		return false, fmt.Errorf("failed to increment artwork favorite count: %w", err)
	}

	return true, nil
}

func (b *bookmarkStore) DeleteBookmark(ctx context.Context, bookmark *store.Bookmark) (bool, error) {
	res, err := b.col.DeleteOne(ctx, bson.M{"user_id": bookmark.UserID, "artwork_id": bookmark.ArtworkID})
	if err != nil {
		return false, fmt.Errorf("failed to delete bookmark: %w", err)
	}

	if res.DeletedCount == 0 {
		return false, nil
	}

	// Guard against the counter drifting below zero under concurrency.
	if _, err := b.artworks().UpdateOne(
		ctx,
		bson.M{"artwork_id": bookmark.ArtworkID, "favourites": bson.M{"$gt": 0}},
		bson.M{"$inc": bson.M{"favourites": -1}},
	); err != nil {
		return false, fmt.Errorf("failed to decrement artwork favorite count: %w", err)
	}

	return true, nil
}

func (b *bookmarkStore) artworks() *mongo.Collection {
	return b.db.Collection("artworks")
}
