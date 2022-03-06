package store

import (
	"context"
	"time"
)

type UserStore interface {
	CreateUser(ctx context.Context, userID string) (*User, error)
	UpdateUser(ctx context.Context, user *User) (*User, error)
	User(ctx context.Context, userID string) (*User, error)

	AddFavourite(ctx context.Context, userID string, fav *Favourite) error
	DeleteFavourite(ctx context.Context, userID string, fav *Favourite) error

	CreateCrosspostGroup(ctx context.Context, userID string, group *Group) (*User, error)
	DeleteCrosspostGroup(ctx context.Context, userID string, group string) (*User, error)
	AddCrosspostChannel(ctx context.Context, userID string, group string, child string) (*User, error)
	DeleteCrosspostChannel(ctx context.Context, userID string, group string, child string) (*User, error)
}

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

func DefaultUser(id string) *User {
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
