package commands

import (
	"fmt"
	"strings"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
)

func init() {
	cp := CommandFramework.AddGroup("crosspost", gumi.GroupDescription("Cross-posting feature settings"))
	cr := cp.AddCommand("create", createGroup, gumi.CommandDescription("Creates a new cross-post group."))
	cr.Help.AddField("Usage", "bt!create <group name> [channel IDs or mentions]", false)

	dl := cp.AddCommand("delete", deleteGroup, gumi.CommandDescription("Deletes a cross-post group."))
	dl.Help.AddField("Usage", "bt!delete <group name>", false)

	cp.AddCommand("groups", groups, gumi.CommandDescription("Lists all your cross-post groups."), gumi.WithAliases("list", "allgroups", "ls"))

	pop := cp.AddCommand("pop", removeFromGroup, gumi.CommandDescription("Removes channels from a cross-post group."), gumi.WithAliases("remove"))
	pop.Help.AddField("Usage", "bt!pop <group name> [channel IDs or mentions]", false)

	push := cp.AddCommand("push", addToGroup, gumi.CommandDescription("Adds channels to a cross-post group."), gumi.WithAliases("add"))
	push.Help.AddField("Usage", "bt!push <group name> [channel IDs or mentions]", false)
}

func groups(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		return fmt.Errorf("user settings not found, create create a group first with the following command: ``bt!create <group name> [channel IDs]``")
	}

	var sb strings.Builder

	for _, g := range user.ChannelGroups {
		sb.WriteString(fmt.Sprintf("***Group %v:***\n%v\n", g.Name, utils.Map(g.ChannelIDs, func(s string) string {
			return fmt.Sprintf("<#%v>", s)
		})))
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("%v's cross-post groups", m.Author.Username),
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: m.Author.AvatarURL("")},
	}

	embed.Description = sb.String()
	if embed.Description == "" {
		embed.Description = ":gun:🤠 *This town ain't big enough for the both of us!*\n"
		embed.Image = &discordgo.MessageEmbedImage{URL: "https://thumbs.gfycat.com/InconsequentialPerfumedGadwall-size_restricted.gif"}
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
	return nil
}

func createGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("``bt!create`` requires at least two arguments. Usage: ``bt!create Touhou #lewdtouhouart``")
	}

	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		user = database.NewUserSettings(m.Author.ID)
		err := database.DB.InsertOneUser(user)
		if err != nil {
			return fmt.Errorf("Fatal database error: %v", err)
		}
	}

	groupName := args[0]
	channelsMap := make(map[string]bool, 0)
	for _, id := range args[1:] {
		channelsMap[id] = true
	}

	channels := make([]string, 0)
	for ch := range channelsMap {
		if strings.HasPrefix(ch, "<#") {
			ch = strings.Trim(ch, "<#>")
		}

		if g := user.GroupByChannelID(ch); g != nil {
			return fmt.Errorf("Channel %v is already a part of group %v", ch, g.Name)
		}

		if _, err := s.State.Channel(ch); err != nil {
			return fmt.Errorf("unable to find channel ``%v``. Make sure Boe Tea is present on the server and able to read the channel", ch)
		}

		channels = append(channels, ch)
	}

	err := database.DB.CreateGroup(m.Author.ID, groupName, channels...)
	if err != nil {
		return fmt.Errorf("Fatal database error: %v", err)
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:     "✅ Sucessfully created a cross-post group!",
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
		Fields: []*discordgo.MessageEmbedField{{Name: "Name", Value: groupName}, {Name: "Channels", Value: fmt.Sprintf("%v", utils.Map(channels, func(s string) string {
			return fmt.Sprintf("<#%v>", s)
		}))}},
	})

	return nil
}

func deleteGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("``bt!delete`` requires at least one arguments.\n**Usage:** ``bt!delete ntr``")
	}

	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to delete a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: "You have no cross-post groups yet."}},
		})
		return nil
	}

	err := database.DB.DeleteGroup(m.Author.ID, args[0])
	if err != nil {
		return fmt.Errorf("Fatal database error: %v", err)
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:     "✅ Sucessfully deleted a cross-post group!",
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
		Fields:    []*discordgo.MessageEmbedField{{Name: "Name", Value: args[0]}},
	})

	return nil
}

func removeFromGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("``bt!remove`` requires at least two arguments.\n**Usage:** ``bt!remove nudes #nsfw``")
	}

	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to remove from a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: "You have no cross-post groups yet."}},
		})
		return nil
	}

	ids := utils.Map(args[1:], func(s string) string {
		return strings.Trim(s, "<#>")
	})

	found, err := database.DB.RemoveFromGroup(m.Author.ID, args[0], ids...)
	if err != nil {
		return fmt.Errorf("Fatal database error: %v", err)
	}

	if len(found) > 0 {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "✅ Sucessfully removed channels from a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields: []*discordgo.MessageEmbedField{{Name: "Group name", Value: args[0]}, {Name: "Channels", Value: strings.Join(utils.Map(found, func(s string) string {
				return fmt.Sprintf("<#%v>", s)
			}), " ")}},
		})
	} else {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to remove channels from a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Group name", Value: args[0]}, {Name: "Reason", Value: "No valid channels were found"}},
		})
	}

	return nil
}

func addToGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("``bt!add`` requires at least two arguments.\n**Usage:** ``bt!push hololive #marine-booty``")
	}

	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:     "❎ Failed to add to a cross-post group!",
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
			Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: "You have no cross-post groups yet."}},
		})
		return nil
	}

	groupName := args[0]
	channelsMap := make(map[string]bool, 0)
	for _, id := range args[1:] {
		channelsMap[id] = true
	}

	channels := make([]string, 0)
	for ch := range channelsMap {
		if strings.HasPrefix(ch, "<#") {
			ch = strings.Trim(ch, "<#>")
		}

		if g := user.GroupByChannelID(ch); g != nil {
			return fmt.Errorf("Channel <#%v> is already a part of group **%v**", ch, g.Name)
		}

		if _, err := s.State.Channel(ch); err != nil {
			return fmt.Errorf("unable to find channel ``%v``. Make sure Boe Tea is present on the server and able to read the channel", ch)
		}

		channels = append(channels, ch)
	}

	err := database.DB.AddToGroup(m.Author.ID, groupName, channels...)
	if err != nil {
		return fmt.Errorf("Fatal database error: %v", err)
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:     "✅ Sucessfully added channels to a cross-post group!",
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
		Fields: []*discordgo.MessageEmbedField{{Name: "Name", Value: args[0]}, {Name: "Channels", Value: strings.Join(utils.Map(channels, func(s string) string {
			return fmt.Sprintf("<#%v>", s)
		}), " ")}},
	})

	return nil
}
