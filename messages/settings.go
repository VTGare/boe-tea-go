package messages

import "fmt"

func ErrUnknownSetting(setting string) error {
	return newUserError(fmt.Sprintf("Unknown setting: `%v`. Please use `bt!set` to view existing settings", setting))
}

func ErrParseBool(value string) error {
	return newUserError(fmt.Sprintf("Failed to parse %v to boolean", value))
}

func ErrParseInt(value string) error {
	return newUserError(fmt.Sprintf("Failed to parse %v to integer", value))
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
