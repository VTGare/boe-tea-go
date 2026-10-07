package commands

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi/v2"
	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// artChannelTypes are channels Boe Tea can post artwork in.
var artChannelTypes = []discord.ChannelType{discord.ChannelTypeGuildText, discord.ChannelTypeGuildNews}

// channelsPerPage is how many channels one /channels list page shows.
const channelsPerPage = 20

func channelsCommand(b *bot.Bot) *gumi.Command {
	manage := []gumi.Check{gumi.HasPermissions(discord.PermissionManageGuild)}
	targets := gumi.String("channels", "Channel mentions or IDs; a category adds all its channels").Require().Greedy()

	return &gumi.Command{
		Name:        "channels",
		Category:    "Settings",
		Aliases:     []string{"artchannels", "artchannel", "ac"},
		Description: "Lists or changes where Boe Tea posts artwork.",
		Checks:      []gumi.Check{gumi.GuildOnly},
		Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
		Subcommands: []*gumi.Command{
			{
				Name:        "list",
				Description: "Shows art channels.",
				Examples:    []string{"channels list"},
				Handler:     listChannels(b),
			},
			{
				Name:        "add",
				Description: "Adds channels or whole categories.",
				Checks:      manage,
				Options:     []*gumi.Option{targets},
				Examples:    []string{"channels add #sfw #nsfw"},
				Handler:     changeChannels(b, true),
			},
			{
				Name:        "remove",
				Aliases:     []string{"rm"},
				Description: "Removes channels or whole categories.",
				Checks:      manage,
				Options:     []*gumi.Option{targets},
				Examples:    []string{"channels remove #sfw"},
				Handler:     changeChannels(b, false),
			},
		},
	}
}

// channel holds what art channel commands need, with IDs as the store
// keeps them.
type channel struct {
	ID       string
	Type     discord.ChannelType
	ParentID string
}

// guildChannels returns a guild's channels, from the cache if it can.
func guildChannels(c *disgobot.Client, guildID snowflake.ID) ([]channel, error) {
	var found []discord.GuildChannel
	for ch := range c.Caches.ChannelsForGuild(guildID) {
		found = append(found, ch)
	}

	if len(found) == 0 {
		var err error
		if found, err = c.Rest.GetGuildChannels(guildID); err != nil {
			return nil, err
		}
	}

	channels := make([]channel, 0, len(found))
	for _, ch := range found {
		parentID := ""
		if id := ch.ParentID(); id != nil {
			parentID = id.String()
		}

		channels = append(channels, channel{ID: ch.ID().String(), Type: ch.Type(), ParentID: parentID})
	}

	return channels, nil
}

func channelByID(c *disgobot.Client, id string) (discord.Channel, error) {
	channelID, err := snowflake.Parse(id)
	if err != nil {
		return nil, err
	}

	if ch, ok := c.Caches.Channel(channelID); ok {
		return ch, nil
	}

	return c.Rest.GetChannel(channelID)
}

// pickedChannels looks up IDs among the guild's channels. Unknown IDs
// (deleted channels) become placeholders so they can still be removed.
func pickedChannels(all []channel, ids []string) []channel {
	picked := make([]channel, 0, len(ids))
	for _, id := range ids {
		idx := slices.IndexFunc(all, func(c channel) bool { return c.ID == id })
		if idx >= 0 {
			picked = append(picked, all[idx])
		} else {
			picked = append(picked, channel{ID: id, Type: discord.ChannelTypeGuildText})
		}
	}

	return picked
}

// expandArtChannels turns picked channels into the IDs to store: a
// category becomes its text channels, other channel types are dropped,
// and duplicates are skipped.
func expandArtChannels(all, picked []channel) []string {
	ids := make([]string, 0, len(picked))
	add := func(c channel) {
		if slices.Contains(artChannelTypes, c.Type) && !slices.Contains(ids, c.ID) {
			ids = append(ids, c.ID)
		}
	}

	for _, p := range picked {
		if p.Type != discord.ChannelTypeGuildCategory {
			add(p)
			continue
		}

		for _, c := range all {
			if c.ParentID == p.ID {
				add(c)
			}
		}
	}

	return ids
}

