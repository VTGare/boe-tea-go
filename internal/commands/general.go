package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
)

func init() {
	groupName := "general"
	Commands = append(Commands, &gumi.Command{
		Name:        "ping",
		Group:       groupName,
		Description: "Ping pong! Checks if bot is online.",
		Usage:       "bt!ping",
		Example:     "Do you really need an example?",
		Exec:        ping,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "about",
		Group:       groupName,
		Aliases:     []string{"invite", "support"},
		Description: "About page. Invite link, support server link, patreon link and special thanks :)",
		Exec:        about,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "feedback",
		Group:       groupName,
		Description: "Reach out to bot's author!",
		Usage:       "bt!feedback <message>",
		Example:     "bt!feedback Haha epic.",
		Exec:        feedback,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "set",
		Group:       groupName,
		Aliases:     []string{"config", "cfg", "settings"},
		Description: "Show or change server's settings",
		GuildOnly:   true,
		Usage:       "bt!set <setting name> <new setting>",
		Flags: map[string]string{
			"Setting name":    "Optional. If no arguments given command shows current settings.",
			"New setting":     "Required if setting name is given.",
			"prefix":          "Bot's prefix. Up to __5 characters__. Whitespace will be appended if last character is a letter.",
			"limit":           "Album size limit. Only fist image will be posted if exceeded. Has to be an integer.",
			"pixiv | twitter": "Pixiv or Twitter reposting switch, valid parameters: ***[enabled, on, t, true], [disabled, off, f, false]***",
			"repost ":         "Repost check setting, valid parameters: ***[enabled, disabled, strict]***. Strict mode disables a prompt and removes reposts on sight.",
		},
		Example:     "bt!set limit 420",
		Permissions: discordgo.PermissionAdministrator | discordgo.PermissionManageServer,
		Exec:        set,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "addchannel",
		Group:       groupName,
		GuildOnly:   true,
		Description: "Adds an art channel.",
		Usage:       "bt!addchannel [channel/category IDs or mentions]",
		Example:     "bt!addchannel #lolis",
		Permissions: discordgo.PermissionAdministrator | discordgo.PermissionManageServer,
		Exec:        addArtChannel,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "removechannel",
		Group:       groupName,
		GuildOnly:   true,
		Description: "Removes an art channel.",
		Usage:       "bt!removechannel [channel/category IDs or mentions]",
		Example:     "bt!removechannel #general",
		Permissions: discordgo.PermissionAdministrator | discordgo.PermissionManageServer,
		Exec:        removeArtChannel,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "help",
		Group:       groupName,
		Description: "Sends this message",
		Usage:       "bt!help",
		Example:     "",
		Exec:        help,
	})
}

func ping(ctx *gumi.Ctx) error {
	eb := embeds.NewBuilder()

	return ctx.ReplyEmbed(eb.Title("🏓 Pong!").AddField("Heartbeat latency", ctx.Session.HeartbeatLatency().Round(1*time.Millisecond).String()).Finalize())
}

func about(ctx *gumi.Ctx) error {
	eb := embeds.NewBuilder()
	eb.Title("ℹ About").Thumbnail(ctx.Session.State.User.AvatarURL(""))
	eb.Description(
		`Boe Tea is a Swiss Army Knife of art sharing and moderation on Discord.
If you want to copy an invite link, simply right click it and press Copy Link.

***Special thanks to our patron(s):***
- Nom (Indy#4649) | 4 months
`)
	eb.AddField("Support server", "[Click here desu~](https://discord.gg/hcxuHE7)", true)
	eb.AddField("Invite link", "[Click here desu~](https://discord.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537259072&scope=bot)", true)
	eb.AddField("Patreon", "[Click here desu~](https://patreon.com/vtgare)", true)

	ctx.ReplyEmbed(eb.Finalize())
	return nil
}

func feedback(ctx *gumi.Ctx) error {
	if ctx.Args.Len() == 0 {
		return utils.ErrNotEnoughArguments
	}

	eb := embeds.NewBuilder()
	eb.Title(
		fmt.Sprintf("Feedback from %v (%v)", ctx.Event.Author.String(), ctx.Event.Author.ID),
	).Thumbnail(
		ctx.Event.Author.AvatarURL(""),
	).Description(ctx.Args.Raw)

	if len(ctx.Event.Attachments) >= 1 {
		eb.Image(ctx.Event.Attachments[0].URL)
	}

	ch, err := ctx.Session.UserChannelCreate(utils.AuthorID)
	if err != nil {
		return err
	}

	_, err = ctx.Session.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
	if err != nil {
		return err
	}

	eb.Clear()
	ctx.ReplyEmbed(eb.SuccessTemplate("Feedback message has been sent.").Finalize())
	return nil
}

func help(ctx *gumi.Ctx) error {
	var (
		groups = make(map[string][]*gumi.Command, 0)
		eb     = embeds.NewBuilder().Thumbnail(ctx.Session.State.User.AvatarURL(""))
	)

	for _, cmd := range ctx.Router.Commands {
		if _, ok := groups[cmd.Group]; !ok {
			groups[cmd.Group] = make([]*gumi.Command, 0)
			groups[cmd.Group] = append(groups[cmd.Group], cmd)
		} else {
			groups[cmd.Group] = append(groups[cmd.Group], cmd)
		}
	}

	if ctx.Args.Len() == 0 {
		eb.Title("Boe Tea's Documentation")

		sb := strings.Builder{}
		sb.WriteString("List of groups. Run `bt!help <group name>`  to get a list of commands in a group.\n\n")
		for group := range groups {
			sb.WriteString(fmt.Sprintf("**•** __*%v*__\n", group))
		}

		ctx.ReplyEmbed(eb.Description(sb.String()).Finalize())
		return nil
	}

	if ctx.Args.Len() == 1 {
		arg := ctx.Args.Get(0).Raw
		if !ctx.Router.CaseSensitive {
			arg = strings.ToLower(arg)
		}

		if group, ok := groups[arg]; ok {
			eb.Title("Boe Tea's Documentation: Group **" + arg + "**")
			eb.Description("Run `bt!help <command name>` to get details about a specific command.")

			addedNames := make(map[string]bool)
			for _, cmd := range group {
				if _, ok := addedNames[cmd.Name]; !ok {
					addedNames[cmd.Name] = true
					eb.AddField(cmd.Name, cmd.Description, true)
				}
			}

			ctx.ReplyEmbed(eb.Finalize())
		} else if command, ok := ctx.Router.Commands[arg]; ok {
			eb.Title("Boe Tea's Documentation: Command **" + command.Name + "**")
			eb.Description(command.Description)

			if command.Usage != "" {
				eb.AddField("Usage", fmt.Sprintf("```\n%v\n```", command.Usage), true)
			}

			if command.Example != "" {
				eb.AddField("Example", fmt.Sprintf("```\n%v\n```", command.Example), true)
			}

			if len(command.Aliases) != 0 {
				eb.AddField("Aliases", fmt.Sprintf("```\n%v\n```", command.Aliases), true)
			}

			if len(command.Flags) != 0 {
				for name, value := range command.Flags {
					eb.AddField(name, value)
				}
			}

			ctx.ReplyEmbed(eb.Finalize())
		} else {
			eb.Clear()
			eb.FailureTemplate(fmt.Sprintf("Couldn't find a command or a group named `%v`", arg))
			ctx.ReplyEmbed(eb.Finalize())
		}
	}

	return nil
}
