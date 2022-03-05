package messages

import "fmt"

type EmbedType int

const (
	artworkSearchWarning EmbedType = iota
	repost
	about
	sauce
	set
	favAdded
	favRemoved
)

type Language int

const (
	English Language = iota
	Russian
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

type SetCommand struct {
	CurrentSettings    string
	General            *General
	Features           *Features
	PixivSettings      *PixivSettings
	TwitterSettings    *ProviderSettings
	DeviantSettings    *ProviderSettings
	ArtstationSettings *ProviderSettings
	ArtChannels        string
}

type General struct {
	Title  string
	Prefix string
	NSFW   string
}

type ProviderSettings struct {
	Title   string
	Enabled string
}

type PixivSettings struct {
	ProviderSettings
	Limit string
}

type Features struct {
	Title            string
	Repost           string
	RepostExpiration string
	Crosspost        string
	Reactions        string
}

type UserProfile struct {
	Title      string
	Settings   string
	DM         string
	Crosspost  string
	Stats      string
	Groups     string
	Favourites string
}

type UserGroups struct {
	Title       string
	Description string
	Group       string
	Parent      string
	Children    string
}

var embeds = map[Language]map[EmbedType]interface{}{
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
				"Boe Tea is an ultimate artwork bot for all your artwork related needs. %v\n***%v:***\n%v\nYou guys are epic!",
				"If you want to copy the invite link, simply right-click it and press Copy Link.",
				"Many thanks to our patrons",
				"• Nom\n• Danyo\n• tuba\n• Jeffrey\n• ... and other anonymous supporters!",
			),
			SupportServer: "Support server",
			InviteLink:    "Invite link",
			Patreon:       "Patreon",
		},

		sauce: &Sauce{
			Author:      "Author",
			Similarity:  "Similarity",
			OtherURLs:   "Other URLs",
			ExternalURL: "External URL",
			NoTitle:     "No title",
		},

		set: &SetCommand{
			CurrentSettings: "Current settings",
			ArtChannels:     "Art channels",
			General: &General{
				Title:  "General",
				Prefix: "Prefix",
				NSFW:   "NSFW",
			},
			TwitterSettings: &ProviderSettings{
				Title:   "Twitter settings",
				Enabled: "Status __(twitter)__",
			},
			DeviantSettings: &ProviderSettings{
				Title:   "DeviantArt settings",
				Enabled: "Status __(deviant)__",
			},
			ArtstationSettings: &ProviderSettings{
				Title:   "ArtStation settings",
				Enabled: "Status __(artstation)__",
			},
			PixivSettings: &PixivSettings{
				ProviderSettings: ProviderSettings{
					Title:   "Pixiv settings",
					Enabled: "Status __(pixiv)__",
				},
				Limit: "Limit",
			},
			Features: &Features{
				Title:            "Features",
				Repost:           "Repost",
				RepostExpiration: "Expiration __(repost.expiration)__",
				Crosspost:        "Crosspost",
				Reactions:        "Reactions",
			},
		},

		favAdded: &BaseEmbed{
			Title:       "💖 Successfully added an artwork to favourites",
			Description: "If you dislike direct messages disable them by running `bt!userset dm off` command",
		},

		favRemoved: &BaseEmbed{
			Title:       "💔 Successfully removed an artwork from favourites",
			Description: "If you dislike direct messages disable them by running `bt!userset dm off` command",
		},
	},
}

func SearchWarningEmbed() *BaseEmbed {
	return embeds[English][artworkSearchWarning].(*BaseEmbed)
}

func AboutEmbed() *About {
	return embeds[English][about].(*About)
}

func RepostEmbed() *Repost {
	return embeds[English][repost].(*Repost)
}

func SauceEmbed() *Sauce {
	return embeds[English][sauce].(*Sauce)
}

func SetEmbed() *SetCommand {
	return embeds[English][set].(*SetCommand)
}

func FavouriteAddedEmbed() *BaseEmbed {
	return embeds[English][favAdded].(*BaseEmbed)
}

func FavouriteRemovedEmbed() *BaseEmbed {
	return embeds[English][favRemoved].(*BaseEmbed)
}
