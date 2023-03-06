package messages

import "fmt"

func ErrPrefixTooLong(prefix string) error {
	return newUserError(fmt.Sprintf("Prefix `%v` is too long. Maximum length is 5 characters", prefix))
}

func ErrUnknownSetting(setting string) error {
	return newUserError(fmt.Sprintf("Unknown setting: `%v`. Please use `bt!set` to view existing settings", setting))
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

func ErrExpirationOutOfRange(value string) error {
	msg := fmt.Sprintf("Expiration duration `%v` is out of range. Minimum is `1m`, and maximum is `168h`.", value)
	return newUserError(msg)
}

func ErrUnknownRepostOption(option string) error {
	return newUserError(fmt.Sprintf("Unknown option: `%v`. Use one of the following options: `[enabled, disabled, strict]`", option))
}

func ErrForeignChannel(id string) error {
	return newUserError(
		fmt.Sprintf("Couldn't get <#%v>. Please use channels from this server.", id),
	)
}

func ErrAlreadyArtChannel(id string) error {
	return newUserError(
		fmt.Sprintf("Couldn't add <#%v> to art channels. It's already an art channel.", id),
	)
}

func ErrNotArtChannel(id string) error {
	return newUserError(
		fmt.Sprintf("Couldn't remove <#%v> from art channels. It's not an art channel.", id),
	)
}

func ErrWrongChannelType(id string) error {
	return newUserError(
		fmt.Sprintf("Couldn't add <#%v> to art channels. Unsupported channel type.", id),
	)
}

func AddArtChannelSuccess(channels []string) string {
	return fmt.Sprintf(
		"Successfully added the following to art channels:\n%v",
		ListChannels(channels),
	)
}

func RemoveArtChannelSuccess(channels []string) string {
	return fmt.Sprintf(
		"Successfully removed the following from art channels:\n%v",
		ListChannels(channels),
	)
}

func RemoveArtChannelFail(channels []string) error {
	return newUserError(
		fmt.Sprintf(
			"Failed to remove the following channels from art channels:\n%v",
			ListChannels(channels),
		),
	)
}
