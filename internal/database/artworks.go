package database

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SortArtworks int

const (
	_ SortArtworks = iota
	ByID
	ByFavourites
	ByTime
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
	logrus.Infof("Creating an artwork. URL: %s", artwork.URL)
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
	logrus.Infof("Incrementing a favourite count. Artwork ID: %s", fav.ID)
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
	logrus.Infof("Decrementing a favourite count. Artwork ID: %s", fav.ID)
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

func (d *Database) FindManyArtworks(favs []*NewFavourite, sortType SortArtworks) ([]*Artwork, error) {
	IDs := make([]int, 0, len(favs))
	for _, f := range favs {
		IDs = append(IDs, f.ID)
	}

	logrus.Infof("Finding many artworks. Artwork IDs: %v", IDs)
	var (
		sort   bson.D
		filter bson.D
		opts   = options.Find()
	)

	switch sortType {
	case ByID:
		sort = bson.D{{"artwork_id", 1}}
	case ByFavourites:
		sort = bson.D{{"favourites", -1}}
	case ByTime:
		sort = bson.D{{"created_at", 1}}
	}
	opts.SetSort(sort)

	if len(IDs) > 0 {
		filter = bson.D{{"artwork_id", bson.D{{"$in", IDs}}}}
	} else {
		filter = bson.D{}
		opts.SetLimit(10)
	}

	cur, err := d.artworks.Find(
		context.Background(),
		filter,
		opts,
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
	logrus.Infof("Finding artwork by URL: %s", url)
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
	logrus.Infof("Adding a favourite. User ID: %s", userID)
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
	logrus.Infof("Removing a favourite by URL. User ID: %s. URL: %s", userID, url)
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
	logrus.Infof("Removing a favourite by ID. User ID: %s. URL: %s", userID, id)
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
