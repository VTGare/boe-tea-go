package messages

import (
	"fmt"
	"net/url"
)

func SauceNotFound(uri string) error {
	ascii2d := NamedLink(
		"• ascii2d",
		"https://ascii2d.net/search/url/"+url.QueryEscape(uri),
	)

	google := NamedLink("• Google Image Search", "https://www.google.com/imghp")

	return newUserError(
		fmt.Sprintf(
			"Sorry, Boe Tea couldn't find the source, follow one of the links below to use other methods:\n%v\n%v",
			ascii2d,
			google,
		),
	)
}

func SauceNoImage() error {
	return newUserError(
		fmt.Sprintf(
			"Boe Tea couldn't find an image URL to search sauce for. Make sure there's an image:\n%v\n%v\n%v\n%v",
			"• In command argument, either direct image URL or Discord message URL",
			"• In last 5 messages in the channel, including embeds",
			"• In a message you're replying to",
			"• Attached to your message",
		),
	)
}

func SauceRateLimit() error {
	return newUserError("SauceNAO ratelimited Boe Tea. Please try again later.")
}

func SauceError(err error) error {
	msg := fmt.Sprintf("SauceNAO returned an error. Please report it to the developer using with `bt!feedback command`.\n```\n%v\n```", err)
	return newUserError(msg, err)
}

func DoujinNotFound(id string) error {
	return newUserError(fmt.Sprintf("Couldn't find a doujin with the following ID: `%v`.", id))
}

func CloudflareError() error {
	return newUserError("Failed to bypass Cloudflare protection.")
}
