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
	GuildID       string `bson:"guild_id" json:"guild_id"`
	Prefix        string `bson:"prefix" json:"prefix"`
	ReverseSearch string `bson:"reversesearch" json:"reversesearch"`
	LargeSet      int    `bson:"largeset" json:"largeset"`
	Pixiv         bool   `bson:"pixiv" json:"pixiv"`
	Repost        string `bson:"repost" json:"repost"`
	PromptEmoji   string `bson:"promptemoji" json:"promptemoji"`
}

//NewGuildSettings returns a new GuildSettings instance with given parameters.
func NewGuildSettings(guildID, prefix, repost, reverseSearch, promptemoji string, largeset int, pixiv bool) *GuildSettings {
	return &GuildSettings{
		GuildID:       guildID,
		ReverseSearch: reverseSearch,
		Prefix:        prefix,
		LargeSet:      largeset,
		Pixiv:         pixiv,
		Repost:        repost,
		PromptEmoji:   promptemoji,
	}
}

//DefaultGuildSettings returns a default GuildSettings struct.
func DefaultGuildSettings(guildID string) *GuildSettings {
	return &GuildSettings{
		GuildID:       guildID,
		Prefix:        "bt!",
		ReverseSearch: "saucenao",
		LargeSet:      3,
		Pixiv:         true,
		Repost:        "ask",
		PromptEmoji:   "👌",
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
