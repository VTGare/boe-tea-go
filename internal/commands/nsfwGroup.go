package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/internal/widget"
	"github.com/VTGare/boe-tea-go/pkg/nozoki"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

var (
	nh = nozoki.NewNozoki()
)

func init() {
	groupName := "nsfw"
	Commands = append(Commands, &gumi.Command{
		Name:        "nhentai",
		Aliases:     []string{"nh"},
		Description: "Sends a description of a nhentai doujinshi.",
		Group:       groupName,
		Usage:       "bt!nhentai <six digits>",
		Example:     "bt!nhentai 177013",
		Exec:        nhentai,
		NSFW:        true,
	})
}

func nhentai(ctx *gumi.Ctx) error {
	var (
		m    = ctx.Event
		s    = ctx.Session
		args = strings.Fields(ctx.Args.Raw)
		eb   = embeds.NewBuilder()
	)

	if g, ok := database.GuildCache.Get(m.GuildID); ok {
		if !g.(*database.GuildSettings).NSFW {
			eb.FailureTemplate("You're trying to execute an NSFW command. The server prohibits NSFW content.")
			s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
			return nil
		}
	}

	if len(args) == 0 {
		eb.FailureTemplate("``bt!nhentai`` requires an nhentai ID argument.")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	if _, err := strconv.Atoi(args[0]); err != nil {
		eb.FailureTemplate(fmt.Sprintf("Failed to parse [%v]. Please provide a valid number", args[0]))
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	book, err := nh.GetBook(args[0])
	if err != nil {
		return err
	}

	artists := ""
	tags := ""
	if str := strings.Join(book.Artists, ", "); str != "" {
		artists = str
	} else {
		artists = "-"
	}

	if str := strings.Join(book.Tags, ", "); str != "" {
		tags = str
	} else {
		tags = "-"
	}

	eb.Title(book.Titles.Pretty).URL(book.URL).Thumbnail(book.Cover)
	eb.AddField("Artists", artists).AddField("Tags", tags).AddField("Favourites", strconv.Itoa(book.Favourites)).AddField("Pages", strconv.Itoa(book.PageCount))

	doujin := make([]*discordgo.MessageEmbed, 0)

	//Title embed
	doujin = append(doujin, eb.Finalize())
	for ind, page := range book.Pages {
		eb := embeds.NewBuilder()

		eb.Footer(fmt.Sprintf("Page %v/%v", ind+1, book.PageCount), "")
		eb.Author(book.Titles.Pretty, page, "")
		eb.Image(page)

		doujin = append(doujin, eb.Finalize())
	}

	wg := widget.NewWidget(ctx.Session, m.Author.ID, doujin)
	func() {
		err := wg.Start(ctx.Event.ChannelID)
		if err != nil {
			logrus.Warnln("Widget error:", err)
		}
	}()

	return nil
}
