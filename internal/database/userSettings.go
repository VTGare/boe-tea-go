package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	userCache = make(map[string]*UserSettings, 0)
)

type UserSettings struct {
	ID            string          `json:"user_id" bson:"user_id"`
	DM            bool            `json:"dm" bson:"dm"`
	Crosspost     bool            `json:"crosspost" bson:"crosspost"`
	NewFavourites []*NewFavourite `json:"new_favourites" bson:"new_favourites"`
	ChannelGroups []*Group        `json:"channel_groups" bson:"channel_groups"`
	CreatedAt     time.Time       `json:"created_at" bson:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" bson:"updated_at"`
}

type NewFavourite struct {
	ID        int       `json:"artwork_id" bson:"artwork_id"`
	NSFW      bool      `json:"nsfw" bson:"nsfw"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

type Group struct {
	Name     string   `json:"name" bson:"name"`
	Parent   string   `json:"parent" bson:"parent"`
	Children []string `json:"children" bson:"children"`
}

func NewUserSettings(id string) *UserSettings {
	return &UserSettings{
		ID:            id,
		Crosspost:     true,
		NewFavourites: make([]*NewFavourite, 0),
		ChannelGroups: make([]*Group, 0),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func FilterFavourites(favourites []*NewFavourite, filter func(*NewFavourite) bool) []*NewFavourite {
	filtered := make([]*NewFavourite, 0)
	for _, f := range favourites {
		if filter(f) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func (d *Database) AllUsers() ([]*UserSettings, error) {
	cur, err := d.UserSettings.Find(context.Background(), bson.M{})

	if err != nil {
		return nil, err
	}

	s := make([]*UserSettings, 0)
	cur.All(context.Background(), &s)

	if err != nil {
		return nil, err
	}

	for _, u := range s {
		userCache[u.ID] = u
	}
	return s, nil
}

func (d *Database) FindUser(id string) *UserSettings {
	if user, ok := userCache[id]; ok {
		return user
	}

	return nil
}

func (d *Database) InsertOneUser(user *UserSettings) error {
	_, err := d.UserSettings.InsertOne(context.Background(), user)
	if err != nil {
		return err
	}

	userCache[user.ID] = user
	return nil
}

func (d *Database) RemoveUser(id string) error {
	_, err := d.UserSettings.DeleteOne(context.Background(), bson.M{"user_id": id})
	if err != nil {
		return err
	}

	delete(userCache, id)
	return nil
}

func (d *Database) CreateGroup(userID string, groupName string, parentID string) error {
	user := d.FindUser(userID)
	if user == nil {
		return fmt.Errorf("User not found: %v", userID)
	}

	for _, g := range user.ChannelGroups {
		if g.Name == groupName {
			return fmt.Errorf("Group %v already exists", groupName)
		}

		if g.Parent == parentID {
			return fmt.Errorf("Group with a parent channel ID [%v] already exists", parentID)
		}
	}

	user.ChannelGroups = append(user.ChannelGroups, &Group{groupName, parentID, make([]string, 0)})
	user.UpdatedAt = time.Now()
	res := d.UserSettings.FindOneAndReplace(context.Background(), bson.M{"user_id": userID}, user)
	if res.Err() != nil {
		return res.Err()
	}

	return nil
}

func (d *Database) PushGroup(userID string, group *Group) error {
	user := d.FindUser(userID)
	if user == nil {
		return fmt.Errorf("User not found: %v", userID)
	}

	for _, g := range user.ChannelGroups {
		if g.Name == group.Name {
			return fmt.Errorf("Group %v already exists", group.Name)
		}

		if g.Parent == group.Parent {
			return fmt.Errorf("Group with a parent channel ID [%v] already exists", group.Parent)
		}
	}

	user.ChannelGroups = append(user.ChannelGroups, group)
	res := d.UserSettings.FindOneAndReplace(context.Background(), bson.M{"user_id": userID}, user)
	user.UpdatedAt = time.Now()
	if res.Err() != nil {
		return res.Err()
	}

	return nil
}

func (d *Database) DeleteGroup(userID string, groupName string) error {
	user := d.FindUser(userID)
	if user == nil {
		return fmt.Errorf("User not found: %v", userID)
	}

	for ind, group := range user.ChannelGroups {
		if group.Name == groupName {
			user.ChannelGroups = append(user.ChannelGroups[:ind], user.ChannelGroups[ind+1:]...)
			break
		}
	}

	res := d.UserSettings.FindOneAndReplace(context.Background(), bson.M{"user_id": userID}, user)
	user.UpdatedAt = time.Now()
	if res.Err() != nil {
		return res.Err()
	}

	return nil
}

func (d *Database) AddToGroup(userID string, groupName string, channelIDs ...string) ([]string, error) {
	var (
		added = make([]string, 0)
	)

	if len(channelIDs) == 0 {
		return nil, fmt.Errorf("no valid channel IDs were found")
	}

	user := d.FindUser(userID)
	if user == nil {
		return nil, fmt.Errorf("User not found: %v", userID)
	}

	group, _ := user.FindGroup(groupName)
	if group == nil {
		return nil, fmt.Errorf("Group doesn't exist: %v", groupName)
	}

	for _, c := range channelIDs {
		if c != group.Parent {
			added = append(added, c)
			group.Children = append(group.Children, c)
		}
	}

	res := d.UserSettings.FindOneAndReplace(context.Background(), bson.M{"user_id": userID}, user)
	user.UpdatedAt = time.Now()
	if res.Err() != nil {
		return nil, res.Err()
	}

	return added, nil
}

//RemoveFromGroup removes channel IDs from a cross-post group and returns removed elements.
func (d *Database) RemoveFromGroup(userID string, groupName string, channelID ...string) ([]string, error) {
	var (
		user  = d.FindUser(userID)
		found = make([]string, 0)
	)

	if user == nil {
		return nil, fmt.Errorf("User not found: %v", userID)
	}

	group, _ := user.FindGroup(groupName)
	if group != nil {
		for _, id := range channelID {
			for ind, channel := range group.Children {
				if channel == id {
					found = append(found, group.Children[ind])
					group.Children = append(group.Children[:ind], group.Children[ind+1:]...)
					break
				}
			}
		}

		if len(found) > 0 {
			res := d.UserSettings.FindOneAndReplace(context.Background(), bson.M{"user_id": userID}, user)
			user.UpdatedAt = time.Now()
			if res.Err() != nil {
				return nil, res.Err()
			}
			userCache[userID] = user
		}
	}

	return found, nil
}

func (us *UserSettings) FindGroup(name string) (*Group, int) {
	for ind, group := range us.ChannelGroups {
		if group.Name == name {
			return group, ind
		}
	}

	return nil, -1
}

func (us *UserSettings) findParent(parent string) (*Group, int) {
	for ind, group := range us.ChannelGroups {
		if group.Parent == parent {
			return group, ind
		}
	}

	return nil, -1
}

func (us *UserSettings) Channels(parent string) []string {
	g, _ := us.findParent(parent)

	if g != nil {
		return g.Children
	}
	return nil
}

func (d *Database) UserAddFavourite(userID string, fav *NewFavourite) (bool, error) {
	var user *UserSettings
	if user = d.FindUser(userID); user == nil {
		user = NewUserSettings(userID)
		err := d.InsertOneUser(user)
		if err != nil {
			return false, err
		}
	}

	res := d.UserSettings.FindOneAndUpdate(
		context.Background(),
		bson.D{{"user_id", userID}, {"new_favourites.artwork_id", bson.D{{"$nin", []int{fav.ID}}}}},
		bson.D{{"$addToSet", bson.D{{"new_favourites", fav}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var updated = &UserSettings{}
	if err := res.Decode(updated); err != nil {
		return false, err
	}

	userCache[userID] = updated
	return true, nil
}

func (d *Database) UserDeleteFavourite(userID string, artworkID int) (*NewFavourite, error) {
	var user *UserSettings
	if user = d.FindUser(userID); user == nil {
		user = NewUserSettings(userID)
		err := d.InsertOneUser(user)
		if err != nil {
			return nil, err
		}
	}

	var fav *NewFavourite
	for _, f := range user.NewFavourites {
		if f.ID == artworkID {
			fav = f
		}
	}

	if fav != nil {
		res := d.UserSettings.FindOneAndUpdate(
			context.Background(),
			bson.D{{"user_id", userID}, {"new_favourites.artwork_id", artworkID}},
			bson.D{{"$pull", bson.D{{"new_favourites", bson.D{{"artwork_id", artworkID}}}}}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		)

		if err := res.Err(); err == nil {
			var user = &UserSettings{}
			res.Decode(user)

			userCache[userID] = user
			return fav, nil
		}
	}

	return nil, ErrFavouriteNotFound
}

func (d *Database) ChangeUserSetting(userID, setting string, newSetting interface{}) error {
	res := d.UserSettings.FindOneAndUpdate(context.Background(), bson.M{
		"user_id": userID,
	}, bson.M{
		"$set": bson.M{
			setting:      newSetting,
			"updated_at": time.Now(),
		},
	}, options.FindOneAndUpdate().SetReturnDocument(options.After))

	user := &UserSettings{}
	err := res.Decode(user)
	if err != nil {
		return err
	}

	userCache[userID] = user
	return nil
}
