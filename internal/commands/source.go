package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	ascii2dgo "github.com/VTGare/ascii2d-go"
	"github.com/VTGare/boe-tea-go/internal/widget"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/VTGare/iqdbgo"
	"github.com/VTGare/sengoku"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

func init() {
	sg := Router.AddGroup(&gumi.Group{
		Name:        "source",
		Description: "Source, repost and image manipulation commands",
		IsVisible:   true,
	})

	sauceCmd := sg.AddCommand(&gumi.Command{
		Name:        "sauce",
		Description: "Finds source of an image using all reverse search engines",
		Aliases:     []string{},
		Exec:        sauce,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	sauceCmd.Help.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!sauce <image link>",
		},
		{
			Name:  "image link",
			Value: "Not required. If not provided, looks for images in 5 previous messages. Link should either have one of following suffixes [*jpg*, *jpeg*, *png*, *gif*, *webp*] or be a Discord message link in the following format: ``https://discord.com/channels/%GUILD_ID%/%CHANNEL_ID%/%MESSAGE_ID%``",
		},
	}

	saucenaoCmd := sg.AddCommand(&gumi.Command{
		Name:        "saucenao",
		Description: "Finds source of an image using SauceNAO reverse search engine",
		Exec:        saucenao,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})

	saucenaoCmd.Help.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!saucenao <image link>",
		},
		{
			Name:  "image link",
			Value: "Not required. If not provided, looks for images in 5 previous messages. Link should either have one of following suffixes [*jpg*, *jpeg*, *png*, *gif*, *webp*] or be a Discord message link in the following format: ``https://discord.com/channels/%GUILD_ID%/%CHANNEL_ID%/%MESSAGE_ID%``",
		},
	}

	ascii2dCmd := sg.AddCommand(&gumi.Command{
		Name:        "ascii2d",
		Description: "Finds source of an image using ASCII2D reverse search engine",
		Exec:        ascii2d,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	ascii2dCmd.Help.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!ascii2d <image link>",
		},
		{
			Name:  "image link",
			Value: "Not required. If not provided, looks for images in 5 previous messages. Link should either have one of following suffixes [*jpg*, *jpeg*, *png*, *gif*, *webp*] or be a Discord message link in the following format: ``https://discord.com/channels/%GUILD_ID%/%CHANNEL_ID%/%MESSAGE_ID%``",
		},
	}

	iqdbCmd := sg.AddCommand(&gumi.Command{
		Name:        "iqdb",
		Description: "Finds source of an image using iqdb reverse search engine",
		Exec:        iqdb,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	iqdbCmd.Help.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!iqdb <image link>",
		},
		{
			Name:  "image link",
			Value: "Not required. If not provided, looks for images in 5 previous messages. Link should either have one of following suffixes [*jpg*, *jpeg*, *png*, *gif*, *webp*] or be a Discord message link in the following format: ``https://discord.com/channels/%GUILD_ID%/%CHANNEL_ID%/%MESSAGE_ID%``",
		},
	}

	waitCmd := sg.AddCommand(&gumi.Command{
		Name:        "wait",
		Description: "Finds an anime source from a screenshot.",
		Aliases:     []string{"trace", "tracemoe"},
		Exec:        saucenao,
		Cooldown:    10 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	waitCmd.Help.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!wait <image link>",
		},
		{
			Name:  "image link",
			Value: "Not required. If not provided, looks for images in 5 previous messages. Link should either have one of following suffixes [*jpg*, *jpeg*, *png*, *gif*, *webp*] or be a Discord message link in the following format: ``https://discord.com/channels/%GUILD_ID%/%CHANNEL_ID%/%MESSAGE_ID%``",
		},
	}
}

