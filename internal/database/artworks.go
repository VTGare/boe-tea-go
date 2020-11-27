package database

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type FindManyOptions struct {
	Sort         SortType
	SortOrder    SortOrder
	ArtworkLimit int
	Time         time.Time
}

func NewFindManyOptions() *FindManyOptions {
	return &FindManyOptions{
		Sort:         ByID,
		SortOrder:    Ascending,
		ArtworkLimit: 0,
		Time:         time.Time{},
	}
}

func (o *FindManyOptions) Limit(limit int) *FindManyOptions {
	o.ArtworkLimit = limit
	return o
}

func (o *FindManyOptions) SortType(t SortType) *FindManyOptions {
	o.Sort = t
	return o
}

func (o *FindManyOptions) Order(or SortOrder) *FindManyOptions {
	o.SortOrder = or
	return o
}

func (o *FindManyOptions) SetTime(t time.Time) *FindManyOptions {
	o.Time = t
	return o
}

type SortType int

const (
	_ SortType = iota
	ByID
	ByFavourites
	ByTime
)

type SortOrder int

const (
	Ascending  SortOrder = 1
	Descending SortOrder = -1
)

type Artwork struct {
	ID         int       `json:"artwork_id" bson:"artwork_id"`
	Title      string    `json:"title" bson:"title"`
	Author     string    `json:"author" bson:"author"`
	URL        string    `json:"url" bson:"url"`
	Images     []string  `json:"images" bson:"images"`
	Favourites int       `json:"favourites" bson:"favourites"`
	NSFW       int       `json:"nsfw" bson:"nsfw"`
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`
}

type Counter struct {
	ID      string `json:"_id" bson:"_id"`
	Counter int    `json:"counter" bson:"counter"`
}

func (d *Database) nextID() (int, error) {
	sres := d.counters.FindOneAndUpdate(
		context.Background(),
		bson.D{{"_id", "artworks"}},
		bson.D{{"$inc", bson.D{{"counter", 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetUpsert(true),
	)

	counter := &Counter{}
	err := sres.Decode(counter)
	if err != nil {
		return 0, err
	}

	return counter.Counter, nil
}

func (d *Database) CreateArtwork(artwork *Artwork) (*Artwork, error) {
	logrus.Infof("Creating an artwork. URL: %v", artwork.URL)
	id, err := d.nextID()
	if err != nil {
		return nil, err
	}

	artwork.ID = id
	artwork.Favourites = 0
	_, err = d.artworks.InsertOne(context.Background(), artwork)
	if err != nil {
		return nil, err
	}

	return artwork, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (d *Database) IncrementFavourites(fav *NewFavourite) (*Artwork, error) {
	logrus.Infof("Incrementing a favourite count. Artwork ID: %v", fav.ID)
	res := d.artworks.FindOneAndUpdate(
		context.Background(),
		bson.D{{"artwork_id", fav.ID}},
		bson.D{
			{"$inc", bson.D{{"favourites", 1}, {"nsfw", boolToInt(fav.NSFW)}}},
			{"$currentDate", bson.D{{"updated_at", true}}},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	artwork := &Artwork{}
	err := res.Decode(artwork)
	if err != nil {
		return nil, err
	}

	return artwork, nil
}

func (d *Database) DecrementFavourites(fav *NewFavourite) (*Artwork, error) {
	logrus.Infof("Decrementing a favourite count. Artwork ID: %v", fav.ID)
	res := d.artworks.FindOneAndUpdate(
		context.Background(),
		bson.D{{"artwork_id", fav.ID}},
		bson.D{
			{"$inc", bson.D{{"favourites", -1}, {"nsfw", boolToInt(fav.NSFW) * -1}}},
			{"$currentDate", bson.D{{"updated_at", true}}},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	artwork := &Artwork{}
	err := res.Decode(artwork)
	if err != nil {
		return nil, err
	}

	return artwork, nil
}

func (d *Database) FindArtworkByID(id int) (*Artwork, error) {
	logrus.Infof("Finding an artwork by ID: %v", id)
	res := d.artworks.FindOne(
		context.Background(),
		bson.D{{"artwork_id", id}},
	)

	artwork := &Artwork{}
	err := res.Decode(artwork)
	if err != nil {
		return nil, err
	}

	return artwork, nil
}

func (d *Database) FindManyArtworks(favs []*NewFavourite, opts *FindManyOptions) ([]*Artwork, error) {
	IDs := make([]int, 0, len(favs))
	for _, f := range favs {
		IDs = append(IDs, f.ID)
	}

	logrus.Infof("Finding many artworks. Artwork IDs: %v", IDs)
	var (
		sort      = bson.D{}
		filter    = bson.D{}
		mongoOpts = options.Find()
	)

	switch opts.Sort {
	case ByID:
		sort = bson.D{{"artwork_id", opts.SortOrder}}
	case ByFavourites:
		sort = bson.D{{"favourites", opts.SortOrder}}
	case ByTime:
		sort = bson.D{{"created_at", opts.SortOrder}}
	}

	if !opts.Time.IsZero() {
		if len(IDs) > 0 {
			filter = bson.D{{"artwork_id", bson.D{{"$in", IDs}}}, {"created_at", bson.D{{"$gte", opts.Time}}}}
		} else {
			filter = bson.D{{"created_at", bson.D{{"$gte", opts.Time}}}}
		}
	} else if len(IDs) > 0 {
		filter = bson.D{{"artwork_id", bson.D{{"$in", IDs}}}}
	}

	mongoOpts.SetSort(sort)

	if opts.ArtworkLimit > 0 {
		mongoOpts.SetLimit(int64(opts.ArtworkLimit))
	}
	cur, err := d.artworks.Find(
		context.Background(),
		filter,
		mongoOpts,
	)
	if err != nil {
		return nil, err
	}

	artworks := make([]*Artwork, 0, len(IDs))
	err = cur.All(context.Background(), &artworks)
	if err != nil {
		return nil, err
	}

	return artworks, nil
}

func (d *Database) FindArtworkByURL(url string) (*Artwork, error) {
	logrus.Infof("Finding artwork by URL: %v", url)
	res := d.artworks.FindOne(
		context.Background(),
		bson.D{{"url", url}},
	)

	artwork := &Artwork{}
	err := res.Decode(artwork)
	if err != nil {
		return nil, err
	}

	return artwork, nil
}

func (d *Database) AddFavourite(userID string, artwork *Artwork, nsfw bool) (*Artwork, error) {
	logrus.Infof("Adding a favourite. User ID: %v", userID)
	found, err := d.FindArtworkByURL(artwork.URL)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			found, err = d.CreateArtwork(artwork)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	if found != nil {
		fav := &NewFavourite{found.ID, nsfw, time.Now()}
		success, err := d.UserAddFavourite(userID, fav)
		if err != nil {
			return nil, err
		}

		if success {
			artwork, err := d.IncrementFavourites(fav)
			if err != nil {
				return nil, err
			}

			return artwork, nil
		}
	}

	return nil, ErrFavouriteNotFound
}

func (d *Database) RemoveFavouriteURL(userID, url string) (*Artwork, error) {
	logrus.Infof("Removing a favourite by URL. User ID: %v. URL: %v", userID, url)
	artwork, err := d.FindArtworkByURL(url)
	if err != nil {
		return nil, err
	}

	fav, err := d.UserDeleteFavourite(userID, artwork.ID)
	if fav != nil {
		artwork, err := d.DecrementFavourites(fav)
		if err != nil {
			return nil, err
		}
		return artwork, nil
	}

	return nil, err
}

func (d *Database) RemoveFavouriteID(userID string, id int) (*Artwork, error) {
	logrus.Infof("Removing a favourite by ID. User ID: %v. URL: %v", userID, id)
	artwork, err := d.FindArtworkByID(id)
	if err != nil {
		return nil, err
	}

	fav, err := d.UserDeleteFavourite(userID, artwork.ID)
	if fav != nil {
		artwork, err := d.DecrementFavourites(fav)
		if err != nil {
			return nil, err
		}
		return artwork, nil
	}

	return nil, err
}
