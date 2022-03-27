package commands

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arikawautils"
	"github.com/VTGare/boe-tea-go/internal/arikawautils/embeds"
	"github.com/VTGare/boe-tea-go/internal/slices"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/responses"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/session/shard"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"go.mongodb.org/mongo-driver/mongo"
)

type setError struct {
	message string
}

func (se *setError) Error() string {
	return se.message
}

// Set command errors
var (
	errPrefixTooLong = &setError{"Prefix is too long, maximum 5 characters allowed"}
)

func ping(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	eb := embeds.NewBuilder().Title("Ping!")

	start := time.Now()
	s.EditInteractionResponse(ie.AppID, ie.Token, api.EditInteractionResponseData{
		Embeds: &[]discord.Embed{eb.Build()},
	})

	dur := time.Since(start).Round(time.Millisecond) / 2
	eb.Title("🏓 Pong!").AddField("Response time", dur.String())

	return responses.FromEmbed(eb.Build()), nil
}

func showSettings(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	resp := api.InteractionResponse{}

	guild, err := s.Guild(ie.GuildID)
	if err != nil {
		return resp, err
	}

	settings, err := b.Store.Guild(ctx, ie.GuildID.String())
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// TODO: guild not found response
			return resp, nil
		}

		return resp, err
	}

	eb := embeds.NewBuilder()
	eb.Title("Server Settings").Description("**"+guild.Name+"**").
		Thumbnail(guild.IconURL()).
		Footer("Ebin message.", "")

	eb.AddField(
		"General",
		fmt.Sprintf(
			"**%v**: %v | **%v**: %v",
			"Prefix", settings.Prefix,
			"NSFW", messages.FormatBool(settings.NSFW),
		),
	)

	eb.AddField(
		"Features",
		fmt.Sprintf(
			"**%v**: %v | **%v**: %v\n**%v**: %v | **%v**: %v\n**%v**: %v | **%v**: %v",
			"Repost", settings.Repost,
			"Expiration (repost.expiration)", settings.RepostExpiration,
			"Crosspost", messages.FormatBool(settings.Crosspost),
			"Reactions", messages.FormatBool(settings.Reactions),
			"Tags", messages.FormatBool(settings.Tags),
			"Footer", messages.FormatBool(settings.FlavourText),
		),
	)

	eb.AddField(
		"Pixiv Settings",
		fmt.Sprintf(
			"**%v**: %v | **%v**: %v",
			"Status __(pixiv)__", messages.FormatBool(settings.Pixiv),
			"Limit", strconv.Itoa(settings.Limit),
		),
	)

	eb.AddField(
		"Twitter Settings",
		fmt.Sprintf(
			"**%v**: %v",
			"Status __(twitter)__", messages.FormatBool(settings.Twitter),
		),
	)

	eb.AddField(
		"DeviantArt Settings",
		fmt.Sprintf(
			"**%v**: %v",
			"Status __(deviant)__", messages.FormatBool(settings.Deviant),
		),
	)

	eb.AddField(
		"ArtStation Settings",
		fmt.Sprintf(
			"**%v**: %v",
			"Status __(artstation)__", messages.FormatBool(settings.Artstation),
		),
	)

	eb.AddField(
		"Art Channels",
		"Use `artchannels` command to list or manage art channels!",
	)

	return responses.FromEmbed(eb.Build()), nil
}

