package messages

import (
	"fmt"
	"net/url"
)

func SauceNotFound(uri string) error {
	return newUserError(
		fmt.Sprintf(
			"Sorry, Boe Tea couldn't find source of the image, press one of the links below to use other methods:\n%v\n%v",
			NamedLink(
				"• ascii2d [recommended, will search directly for your image]",
				"https://ascii2d.net/search/url/"+url.QueryEscape(uri),
			),
			NamedLink("• Google Image Search", "https://www.google.com/imghp"),
		),
	)
}

func SauceNoImage() error {
	return newUserError(
		fmt.Sprintf(
			"Boe Tea couldn't find an image URL to search sauce for. Image URL can be found in:\n%v\n%v\n%v\n%v",
			"• Argument to the command, a direct image or Discord message URL;",
			"• Last 5 messages in the channel, including embeds;",
			"• Message attachment;",
			"• Message reply.",
		),
	)
}

func SauceRateLimit() error {
	return newUserError(
		fmt.Sprintf(
			"Boe Tea's been rate limited by SauceNAO.\n%v\n%v",
			"Please ask a server mod to set SauceNAO API key using `bt!set saucenao <api_key>` command.",
			"You can get the API key [here](https://saucenao.com/user.php?page=search-api). Don't be concerned, the key will be securely encrypted.",
		),
	)
}

func SauceInvalidKey() error {
	return newUserError(
		fmt.Sprintf(
			"Server's custom API key is invalid.\n%v\n%v",
			"Please re-set the key to a valid one using `bt!set saucenao <api_key>` command.",
			"You can get the API key [here](https://saucenao.com/user.php?page=search-api).",
		),
	)
}

func SauceError(err error) error {
	return newUserError(
		fmt.Sprintf(
			"SauceNAO has returned an error. Please report it to bot's author using a `bt!feedback` command.\n```\n%v\n```",
			err,
		),
		err,
	)
}

func DoujinNotFound(id string) error {
	return newUserError(
		fmt.Sprintf("Couldn't find a doujin with `%v` ID. It doesn't exist or the server is not responding.", id),
	)
}
