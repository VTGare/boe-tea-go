package guilds

import (
	"context"
	"time"

	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/database/mongodb"
	"github.com/VTGare/boe-tea-go/internal/validate"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type Guild struct {
	ID          string    `json:"id" bson:"guild_id" validate:"required"`
	Prefix      string    `json:"prefix" bson:"prefix" validate:"required,max=5"`
	Limit       int       `json:"limit" bson:"limit" validate:"required"`
	NSFW        bool      `json:"nsfw" bson:"nsfw" validate:"required"`
	Pixiv       bool      `json:"pixiv" bson:"pixiv" validate:"required"`
	Twitter     bool      `json:"twitter" bson:"twitter" validate:"required"`
	Repost      string    `json:"repost" bson:"repost" validate:"required"`
	Crosspost   bool      `json:"crosspost" bson:"crosspost" validate:"required"`
	Reactions   bool      `json:"reactions" bson:"reactions" validate:"required"`
	ArtChannels []string  `json:"art_channels" bson:"art_channels" validate:"required"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at" validate:"required"`
	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
}

type Service interface {
	All(context.Context) ([]*Guild, error)
	FindOne(context.Context, string) (*Guild, error)
	InsertOne(context.Context, string) (*Guild, error)
	DeleteOne(context.Context, string) (*Guild, error)
	ReplaceOne(context.Context, *Guild) (*Guild, error)
}

type guildService struct {
	db     *mongodb.Mongo
	logger *zap.SugaredLogger
	cache  *cache.Cache
}

func NewService(db *mongodb.Mongo, logger *zap.SugaredLogger) Service {
	cache := cache.New()

	return &guildService{db, logger, cache}
}

func (g guildService) col() *mongo.Collection {
	return g.db.Database.Collection("guildsettings")
}

func (g guildService) All(ctx context.Context) ([]*Guild, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cur, err := g.col().Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}

	guilds := make([]*Guild, 0)
	err = cur.All(ctx, &guilds)
	if err != nil {
		return nil, err
	}

	return guilds, nil
}

func (g guildService) FindOne(ctx context.Context, id string) (*Guild, error) {
	if guild, ok := g.get(id); ok {
		return guild, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	res := g.col().FindOne(ctx, bson.M{"guild_id": id})

	var guild Guild
	err := res.Decode(&guild)

	g.set(id, &guild)
	return &guild, err
}

func (g guildService) InsertOne(ctx context.Context, id string) (*Guild, error) {
	guild := DefaultGuild(id)

	_, err := g.col().InsertOne(ctx, guild)
	if err != nil {
		return nil, err
	}

	g.set(id, guild)
	return guild, nil
}

func (g guildService) DeleteOne(ctx context.Context, id string) (*Guild, error) {
	res := g.col().FindOneAndDelete(ctx, bson.M{"guild_id": id})

	var guild Guild
	err := res.Decode(&guild)
	if err != nil {
		return nil, err
	}

	return &guild, nil
}

func (g guildService) ReplaceOne(ctx context.Context, guild *Guild) (*Guild, error) {
	errs := validate.Struct(guild)
	if len(errs) != 0 {
		return nil, errs[0]
	}

	guild.UpdatedAt = time.Now()
	_, err := g.col().ReplaceOne(ctx, bson.M{"guild_id": guild.ID}, guild, options.Replace().SetUpsert(false))
	if err != nil {
		return nil, err
	}

	g.set(guild.ID, guild)
	return guild, nil
}

func (g guildService) set(id string, guild *Guild) {
	g.cache.Set(guild.ID, guild)
}

func (g guildService) get(id string) (*Guild, bool) {
	guild, ok := g.cache.Get(id)
	if !ok {
		return nil, false
	}

	return guild.(*Guild), true
}

func DefaultGuild(id string) *Guild {
	return &Guild{
		ID:          id,
		Prefix:      "bt!",
		Limit:       10,
		NSFW:        true,
		Pixiv:       true,
		Twitter:     true,
		Repost:      "enabled",
		Crosspost:   true,
		Reactions:   true,
		ArtChannels: make([]string, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}