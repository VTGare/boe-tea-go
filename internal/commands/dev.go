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
		"pixiv":  setInt,
		"nitter": setString,
	}
)

func init() {
	dg := Router.AddGroup(&gumi.Group{
		Name: "dev",
	})
	dg.IsVisible = false
	dg.AddCommand(&gumi.Command{
		Name: "test",
		Exec: test,
	})
	dg.AddCommand(&gumi.Command{
		Name: "message",
		Exec: message,
	})
	dg.AddCommand(&gumi.Command{
		Name: "stats",
		Exec: devstats,
	})

	dg.AddCommand(&gumi.Command{
		Name: "devset",
		Exec: devset,
	})
}

func message(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

	if len(args) == 0 {
		return nil
	}

	for _, g := range s.State.Guilds {
		for _, ch := range g.Channels {
			if (strings.Contains(ch.Name, "general") || strings.Contains(ch.Name, "art") || strings.Contains(ch.Name, "sfw") || strings.Contains(ch.Name, "discussion")) && ch.Type == discordgo.ChannelTypeGuildText {
				s.ChannelMessageSend(ch.ID, strings.Join(args, " "))
				break
			}
		}
	}

	return nil
}

func test(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

	if len(args) != 2 {
		return nil
	}

	msg, err := s.ChannelMessage(args[0], args[1])
	if err != nil {
		return err
	}

	println(msg.Content)

	return nil
}

func devstats(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	var (
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

func devset(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

	if length := len(args); length == 0 {
		showDevSettings(s, m)
	} else if length >= 2 {
		setting := args[0]
		newSetting := strings.ToLower(args[1])

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
	eb.AddField("Pog", fmt.Sprintf("**Pixiv:** %v | **Nitter:** %v", database.DevSet.PixivReverseProxy, database.DevSet.NitterInstance))

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
}
