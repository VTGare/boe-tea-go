package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	ascii2dgo "github.com/VTGare/ascii2d-go"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/internal/widget"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/VTGare/iqdbgo"
	"github.com/VTGare/sengoku"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	noSauceEmbed = embeds.NewBuilder().InfoTemplate("Sorry, Boe Tea couldnt find source or the image, if you haven't yet please consider using methods below").AddField("iqdb", "`bt!iqdb`", true).AddField("ascii2d", "[Click here desu~](https://ascii2d.net)").AddField("Google Image Search", "[Click here desu~](https://www.google.com/imghp?hl=EN)").Finalize()
)

func init() {
	sg := Router.AddGroup(&gumi.Group{
		Name:        "source",
		Description: "Reverse search engines",
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

	log.Infof("Searching source on SauceNAO. Image URL: %v", url)
	sauceEmbeds, err := saucenaoEmbeds(url, false)
	if err != nil {
		log.Warnf("saucenaoEmbeds: %v", err)
		if err == sengoku.ErrRateLimitReached {
			eb := embeds.NewBuilder().InfoTemplate("Boe Tea's getting rate limited by SauceNAO. If you want to support me, so I can afford monthly SauceNAO subscription consider becoming a patron!")
			eb.AddField("Patreon", "[Click here desu~](https://www.patreon.com/vtgare)")

			s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		}
	}

	if len(sauceEmbeds) != 0 {
		err := send(sauceEmbeds...)
		if err != nil {
			return err
		}
		return nil
	}

	eb := embeds.NewBuilder()
	eb.InfoTemplate("<:peepoRainy:530050503955054593> Source couldn't be found on SauceNAO. Would you like to try your luck with ascii2d? (⚠ Boe Tea works inconsistently with it)")
	prompt := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
		Embed: eb.Finalize(),
	})

	if prompt {
		log.Infof("Searching source on ascii2d. Image URL: %v", url)
		res, err := ascii2dgo.Search(url)
		if err != nil {
			return err
		}

		var embeds = make([]*discordgo.MessageEmbed, 0)
		length := len(res.Sources)
		if length == 0 {
			s.ChannelMessageSendEmbed(m.ChannelID, noSauceEmbed)
		}

		for i, s := range res.Sources {
			embeds = append(embeds, ascii2dEmbed(s, i, length))
		}

		if len(embeds) > 0 {
			w := widget.NewWidget(s, m.Author.ID, embeds)
			err = w.Start(m.ChannelID)
			if err != nil {
				return err
			}
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

	log.Infof("Searching source on SauceNAO. Image URL: %v", url)
	sauceEmbeds, err := saucenaoEmbeds(url, true)
	if err != nil {
		if err == sengoku.ErrRateLimitReached {
			eb := embeds.NewBuilder().InfoTemplate("Boe Tea's getting rate limited by SauceNAO. If you want to support me, so I can afford monthly SauceNAO subscription consider becoming a patron!")
			eb.AddField("Patreon", "[Click here desu~](https://www.patreon.com/vtgare)")

			s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		}
		return err
	}

	w := widget.NewWidget(s, m.Author.ID, sauceEmbeds)
	err = w.Start(m.ChannelID)
	if err != nil {
		return err
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

	eb := embeds.NewBuilder()
	eb.Title(title).Thumbnail(source.Thumbnail)
	if source.URLs.Source != "" {
		eb.URL(source.URLs.Source)
		eb.AddField("Source", source.URLs.Source)
	}
	eb.AddField("Similarity", fmt.Sprintf("%v", source.Similarity))

	if source.Author != nil {
		if source.Author.Name != "" {
			str := ""
			if source.Author.URL != "" {
				str = fmt.Sprintf("[%v](%v)", source.Author.Name, source.Author.URL)
			} else {
				str = source.Author.Name
			}
			eb.AddField("Author", str)
		}
	}

	if str := joinSauceURLs(source.URLs.ExternalURLs, " • "); str != "" {
		eb.AddField("Other URLs", str)
	}
	return eb.Finalize()
}

func ascii2d(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	url, err := findImage(s, m, args)
	if err != nil {
		return err
	}

	if url == "" {
		return utils.ErrNotEnoughArguments
	}

	log.Infof("Searching source on Ascii2d. Image URL: %v", url)
	res, err := ascii2dgo.Search(url)
	if err != nil {
		return err
	}

	if len(res.Sources) == 0 {
		s.ChannelMessageSendEmbed(m.ChannelID, noSauceEmbed)
		return nil
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

	eb := embeds.NewBuilder()
	eb.Title(title).URL(source.URL).Image(source.Thumbnail).AddField("Source", source.URL)
	eb.AddField("Author", fmt.Sprintf("[%v](%v)", source.Author.Name, source.Author.URL))
	return eb.Finalize()
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

	var messageEmbeds = make([]*discordgo.MessageEmbed, 0)
	length := len(res.PossibleMatches)
	if res.BestMatch != nil {
		length++
	}

	if length == 0 {
		s.ChannelMessageSendEmbed(m.ChannelID, noSauceEmbed)
		return nil
	}

	if res.BestMatch != nil {
		messageEmbeds = append(messageEmbeds, iqdbEmbed(res.BestMatch, true, 0, length))
	}
	for i, s := range res.PossibleMatches {
		if res.BestMatch != nil {
			i++
		}
		messageEmbeds = append(messageEmbeds, iqdbEmbed(s, false, i, length))
	}

	if len(messageEmbeds) > 1 {
		w := widget.NewWidget(s, m.Author.ID, messageEmbeds)
		err := w.Start(m.ChannelID)
		if err != nil {
			return err
		}
	} else {
		_, err = s.ChannelMessageSendEmbed(m.ChannelID, messageEmbeds[0])
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

	if strings.HasPrefix(source.URL, "http:") {
		source.URL = strings.Replace(source.URL, "http:", "https:", 1)
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

	eb := embeds.NewBuilder()

	eb.Title(title).URL(source.URL).Image(source.Thumbnail)
	eb.AddField("Source", source.URL).AddField("Info", fmt.Sprintf("%v", source.Tags)).AddField("Similarity", strconv.Itoa(source.Similarity))
	return eb.Finalize()
}