func sauce(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	url, err := findImage(s, m, args)
	if err != nil {
		return err
	}

	if url == "" {
		return utils.ErrNotEnoughArguments
	}

	send := func(embeds ...*discordgo.MessageEmbed) error {
		if len(embeds) > 1 {
			w := widget.NewWidget(s, m.Author.ID, embeds)
			err := w.Start(m.ChannelID)
			if err != nil {
				return err
			}
		} else {
			_, err = s.ChannelMessageSendEmbed(m.ChannelID, embeds[0])
			if err != nil {
				return err
			}
		}

		return nil
	}

	log.Infof("Searching source on SauceNAO. Image URL: %s", url)
	embeds, err := saucenaoEmbeds(url, false)
	if err != nil {
		log.Warnf("saucenaoEmbeds: %v", err)
	}

	if len(embeds) != 0 {
		err := send(embeds...)
		if err != nil {
			return err
		}
		return nil
	}

	if prompt := utils.CreatePrompt(s, m, &utils.PromptOptions{
		Actions: map[string]bool{
			"✅": true,
			"❎": false,
		},
		Message: "<:peepoRainy:530050503955054593> Source couldn't be found on SauceNAO. Would you like to try __*ascii2d?*__",
		Timeout: 15 * time.Second,
	}); prompt == true {
		log.Infof("Searching source on ascii2d. Image URL: %s", url)
		res, err := ascii2dgo.Search(url)
		if err != nil {
			return err
		}

		var embeds = make([]*discordgo.MessageEmbed, 0)
		l := len(res.Sources)
		for i, s := range res.Sources {
			embeds = append(embeds, ascii2dEmbed(s, i, l))
		}
		if len(embeds) > 1 {
			w := widget.NewWidget(s, m.Author.ID, embeds)
			err := w.Start(m.ChannelID)
			if err != nil {
				return err
			}
		} else if len(embeds) > 0 {
			_, err = s.ChannelMessageSendEmbed(m.ChannelID, embeds[0])
			if err != nil {
				return err
			}
		} else {
			s.ChannelMessageSendEmbed(m.ChannelID, noSauceEmbed)
		}
	}
	return nil
}

func saucenao(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	url, err := findImage(s, m, args)
	if err != nil {
		return err
	}

	if url == "" {
		return utils.ErrNotEnoughArguments
	}

	log.Infof("Searching source on SauceNAO. Image URL: %s", url)
	embeds, err := saucenaoEmbeds(url, true)
	if err != nil {
		return err
	}

	if len(embeds) > 1 {
		w := widget.NewWidget(s, m.Author.ID, embeds)
		err := w.Start(m.ChannelID)
		if err != nil {
			return err
		}
	} else {
		_, err = s.ChannelMessageSendEmbed(m.ChannelID, embeds[0])
		if err != nil {
			return err
		}
	}

	return nil
}

func saucenaoEmbeds(link string, nosauce bool) ([]*discordgo.MessageEmbed, error) {
	res, err := sc.Search(link)
	if err != nil {
		return nil, err
	}

	filtered := make([]*sengoku.Sauce, 0)
	for _, r := range res {
		if err != nil {
			continue
		}

		if r.Similarity < 70.0 {
			continue
		}

		if !r.Pretty {
			continue
		}

		filtered = append(filtered, r)
	}

	l := len(filtered)
	if l == 0 {
		if nosauce {
			return []*discordgo.MessageEmbed{noSauceEmbed}, nil
		}
		return nil, nil
	}

	log.Infof("Found source. Results: %v", l)
	embeds := make([]*discordgo.MessageEmbed, l)
	for ind, source := range filtered {
		embed := saucenaoToEmbed(source, ind, l)
		embeds[ind] = embed
	}

	return embeds, nil
}

func saucenaoToEmbed(source *sengoku.Sauce, index, length int) *discordgo.MessageEmbed {
	title := ""
	if length > 1 {
		title = fmt.Sprintf("[%v/%v] %v", index+1, length, source.Title)
	} else {
		title = fmt.Sprintf("%v", source.Title)
	}

	embed := &discordgo.MessageEmbed{
		Title:     title,
		URL:       source.URLs.Source,
		Timestamp: utils.EmbedTimestamp(),
		Color:     utils.EmbedColor,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: source.Thumbnail,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Source",
				Value: source.URLs.Source,
			},
			{
				Name:  "Similarity",
				Value: fmt.Sprintf("%v", source.Similarity),
			},
		},
	}

	if source.Author != nil {
		str := ""
		if source.Author.URL != "" {
			str = fmt.Sprintf("[%v](%v)", source.Author.Name, source.Author.URL)
		} else {
			str = source.Author.Name
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Author", Value: str})
	}

	if str := joinSauceURLs(source.URLs.ExternalURLs, " • "); str != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Other URLs", Value: str})
	}
	return embed
}

