package messages

import (
	"fmt"
)

func UserGroupsEmbed(username string) *UserGroups {
	return &UserGroups{
		Title:       fmt.Sprintf("%v's groups", username),
		Description: "To add a new group use `bt!newgroup` command.",
		Group:       "Group",
		Parent:      "Parent",
		Children:    "Children",
	}
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

func UserPushSuccess(name string, channels []string) string {
	return fmt.Sprintf(
		"Added following channels to group `%v`:\n%v",
		name,
		ListChannels(channels),
	)
}

func ErrUserPushFail(name string) error {
	return newUserError(fmt.Sprintf(
		"No channels were added to group `%v`. One of the following is true:\n%v\n%v\n%v",
		name,
		"• Group "+name+" doesn't exist;",
		"• All channels are already part of the group;",
		"• A channel is also a parent of this group.",
	))
}

func UserRemoveSuccess(name string, channels []string) string {
	return fmt.Sprintf(
		"Removed following channels from group `%v`:\n%v",
		name,
		ListChannels(channels),
	)
}

func ErrUserRemoveFail(name string) error {
	return newUserError(fmt.Sprintf(
		"No channels were removed from group `%v`. None of the channel were part of the group or group doesn't exist.",
		name,
	))
}

func ErrUserChannelAlreadyParent(id string) error {
	return newUserError(fmt.Sprintf(
		"Channel <#%v> | `%v` is already a parent",
		id, id,
	))
}

func UserCopyGroupSuccess(src, dest string, channels []string) string {
	return fmt.Sprintf(
		"Copied group `%v` to `%v`. Children channels:\n%v",
		src, dest, ListChannels(channels),
	)
}

func ErrUserCopyGroupFail(src, dest string) error {
	return newUserError(fmt.Sprintf(
		"Couldn't copy group `%v` to `%v`. One of the following is true:\n%v\n%v",
		src, dest,
		"• Group "+src+" doesn't exist;",
		"• Group "+dest+" already exists.",
	))
}

func ErrUserNoFavourites(id string) error {
	return newUserError(fmt.Sprintf(
		"User <@%v> doesn't have any favourites", id,
	))
}

func ErrUnknownUserSetting(setting string) error {
	return newUserError(
		fmt.Sprintf("Setting `%v` doesn't exist. Please use `bt!profile` to see existing settings.", setting),
	)
}