func changeSetting(fn func(*store.Guild, discord.CommandInteractionOptions) (interface{}, error)) ExecFunc {
	return func(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
		var (
			resp  = api.InteractionResponse{}
			perms = discord.PermissionAdministrator | discord.PermissionManageGuild
		)

		ok, err := arikawautils.MemberHasPermission(s, ie.GuildID, ie.SenderID(), perms)
		if err != nil {
			return resp, err
		}

		if !ok {
			return resp, nil
		}

		g, err := b.Store.Guild(ctx, ie.GuildID.String())
		if err != nil {
			return resp, err
		}

		ci := ie.Data.(*discord.CommandInteraction)
		subcommand := ci.Options[0]
		setting := subcommand.Options[0]

		var new interface{}
		if err := setting.Value.UnmarshalTo(&new); err != nil {
			return resp, err
		}

		old, err := fn(g, subcommand.Options)
		if err != nil {
			return resp, err
		}

		se := &setError{}
		if errors.As(err, &se) {
			return responses.FromEmbed(embeds.NewFail(se.message).Build()), nil
		}

		if _, err := b.Store.UpdateGuild(ctx, g); err != nil {
			return resp, err
		}

		eb := embeds.NewInfo("Successfully changed setting.").
			AddField("Setting name", subcommand.Name, true).
			AddField("Old setting", fmt.Sprintf("%v", old), true).
			AddField("New setting", fmt.Sprintf("%v", new), true)

		return responses.FromEmbed(eb.Build()), nil
	}
}

func about(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	eb := embeds.NewBuilder()
	eb.Title("ℹ About").
		Description("Boe Tea is a bot that makes artwork sharing headache-free.\n" +
			"Click on bot's profile picture, then 'Add to Server' to invite it to your server.")

	rb := responses.Builder{}
	rb.AddEmbed(eb.Build())
	rb.AddActionRow(
		responses.LinkButton("Support server", "https://discord.gg/hcxuHE7"),
		responses.LinkButton("Patreon", "https://patreon.com/vtgare"),
	)

	return rb.Build(), nil
}

func runtimeStats(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	eb := embeds.NewBuilder()

	var (
		artworksSent int64
		commandsSent int64
		lenGuilds    int
		lenChannels  int
	)

	for _, provider := range b.ArtworkProviders {
		artworksSent += provider.Hits()
	}

	for _, hits := range Hits {
		commandsSent += hits.Load()
	}

	b.ShardManager.ForEach(func(shard shard.Shard) {
		s := shard.(*state.State)
		guilds, _ := s.Cabinet.Guilds()
		lenGuilds = len(guilds)

		for _, guild := range guilds {
			channels, _ := s.Channels(guild.ID)
			lenChannels += len(channels)
		}
	})

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	eb.Title("Runtime stats").
		AddField("Uptime", time.Since(b.StartupTime).Round(1*time.Second).String(), true).
		AddField("Artworks sent", strconv.FormatInt(artworksSent, 10), true).
		AddField("Commands sent", strconv.FormatInt(commandsSent, 10), true).
		AddField("Servers", strconv.Itoa(lenGuilds), true).
		AddField("Channels", strconv.Itoa(lenChannels), true).
		AddField("Shards", strconv.Itoa(b.ShardManager.NumShards()), true).
		AddField("RAM", fmt.Sprintf("%v MB", mem.Alloc/1024/1024), true)

	return responses.FromEmbed(eb.Build()), nil
}

func artworkStats(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	eb := embeds.NewBuilder()
	eb.Title("Artwork stats")

	for _, provider := range b.ArtworkProviders {
		t := reflect.TypeOf(provider).String()
		t = strings.Split(t, ".")[1]
		eb.AddField(t, strconv.FormatInt(provider.Hits(), 10))
	}

	return responses.FromEmbed(eb.Build()), nil
}

func commandStats(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	eb := embeds.NewBuilder()
	eb.Title("Command stats")

	stats := make([]struct {
		name string
		hits int64
	}, 0)

	for command, hits := range Hits {
		stats = append(stats, struct {
			name string
			hits int64
		}{command, hits.Load()})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].hits > stats[j].hits
	})

	for _, stat := range stats {
		eb.AddField(stat.name, strconv.FormatInt(stat.hits, 10), true)
	}

	return responses.FromEmbed(eb.Build()), nil
}