func wait(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	url, err := findImage(s, m, args)
	if err != nil {
		return err
	}

	if url == "" {
		return utils.ErrNotEnoughArguments
	}

	log.Infof("Searching source on trace.moe. Image URL: %s", url)
	embed, err := waitEmbed(url)
	if err != nil {
		return err
	}

	_, err = s.ChannelMessageSendEmbed(m.ChannelID, embed)
	if err != nil {
		return err
	}

	return nil
}

func ascii2d(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	url, err := findImage(s, m, args)
	if err != nil {
		return err
	}

	if url == "" {
		return utils.ErrNotEnoughArguments
	}

	log.Infof("Searching source on Ascii2d. Image URL: %s", url)
	res, err := ascii2dgo.Search(url)
	if err != nil {
		return err
	}

	var embeds = make([]*discordgo.MessageEmbed, 0)
	l := len(res.Sources)
	for i, s := range res.Sources {
		embeds = append(embeds, ascii2dEmbed(s, i, l))
	}

	if len(embeds) > 1 {
		w := widget.NewWidget(s, m.Author.ID, embeds)
		err := w.Start(m.ChannelID)
		if err != nil {
			return err
		}
	} else {
		_, err = s.ChannelMessageSendEmbed(m.ChannelID, embeds[0])
		if err != nil {
			return err
		}
	}

	return nil
}

func ascii2dEmbed(source *ascii2dgo.Source, index, length int) *discordgo.MessageEmbed {
	title := ""
	if length > 1 {
		title = fmt.Sprintf("[%v/%v] %v", index+1, length, source.Title)
	} else {
		title = fmt.Sprintf("%v", source.Title)
	}

	embed := &discordgo.MessageEmbed{
		Title:     title,
		URL:       source.URL,
		Timestamp: utils.EmbedTimestamp(),
		Color:     utils.EmbedColor,
		Image: &discordgo.MessageEmbedImage{
			URL: source.Thumbnail,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Source",
				Value: source.URL,
			},
			{
				Name:  "Author",
				Value: fmt.Sprintf("[%v](%v)", source.Author.Name, source.Author.URL),
			},
		},
	}
	return embed
}

func iqdb(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	url, err := findImage(s, m, args)
	if err != nil {
		return err
	}

	if url == "" {
		return utils.ErrNotEnoughArguments
	}

	log.Infof("Searching source on iqdb. Image URL: %s", url)
	res, err := iqdbgo.Search(url)
	if err != nil {
		return err
	}

	var embeds = make([]*discordgo.MessageEmbed, 0)
	l := len(res.PossibleMatches)
	if res.BestMatch != nil {
		l++
	}

	if res.BestMatch != nil {
		embeds = append(embeds, iqdbEmbed(res.BestMatch, true, 0, l))
	}
	for i, s := range res.PossibleMatches {
		if res.BestMatch != nil {
			i++
		}
		embeds = append(embeds, iqdbEmbed(s, false, i, l))
	}

	if len(embeds) > 1 {
		w := widget.NewWidget(s, m.Author.ID, embeds)
		err := w.Start(m.ChannelID)
		if err != nil {
			return err
		}
	} else {
		_, err = s.ChannelMessageSendEmbed(m.ChannelID, embeds[0])
		if err != nil {
			return err
		}
	}

	return nil
}

func iqdbEmbed(source *iqdbgo.Match, best bool, index, length int) *discordgo.MessageEmbed {
	matchType := ""
	if best {
		matchType = "Best match"
	} else {
		matchType = "Possible match"
	}

	if !strings.HasPrefix(source.URL, "https:") {
		source.URL = "https:" + source.URL
	}

	title := ""
	if length > 1 {
		title = fmt.Sprintf("[%v/%v] %v", index+1, length, matchType)
	} else {
		title = fmt.Sprintf("%v", matchType)
	}

	embed := &discordgo.MessageEmbed{
		Title:     title,
		URL:       source.URL,
		Timestamp: utils.EmbedTimestamp(),
		Color:     utils.EmbedColor,
		Image: &discordgo.MessageEmbedImage{
			URL: source.Thumbnail,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Source",
				Value: source.URL,
			},
			{
				Name:  "Info",
				Value: fmt.Sprintf("%v", source.Tags),
			},
			{
				Name:  "Similarity",
				Value: strconv.Itoa(source.Similarity),
			},
		},
	}

	return embed
}
