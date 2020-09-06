package database

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
)

var (
	userCache = make(map[string]*UserSettings, 0)
)

type UserSettings struct {
	ID            string   `json:"user_id" bson:"user_id"`
	Crosspost     bool     `json:"crosspost" bson:"crosspost"`
	ChannelGroups []*Group `json:"channel_groups" bson:"channel_groups"`
}

type Group struct {
	Name       string   `json:"name" bson:"name"`
	ChannelIDs []string `json:"channel_id" bson:"channel_id"`
}

func NewUserSettings(id string) *UserSettings {
	return &UserSettings{
		ID:            id,
		Crosspost:     true,
		ChannelGroups: make([]*Group, 0),
	}
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

func (d *Database) CreateGroup(userID string, groupName string, channelIDs ...string) error {
	user := d.FindUser(userID)
	if user == nil {
		return fmt.Errorf("User not found: %v", userID)
	}

	for _, g := range user.ChannelGroups {
		if g.Name == groupName {
			return fmt.Errorf("Group %v already exists", groupName)
		}
	}

	user.ChannelGroups = append(user.ChannelGroups, &Group{groupName, channelIDs})
	res := d.UserSettings.FindOneAndReplace(context.Background(), bson.M{"user_id": userID}, user)
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
	if res.Err() != nil {
		return res.Err()
	}

	return nil
}

func (d *Database) AddToGroup(userID string, groupName string, channelIDs ...string) error {
	user := d.FindUser(userID)
	if user == nil {
		return fmt.Errorf("User not found: %v", userID)
	}

	group, _ := user.findGroup(groupName)
	if group == nil {
		err := d.CreateGroup(userID, groupName, channelIDs...)
		if err != nil {
			return err
		}

		return nil
	}

	group.ChannelIDs = append(group.ChannelIDs, channelIDs...)
	res := d.UserSettings.FindOneAndReplace(context.Background(), bson.M{"user_id": userID}, user)
	if res.Err() != nil {
		return res.Err()
	}

	return nil
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

	group, index := user.findGroup(groupName)
	if group != nil {
		for _, id := range channelID {
			for ind, channel := range group.ChannelIDs {
				if channel == id {
					found = append(found, group.ChannelIDs[ind])
					group.ChannelIDs = append(group.ChannelIDs[:ind], group.ChannelIDs[ind+1:]...)
					break
				}
			}
		}

		if len(group.ChannelIDs) == 0 {
			user.ChannelGroups = append(user.ChannelGroups[:index], user.ChannelGroups[index+1:]...)
		}

		if len(found) > 0 {
			res := d.UserSettings.FindOneAndReplace(context.Background(), bson.M{"user_id": userID}, user)
			if res.Err() != nil {
				return nil, res.Err()
			}
			userCache[userID] = user
		}
	}

	return found, nil
}

func (us *UserSettings) findGroup(name string) (*Group, int) {
	for ind, group := range us.ChannelGroups {
		if group.Name == name {
			return group, ind
		}
	}

	return nil, -1
}

func (us *UserSettings) GroupByChannelID(channelID string) *Group {
	for _, g := range us.ChannelGroups {
		for _, c := range g.ChannelIDs {
			if c == channelID {
				return g
			}
		}
	}
	return nil
}

func (us *UserSettings) Channels(id string) []string {
	for _, g := range us.ChannelGroups {
		for _, c := range g.ChannelIDs {
			if c == id {
				return g.ChannelIDs
			}
		}
	}
	return nil
}
