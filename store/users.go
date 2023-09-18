package store

import (
	"context"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
)

type UserStore interface {
	CreateUser(ctx context.Context, userID string) (*User, error)
	UpdateUser(ctx context.Context, user *User) (*User, error)
	User(ctx context.Context, userID string) (*User, error)

	CreateCrosspostGroup(ctx context.Context, userID string, group *Group) (*User, error)
	CreateCrosspostPair(ctx context.Context, userID string, pair *Group) (*User, error)
	DeleteCrosspostGroup(ctx context.Context, userID string, group string) (*User, error)
	EditCrosspostParent(ctx context.Context, userID string, group string, parent string) (*User, error)
	RenameCrosspostGroup(ctx context.Context, userID string, name string, newName string) (*User, error)
	AddCrosspostChannel(ctx context.Context, userID string, group string, child string) (*User, error)
	DeleteCrosspostChannel(ctx context.Context, userID string, group string, child string) (*User, error)
}

type User struct {
	ID        string    `json:"id" bson:"user_id"`
	DM        bool      `json:"dm" bson:"dm"`
	Crosspost bool      `json:"crosspost" bson:"crosspost"`
	Ignore    bool      `json:"ignore" bson:"ignore"`
	Groups    []*Group  `json:"groups" bson:"channel_groups"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

type Group struct {
	Name     string   `json:"name" bson:"name"`
	Parent   string   `json:"parent" bson:"parent"`
	Children []string `json:"children" bson:"children"`
	IsPair   bool     `json:"is_pair" bson:"is_pair"`
}

func DefaultUser(id string) *User {
	return &User{
		ID:        id,
		DM:        true,
		Crosspost: true,
		Groups:    make([]*Group, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (u *User) FindGroup(channelID string) (*Group, bool) {
	for _, group := range u.Groups {
		if group.IsPair {
			if arrays.Any(group.Children, channelID) {
				return group, true
			}

			continue
		}

		if group.Parent == channelID {
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
