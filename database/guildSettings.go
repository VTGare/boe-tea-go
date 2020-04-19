package database

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
)

var (
	//GuildCache stores guild settings locally
	GuildCache = make(map[string]GuildSettings)
)

//GuildSettings is a database model for per guild bot settings
type GuildSettings struct {
	GuildID  string `bson:"guild_id" json:"guild_id"`
	Prefix   string `bson:"prefix" json:"prefix"`
	LargeSet int    `bson:"large_set" json:"large_set"`
	Pixiv    bool   `bson:"pixiv" json:"pixiv"`
	Twitter  bool   `bson:"twitter" json:"twitter"`
	RepostAs string `bson:"repost_as" json:"repost_as"`
}

//NewGuildSettings returns a new GuildSettings instance with given parameters.
func NewGuildSettings(guildID, prefix, repostAs string, largeset int, pixiv, twitter bool) *GuildSettings {
	return &GuildSettings{
		GuildID:  guildID,
		Prefix:   prefix,
		LargeSet: largeset,
		Pixiv:    pixiv,
		Twitter:  twitter,
		RepostAs: repostAs,
	}
}

//DefaultGuildSettings returns a default GuildSettings struct.
func DefaultGuildSettings(guildID string) *GuildSettings {
	return &GuildSettings{
		GuildID:  guildID,
		Prefix:   "bt!",
		LargeSet: 3,
		Pixiv:    true,
		Twitter:  false,
		RepostAs: "ask",
	}
}

//AllGuilds returns all guilds from a database.
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

//InsertOneGuild inserts one guild to a database
func InsertOneGuild(guild *GuildSettings) error {
	collection := DB.Collection("guildsettings")
	_, err := collection.InsertOne(context.Background(), guild)
	if err != nil {
		return err
	}
	return nil
}

//InsertManyGuilds insert a bulk of guilds to a database
func InsertManyGuilds(guilds []interface{}) error {
	collection := DB.Collection("guildsettings")
	_, err := collection.InsertMany(context.Background(), guilds)
	if err != nil {
		return err
	}
	return nil
}

//RemoveGuild removes a guild from a database.
func RemoveGuild(guildID string) error {
	collection := DB.Collection("guildsettings")
	_, err := collection.DeleteOne(context.Background(), bson.M{"guild_id": guildID})
	if err != nil {
		return err
	}

	return nil
}
