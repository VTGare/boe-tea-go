package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/internal/widget"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	ag := Router.AddGroup(&gumi.Group{
		Name:        "artworks",
		Description: "Boe Tea's artworks collection and commands with it",
		NSFW:        false,
	})

	lb := ag.AddCommand(&gumi.Command{
		Name:        "leaderboard",
		Aliases:     []string{"top", "lb"},
		Description: "Leaderboard of artworks",
		GuildOnly:   false,
		NSFW:        false,
		Exec:        leaderboard,
		Help:        gumi.NewHelpSettings(),
		Cooldown:    15 * time.Second,
	})
	lb.Help.AddField("Usage", "bt!leaderboard [flags]", false)
	lb.Help.AddField("Flag syntax", "Flags have following syntax: `name:value`.\n_***Example:***_ `bt!leaderboard limit:100`.\nAccepted flags are listed below", false)
	lb.Help.AddField("limit", "Number of artworks returned.\n_***Default:***_ 10.\nValue should be an _integer number from 1 to 100_", false)
	lb.Help.AddField("last", "Filter artworks by date.\n_***Default:***_ no filter.\nValue should be one of the following strings:\n`[day, week, month]`.", false)

	aw := ag.AddCommand(&gumi.Command{
		Name:        "artwork",
		Description: "Gets an artwork by ID",
		GuildOnly:   false,
		NSFW:        false,
		Exec:        artwork,
		Help:        gumi.NewHelpSettings(),
		Cooldown:    5 * time.Second,
	})
	aw.Help.AddField("Usage", "bt!artwork <id>", false).AddField("ID", "An artwork ID, can be retrieved from your favourites list or anything in range [1:2^32)", false)
}

func leaderboard(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	var (
		options = database.NewFindManyOptions().Limit(10).SortType(database.ByFavourites).Order(database.Descending)
	)

	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "limit:"):
			limitString := strings.TrimPrefix(a, "limit:")
			limit, err := strconv.Atoi(limitString)
			if err != nil || limit > 100 || limit < 1 {
				if limit > 100 || limit < 1 {
					eb := embeds.NewBuilder()
					msg := "Provided limit argument is either not a number or out of allowed range [1:100]"
					s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
					return nil
				}
			}

			options.Limit(limit)
		case strings.HasPrefix(a, "last:"):
			last := strings.TrimPrefix(a, "last:")
			switch last {
			case "day":
				options.SetTime(time.Now().AddDate(0, 0, -1))
			case "week":
				options.SetTime(time.Now().AddDate(0, 0, -7))
			case "month":
				options.SetTime(time.Now().AddDate(0, -1, 0))
			}
		}
	}

	artworks, err := database.DB.FindManyArtworks(nil, options)
	if err != nil {
		return err
	}

	eb := embeds.NewBuilder()
	msg := "The result may contain NSFW content, Boe Tea doesn't filter the leaderboard to stay true to its purpose! Please confirm the operation."
	prompt := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
		Embed: eb.WarnTemplate(msg).Finalize(),
	})

	if prompt {
		embeds := make([]*discordgo.MessageEmbed, 0, len(artworks))
		for ind, a := range artworks {
			embeds = append(embeds, artworkEmbed(a, ind, len(artworks)))
		}
		if len(embeds) > 1 {
			w := widget.NewWidget(s, m.Author.ID, embeds)
			err := w.Start(m.ChannelID)
			if err != nil {
				return err
			}
		} else if len(embeds) == 1 {
			_, err := s.ChannelMessageSendEmbed(m.ChannelID, embeds[0])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func artwork(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	ID, err := strconv.Atoi(args[0])
	if err != nil {
		return err
	}

	artwork, err := database.DB.FindArtworkByID(ID)
	if err != nil {
		switch err {
		case mongo.ErrNoDocuments:
			eb := embeds.NewBuilder()
			s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(fmt.Sprintf("An artwork with ID [%v] doesn't exist", ID)).Finalize())
			return nil
		default:
			return err
		}
	}

	percent := (float64(artwork.NSFW) / float64(artwork.Favourites)) * 100.0
	ch, err := s.Channel(m.ChannelID)
	if err != nil {
		logrus.Warnf("artwork() -> s.Channel(): %v", err)
		ch = &discordgo.Channel{NSFW: false}
	}

	if percent >= 50.0 && !ch.NSFW {
		eb := embeds.NewBuilder()

		msg := fmt.Sprintf("%v out of %v (%v%v) marked this artwork as NSFW. Please confirm the operation.", artwork.NSFW, artwork.Favourites, percent, "%")
		prompt := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
			Embed: eb.WarnTemplate(msg).Finalize(),
		})
		if !prompt {
			return nil
		}
	}

	embed := artworkEmbed(artwork, 1, 1)
	s.ChannelMessageSendEmbed(m.ChannelID, embed)
	return nil
}
