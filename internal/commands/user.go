package commands

import (
	"fmt"
	"sort"
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
)

var (
	userSettingMap = map[string]settingFunc{
		"crosspost": setBool,
		"dm":        setBool,
	}
)

func init() {
	ug := Router.AddGroup(&gumi.Group{
		Name:        "user",
		Description: "User profile and settings",
		NSFW:        false,
		IsVisible:   true,
	})

	profileCmd := ug.AddCommand(&gumi.Command{
		Name:        "profile",
		Description: "Shows your profile",
		GuildOnly:   false,
		NSFW:        false,
		Exec:        profile,
		Cooldown:    5 * time.Second,
	})
	profileCmd.Help = gumi.NewHelpSettings().AddField("Usage", "bt!profile", false).AddField("Description", "Shows your Boe Tea profile.", false)

	favCmd := ug.AddCommand(&gumi.Command{
		Name:        "favourites",
		Aliases:     []string{"favorites", "favs", "fav", "bookmarks", "bm"},
		Description: "Shows a list of your favourites.",
		GuildOnly:   false,
		NSFW:        false,
		Exec:        favourites,
		Cooldown:    5 * time.Second,
	})
	favCmd.Help = gumi.NewHelpSettings()
	favCmd.Help.AddField("Usage", "bt!favourites `[flags]`", false)
	favCmd.Help.AddField("Flag syntax", "Flags have following syntax: `name:value`.\n_***Example***_: `bt!fav limit:100`.\nAccepted flags are listed below", false)
	favCmd.Help.AddField("mode", "Display mode, doubles down as sfw/nsfw filter.\n_***Default:***_ either sfw or nsfw, depends on channel.\nValue should be one of the following strings:\n`[compact, sfw, nsfw, all]`.", false)
	favCmd.Help.AddField("limit", "Number of artworks returned.\n_***Default:***_ 10.\nValue should be an _integer number from 1 to 100_", false)
	favCmd.Help.AddField("last", "Filter artworks by date.\n_***Default:***_ no filter.\nValue should be one of the following strings:\n`[day, week, month]`.", false)
	favCmd.Help.AddField("sort", "Sort type.\n_***Default:***_ time.\nValue should be one of the following strings:\n`[id, likes, favourites, time]`.", false)
	favCmd.Help.AddField("order", "Sort order.\n_***Default:***_ descending.\nValue should be one of the following strings:\n`[asc, ascending, desc, descending]`.", false)

	unfavCmd := ug.AddCommand(&gumi.Command{
		Name:        "unfavourite",
		Aliases:     []string{"unfavourite", "unfav"},
		Description: "Unfavourites an art by favourite ID or URL",
		GuildOnly:   false,
		NSFW:        false,
		Exec:        unfavourite,
		Cooldown:    5 * time.Second,
	})
	unfavCmd.Help = gumi.NewHelpSettings().AddField("Usage", "bt!unfavourite <art link or ID>", false)
	unfavCmd.Help.AddField("Art link", "A Twitter or Pixiv post link. It should match the link in your favourites list. For ease of use I recommend using IDs instead.", false)
	unfavCmd.Help.AddField("ID", "Favourite ID, can be retrieved from favourite list. Use ``bt!fav`` command", false)

	usersetCmd := ug.AddCommand(&gumi.Command{
		Name:        "userset",
		Aliases:     []string{""},
		Description: "Show or change user settings",
		Exec:        userset,
		Cooldown:    5 * time.Second,
	})

	usersetCmd.Help = gumi.NewHelpSettings().AddField("Usage", "bt!userset <setting> <new setting>", false).AddField("Settings", "__Not required.__ Accepts: [crosspost, dm]", false)

}

func profile(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		user = database.NewUserSettings(m.Author.ID)
		err := database.DB.InsertOneUser(user)
		if err != nil {
			return fmt.Errorf("fatal database error: %v", err)
		}
	}

	eb := embeds.NewBuilder()
	eb.Title(fmt.Sprintf("%v's profile", m.Author.Username)).Thumbnail(m.Author.AvatarURL(""))
	eb.AddField("Crosspost", utils.FormatBool(user.Crosspost))
	eb.AddField("DMs", utils.FormatBool(user.DM))
	eb.AddField("Number of favourites", strconv.Itoa(len(user.NewFavourites)))

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	return nil
}

type mode int

const (
	modeAll mode = iota
	modeSFW
	modeNSFW
	modeCompact
)

