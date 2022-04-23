package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type artworkStore struct {
	client *mongo.Client
	db     *mongo.Database
	col    *mongo.Collection
}

func ArtworkStore(client *mongo.Client, database, collection string) store.ArtworkStore {
	db := client.Database(database)
	col := db.Collection(collection)

	return &artworkStore{
		client: client,
		db:     db,
		col:    col,
	}
}

func (a *artworkStore) Artwork(ctx context.Context, id int, url string) (*store.Artwork, error) {
	filter := bson.M{}
	if id != 0 {
		filter["artwork_id"] = id
	}

	if url != "" {
		filter["url"] = url
	}

	res := a.col.FindOne(ctx, filter)

	artwork := &store.Artwork{}
	if err := res.Decode(artwork); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrArtworkNotFound
		}

		return nil, fmt.Errorf("failed to decode an artwork: %w", err)
	}

	return artwork, nil
}

func (a *artworkStore) SearchArtworks(ctx context.Context, filter store.ArtworkFilter, opts ...store.ArtworkSearchOptions) ([]*store.Artwork, error) {
	opt := store.DefaultSearchOptions()
	if len(opts) != 0 {
		opt = opts[0]
	}

	cur, err := a.col.Find(ctx, filterBSON(filter), findOptions(opt))
	if err != nil {
		return nil, err
	}

	artworks := make([]*store.Artwork, 0)
	err = cur.All(ctx, &artworks)
	if err != nil {
		return nil, err
	}

	return artworks, nil
}

func (a *artworkStore) CreateArtwork(ctx context.Context, artwork *store.Artwork) (*store.Artwork, error) {
	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	opts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)

	session, err := a.client.StartSession()
	if err != nil {
		return nil, err
	}

	defer session.EndSession(ctx)

	callback := func(sessionContext mongo.SessionContext) (interface{}, error) {
		sres := a.db.Collection("counters").FindOneAndUpdate(
			sessionContext,
			bson.M{"_id": "artworks"},
			bson.M{"$inc": bson.M{"counter": 1}},
			options.FindOneAndUpdate().SetReturnDocument(options.After).SetUpsert(true),
		)

		counter := &struct {
			ID int `bson:"counter"`
		}{}

		err := sres.Decode(counter)
		if err != nil {
			return nil, err
		}

		artwork.ID = counter.ID
		artwork.CreatedAt = time.Now()
		artwork.UpdatedAt = time.Now()

		_, err = a.col.InsertOne(sessionContext, artwork)
		if err != nil {
			return nil, err
		}

		return artwork, nil
	}

	res, err := session.WithTransaction(ctx, callback, opts)
	if err != nil {
		return nil, err
	}

	return res.(*store.Artwork), nil
}

func findOptions(a store.ArtworkSearchOptions) *options.FindOptions {
	sort := bson.M{a.Sort.String(): a.Order}

	return options.Find().SetLimit(a.Limit).SetSkip(a.Skip).SetSort(sort)
}

func filterBSON(f store.ArtworkFilter) bson.D {
	filter := bson.D{}

	regex := func(key, value string) bson.E {
		return bson.E{Key: key, Value: bson.D{{Key: "$regex", Value: ".*" + value + ".*"}, {Key: "$options", Value: "i"}}}
	}

	regexM := func(key, value string) bson.M {
		return bson.M{key: bson.D{{Key: "$regex", Value: ".*" + value + ".*"}, {Key: "$options", Value: "i"}}}
	}

	switch {
	case len(f.IDs) != 0:
		filter = append(filter, bson.E{Key: "artwork_id", Value: bson.M{"$in": f.IDs}})
	case f.URL != "":
		filter = append(filter, bson.E{Key: "url", Value: f.URL})
	case f.Query != "":
		filter = bson.D{
			{Key: "$or", Value: []bson.M{regexM("author", f.Query), regexM("title", f.Query)}},
		}
	default:
		if f.Author != "" {
			filter = append(filter, regex("author", f.Author))
		}

		if f.Title != "" {
			filter = append(filter, regex("title", f.Title))
		}

		if f.Time != 0 {
			filter = append(filter, bson.E{Key: "created_at", Value: bson.M{"$gte": time.Now().Add(-f.Time)}})
		}
	}

	return filter
}
