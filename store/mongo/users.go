package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type userStore struct {
	client *mongo.Client
	db     *mongo.Database
	col    *mongo.Collection
}

func UserStore(client *mongo.Client, database string) store.UserStore {
	db := client.Database(database)
	col := db.Collection("users")

	return &userStore{
		client: client,
		db:     db,
		col:    col,
	}
}

func (u *userStore) artworks() *mongo.Collection {
	return u.db.Collection("artworks")
}

func (u *userStore) User(ctx context.Context, userID string) (*store.User, error) {
	res := u.col.FindOneAndUpdate(
		ctx,
		bson.M{
			"user_id": userID,
		},
		bson.M{
			"$setOnInsert": store.DefaultUser(userID),
		},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	)

	var user store.User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *userStore) CreateUser(ctx context.Context, id string) (*store.User, error) {
	user := store.DefaultUser(id)

	_, err := u.col.InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (u *userStore) AddFavourite(ctx context.Context, id string, fav *store.Favourite) error {
	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)

	session, err := u.client.StartSession()
	if err != nil {
		return fmt.Errorf("failed to start a session: %w", err)
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

		var artwork *store.Artwork
		err := res.Decode(&artwork)
		if err != nil {
			return nil, fmt.Errorf("failed to decode artwork: %w", err)
		}

		res = u.col.FindOneAndUpdate(
			sessionCtx,
			bson.M{"user_id": id, "new_favourites.artwork_id": bson.M{"$nin": []int{fav.ArtworkID}}},
			bson.M{"$addToSet": bson.M{"new_favourites": fav}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		)

		var user store.User
		err = res.Decode(&user)
		if err != nil {
			return nil, err
		}

		return nil, nil
	}

	if _, err := session.WithTransaction(ctx, callback, txnOpts); err != nil {
		return err
	}

	return nil
}

func (u *userStore) DeleteFavourite(ctx context.Context, id string, fav *store.Favourite) error {
	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)

	session, err := u.client.StartSession()
	if err != nil {
		return fmt.Errorf("failed to start a session: %w", err)
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

		var artwork *store.Artwork
		err := res.Decode(&artwork)
		if err != nil {
			return nil, err
		}

		res = u.col.FindOneAndUpdate(
			ctx,
			bson.M{"user_id": id, "new_favourites.artwork_id": fav.ArtworkID},
			bson.M{"$pull": bson.M{"new_favourites": bson.M{"artwork_id": fav.ArtworkID}}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		)

		var user store.User
		if err := res.Decode(&user); err != nil {
			return nil, err
		}

		return nil, nil
	}

	if _, err := session.WithTransaction(ctx, callback, txnOpts); err != nil {
		return err
	}

	return nil
}

func (u *userStore) CreateCrosspostGroup(ctx context.Context, userID string, group *store.Group) (*store.User, error) {
	res := u.col.FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID, "channel_groups.name": bson.M{"$ne": group.Name}, "channel_groups.parent": bson.M{"$ne": group.Parent}},
		bson.M{"$push": bson.M{"channel_groups": group}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var user store.User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *userStore) DeleteCrosspostGroup(ctx context.Context, userID, group string) (*store.User, error) {
	res := u.col.FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID, "channel_groups.name": group},
		bson.M{"$pull": bson.M{"channel_groups": bson.M{"name": group}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var user store.User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *userStore) AddCrosspostChannel(ctx context.Context, userID, group, child string) (*store.User, error) {
	res := u.col.FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID, "channel_groups.name": group},
		bson.M{"$addToSet": bson.M{"channel_groups.$.children": child}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var user store.User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *userStore) DeleteCrosspostChannel(ctx context.Context, userID, group, child string) (*store.User, error) {
	res := u.col.FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID, "channel_groups.name": group},
		bson.M{"$pull": bson.M{"channel_groups.$.children": child}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var user store.User
	err := res.Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *userStore) UpdateUser(ctx context.Context, user *store.User) (*store.User, error) {
	user.UpdatedAt = time.Now()
	_, err := u.col.ReplaceOne(
		ctx,
		bson.M{"user_id": user.ID},
		user,
	)

	if err != nil {
		return nil, err
	}

	return user, nil
}
