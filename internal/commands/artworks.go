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
	groupName := "artworks"

	Commands = append(Commands, &gumi.Command{
		Name:        "leaderboard",
		Group:       groupName,
		Aliases:     []string{"top", "lb"},
		Description: "Shows a leaderboard of all saved artworks.",
		Usage:       "bt!leaderboard [flags]",
		Flags: map[string]string{
			"Flag syntax": "Flags have following syntax: `name:value`. Example: `limit:10`",
			"limit":       "Number of artworks returned. *Default:* 10. Value should be an _integer number from 1 to 100_",
			"last":        "Filter artworks by date. *Default:* no filter. Value should be one of the following strings: `[day, week, month]`",
		},
		Example:     "bt!leaderboard limit:10 last:week",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        leaderboard,
	})

	Commands = append(Commands, &gumi.Command{
		Name:        "artwork",
		Group:       groupName,
		Description: "Gets an artwork by ID.",
		Usage:       "bt!artwork <id>",
		Flags: map[string]string{
			"ID": "An artwork ID, can be retrieved from your favourites list or anything in range [1:2^32)",
		},
		Example:     "bt!artwork 69",
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        artwork,
	})
}

func leaderboard(ctx *gumi.Ctx) error {
	var (
		options = database.NewFindManyOptions().Limit(10).SortType(database.ByFavourites).Order(database.Descending)
		args    = strings.Fields(ctx.Args.Raw)
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
					ctx.ReplyEmbed(eb.FailureTemplate(msg).Finalize())
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
	prompt := utils.CreatePromptWithMessage(ctx.Session, ctx.Event, &discordgo.MessageSend{
		Embed: eb.WarnTemplate(msg).Finalize(),
	})

	if prompt {
		embeds := make([]*discordgo.MessageEmbed, 0, len(artworks))
		for ind, a := range artworks {
			embeds = append(embeds, artworkEmbed(a, ind, len(artworks)))
		}
		if len(embeds) > 1 {
			w := widget.NewWidget(ctx.Session, ctx.Event.Author.ID, embeds)
			err := w.Start(ctx.Event.ChannelID)
			if err != nil {
				return err
			}
		} else if len(embeds) == 1 {
			err := ctx.ReplyEmbed(embeds[0])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func artwork(ctx *gumi.Ctx) error {
	if ctx.Args.Len() == 0 {
		return utils.ErrNotEnoughArguments
	}

	ID, err := ctx.Args.Get(0).AsInt()
	if err != nil {
		return err
	}

	artwork, err := database.DB.FindArtworkByID(ID)
	if err != nil {
		switch err {
		case mongo.ErrNoDocuments:
			eb := embeds.NewBuilder()
			ctx.ReplyEmbed(eb.FailureTemplate(fmt.Sprintf("An artwork with ID [%v] doesn't exist", ID)).Finalize())
			return nil
		default:
			return err
		}
	}

	percent := (float64(artwork.NSFW) / float64(artwork.Favourites)) * 100.0
	ch, err := ctx.Session.Channel(ctx.Event.ChannelID)
	if err != nil {
		logrus.Warnf("artwork() -> s.Channel(): %v", err)
		ch = &discordgo.Channel{NSFW: false}
	}

	if percent >= 50.0 && !ch.NSFW {
		eb := embeds.NewBuilder()

		msg := fmt.Sprintf("%v out of %v (%v%v) marked this artwork as NSFW. Please confirm the operation.", artwork.NSFW, artwork.Favourites, percent, "%")
		prompt := utils.CreatePromptWithMessage(ctx.Session, ctx.Event, &discordgo.MessageSend{
			Embed: eb.WarnTemplate(msg).Finalize(),
		})
		if !prompt {
			return nil
		}
	}

	embed := artworkEmbed(artwork, 1, 1)
	ctx.ReplyEmbed(embed)
	return nil
}