func favourites(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	var (
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if user == nil {
		user = database.NewUserSettings(m.Author.ID)
		err := database.DB.InsertOneUser(user)
		if err != nil {
			return fmt.Errorf("fatal database error: %v", err)
		}
	}

	if len(user.NewFavourites) == 0 {
		eb.FailureTemplate("Couldn't execute `favourites` command. You've got no favorites.")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	var (
		options    = database.NewFindManyOptions().Order(database.Descending)
		mode       = modeSFW
		favourites = user.NewFavourites
		sortByTime = true
	)

	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "limit:"):
			limitString := strings.TrimPrefix(a, "limit:")
			limit, err := strconv.Atoi(limitString)
			if err != nil || limit < 1 {
				if limit < 1 {
					eb.FailureTemplate("Couldn't execute `favourites` command. Provided limit argument is either not a number or out of allowed range [1:2^32).")
					s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
					return nil
				}
			}

			options.Limit(limit)
		case strings.HasPrefix(a, "sort:"):
			sort := strings.TrimPrefix(a, "sort:")
			switch {
			case sort == "favourites" || sort == "likes":
				sortByTime = false
				options.SortType(database.ByFavourites)
			case sort == "id":
				sortByTime = false
				options.SortType(database.ByID)
			}
		case strings.HasPrefix(a, "order:"):
			order := strings.TrimPrefix(a, "order:")
			switch {
			case order == "ascending" || order == "asc":
				options.Order(database.Ascending)
			case order == "descending" || order == "desc":
				options.Order(database.Descending)
			}
		case strings.HasPrefix(a, "last:"):
			last := strings.TrimPrefix(a, "last:")
			switch last {
			case "day":
				favourites = database.FilterFavourites(favourites, func(f *database.NewFavourite) bool {
					return f.CreatedAt.After(time.Now().AddDate(0, 0, -1))
				})
			case "week":
				favourites = database.FilterFavourites(favourites, func(f *database.NewFavourite) bool {
					return f.CreatedAt.After(time.Now().AddDate(0, 0, -7))
				})
			case "month":
				favourites = database.FilterFavourites(favourites, func(f *database.NewFavourite) bool {
					return f.CreatedAt.After(time.Now().AddDate(0, -1, 0))
				})
			}
		case strings.HasPrefix(a, "mode:"):
			switch strings.TrimPrefix(a, "mode:") {
			case "all":
				mode = modeAll
			case "sfw":
				mode = modeSFW
			case "nsfw":
				mode = modeNSFW
			case "compact":
				mode = modeCompact
			default:
				mode = modeSFW
			}
		}
	}

	ch, _ := s.Channel(m.ChannelID)
	if !ch.NSFW && mode == modeNSFW || mode == modeAll {
		eb.WarnTemplate("The result may contain NSFW images, are you sure about that?")
		f := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
			Embed: eb.Finalize(),
		})

		eb.Clear()
		if !f {
			return nil
		}
	}

	switch mode {
	case modeSFW:
		favourites = database.FilterFavourites(favourites, func(f *database.NewFavourite) bool {
			return !f.NSFW
		})
	case modeNSFW:
		favourites = database.FilterFavourites(favourites, func(f *database.NewFavourite) bool {
			return f.NSFW
		})
	}

	artworks, err := database.DB.FindManyArtworks(favourites, options)
	if err != nil {
		return err
	}

	timeMap := make(map[int]time.Time)
	for _, favourite := range favourites {
		timeMap[favourite.ID] = favourite.CreatedAt
	}
	if sortByTime {
		sort.Slice(artworks, func(i, j int) bool {
			if options.SortOrder == database.Ascending {
				return artworks[i].CreatedAt.Before(artworks[j].CreatedAt)
			}
			return artworks[j].CreatedAt.Before(artworks[i].CreatedAt)
		})
	}

	embeds := favouriteEmbeds(artworks, timeMap, mode == modeCompact)
	if err != nil {
		return err
	}

	l := len(embeds)
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
		eb.FailureTemplate("Failed to execute `favourites` command. Either your favourites list is empty or couldn't find favourites matching the filter")
		_, err := s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		if err != nil {
			return err
		}
	}
	return nil
}

func favouriteEmbeds(artworks []*database.Artwork, timeMap map[int]time.Time, compact bool) []*discordgo.MessageEmbed {
	embeds := make([]*discordgo.MessageEmbed, 0)
	if compact {
		embeds = compactFavourites(artworks)
	} else {
		for ind, art := range artworks {
			embeds = append(embeds, favouriteEmbed(art, timeMap[art.ID], ind, len(artworks)))
		}
	}

	return embeds
}

func artworkEmbed(artwork *database.Artwork, ind, l int) *discordgo.MessageEmbed {
	var (
		title   = ""
		percent = (float64(artwork.NSFW) / float64(artwork.Favourites)) * 100.0
	)

	if l > 1 {
		if artwork.Title == "" {
			title = fmt.Sprintf("[%v/%v] %v", ind+1, l, artwork.Author)
		} else {
			title = fmt.Sprintf("[%v/%v] %v", ind+1, l, artwork.Title)
		}
	} else {
		if artwork.Title == "" {
			title = fmt.Sprintf("%v", artwork.Author)
		} else {
			title = fmt.Sprintf("%v", artwork.Title)
		}
	}

	eb := embeds.NewBuilder()
	eb.Title(title).URL(artwork.URL)
	if len(artwork.Images) > 0 {
		eb.Image(artwork.Images[0])
	}
	eb.AddField("ID", strconv.Itoa(artwork.ID), true).AddField("Author", artwork.Author, true).AddField("Favourites", strconv.Itoa(artwork.Favourites), true)
	eb.AddField("URL", fmt.Sprintf("[%v](%v)", "Click here desu~", artwork.URL)).AddField("Created", artwork.CreatedAt.Format("Jan 2 2006. 15:04:05 MST"))
	eb.AddField("Lewdmeter", fmt.Sprintf("%.2f%s", percent, "%"))
	return eb.Finalize()
}

