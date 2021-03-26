package options

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Order int

const (
	Descending Order = iota - 1
	_
	Ascending
)

type Sort int

const (
	ByTime Sort = iota
	ByFavourites
)

func (s Sort) String() string {
	return map[Sort]string{
		ByTime:       "created_at",
		ByFavourites: "favourites",
	}[s]
}

//Find provides options for 'FindMany' endpoint in ArtworkModel.
type Find struct {
	Limit  int64
	Page   int64
	Order  Order
	Sort   Sort
	Filter *Filter
}

type FilterOne struct {
	ID  int    `query:"id"`
	URL string `query:"url"`
}

type Filter struct {
	ID     int    `query:"id"`
	Title  string `query:"title"`
	Author string `query:"author"`
	URL    string `query:"url"`
	Time   time.Duration
	//TODO: Tags   []string
}

func DefaultFind() Find {
	return Find{
		Limit:  100,
		Page:   0,
		Order:  Descending,
		Sort:   ByTime,
		Filter: &Filter{},
	}
}

func (a *Find) FindOptions() *options.FindOptions {
	skip := a.Limit * a.Page
	sort := bson.M{a.Sort.String(): a.Order}

	return options.Find().SetLimit(a.Limit).SetSkip(skip).SetSort(sort)
}

func (o *Filter) BSON() bson.D {
	filter := bson.D{}

	regex := func(key, value string) bson.E {
		return bson.E{key, bson.D{{"$regex", ".*" + value + ".*"}, {"$options", "i"}}}
	}

	switch {
	case o.ID != 0:
		filter = append(filter, bson.E{"artwork_id", o.ID})
	case o.URL != "":
		filter = append(filter, bson.E{"url", o.URL})
	default:
		if o.Author != "" {
			filter = append(filter, regex("author", o.Author))
		}

		if o.Title != "" {
			filter = append(filter, regex("title", o.Title))
		}

		if o.Time != 0 {
			filter = append(filter, bson.E{"created_at", bson.M{"$gte": time.Now().Add(-o.Time)}})
		}
	}

	return filter
}

//BSON turns the filter to BSON. It will error if both ID and URL are defaulted.
func (o *FilterOne) BSON() (bson.M, error) {
	switch {
	case o.ID != 0:
		return bson.M{"artwork_id": o.ID}, nil
	case o.URL != "":
		return bson.M{"url": o.URL}, nil
	}

	return nil, fmt.Errorf("No filter")
}