func changeChannels(b *bot.Bot, add bool) gumi.Handler {
	return func(ctx *gumi.Context) error {
		reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
		defer cancel()

		guild, _, err := store.GetOrCreateGuild(reqCtx, b.Store, ctx.GuildID().String())
		if err != nil {
			return err
		}

		all, err := guildChannels(ctx.Client, ctx.GuildID())
		if err != nil {
			return err
		}

		raw := strings.Fields(ctx.Options.String("channels"))
		ids := make([]string, 0, len(raw))
		for _, arg := range raw {
			id := dgoutils.TrimmerRaw(arg)
			known := slices.ContainsFunc(all, func(c channel) bool { return c.ID == id })

			// Removing a deleted channel by ID is fine; adding one is not.
			if !known && (add || !slices.Contains(guild.ArtChannels, id)) {
				return gumi.Errorf("`%s` isn't a channel in this server.", arg)
			}

			ids = append(ids, id)
		}

		targets := expandArtChannels(all, pickedChannels(all, ids))
		if len(targets) == 0 {
			return gumi.Errorf("Those aren't text channels or categories with text channels.")
		}

		var changed, skipped []string
		for _, id := range targets {
			if slices.Contains(guild.ArtChannels, id) == add {
				skipped = append(skipped, id)
			} else {
				changed = append(changed, id)
			}
		}

		if add {
			_, err = b.Store.AddArtChannels(reqCtx, guild.ID, changed)
		} else {
			_, err = b.Store.DeleteArtChannels(reqCtx, guild.ID, changed)
		}
		if err != nil {
			return err
		}

		return ctx.ReplyEmbed(channelsChangedEmbed(add, changed, skipped))
	}
}

func channelsChangedEmbed(add bool, changed, skipped []string) discord.Embed {
	verb, already := "Added", "already art channels"
	if !add {
		verb, already = "Removed", "weren't art channels"
	}

	desc := fmt.Sprintf("**%s %d art %s**", verb, len(changed), plural(len(changed), "channel"))
	if len(changed) > 0 {
		desc += "\n" + mentions(changed)
	}
	if len(skipped) > 0 {
		desc += fmt.Sprintf("\n-# %s %s", mentions(skipped), already)
	}

	return discord.Embed{Color: successColor, Description: desc}
}

func listChannels(b *bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
		defer cancel()

		guild, _, err := store.GetOrCreateGuild(reqCtx, b.Store, ctx.GuildID().String())
		if err != nil {
			return err
		}

		all, err := guildChannels(ctx.Client, ctx.GuildID())
		if err != nil {
			return err
		}

		// Channels deleted while the bot was offline never sent a delete event,
		// so clean them up here.
		deleted := slices.DeleteFunc(slices.Clone(guild.ArtChannels), func(id string) bool {
			return slices.ContainsFunc(all, func(c channel) bool { return c.ID == id })
		})
		if len(deleted) > 0 {
			if guild, err = b.Store.DeleteArtChannels(reqCtx, guild.ID, deleted); err != nil {
				return err
			}
		}

		return replyPages(ctx, b, channelPages(guild.ArtChannels, len(deleted)))
	}
}

func channelPages(ids []string, cleaned int) []discord.Embed {
	title := fmt.Sprintf("Art channels · %d", len(ids))
	footer := ""
	if cleaned > 0 {
		footer = fmt.Sprintf("Removed %d deleted %s", cleaned, plural(cleaned, "channel"))
	}

	if len(ids) == 0 {
		embed := discord.Embed{
			Title:       title,
			Color:       0x439ef1,
			Description: "None: Boe Tea posts artwork in every channel.\n-# Add some with `/channels add`",
		}
		if footer != "" {
			embed.Footer = &discord.EmbedFooter{Text: footer}
		}

		return []discord.Embed{embed}
	}

	pages := make([]discord.Embed, 0, len(ids)/channelsPerPage+1)
	for start := 0; start < len(ids); start += channelsPerPage {
		page := ids[start:min(start+channelsPerPage, len(ids))]

		lines := make([]string, 0, len(page))
		for _, id := range page {
			lines = append(lines, "<#"+id+">")
		}

		text := fmt.Sprintf("Page %d of %d", start/channelsPerPage+1, (len(ids)+channelsPerPage-1)/channelsPerPage)
		if footer != "" {
			text += " · " + footer
		}

		pages = append(pages, discord.Embed{
			Title:       title,
			Color:       0x439ef1,
			Description: "Boe Tea only posts artwork in these channels.\n\n" + strings.Join(lines, "\n"),
			Footer:      &discord.EmbedFooter{Text: text},
		})
	}

	return pages
}

func mentions(ids []string) string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, "<#"+id+">")
	}
	return strings.Join(out, " ")
}

func plural(n int, noun string) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}