func listArtChannels(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	var (
		eb   = embeds.NewBuilder()
		resp api.InteractionResponse
	)

	settings, err := b.Store.Guild(ctx, ie.GuildID.String())
	if err != nil {
		return resp, fmt.Errorf("failed to get guild: %w", err)
	}

	guild, err := s.Guild(ie.GuildID)
	if err != nil {
		return resp, fmt.Errorf("failed to get guild: %w", err)
	}

	eb.Title("Art channels list").Thumbnail(guild.IconURL())
	if len(settings.ArtChannels) == 0 {
		eb.Description("No art channels. Please use `/artchannels add` command to add some.")
		return responses.FromEmbed(eb.Build()), nil
	}

	eb.Footer("Total: "+strconv.Itoa(len(settings.ArtChannels)), "")
	eb.Description(messages.ListChannels(settings.ArtChannels))
	return responses.FromEmbed(eb.Build()), nil
}

func addArtChannels(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	var (
		resp api.InteractionResponse
	)

	ok, err := arikawautils.MemberHasPermission(s, ie.GuildID, ie.SenderID(), discord.PermissionAdministrator|discord.PermissionManageGuild)
	if err != nil {
		return resp, err
	}

	if !ok {
		return responses.InsufficientPermissions, nil
	}

	var (
		ci       = ie.Data.(*discord.CommandInteraction)
		channels = make([]string, 0)
	)

	guild, err := b.Store.Guild(ctx, ie.GuildID.String())
	if err != nil {
		return resp, err
	}

	for channelID := range ci.Resolved.Channels {
		channelID := channelID.String()
		if slices.Any(guild.ArtChannels, channelID) {
			channels = append(channels, channelID)
		}
	}

	if _, err := b.Store.AddArtChannels(ctx, ie.GuildID.String(), channels); err != nil {
		return resp, err
	}

	eb := embeds.NewSuccess("Successfully added art channels").
		AddField("New channels", messages.ListChannels(channels))

	return responses.FromEmbed(eb.Build()), nil
}

func removeArtChannels(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	var (
		resp api.InteractionResponse
	)

	ok, err := arikawautils.MemberHasPermission(s, ie.GuildID, ie.SenderID(), discord.PermissionAdministrator|discord.PermissionManageGuild)
	if err != nil {
		return resp, err
	}

	if !ok {
		return responses.InsufficientPermissions, nil
	}

	ci := ie.Data.(*discord.CommandInteraction)

	channels := make([]string, 0)
	for channelID := range ci.Resolved.Channels {
		channels = append(channels, channelID.String())
	}

	if _, err := b.Store.DeleteArtChannels(ctx, ie.GuildID.String(), channels); err != nil {
		return resp, err
	}

	eb := embeds.NewSuccess("Successfully removed art channels").
		AddField("New channels", strings.Join(channels, " "))

	return responses.FromEmbed(eb.Build()), nil
}

func feedback(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	rb := responses.Builder{}
	rb.AddTextInput("Boe Tea feedback", "feedback", &discord.TextInputComponent{
		CustomID:    "feedback",
		Label:       "Your feedback",
		Style:       discord.TextInputParagraphStyle,
		Required:    true,
		Placeholder: option.NewNullableString("Your bot sucks!"),
	})

	return rb.Build(api.ModalResponse), nil
}

func reply(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	var (
		rb = responses.Builder{}
		ci = ie.Data.(*discord.CommandInteraction)
	)

	sf, err := ci.Options[0].SnowflakeValue()
	if err != nil {
		return api.InteractionResponse{}, nil
	}

	rb.AddTextInput("Boe Tea feedback reply", "reply", &discord.TextInputComponent{
		CustomID:    discord.ComponentID("reply:" + sf.String()),
		Label:       "Reply",
		Style:       discord.TextInputParagraphStyle,
		Required:    true,
		Placeholder: option.NewNullableString("Your bot sucks!"),
	})

	return rb.Build(api.ModalResponse), nil
}
