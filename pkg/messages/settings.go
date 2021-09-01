package messages

import "fmt"

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

func ErrParseDuration(value string) error {
	return newUserError(
		fmt.Sprintf("Failed to parse `%v` to duration. Valid time units are \"ns\", \"ms\", \"s\", \"m\", \"h\".\n%v",
			value,
			"**Example:**\n 1h30m",
		),
	)
}

func ErrExpirationTooShort(value string) error {
	return newUserError(
		fmt.Sprintf("Duration `%v` is too short. Minimum one minute `1m` is required.", value),
	)
}

func ErrUnknownRepostOption(option string) error {
	return newUserError(fmt.Sprintf("Repost option `%v` doesn't exist. Available options are `[enabled, disabled, strict]`", option))
}

func ErrForeignChannel(id string) error {
	return newUserError(
		fmt.Sprintf("Cannot get channel <#%v>. It's from a foreign server.", id),
	)
}

func ErrAlreadyArtChannel(id string) error {
	return newUserError(
		fmt.Sprintf("Cannot add channel <#%v> to art channels. It's already an art channel.", id),
	)
}

func ErrNotArtChannel(id string) error {
	return newUserError(
		fmt.Sprintf("Cannot remove channel <#%v> from art channels. It's not an art channel.", id),
	)
}

func ErrWrongChannelType(id string) error {
	return newUserError(
		fmt.Sprintf("Cannot add channel <#%v> to art channels. It's not a text channel or a category.", id),
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
