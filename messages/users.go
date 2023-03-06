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
		Title:     fmt.Sprintf("%v's profile", username),
		Settings:  "Settings",
		DM:        "DM",
		Crosspost: "Crosspost",
		Stats:     "Stats",
		Groups:    "Groups",
		Bookmarks: "Bookmarks",
	}
}

func UserPushSuccess(name string, channels []string) string {
	return fmt.Sprintf("Added the following channels to `%v`:\n%v", name, ListChannels(channels))
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
	return fmt.Sprintf("Removed the following channels from group `%v`:\n%v", name, ListChannels(channels))
}

func ErrUserRemoveFail(name string) error {
	msg := fmt.Sprintf("No channels were removed from `%v`. None of the channels were part of the group or group doesn't exist.", name)
	return newUserError(msg)
}

func ErrUserChannelAlreadyParent(id string) error {
	msg := fmt.Sprintf("Channel <#%v> | `%v` is already a parent", id, id)
	return newUserError(msg)
}

func UserCopyGroupSuccess(src, dest string, channels []string) string {
	return fmt.Sprintf(
		"Copied group `%v` to `%v`. Inherited children channels:\n%v",
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

func ErrUserNoBookmarks(id string) error {
	return newUserError(fmt.Sprintf("User <@%v> doesn't have any bookmarks.", id))
}

func ErrUnknownUserSetting(setting string) error {
	return newUserError(
		fmt.Sprintf("Unknown setting: `%v`. Please use `bt!profile` to see existing settings.", setting),
	)
}

func ErrUserUnbookmarkFail(query interface{}, err error) error {
	return newUserError(
		fmt.Sprintf("Failed to remove a bookmark `[%v]`. Unexpected error occured: %v.", query, err),
		err,
	)
}