func favouriteEmbed(art *database.Artwork, t time.Time, ind, l int) *discordgo.MessageEmbed {
	title := ""
	if l > 1 {
		if art.Title == "" {
			title = fmt.Sprintf("[%v/%v] %v", ind+1, l, art.Author)
		} else {
			title = fmt.Sprintf("[%v/%v] %v", ind+1, l, art.Title)
		}
	} else {
		if art.Title == "" {
			title = fmt.Sprintf("%v", art.Author)
		} else {
			title = fmt.Sprintf("%v", art.Title)
		}
	}

	eb := embeds.NewBuilder().Title(title).URL(art.URL)
	eb.AddField("ID", strconv.Itoa(art.ID), true).AddField("Author", art.Author, true)
	eb.AddField("Favourites", strconv.Itoa(art.Favourites), true).AddField("URL", fmt.Sprintf("[%v](%v)", "Click here desu~", art.URL), true)
	eb.AddField("Added to favourites", t.Format("Jan 2 2006. 15:04:05 MST"), true)
	if len(art.Images) > 0 {
		eb.Image(art.Images[0])
	}

	return eb.Finalize()
}

func compactFavourites(fav []*database.Artwork) []*discordgo.MessageEmbed {
	perPage := 10
	length := len(fav) / perPage
	if len(fav)%perPage != 0 {
		length++
	}

	var (
		pages        = make([]*discordgo.MessageEmbed, length)
		page         = 0
		sb           strings.Builder
		eb           = embeds.NewBuilder()
		defaultEmbed = func() *discordgo.MessageEmbed {
			eb.Clear()
			eb.Title("Compact favourites list")
			eb.AddField("Total favourites", "```"+strconv.Itoa(len(fav))+"```", true)
			eb.AddField("Page", fmt.Sprintf("```%v / %v```", page+1, length), true)
			return eb.Finalize()
		}
	)

	pages[page] = defaultEmbed()
	count := 0
	for ind, f := range fav {
		sb.WriteString(fmt.Sprintf("`%v | ID: %v.` [%v](%v)\n", ind+1, f.ID, f.Title+" ("+f.Author+")", f.URL))
		count++

		if count == perPage {
			pages[page].Description = sb.String()
			page++

			if page != length {
				pages[page] = defaultEmbed()
			}

			sb.Reset()
			count = 0
		}
	}

	if count != 0 {
		pages[page] = defaultEmbed()
		pages[page].Description = sb.String()
	}

	return pages
}

func unfavourite(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		user = database.NewUserSettings(m.Author.ID)
		err := database.DB.InsertOneUser(user)
		if err != nil {
			return fmt.Errorf("fatal database error: %v", err)
		}
	}

	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	var (
		err error
	)

	if id, err := strconv.Atoi(args[0]); err == nil {
		_, err = database.DB.RemoveFavouriteID(user.ID, id)
		if err != nil {
			logrus.Errorf("RemoveFavouriteID() -> %v", err)
		}
	} else {
		_, err = database.DB.RemoveFavouriteURL(user.ID, args[0])
		if err != nil {
			logrus.Errorf("RemoveFavouriteID() -> %v", err)
		}
	}

	if err != nil {
		return err
	} else {
		eb := embeds.NewBuilder()
		eb.FailureTemplate(fmt.Sprintf("Failed to remove a favourite. Couldn't find an item: %v", args[0]))
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	}

	return nil
}

func userset(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	user := database.DB.FindUser(m.Author.ID)

	if user != nil {
		if length := len(args); length == 0 {
			showUserSettings(s, m, user)
		} else if length >= 2 {
			setting := args[0]
			newSetting := strings.ToLower(args[1])

			if new, ok := userSettingMap[setting]; ok {
				n, err := new(s, m, newSetting)
				if err != nil {
					return err
				}
				err = database.DB.ChangeUserSetting(m.Author.ID, setting, n)
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
	}

	return nil
}

func showUserSettings(s *discordgo.Session, m *discordgo.MessageCreate, user *database.UserSettings) {
	eb := embeds.NewBuilder()
	eb.Title("User settings").Description(m.Author.String()).Thumbnail(m.Author.AvatarURL(""))
	eb.AddField("General", fmt.Sprintf("**Crosspost:** %v | **DM:** %v", utils.FormatBool(user.Crosspost), utils.FormatBool(user.DM)))

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
}
