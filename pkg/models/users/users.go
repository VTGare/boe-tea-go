package users

import (
	"context"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type User struct {
	ID         string       `json:"id" bson:"user_id"`
	DM         bool         `json:"dm" bson:"dm"`
	Crosspost  bool         `json:"crosspost" bson:"crosspost"`
	Favourites []*Favourite `json:"favourites" bson:"new_favourites"`
	Groups     []*Group     `json:"groups" bson:"channel_groups"`
	CreatedAt  time.Time    `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at" bson:"updated_at"`
}

type Favourite struct {
	ArtworkID int       `json:"artwork_id" bson:"artwork_id"`
	NSFW      bool      `json:"nsfw" bson:"nsfw"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

type Group struct {
	Name     string   `json:"name" bson:"name"`
	Parent   string   `json:"parent" bson:"parent"`
	Children []string `json:"children" bson:"children"`
}

type Service interface {
	InsertOne(ctx context.Context, userID string) (*User, error)
	FindOne(ctx context.Context, userID string) (*User, error)
	DeleteOne(ctx context.Context, userID string) (*User, error)
	InsertFavourite(ctx context.Context, userID string, fav *Favourite) (*User, error)
	DeleteFavourite(ctx context.Context, userID string, fav *Favourite) (*User, error)
	InsertGroup(ctx context.Context, userID string, group *Group) (*User, error)
	DeleteGroup(ctx context.Context, userID string, group string) (*User, error)
	InsertToGroup(ctx context.Context, userID string, group string, child string) (*User, error)
	DeleteFromGroup(ctx context.Context, userID string, group string, child string) (*User, error)
}

type userService struct {
	db     *mongodb.Mongo
	logger *zap.SugaredLogger
}

func NewService(db *mongodb.Mongo, logger *zap.SugaredLogger) Service {
	return &userService{db, logger}
}

func (u userService) col() *mongo.Collection {
	return u.db.Database.Collection("user_settings")
}

func (u userService) FindOne(ctx context.Context, id string) (*User, error) {
	res := u.col().FindOne(ctx, bson.M{"user_id": id})

	var user User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u userService) InsertOne(ctx context.Context, id string) (*User, error) {
	user := &User{
		ID:         id,
		DM:         true,
		Crosspost:  true,
		Favourites: make([]*Favourite, 0),
		Groups:     make([]*Group, 0),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	_, err := u.col().InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (u userService) DeleteOne(ctx context.Context, id string) (*User, error) {
	res := u.col().FindOneAndDelete(ctx, bson.M{"user_id": id})

	var user User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u userService) InsertFavourite(ctx context.Context, id string, fav *Favourite) (*User, error) {
	res := u.col().FindOneAndUpdate(
		ctx,
		bson.M{"user_id": id, "new_favourites.artwork_id": bson.M{"$nin": []int{fav.ArtworkID}}},
		bson.M{"$addToSet": bson.M{"new_favourites": fav}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var user User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u userService) DeleteFavourite(ctx context.Context, id string, fav *Favourite) (*User, error) {
	res := u.col().FindOneAndUpdate(
		ctx,
		bson.M{"user_id": id, "new_favourites.artwork_id": fav.ArtworkID},
		bson.M{"$pull": bson.M{"new_favourites": bson.M{"artwork_id": fav.ArtworkID}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var user User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u userService) InsertGroup(ctx context.Context, userID string, group *Group) (*User, error) {
	res := u.col().FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID, "channel_groups.name": bson.M{"$ne": group.Name}, "channel_groups.parent": bson.M{"$ne": group.Parent}},
		bson.M{"$push": bson.M{"channel_groups": group}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var user User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u userService) DeleteGroup(ctx context.Context, userID, group string) (*User, error) {
	res := u.col().FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID, "channel_groups.name": group},
		bson.M{"$pull": bson.M{"channel_groups": bson.M{"name": group}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var user User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u userService) InsertToGroup(ctx context.Context, userID, group, child string) (*User, error) {
	res := u.col().FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID, "channel_groups.name": group, "channel_groups.children": bson.M{"$nin": []string{child}}},
		bson.M{"$addToSet": bson.M{"channel_groups.$.children": child}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var user User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u userService) DeleteFromGroup(ctx context.Context, userID, group, child string) (*User, error) {
	res := u.col().FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID, "channel_groups.name": group, "channel_groups.children": bson.M{"$in": []string{child}}},
		bson.M{"$pull": bson.M{"channel_groups.$.children": child}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var user User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *User) FindGroup(parentID string) (*Group, bool) {
	for _, group := range u.Groups {
		if group.Parent == parentID {
			return group, true
		}
	}

	return nil, false
}
