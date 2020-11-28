package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
)

func init() {
	cp := Router.AddGroup(&gumi.Group{
		Name:        "crosspost",
		Description: "Cross-posting feature settings",
		IsVisible:   true,
	})

	mk := cp.AddCommand(&gumi.Command{
		Name:        "create",
		Description: "Creates a new cross-post group",
		Exec:        createGroup,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	mk.Help.AddField("Usage", "bt!create <group name> [channel IDs or mentions]", false)

	dl := cp.AddCommand(&gumi.Command{
		Name:        "delete",
		Description: "Deletes a cross-post group",
		Exec:        deleteGroup,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	dl.Help.AddField("Usage", "bt!delete <group name>", false)

	cp.AddCommand(&gumi.Command{
		Name:        "list",
		Aliases:     []string{"ls", "groups"},
		Description: "List all your cross-post groups",
		Exec:        groups,
		Help:        gumi.NewHelpSettings(),
	})

	pop := cp.AddCommand(&gumi.Command{
		Name:        "pop",
		Aliases:     []string{"remove"},
		Description: "Removes a channel from a group",
		Exec:        removeFromGroup,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	pop.Help.AddField("Usage", "bt!pop <group name> [channel IDs or mentions]", false)

	push := cp.AddCommand(&gumi.Command{
		Name:        "push",
		Aliases:     []string{"add"},
		Description: "Adds a channel to a group",
		Exec:        addToGroup,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	push.Help.AddField("Usage", "bt!push <group name> [channel IDs or mentions]", false)

	copyc := cp.AddCommand(&gumi.Command{
		Name:        "copy",
		Aliases:     []string{"cp", "clone"},
		Description: "Copies a cross-post group",
		Exec:        copyGroup,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	copyc.Help.AddField("Usage", "bt!copy <source group name> <destination group name> <parent ID>", false)
}

func groups(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	var (
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)
	if user == nil {
		return fmt.Errorf("user settings not found, create create a group first with the following command: ``bt!create <group name> <parent ID>``")
	}

	if len(user.ChannelGroups) == 0 {
		msg := "You've got no channel groups! Create one by executing `bt!create <group name>`"
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
		return nil
	}

	eb.Title(fmt.Sprintf("%v's cross-post groups", m.Author.Username)).Thumbnail(m.Author.AvatarURL(""))
	for _, g := range user.ChannelGroups {
		if len(g.Children) > 0 {
			children := utils.Map(g.Children, func(str string) string {
				return fmt.Sprintf("<#%v>", str)
			})

			eb.AddField(g.Name, fmt.Sprintf("**Parent:** [<#%v>]\n**Children:** %v", g.Parent, children))
		} else {
			eb.AddField(g.Name, fmt.Sprintf("**Parent:** [<#%v>]\n**Children:** -", g.Parent))
		}
	}

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	return nil
}

func createGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	var (
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if len(args) < 2 {
		msg := "``bt!create`` requires two arguments. Example: ``bt!create touhou #lewdtouhouart``"
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
		return nil
	}

	if user == nil {
		user = database.NewUserSettings(m.Author.ID)
		err := database.DB.InsertOneUser(user)
		if err != nil {
			return fmt.Errorf("fatal database error: %v", err)
		}
	}

	var (
		groupName = args[0]
		ch        = args[1]
	)

	ch = strings.Trim(ch, "<#>")
	if _, err := s.State.Channel(ch); err != nil {
		msg := fmt.Sprintf("Unable to find channel [%v]. Make sure Boe Tea can read the channel in question!", ch)
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())

		return nil
	}

	err := database.DB.CreateGroup(m.Author.ID, groupName, ch)
	if err != nil {
		return fmt.Errorf("fatal database error: %v", err)
	}

	eb.SuccessTemplate("Successfully created a group").Thumbnail(utils.DefaultEmbedImage).AddField("Name", groupName).AddField("Parent channel", fmt.Sprintf("<#%v>", ch))
	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	return nil
}

func deleteGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	var (
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if len(args) < 1 {
		msg := "``bt!delete`` requires at least one argument.\n**Usage:** ``bt!delete ntr``"
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
		return nil
	}
	if user == nil || len(user.ChannelGroups) == 0 {
		eb.FailureTemplate("Failed to delete a cross-post group. You've got no channel groups! Create one by executing `bt!create <group name>`")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	err := database.DB.DeleteGroup(m.Author.ID, args[0])
	if err != nil {
		return fmt.Errorf("fatal database error: %v", err)
	}

	eb.SuccessTemplate("Sucessfully deleted a cross-post group!").AddField("Name", args[0])
	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	return nil
}

func removeFromGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	var (
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if len(args) < 2 {
		eb.FailureTemplate("``bt!remove`` requires at least two arguments.\n**Usage:** ``bt!remove nudes #nsfw``")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	if user == nil || len(user.ChannelGroups) == 0 {
		eb.FailureTemplate("Failed to remove an item from a cross-post group. You've got no channel groups! Create one by executing `bt!create <group name>`")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	ids := utils.Map(args[1:], func(s string) string {
		return strings.Trim(s, "<#>")
	})

	found, err := database.DB.RemoveFromGroup(m.Author.ID, args[0], ids...)
	if err != nil {
		return fmt.Errorf("fatal database error: %v", err)
	}

	if len(found) > 0 {
		channels := strings.Join(utils.Map(found, func(s string) string {
			return fmt.Sprintf("<#%v>", s)
		}), " ")

		eb.SuccessTemplate("Successfully removed channels from a cross-post group!").AddField("Group name", args[0]).AddField("Channels", channels)
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	} else {
		eb.FailureTemplate("Failed to remove channels from a cross-post group! None of the specified channels were found.").AddField("Group name", args[0])
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	}

	return nil
}

func addToGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	var (
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if len(args) < 2 {
		msg := "``bt!push`` requires at least two arguments.\n**Usage:** ``bt!push hololive #marine-booty``"
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
		return nil
	}

	if user == nil {
		eb.FailureTemplate("Failed to add a channel to a group! You've got no channel groups! Create one by executing `bt!create <group name>`")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	groupName := args[0]
	channelsMap := make(map[string]bool)
	for _, id := range args[1:] {
		channelsMap[id] = true
	}

	channels := make([]string, 0)

	group, _ := user.FindGroup(groupName)
	if group == nil {
		eb.FailureTemplate("Failed to a channel to a cross-post group! Cross-post group [" + groupName + "] has not been found.")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	existsMap := make(map[string]bool)
	existsMap[group.Parent] = true
	for _, id := range group.Children {
		existsMap[id] = true
	}

	for ch := range channelsMap {
		ch = strings.Trim(ch, "<#>")

		if _, err := s.State.Channel(ch); err != nil {
			msg := fmt.Sprintf("Unable to find channel [%v]. Make sure Boe Tea can read the channel in question!", ch)
			s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())

			return nil
		}

		if _, ok := existsMap[ch]; ok {
			eb.FailureTemplate("Failed to add a channel to a cross-post group! " + fmt.Sprintf("Channel <#%v> is already part of group %v", ch, groupName))
			s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
			return nil
		}

		channels = append(channels, ch)
	}

	added, err := database.DB.AddToGroup(m.Author.ID, groupName, channels...)
	if err != nil {
		return fmt.Errorf("fatal database error: %v", err)
	}

	if len(added) > 0 {
		channels := strings.Join(utils.Map(added, func(s string) string {
			return fmt.Sprintf("<#%v>", s)
		}), " ")

		eb.SuccessTemplate("Successfully added channels to a cross-post group!").AddField("Name", args[0]).AddField("Channels", channels)
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	} else {
		eb.FailureTemplate("Failed to add channels to a cross-post group! None of the specified channels were found.").AddField("Group name", args[0])
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	}

	return nil
}

func copyGroup(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	var (
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if len(args) < 3 {
		msg := "``bt!copy`` requires at least three arguments.\n**Usage:** ``bt!copy <source> <destination> <new parent channel>``"
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
		return nil
	}

	if user == nil || len(user.ChannelGroups) == 0 {
		eb.FailureTemplate("Failed to copy a cross-post group. You've got no channel groups! Create one by executing `bt!create <group name>`")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	var (
		group    *database.Group
		src      = args[0]
		dest     = args[1]
		exists   bool
		parent   = strings.Trim(args[2], "<#>")
		isParent bool
	)

	if _, err := s.State.Channel(parent); err != nil {
		msg := fmt.Sprintf("Unable to find channel [%v]. Make sure Boe Tea can read the channel in question!", parent)
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())

		return nil
	}

	for _, g := range user.ChannelGroups {
		if g.Name == src {
			group = g
		}

		if g.Name == dest {
			exists = true
		}

		if g.Parent == parent {
			isParent = true
		}
	}

	if group == nil {
		eb.FailureTemplate("Failed to copy a cross-post group! " + "Couldn't find a source group ``" + src + "``")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	if isParent {
		eb.FailureTemplate("Failed to copy a cross-post group! " + fmt.Sprintf("Channel <#%v> is already a parent channel", parent))
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())

		return nil
	}

	if exists {
		eb.FailureTemplate("Failed to copy a cross-post group! " + fmt.Sprintf("Group name %v is already taken", dest))
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	new := &database.Group{
		Name:     dest,
		Parent:   parent,
		Children: make([]string, len(group.Children)),
	}

	copy(new.Children, group.Children)
	for ind, c := range new.Children {
		if c == parent {
			new.Children[ind] = group.Parent
		}
	}

	err := database.DB.PushGroup(m.Author.ID, new)
	if err != nil {
		return fmt.Errorf("fatal database error: %v", err)
	}

	eb.SuccessTemplate("Sucessfully copied a cross-post group!").AddField("Name", new.Name).AddField("Parent", new.Parent)
	eb.AddField("Channels", strings.Join(utils.Map(new.Children, func(s string) string {
		return fmt.Sprintf("<#%v>", s)
	}), " "))

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())

	return nil
}
