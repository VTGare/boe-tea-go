package messages

import "fmt"

type EmbedType int

const (
	artworkSearchWarning EmbedType = iota
	repost
	about
	sauce
	bookmarkAdded
	bookmarkRemoved
)

type Language int

const (
	English Language = iota
	Japanese
)

type BaseEmbed struct {
	Title       string
	Description string
}

type CommandHelp struct {
	Usage   string
	Example string
}

type Repost struct {
	Title           string
	OriginalMessage string
	Expires         string
}

type About struct {
	Title         string
	Description   string
	SupportServer string
	InviteLink    string
	Patreon       string
}

type Sauce struct {
	Author      string
	Similarity  string
	ExternalURL string
	OtherURLs   string
	NoTitle     string
}

type UserProfile struct {
	Title     string
	Settings  string
	DM        string
	Crosspost string
	Stats     string
	Groups    string
	Bookmarks string
}

type UserGroups struct {
	Title       string
	Description string
	Group       string
	Pair        string
	Parent      string
	Children    string
}

var embeds = map[Language]map[EmbedType]any{
	English: {
		artworkSearchWarning: &BaseEmbed{
			Title:       "⚠ Warning!",
			Description: "Boe Tea's artworks database __may contain not safe for work results__, **there's no good way to filter them.** Use controls below to skip this warning.",
		},

		repost: &Repost{
			Title:           "Repost detected",
			OriginalMessage: "Jump to original message.",
			Expires:         "Expires",
		},

		about: &About{
			Title: "ℹ About",
			Description: fmt.Sprintf(
				"Boe Tea is an artwork bot for all your artwork related needs. %v\n***%v:***\n%v\nYou guys are epic!",
				"If you want to copy the invite link, simply right-click it and press Copy Link.",
				"Many thanks to my early patrons",
				"• Nom\n• Danyo\n• tuba\n• Jeffrey\n• ... and other anonymous supporters!",
			),
			SupportServer: "Support server",
			InviteLink:    "Invite link",
			Patreon:       "Patreon",
		},

		bookmarkAdded: &BaseEmbed{
			Title:       "💖 Successfully bookmarked an artwork",
			Description: "If you dislike direct messages, disable them by running `bt!userset dm off` command",
		},

		bookmarkRemoved: &BaseEmbed{
			Title:       "💔 Successfully removed a bookmark",
			Description: "If you dislike direct messages, disable them by running `bt!userset dm off` command",
		},
	},
}

func embedByType[T any](lang Language, typ EmbedType) *T {
	byLang, ok := embeds[lang]
	if !ok {
		byLang = embeds[English]
	}

	v, ok := byLang[typ]
	if !ok {
		return new(T)
	}

	typed, ok := v.(*T)
	if !ok || typed == nil {
		return new(T)
	}

	return typed
}

func SearchWarningEmbed() *BaseEmbed {
	return embedByType[BaseEmbed](English, artworkSearchWarning)
}

func AboutEmbed() *About {
	return embedByType[About](English, about)
}

func RepostEmbed() *Repost {
	return embedByType[Repost](English, repost)
}

func BookmarkAddedEmbed() *BaseEmbed {
	return embedByType[BaseEmbed](English, bookmarkAdded)
}

func BookmarkRemovedEmbed() *BaseEmbed {
	return embedByType[BaseEmbed](English, bookmarkRemoved)
}
