package database

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
)

var (
	GuildCache = make(map[string]GuildSettings)
)

type GuildSettings struct {
	GuildID  string `bson:"guild_id" json:"guild_id"`
	Prefix   string `bson:"prefix" json:"prefix"`
	LargeSet int    `bson:"large_set" json:"large_set"`
	Pixiv    bool   `bson:"pixiv" json:"pixiv"`
	Twitter  bool   `bson:"twitter" json:"twitter"`
}

func NewGuildSettings(guildID, prefix string, large_set int, pixiv, twitter bool) *GuildSettings {
	return &GuildSettings{
		GuildID:  guildID,
		Prefix:   prefix,
		LargeSet: large_set,
		Pixiv:    pixiv,
		Twitter:  twitter,
	}
}

func DefaultGuildSettings(guildID string) *GuildSettings {
	return &GuildSettings{
		GuildID:  guildID,
		Prefix:   "bt!",
		LargeSet: 3,
		Pixiv:    true,
		Twitter:  true,
	}
}

func AllGuilds() *[]GuildSettings {
	collection := DB.Collection("guildsettings")
	cur, err := collection.Find(context.Background(), bson.M{})

	if err != nil {
		return &[]GuildSettings{}
	}

	guilds := make([]GuildSettings, 0)
	cur.All(context.Background(), &guilds)

	if err != nil {
		log.Println("Error decoding", err)
	}

	return &guilds
}

func InsertOneGuild(guild *GuildSettings) error {
	collection := DB.Collection("guildsettings")
	_, err := collection.InsertOne(context.Background(), guild)
	if err != nil {
		return err
	}
	return nil
}

func InserManyGuilds(guilds []interface{}) error {
	collection := DB.Collection("guildsettings")
	_, err := collection.InsertMany(context.Background(), guilds)
	if err != nil {
		return err
	}
	return nil
}

func RemoveGuild(guildID string) error {
	collection := DB.Collection("guildsettings")
	_, err := collection.DeleteOne(context.Background(), bson.M{"guild_id": guildID})
	if err != nil {
		return err
	}

	return nil
}
