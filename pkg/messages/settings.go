package messages

import "fmt"

type SetCommand struct {
	CurrentSettings string
	General         *General
	Features        *Features
	PixivSettings   *PixivSettings
	TwitterSettings *TwitterSettings
	ArtChannels     string
}

type General struct {
	Title  string
	Prefix string
	NSFW   string
}

type PixivSettings struct {
	Title   string
	Enabled string
	Limit   string
}

type TwitterSettings struct {
	Title   string
	Enabled string
	Prompt  string
}

type Features struct {
	Title     string
	Repost    string
	Crosspost string
	Reactions string
}

func Set() *SetCommand {
	return &SetCommand{
		CurrentSettings: "Current settings",
		ArtChannels:     "Art channels",
		General: &General{
			Title:  "General",
			Prefix: "Prefix",
			NSFW:   "NSFW",
		},
		TwitterSettings: &TwitterSettings{
			Title:   "Twitter settings",
			Enabled: "Status (twitter)",
			Prompt:  "Prompt",
		},
		PixivSettings: &PixivSettings{
			Title:   "Pixiv settings",
			Enabled: "Status (pixiv)",
			Limit:   "Limit",
		},
		Features: &Features{
			Title:     "Features",
			Repost:    "Repost",
			Crosspost: "Crosspost",
			Reactions: "Reactions",
		},
	}
}

func ErrPrefixTooLong(prefix string) error {
	return newUserError(fmt.Sprintf("Prefix `%v` is too long. Maximum length is 5 characters", prefix))
}

func ErrUnknownSetting(setting string) error {
	return newUserError(fmt.Sprintf("Setting `%v` doesn't exist. Please use `bt!set` to view existing settings", setting))
}

func ErrParseBool(value string) error {
	return newUserError(fmt.Sprintf("Failed to parse %v to boolean", value))
}

func ErrParseInt(value string) error {
	return newUserError(fmt.Sprintf("Failed to parse %v to integer", value))
}

func ErrUnknownRepostOption(option string) error {
	return newUserError(fmt.Sprintf("Repost option `%v` doesn't exist. Available options are `[enabled, disabled, strict]`", option))
}

func ErrForeignChannel(id string) error {
	return newUserError(
		fmt.Sprintf("Cannot get channel <#%v>. The channel is from foreign server.", id),
	)
}

func ErrAlreadyArtChannel(id string) error {
	return newUserError(
		fmt.Sprintf("Cannot add channel <#%v> to art channels. The channel is already an art channel.", id),
	)
}

func ErrWrongChannelType(id string) error {
	return newUserError(
		fmt.Sprintf("Cannot add channel <#%v> to art channels. The channel is not a text channel or a category.", id),
	)
}

func AddArtChannelSuccess(channels []string) string {
	return fmt.Sprintf(
		"Successfully added following channels to server's art channels:\n%v",
		ListChannels(channels),
	)
}

func RemoveArtChannelSuccess(channels []string) string {
	return fmt.Sprintf(
		"Successfully removed following channels from server's art channels:\n%v",
		ListChannels(channels),
	)
}

func RemoveArtChannelFail(channels []string) error {
	return newUserError(
		fmt.Sprintf(
			"Failed to remove following channels from art channels:\n%v\n%v",
			ListChannels(channels),
			"All channels passed to the command should be art channels.",
		),
	)
}
