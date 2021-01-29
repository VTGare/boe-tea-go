package commands

import (
	"fmt"
	"strings"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
)

func init() {
	groupName := "crosspost"

	Commands = append(Commands, &gumi.Command{
		Name:        "create",
		Group:       groupName,
		Description: "Creates a crosspost group.",
		Usage:       "bt!create <group name> <parent channel ID>",
		Example:     "bt!create poggers-art-group #general-art",
		Exec:        createGroup,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "delete",
		Group:       groupName,
		Description: "Deletes a crosspost group.",
		Usage:       "bt!delete <group name>",
		Example:     "bt!delete bad-group",
		Exec:        deleteGroup,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "groups",
		Aliases:     []string{"list"},
		Group:       groupName,
		Description: "Lists your crosspost groups.",
		Usage:       "bt!list",
		Example:     "",
		Exec:        groups,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "pop",
		Group:       groupName,
		Description: "Pops (removes) channels from a group.",
		Usage:       "bt!pop <group name> [channel IDs or mentions]",
		Example:     "bt!pop porn #general",
		Exec:        removeFromGroup,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "push",
		Group:       groupName,
		Description: "Pushes (adds) channels to a group.",
		Usage:       "bt!push <group name> [channel IDs or mentions]",
		Example:     "bt!push hololive #hololive-sfw",
		Exec:        addToGroup,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "copy",
		Group:       groupName,
		Description: "Copies a crosspost group.",
		Usage:       "bt!copy <source group name> <new group name> <new parent ID>",
		Example:     "bt!copy hololive-sfw general-sfw #general-art",
		Exec:        copyGroup,
	})
}

func groups(ctx *gumi.Ctx) error {
	var (
		s    = ctx.Session
		m    = ctx.Event
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

func createGroup(ctx *gumi.Ctx) error {
	var (
		s    = ctx.Session
		m    = ctx.Event
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if ctx.Args.Len() < 2 {
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
		groupName = ctx.Args.Get(0).Raw
		ch        = ctx.Args.Get(1).Raw
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

func deleteGroup(ctx *gumi.Ctx) error {
	var (
		s    = ctx.Session
		m    = ctx.Event
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if ctx.Args.Len() < 1 {
		msg := "``bt!delete`` requires at least one argument.\n**Usage:** ``bt!delete ntr``"
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
		return nil
	}
	if user == nil || len(user.ChannelGroups) == 0 {
		eb.FailureTemplate("Failed to delete a cross-post group. You've got no channel groups! Create one by executing `bt!create <group name>`")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	err := database.DB.DeleteGroup(m.Author.ID, ctx.Args.Get(0).Raw)
	if err != nil {
		return fmt.Errorf("fatal database error: %v", err)
	}

	eb.SuccessTemplate("Sucessfully deleted a cross-post group!").AddField("Name", ctx.Args.Get(0).Raw)
	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	return nil
}

func removeFromGroup(ctx *gumi.Ctx) error {
	var (
		s    = ctx.Session
		m    = ctx.Event
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if ctx.Args.Len() < 2 {
		eb.FailureTemplate("``bt!remove`` requires at least two arguments.\n**Usage:** ``bt!remove nudes #nsfw``")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	if user == nil || len(user.ChannelGroups) == 0 {
		eb.FailureTemplate("Failed to remove an item from a cross-post group. You've got no channel groups! Create one by executing `bt!create <group name>`")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	args := strings.Fields(ctx.Args.Raw)
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

func addToGroup(ctx *gumi.Ctx) error {
	var (
		s    = ctx.Session
		m    = ctx.Event
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if ctx.Args.Len() < 2 {
		msg := "``bt!push`` requires at least two arguments.\n**Usage:** ``bt!push hololive #marine-booty``"
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
		return nil
	}

	if user == nil {
		eb.FailureTemplate("Failed to add a channel to a group! You've got no channel groups! Create one by executing `bt!create <group name>`")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	groupName := ctx.Args.Get(0).Raw

	channelsMap := make(map[string]bool)
	for _, arg := range ctx.Args.Arguments[1:] {
		channelsMap[arg.Raw] = true
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

		eb.SuccessTemplate("Successfully added channels to a cross-post group!").AddField("Name", ctx.Args.Get(0).Raw).AddField("Channels", channels)
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	} else {
		eb.FailureTemplate("Failed to add channels to a cross-post group! None of the specified channels were found.").AddField("Group name", ctx.Args.Get(0).Raw)
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	}

	return nil
}

func copyGroup(ctx *gumi.Ctx) error {
	var (
		s    = ctx.Session
		m    = ctx.Event
		user = database.DB.FindUser(m.Author.ID)
		eb   = embeds.NewBuilder()
	)

	if ctx.Args.Len() < 3 {
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
		src      = ctx.Args.Get(0).Raw
		dest     = ctx.Args.Get(1).Raw
		exists   bool
		parent   = strings.Trim(ctx.Args.Get(2).Raw, "<#>")
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
