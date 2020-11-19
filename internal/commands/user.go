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
)

func init() {
	ug := Router.AddGroup(&gumi.Group{
		Name:        "user",
		Description: "User profile and settings",
		NSFW:        false,
		IsVisible:   true,
	})

	ug.AddCommand(&gumi.Command{
		Name:        "profile",
		Description: "Shows your profile",
		GuildOnly:   false,
		NSFW:        false,
		Exec:        profile,
		Help: &gumi.HelpSettings{
			IsVisible: true,
			ExtendedHelp: []*discordgo.MessageEmbedField{
				{Name: "Usage", Value: "bt!profile"},
			},
		},
		Cooldown: 5 * time.Second,
	})

	ug.AddCommand(&gumi.Command{
		Name:        "favourites",
		Aliases:     []string{"favorites", "favs"},
		Description: "Shows a list of your favourites.",
		GuildOnly:   false,
		NSFW:        false,
		Exec:        favourites,
		Help: &gumi.HelpSettings{
			IsVisible: true,
			ExtendedHelp: []*discordgo.MessageEmbedField{
				{Name: "Usage", Value: "bt!favourites <type>"},
				{Name: "type", Value: "__Not required.__ Accepts: [all, nsfw, sfw]. If not provided deducts the type from channel's NSFW status."},
			},
		},
		Cooldown: 5 * time.Second,
	})

	ug.AddCommand(&gumi.Command{
		Name:        "unfavourite",
		Aliases:     []string{"unfavourite", "unfav"},
		Description: "Unfavourites an art by favourite ID or URL",
		GuildOnly:   false,
		NSFW:        false,
		Exec:        unfavourite,
		Help: &gumi.HelpSettings{
			IsVisible: true,
			ExtendedHelp: []*discordgo.MessageEmbedField{
				{Name: "Usage", Value: "bt!unfavourite <art link or ID>"},
				{Name: "Art link", Value: "A Twitter or Pixiv post link. It should match the link in your favourites list. For ease of use I recommend using IDs instead."},
				{Name: "ID", Value: "Favourited art ID, retrieve one from favourites list."},
			},
		},
		Cooldown: 5 * time.Second,
	})
}

func profile(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		user = database.NewUserSettings(m.Author.ID)
		err := database.DB.InsertOneUser(user)
		if err != nil {
			return fmt.Errorf("Fatal database error: %v", err)
		}
	}

	artists := map[string]int{}
	for _, fav := range user.Favourites {
		artists[fav.Author]++
	}
	favourite := ""
	greatest := 0
	for key, val := range artists {
		if val > greatest {
			favourite = key
			greatest = val
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("%v's profile", m.Author.Username),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: m.Author.AvatarURL("")},
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Crosspost", Value: utils.FormatBool(user.Crosspost)},
			{Name: "Favourites", Value: strconv.Itoa(len(user.Favourites))},
		},
	}
	if favourite != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Favourite artist", Value: favourite})
	}
	s.ChannelMessageSendEmbed(m.ChannelID, embed)
	return nil
}

type mode int

const (
	modeAll  mode = 0
	modeSFW  mode = 1
	modeNSFW mode = 2
)

func favourites(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		user = database.NewUserSettings(m.Author.ID)
		err := database.DB.InsertOneUser(user)
		if err != nil {
			return fmt.Errorf("Fatal database error: %v", err)
		}
	}

	mode := modeSFW
	if len(args) > 0 {
		switch args[0] {
		case "all":
			mode = modeAll
		case "sfw":
			mode = modeSFW
		case "nsfw":
			mode = modeNSFW
		default:
			mode = modeSFW
		}
	}

	ch, _ := s.Channel(m.ChannelID)
	if !ch.NSFW && mode == modeNSFW || mode == modeAll {
		if f := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
			Content: "The result may contain NSFW images, are you sure about that?",
		}); f == false {
			return nil
		}
	}

	embeds := make([]*discordgo.MessageEmbed, 0)
	filtered := make([]*database.Favourite, 0)
	if mode != modeAll {
		for _, f := range user.Favourites {
			switch mode {
			case modeSFW:
				if f.NSFW {
					continue
				}
			case modeNSFW:
				if !f.NSFW {
					continue
				}
			}

			filtered = append(filtered, f)
		}
	} else {
		filtered = user.Favourites
	}

	l := len(filtered)
	for ind, f := range filtered {
		embeds = append(embeds, favouriteEmbed(f, ind, l))
	}

	if l > 1 {
		w := widget.NewWidget(s, m.Author.ID, embeds)
		err := w.Start(m.ChannelID)
		if err != nil {
			return err
		}
	} else if l == 1 {
		_, err := s.ChannelMessageSendEmbed(m.ChannelID, embeds[0])
		if err != nil {
			return err
		}
	} else {
		_, err := s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to execute a command.",
			Color:     utils.EmbedColor,
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Timestamp: utils.EmbedTimestamp(),
			Fields:    []*discordgo.MessageEmbedField{{"Reason", "Either your favourites list is empty or couldn't find favourite matching the filter", false}},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func favouriteEmbed(fav *database.Favourite, ind, l int) *discordgo.MessageEmbed {
	title := ""
	if l > 1 {
		if fav.Title == "" {
			title = fmt.Sprintf("[%v/%v] %v", ind+1, l, fav.Author)
		} else {
			title = fmt.Sprintf("[%v/%v] %v", ind+1, l, fav.Title)
		}
	} else {
		if fav.Title == "" {
			title = fmt.Sprintf("%v", fav.Author)
		} else {
			title = fmt.Sprintf("%v", fav.Title)
		}
	}

	embed := &discordgo.MessageEmbed{
		Title: title,
		Image: &discordgo.MessageEmbedImage{URL: fav.Thumbnail},
		URL:   fav.URL,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "ID", Value: strconv.Itoa(fav.ID), Inline: true},
			{Name: "Author", Value: fav.Author, Inline: true},
			{Name: "NSFW", Value: strconv.FormatBool(fav.NSFW), Inline: true},
			{Name: "Favourited at (GMT)", Value: fav.CreatedAt.Format("Jan 2 2006. 15:04:05"), Inline: true},
		},

		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
	}
	return embed
}

func unfavourite(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		user = database.NewUserSettings(m.Author.ID)
		err := database.DB.InsertOneUser(user)
		if err != nil {
			return fmt.Errorf("Fatal database error: %v", err)
		}
	}

	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	var (
		err     error
		removed = false
	)

	if id, err := strconv.Atoi(args[0]); err == nil {
		removed, err = database.DB.DeleteFavouriteID(user.ID, id)
	} else {
		removed, err = database.DB.DeleteFavouriteURL(user.ID, args[0])
	}

	if err != nil {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:       "❎ An error occurred",
			Color:       utils.EmbedColor,
			Timestamp:   utils.EmbedTimestamp(),
			Description: fmt.Sprintf("Error message: ``%v``\n\nPlease report the error to the developer using ``bt!feedback`` command", err),
		})
	} else if removed {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:       "✅ Successfully removed a favourite",
			Color:       utils.EmbedColor,
			Timestamp:   utils.EmbedTimestamp(),
			Description: fmt.Sprintf("Removed item: %v", args[0]),
		})
	} else {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:       "❎ Failed to remove a favourite",
			Color:       utils.EmbedColor,
			Timestamp:   utils.EmbedTimestamp(),
			Description: fmt.Sprintf("Couldn't find an item: %v", args[0]),
		})
	}
	return nil
}
