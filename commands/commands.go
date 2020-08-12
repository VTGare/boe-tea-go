package commands

import (
	"fmt"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
)

var (
	CommandFramework *gumi.Gumi
)

func init() {
	CommandFramework = gumi.NewGumi(gumi.WithErrorHandler(func(e error) *discordgo.MessageSend {
		if e != nil {
			embed := &discordgo.MessageEmbed{
				Title: "Oops, something went wrong!",
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: "https://i.imgur.com/OZ1Al5h.png",
				},
				Description: fmt.Sprintf("***Error message:***\n%v\n\nPlease contact bot's author using bt!feedback command or directly at VTGare#3370 if you can't understand the error. ", e),
				Color:       utils.EmbedColor,
				Timestamp:   utils.EmbedTimestamp(),
			}

			return &discordgo.MessageSend{
				Embed: embed,
			}
		}
		return nil
	}), gumi.WithPrefixResolver(func(g *gumi.Gumi, s *discordgo.Session, m *discordgo.MessageCreate) []string {
		if guild, ok := database.GuildCache[m.GuildID]; ok {
			if guild.Prefix == "bt!" {
				return []string{"bt!", "bt ", "bt.", "<@!" + s.State.User.ID + ">"}
			}
			return []string{guild.Prefix, "<@!" + s.State.User.ID + ">"}
		}
		return []string{"bt!", "bt ", "bt.", "<@!" + s.State.User.ID + ">"}
	}))

	generalGroup := CommandFramework.Groups["general"]
	generalGroup.AddCommand("ping", ping, gumi.CommandDescription("Checks if Boe Tea is online and sends response back"))

	feedbackHelp := gumi.NewHelpSettings()
	feedbackHelp.AddField("Usage", "``bt!feedback [feedback message]``. Please use this command to report bugs or suggest new features only. If you misuse this command you'll get blacklisted!", false)
	feedbackHelp.AddField("feedback message", "While suggestions can be plain text, bug reports are expected to be formatted in a specific way. Template shown below:\n```**Summary:** -\n**Reproduction:** -\n**Expected result:** -\n**Actual result:** -```\nYou can provide images as links or a single image as an attachment to the feedback message!", false)

	generalGroup.AddCommand("feedback", feedback, gumi.CommandDescription("Sends a feedback message to bot's author. Use ``bt!help general feedback`` to see bugreport template"), gumi.WithHelp(feedbackHelp))
	generalGroup.AddCommand("invite", invite, gumi.CommandDescription("Sends Boe Tea's invite link!"))

	setHelp := gumi.NewHelpSettings()
	setHelp.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!set ``<setting>`` ``<new setting>``",
		},
		{
			Name:  "prefix",
			Value: "Changes bot's prefix. Maximum ***5 characters***. If last character is a letter whitespace is assumed (takes one character).",
		},
		{
			Name:  "largeset",
			Value: "Amount of pictures considered a large set, which invokes a prompt. Must be an ***integer***. Set to 0 to ask every time",
		},
		{
			Name:  "limit",
			Value: "Image set size hard limit. If you attempt to repost a post or bulk post more than the limit it'll fail",
		},
		{
			Name:  "pixiv",
			Value: "Pixiv reposting switch, accepts ***f or false (case-insensitive)*** to disable and ***t or true*** to enable.",
		},
		{
			Name:  "repost",
			Value: "Repost check setting, accepts ***enabled***, ***disabled***, and ***strict*** settings. Strict mode disables a prompt and removes Twitter reposts (if bot has Manage Messages permission)",
		},
		{
			Name:  "reversesearch",
			Value: "Default reverse image search engine. Only ***SauceNAO*** or ***WAIT*** are available as of now.",
		},
		{
			Name:  "promptemoji",
			Value: "Confirmation prompt emoji. Only unicode or local server emoji's are allowed.",
		},
	}
	generalGroup.AddCommand("set", set, gumi.CommandDescription("Show or change server's settings."), gumi.WithHelp(setHelp), gumi.GuildOnly(), gumi.WithAliases("config", "cfg", "settings"))
	generalGroup.AddCommand("support", support, gumi.CommandDescription("Sends a support server invite link"))
}
