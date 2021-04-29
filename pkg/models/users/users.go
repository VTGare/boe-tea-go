package users

import (
	"context"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database/mongodb"
	"github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
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

type UserFavouriteResult struct {
	User    *User
	Artwork *artworks.Artwork
}

type Service interface {
	InsertOne(ctx context.Context, userID string) (*User, error)
	FindOne(ctx context.Context, userID string) (*User, error)
	FindOneOrCreate(ctx context.Context, userID string) (*User, error)
	DeleteOne(ctx context.Context, userID string) (*User, error)
	InsertFavourite(ctx context.Context, userID string, fav *Favourite) (*UserFavouriteResult, error)
	DeleteFavourite(ctx context.Context, userID string, fav *Favourite) (*UserFavouriteResult, error)
	InsertGroup(ctx context.Context, userID string, group *Group) (*User, error)
	DeleteGroup(ctx context.Context, userID string, group string) (*User, error)
	InsertToGroup(ctx context.Context, userID string, group string, child string) (*User, error)
	DeleteFromGroup(ctx context.Context, userID string, group string, child string) (*User, error)
	ReplaceOne(ctx context.Context, user *User) (*User, error)
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

func (u userService) artworks() *mongo.Collection {
	return u.db.Database.Collection("artworks")
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
	user := defaultUser(id)

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

func (u userService) InsertFavourite(ctx context.Context, id string, fav *Favourite) (*UserFavouriteResult, error) {
	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)

	session, err := u.db.Client.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	callback := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		nsfw := 0
		if fav.NSFW {
			nsfw++
		}

		res := u.artworks().FindOneAndUpdate(
			sessionCtx,
			bson.M{"artwork_id": fav.ArtworkID},
			bson.M{"$inc": bson.M{"favourites": 1, "nsfw": nsfw}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		)

		var artwork artworks.Artwork
		err := res.Decode(&artwork)
		if err != nil {
			return nil, err
		}

		res = u.col().FindOneAndUpdate(
			sessionCtx,
			bson.M{"user_id": id, "new_favourites.artwork_id": bson.M{"$nin": []int{fav.ArtworkID}}},
			bson.M{"$addToSet": bson.M{"new_favourites": fav}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		)

		var user User
		err = res.Decode(&user)
		if err != nil {
			return nil, err
		}

		return &UserFavouriteResult{
			Artwork: &artwork,
			User:    &user,
		}, nil
	}

	res, err := session.WithTransaction(ctx, callback, txnOpts)
	if err != nil {
		return nil, err
	}

	return res.(*UserFavouriteResult), nil
}

func (u userService) DeleteFavourite(ctx context.Context, id string, fav *Favourite) (*UserFavouriteResult, error) {
	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)

	session, err := u.db.Client.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	callback := func(sessionCtx mongo.SessionContext) (interface{}, error) {
		nsfw := 0
		if fav.NSFW {
			nsfw--
		}

		res := u.artworks().FindOneAndUpdate(
			sessionCtx,
			bson.M{"artwork_id": fav.ArtworkID},
			bson.M{"$inc": bson.M{"favourites": -1, "nsfw": nsfw}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		)

		var artwork artworks.Artwork
		err := res.Decode(&artwork)
		if err != nil {
			return nil, err
		}

		res = u.col().FindOneAndUpdate(
			ctx,
			bson.M{"user_id": id, "new_favourites.artwork_id": fav.ArtworkID},
			bson.M{"$pull": bson.M{"new_favourites": bson.M{"artwork_id": fav.ArtworkID}}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		)

		var user User
		err = res.Decode(&user)
		if err != nil {
			return nil, err
		}

		return &UserFavouriteResult{
			Artwork: &artwork,
			User:    &user,
		}, nil
	}

	res, err := session.WithTransaction(ctx, callback, txnOpts)
	if err != nil {
		return nil, err
	}

	return res.(*UserFavouriteResult), nil
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
		bson.M{"user_id": userID, "channel_groups.name": group},
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
		bson.M{"user_id": userID, "channel_groups.name": group},
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

func (u userService) FindOneOrCreate(ctx context.Context, userID string) (*User, error) {
	res := u.col().FindOneAndUpdate(
		ctx,
		bson.M{
			"user_id": userID,
		},
		bson.M{
			"$setOnInsert": defaultUser(userID),
		},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	)

	var user User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u userService) ReplaceOne(ctx context.Context, user *User) (*User, error) {
	user.UpdatedAt = time.Now()
	_, err := u.col().ReplaceOne(
		ctx,
		bson.M{"user_id": user.ID},
		user,
	)

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (u *User) FindGroup(parentID string) (*Group, bool) {
	for _, group := range u.Groups {
		if group.Parent == parentID {
			return group, true
		}
	}

	return nil, false
}

func (u *User) FindGroupByName(name string) (*Group, bool) {
	for _, group := range u.Groups {
		if group.Name == name {
			return group, true
		}
	}

	return nil, false
}

func (u *User) FindFavourite(artworkID int) (*Favourite, bool) {
	for _, fav := range u.Favourites {
		if fav.ArtworkID == artworkID {
			return fav, true
		}
	}

	return nil, false
}

func defaultUser(id string) *User {
	return &User{
		ID:         id,
		DM:         true,
		Crosspost:  true,
		Favourites: make([]*Favourite, 0),
		Groups:     make([]*Group, 0),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
