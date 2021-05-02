package messages

import (
	"fmt"
	"net/url"
)

type SauceEmbed struct {
	Author      string
	Similarity  string
	ExternalURL string
	OtherURLs   string
	NoTitle     string
}

func Sauce() *SauceEmbed {
	return &SauceEmbed{
		Author:      "Author",
		Similarity:  "Similarity",
		OtherURLs:   "Other URLs",
		ExternalURL: "External URL",
		NoTitle:     "No title",
	}
}

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

func SauceError(err error) error {
	return newUserError(
		fmt.Sprintf(
			"SauceNAO has returned an error. Please report it to bot's author using a `bt!feedback` command.\n```\n%v\n```",
			err,
		),
		err,
	)
}
