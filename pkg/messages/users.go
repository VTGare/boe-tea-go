package messages

import (
	"fmt"
	"strings"

	"github.com/VTGare/boe-tea-go/internal/arrays"
)

type UserProfile struct {
	Title      string
	Settings   string
	DM         string
	Crosspost  string
	Stats      string
	Groups     string
	Favourites string
}

func UserProfileEmbed(username string) *UserProfile {
	return &UserProfile{
		Title:      fmt.Sprintf("%v's profile", username),
		Settings:   "Settings",
		DM:         "DM",
		Crosspost:  "Crosspost",
		Stats:      "Stats",
		Groups:     "Groups",
		Favourites: "Favourites",
	}
}

type UserGroups struct {
	Title       string
	Description string
	Group       string
	Parent      string
	Children    string
}

func UserGroupsEmbed(username string) *UserGroups {
	return &UserGroups{
		Title:       fmt.Sprintf("%v's groups", username),
		Description: "To add a new group use `bt!newgroup` command.",
		Group:       "Group",
		Parent:      "Parent",
		Children:    "Children",
	}
}

func UserPushSuccess(name string, channels []string) string {
	return fmt.Sprintf(
		"Added following channels to group `%v`:\n%v",
		name,
		strings.Join(
			arrays.MapString(
				channels,
				func(s string) string {
					return fmt.Sprintf("<#%v> | `$%v`", s, s)
				},
			),
			" • ",
		))
}

func UserPushFail(name string) error {
	return newUserError(fmt.Sprintf(
		"No channels were added to group `%v`. One of the following is true:\n%v\n%v",
		name,
		"• All channels are already part of the group;",
		"• A channel is also a parent of this group.",
	))
}

func UserRemoveSuccess(name string, channels []string) string {
	return fmt.Sprintf(
		"Remove following channels from group `%v`:\n%v",
		name,
		strings.Join(
			arrays.MapString(
				channels,
				func(s string) string {
					return fmt.Sprintf("<#%v> | `$%v`", s, s)
				},
			),
			" • ",
		))
}

func UserRemoveFail(name string) error {
	return newUserError(fmt.Sprintf(
		"No channels were removed from group `%v`. None of the channel were part of the group.",
		name,
	))
}
