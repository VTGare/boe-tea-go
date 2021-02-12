package commands

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
)

var (
	devSettingMap = map[string]settingFunc{
		"pixiv": setInt,
	}
)

func init() {
	groupName := "dev"
	Commands = append(Commands, &gumi.Command{
		Name:        "devset",
		Group:       groupName,
		Description: "Developer settings",
		Usage:       "bt!devset <setting name> <new setting>",
		Example:     "bt!devset notYour business",
		Exec:        devset,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "stats",
		Group:       groupName,
		Description: "Boe Tea's stats.",
		Usage:       "bt!stats",
		Example:     "",
		Exec:        devstats,
	})

	Commands = append(Commands, &gumi.Command{
		Name:        "download",
		Group:       groupName,
		Description: "Download images",
		Usage:       "bt!download <channel ID>",
		Exec:        download,
	})
}

func devstats(ctx *gumi.Ctx) error {
	var (
		s   = ctx.Session
		m   = ctx.Event
		mem runtime.MemStats
	)
	runtime.ReadMemStats(&mem)

	guilds := len(s.State.Guilds)
	channels := 0
	for _, g := range s.State.Guilds {
		channels += len(g.Channels)
	}
	latency := s.HeartbeatLatency().Round(1 * time.Millisecond)

	eb := embeds.NewBuilder()
	eb.Title("Bot stats").Thumbnail(utils.DefaultEmbedImage)
	eb.AddField("Guilds", strconv.Itoa(guilds), true).AddField("Channels", strconv.Itoa(channels), true)
	eb.AddField("Latency", latency.String(), true).AddField("Shards", strconv.Itoa(s.ShardCount))
	eb.AddField("RAM used", fmt.Sprintf("%v MB", mem.Alloc/1024/1024))

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	return nil
}

func devset(ctx *gumi.Ctx) error {
	var (
		s = ctx.Session
		m = ctx.Event
	)

	if m.Author.ID != utils.AuthorID {
		return nil
	}

	if ctx.Args.Len() == 0 {
		showDevSettings(s, m)
	} else if ctx.Args.Len() >= 2 {
		setting := ctx.Args.Get(0).Raw
		newSetting := ctx.Args.Get(1).Raw

		if setting == "nitter" {
			newSetting = strings.TrimSuffix(newSetting, "/")
		}

		if new, ok := devSettingMap[setting]; ok {
			n, err := new(s, m, newSetting)
			if err != nil {
				return err
			}

			err = database.DB.ChangeDevSetting(setting, n)
			if err != nil {
				return err
			}

			eb := embeds.NewBuilder()
			eb.SuccessTemplate("Successfully changed a setting!")
			eb.AddField("Setting", setting, true).AddField("New value", newSetting, true)
			s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		} else {
			return fmt.Errorf("invalid setting name: %v", setting)
		}
	}

	return nil
}

func download(ctx *gumi.Ctx) error {
	if ctx.Event.Author.ID != utils.AuthorID {
		return nil
	}

	var (
		eb = embeds.NewBuilder()
	)

	if ctx.Args.Len() == 0 {
		return ctx.ReplyEmbed(eb.FailureTemplate("Please provide a channel ID").Finalize())
	}

	channelID := ctx.Args.Get(0).Raw
	channelID = strings.Trim(channelID, "<#>")

	ch, err := ctx.Session.Channel(channelID)
	if err != nil {
		return err
	}

	finalMessages, err := ctx.Session.ChannelMessages(ch.ID, 100, "", "", "")
	if err != nil {
		return err
	}

	status := eb.InfoTemplate(fmt.Sprintf("Fetching messages.\nCurrent count: %v", len(finalMessages))).Finalize()
	statusMessage, err := ctx.Session.ChannelMessageSendEmbed(ctx.Event.ChannelID, status)
	if err != nil {
		return err
	}

	for {
		messages, err := ctx.Session.ChannelMessages(ch.ID, 100, finalMessages[len(finalMessages)-1].ID, "", "")
		if err != nil {
			return err
		}

		if len(messages) == 0 {
			status := eb.InfoTemplate(fmt.Sprintf("Finished fetching messages.\nMessage count: %v", len(finalMessages))).Finalize()
			ctx.Session.ChannelMessageEditEmbed(ctx.Event.ChannelID, statusMessage.ID, status)

			break
		}

		finalMessages = append(finalMessages, messages...)
		status := eb.InfoTemplate(fmt.Sprintf("Fetching messages.\nCurrent count: %v", len(finalMessages))).Finalize()
		ctx.Session.ChannelMessageEditEmbed(ctx.Event.ChannelID, statusMessage.ID, status)
	}

	status = eb.InfoTemplate("Filtering images.").Finalize()
	ctx.Session.ChannelMessageEditEmbed(ctx.Event.ChannelID, statusMessage.ID, status)

	URLs := make([]string, 0)
	for _, msg := range finalMessages {
		switch {
		case len(msg.Attachments) != 0:
			for _, att := range msg.Attachments {
				if strings.HasSuffix(att.Filename, ".png") || strings.HasSuffix(att.Filename, ".jpeg") || strings.HasSuffix(att.Filename, ".jpg") {
					URLs = append(URLs, att.URL)
				}
			}
		case len(msg.Embeds) != 0:
			for _, embed := range msg.Embeds {
				if embed.Image != nil {
					URLs = append(URLs, embed.Image.URL)
				}
			}
		}
	}

	status = eb.InfoTemplate(fmt.Sprintf("Finished filtering images. Image count: %v", len(URLs))).Finalize()
	ctx.Session.ChannelMessageEditEmbed(ctx.Event.ChannelID, statusMessage.ID, status)

	file, err := os.Create("urls.txt")
	defer func() {
		file.Close()
		os.Remove("urls.txt")
	}()

	if err != nil {
		return err
	}

	for _, url := range URLs {
		_, err := fmt.Fprintln(file, url)
		if err != nil {
			file.Close()
			return err
		}
	}

	file.Close()
	file, err = os.Open("urls.txt")
	if err != nil {
		return err
	}

	ctx.Session.ChannelMessageSendComplex(ctx.Event.ChannelID, &discordgo.MessageSend{
		Embed: eb.InfoTemplate(fmt.Sprintf("Successfully fetched %v images. Text file with URLs attached.", len(URLs))).Finalize(),
		File: &discordgo.File{
			Name:   "urls.txt",
			Reader: file,
		},
	})

	return nil
}

func showDevSettings(s *discordgo.Session, m *discordgo.MessageCreate) {
	eb := embeds.NewBuilder()
	eb.Title("Dev settings").Thumbnail(s.State.User.AvatarURL(""))
	eb.AddField("Pog", fmt.Sprintf("**Pixiv:** %v", database.DevSet.PixivReverseProxy))

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
}
