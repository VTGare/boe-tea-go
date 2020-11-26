package commands

import (
	"fmt"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
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
		Aliases:     []string{"top"},
		Description: "Leaderboard of artworks",
		GuildOnly:   false,
		NSFW:        false,
		Exec:        leaderboard,
		Help:        gumi.NewHelpSettings(),
		Cooldown:    15 * time.Second,
	})
	lb.Help.AddField("Usage", "bt!leaderboard", false).AddField("Result", "Returns an embed with top 10 most favourited artworks of all time", false)

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
	artworks, err := database.DB.FindManyArtworks(nil, database.ByFavourites)
	if err != nil {
		return err
	}

	if utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
		Content: "Warning! The result may contain NSFW content, Boe Tea doesn't filter the leaderboard to stay true to its purpose! Please confirm the operation",
	}) {
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
			s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("❎ Couldn't send an artwork"),
				Description: fmt.Sprintf("An artwork with %v ID doesn't exist", ID),
				Timestamp:   utils.EmbedTimestamp(),
				Color:       utils.EmbedColor,
			})

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
		prompt := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
			Embed: &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("🛑 Attention"),
				Description: fmt.Sprintf("%v out of %v (%v%v) marked this artwork as NSFW. Please confirm the operation.", artwork.NSFW, artwork.Favourites, percent, "%"),
				Timestamp:   utils.EmbedTimestamp(),
				Color:       utils.EmbedColor,
			},
		})

		if !prompt {
			return nil
		}
	}

	embed := artworkEmbed(artwork, 1, 1)
	s.ChannelMessageSendEmbed(m.ChannelID, embed)
	return nil
}
