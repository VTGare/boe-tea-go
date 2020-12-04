package database

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PixivReverseProxy int

const (
	KotoriLove PixivReverseProxy = iota
	PixivCatProxy
	PixivCat
)

var (
	DevSet = &DevSettings{}
)

type DevSettings struct {
	PixivReverseProxy PixivReverseProxy `json:"pixiv" bson:"pixiv"`
	NitterInstance    string            `json:"nitter" bson:"nitter"`
}

func (d *Database) LoadSettings() error {
	res := d.devSettings.FindOne(context.Background(), bson.M{}, options.FindOne())
	err := res.Decode(DevSet)

	if err != nil {
		return err
	}

	return nil
}

func (d *Database) ChangeDevSetting(setting string, newSetting interface{}) error {
	res := d.devSettings.FindOneAndUpdate(context.Background(), bson.M{}, bson.M{
		"$set": bson.M{
			setting:      newSetting,
			"updated_at": time.Now(),
		},
	}, options.FindOneAndUpdate().SetReturnDocument(options.After))

	err := res.Decode(DevSet)
	if err != nil {
		return err
	}

	return nil
}

func (r PixivReverseProxy) String() string {
	switch r {
	case KotoriLove:
		return "Kotori"
	case PixivCat:
		return "PixivCat"
	case PixivCatProxy:
		return "PixivCatProxy"
	}

	return "unknown"
}
