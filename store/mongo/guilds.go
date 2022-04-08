package mongo

import (
	"context"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type guildStore struct {
	client *mongo.Client
	db     *mongo.Database
	col    *mongo.Collection
}

func (g guildStore) Guild(ctx context.Context, id string) (*store.Guild, error) {
	//If guild ID is empty, return DM guild settings.
	if id == "" {
		return store.UserGuild(), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	res := g.col.FindOne(ctx, bson.M{"guild_id": id})

	var guild store.Guild
	err := res.Decode(&guild)

	return &guild, err
}

func (g *guildStore) CreateGuild(ctx context.Context, id string) (*store.Guild, error) {
	guild := store.DefaultGuild(id)

	_, err := g.col.InsertOne(ctx, guild)
	if err != nil {
		return nil, err
	}

	return guild, nil
}

func (g *guildStore) UpdateGuild(ctx context.Context, guild *store.Guild) (*store.Guild, error) {
	guild.UpdatedAt = time.Now()
	_, err := g.col.ReplaceOne(ctx, bson.M{"guild_id": guild.ID}, guild, options.Replace().SetUpsert(false))
	if err != nil {
		return nil, err
	}

	return guild, nil
}

func (g *guildStore) AddArtChannels(ctx context.Context, guildID string, channels []string) (*store.Guild, error) {
	res := g.col.FindOneAndUpdate(
		ctx,
		bson.M{"guild_id": guildID, "art_channels": bson.M{"$nin": channels}},
		bson.M{"$addToSet": bson.M{"art_channels": bson.M{"$each": channels}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var guild store.Guild
	err := res.Decode(&guild)
	if err != nil {
		return nil, err
	}

	return &guild, nil
}

func (g *guildStore) DeleteArtChannels(ctx context.Context, guildID string, channels []string) (*store.Guild, error) {
	res := g.col.FindOneAndUpdate(
		ctx,
		bson.M{"guild_id": guildID, "art_channels": bson.M{"$all": channels}},
		bson.M{"$pullAll": bson.M{"art_channels": channels}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var guild store.Guild
	err := res.Decode(&guild)
	if err != nil {
		return nil, err
	}

	return &guild, nil
}
