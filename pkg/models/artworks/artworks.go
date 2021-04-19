package artworks

import (
	"context"
	"errors"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database/mongodb"
	"github.com/VTGare/boe-tea-go/internal/validate"
	mo "github.com/VTGare/boe-tea-go/pkg/models/artworks/options"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type Artwork struct {
	ID         int       `json:"id" bson:"artwork_id"`
	Title      string    `json:"title" bson:"title"`
	Author     string    `json:"author" bson:"author"`
	URL        string    `json:"url" bson:"url"`
	Images     []string  `json:"images" bson:"images"`
	Favourites int       `json:"favourites" bson:"favourites"`
	NSFW       int       `json:"nsfw" bson:"nsfw"`
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`
}

type ArtworkInsert struct {
	Title  string   `json:"title"`
	Author string   `json:"author" validate:"required"`
	URL    string   `json:"url" validate:"required,url"`
	Images []string `json:"images" validate:"required,min=1,dive,url"`
}

func (insert *ArtworkInsert) toArtwork(id int) *Artwork {
	return &Artwork{
		ID:        id,
		Title:     insert.Title,
		Author:    insert.Author,
		URL:       insert.URL,
		Images:    insert.Images,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

type Service interface {
	FindMany(context.Context, ...mo.Find) ([]*Artwork, error)
	FindOne(context.Context, *mo.FilterOne) (*Artwork, error)
	InsertOne(context.Context, *ArtworkInsert) (*Artwork, error)
	FindOneOrCreate(context.Context, *mo.FilterOne, *ArtworkInsert) (*Artwork, error)
	IncrementFavourite(ctx context.Context, id int, inc int, nsfw bool) (*Artwork, error)
	DeleteOne(context.Context, *mo.FilterOne) (*Artwork, error)
}

type artwork struct {
	db     *mongodb.Mongo
	logger *zap.SugaredLogger
}

func NewService(db *mongodb.Mongo, logger *zap.SugaredLogger) Service {
	return &artwork{db, logger}
}

func (a artwork) col() *mongo.Collection {
	return a.db.Database.Collection("artworks")
}

func (a artwork) FindMany(ctx context.Context, opts ...mo.Find) ([]*Artwork, error) {
	opt := mo.DefaultFind()
	if len(opts) != 0 {
		opt = opts[0]
	}

	cur, err := a.col().Find(ctx, opt.Filter.BSON(), opt.FindOptions())
	if err != nil {
		return nil, err
	}

	artworks := make([]*Artwork, 0)
	err = cur.All(ctx, &artworks)
	if err != nil {
		return nil, err
	}

	return artworks, nil
}

func (a artwork) FindOne(ctx context.Context, filter *mo.FilterOne) (*Artwork, error) {
	filt, err := filter.BSON()
	if err != nil {
		return nil, err
	}

	res := a.col().FindOne(ctx, filt)

	var artwork Artwork
	err = res.Decode(&artwork)

	return &artwork, err
}

func (a artwork) InsertOne(ctx context.Context, req *ArtworkInsert) (*Artwork, error) {
	id, err := a.nextID(ctx)
	if err != nil {
		return nil, err
	}

	if errs := validate.Struct(req); len(errs) != 0 {
		return nil, errs[0]
	}

	artwork := req.toArtwork(id)
	_, err = a.col().InsertOne(ctx, artwork)
	if err != nil {
		return nil, err
	}

	return artwork, nil
}

func (a artwork) DeleteOne(ctx context.Context, filter *mo.FilterOne) (*Artwork, error) {
	filt, err := filter.BSON()
	if err != nil {
		return nil, err
	}

	res := a.col().FindOneAndDelete(ctx, filt)

	var artwork Artwork
	err = res.Decode(&artwork)

	return &artwork, err
}

func (a artwork) FindOneOrCreate(ctx context.Context, filter *mo.FilterOne, insert *ArtworkInsert) (*Artwork, error) {
	art, err := a.FindOne(context.Background(), filter)
	if err != nil {
		switch {
		case errors.Is(err, mongo.ErrNoDocuments):
			art, err = a.InsertOne(
				context.Background(),
				insert,
			)

			if err != nil {
				return nil, err
			}
		default:
			return nil, err
		}
	}

	return art, nil
}

func (a artwork) IncrementFavourite(ctx context.Context, id int, inc int, nsfw bool) (*Artwork, error) {
	nsfwInc := 0
	if nsfw {
		nsfwInc += inc
	}

	res := a.col().FindOneAndUpdate(
		ctx,
		bson.M{"artwork_id": id},
		bson.M{"$inc": bson.M{"favourites": inc, "nsfw": nsfwInc}},
	)

	var artwork Artwork
	err := res.Decode(&artwork)
	if err != nil {
		return nil, err
	}

	return &artwork, nil
}

func (a artwork) nextID(ctx context.Context) (int, error) {
	sres := a.db.Database.Collection("counters").FindOneAndUpdate(
		ctx,
		bson.M{"_id": "artworks"},
		bson.M{"$inc": bson.M{"counter": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetUpsert(true),
	)

	counter := &struct {
		Counter int `bson:"counter"`
	}{}

	err := sres.Decode(counter)
	if err != nil {
		return 0, err
	}

	return counter.Counter, nil
}
