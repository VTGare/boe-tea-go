package commands

import (
	"fmt"
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
		AuthorOnly:  true,
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
		AuthorOnly:  true,
		Description: "Download images",
		Usage:       "bt!download <channel ID>",
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

func showDevSettings(s *discordgo.Session, m *discordgo.MessageCreate) {
	eb := embeds.NewBuilder()
	eb.Title("Dev settings").Thumbnail(s.State.User.AvatarURL(""))
	eb.AddField("Pog", fmt.Sprintf("**Pixiv:** %v", database.DevSet.PixivReverseProxy))

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
}
