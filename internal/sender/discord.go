package sender

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VTGare/boe-tea-go/internal/spool"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"go.uber.org/zap"
)

const (
	defaultRetries = 3
	defaultBackoff = 500 * time.Millisecond
)

type DiscordSender struct {
	client *bot.Client
	log    *zap.SugaredLogger

	retries int
	backoff time.Duration
}

func NewDiscordSender(client *bot.Client, log *zap.SugaredLogger) *DiscordSender {
	if log == nil {
		log = zap.NewNop().Sugar()
	}

	return &DiscordSender{
		client:  client,
		log:     log,
		retries: defaultRetries,
		backoff: defaultBackoff,
	}
}

func (d *DiscordSender) SendComplex(channelID snowflake.ID, message discord.MessageCreate) (*discord.Message, error) {
	required := SendPermissions
	if len(message.Files) > 0 {
		required |= discord.PermissionAttachFiles
	}

	if ok, err := d.channelPerms(channelID, required); !ok {
		d.log.With("channel_id", channelID).Warn("skipping send, missing permissions")

		return nil, ErrSkipped
	} else if err != nil {
		d.log.With("error", err).Debug("permission lookup failed, attempting send")
	}

	if len(message.Files) > 0 {
		spool.Acquire()
		defer spool.Release()
		defer spool.RemoveFiles(message.Files)
	}

	var msg *discord.Message
	err := d.retry(func() error {
		rewind(message.Files)

		var err error
		msg, err = d.client.Rest.CreateMessage(channelID, message)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	return msg, nil
}

func (d *DiscordSender) SendEmbed(channelID snowflake.ID, embed discord.Embed) (*discord.Message, error) {
	return d.SendComplex(channelID, discord.MessageCreate{Embeds: []discord.Embed{embed}})
}

func (d *DiscordSender) DeleteMessage(channelID, messageID snowflake.ID) error {
	err := d.retry(func() error {
		return d.client.Rest.DeleteMessage(channelID, messageID)
	})
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

func (d *DiscordSender) AddReaction(channelID, messageID snowflake.ID, emoji string) error {
	err := d.retry(func() error {
		return d.client.Rest.AddReaction(channelID, messageID, emoji)
	})
	if err != nil {
		return fmt.Errorf("failed to add reaction: %w", err)
	}

	return nil
}

func (d *DiscordSender) ChannelGuildID(channelID snowflake.ID) (snowflake.ID, error) {
	if ch, ok := d.client.Caches.Channel(channelID); ok {
		return ch.GuildID(), nil
	}

	var channel discord.Channel
	err := d.retry(func() error {
		var err error
		channel, err = d.client.Rest.GetChannel(channelID)

		return err
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get channel: %w", err)
	}

	if ch, ok := channel.(discord.GuildChannel); ok {
		return ch.GuildID(), nil
	}

	return 0, nil
}

func (d *DiscordSender) IsMember(guildID, userID snowflake.ID) (bool, error) {
	err := d.retry(func() error {
		_, err := d.client.Rest.GetMember(guildID, userID)

		return err
	})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get member: %w", err)
	}

	return true, nil
}

func (d *DiscordSender) HasChannelPerms(guildID, channelID snowflake.ID, permissions discord.Permissions) (bool, error) {
	// DMs have no guild permissions.
	if guildID == 0 {
		return true, nil
	}

	return d.channelPerms(channelID, permissions)
}

func (d *DiscordSender) BotHasGuildPerms(guildID snowflake.ID, permission discord.Permissions) (bool, error) {
	// DMs have no guild permissions.
	if guildID == 0 {
		return false, nil
	}

	member, err := d.self(guildID)
	if err != nil {
		return false, err
	}

	return d.client.Caches.MemberPermissions(member).Has(permission), nil
}

func (d *DiscordSender) Expire(message *discord.Message, after ...time.Duration) {
	if message == nil {
		return
	}

	wait := 15 * time.Second
	if len(after) > 0 {
		wait = after[0]
	}

	go func() {
		time.Sleep(wait)

		if err := d.DeleteMessage(message.ChannelID, message.ID); err != nil {
			d.log.With(
				"channel_id", message.ChannelID,
				"message_id", message.ID,
				"error", err,
			).Warn("failed to expire message")
		}
	}()
}

// channelPerms reports whether the bot has the given permissions in a
// channel. If the lookup fails it returns true along with the error, so
// the caller can still try to send.
func (d *DiscordSender) channelPerms(channelID snowflake.ID, permissions discord.Permissions) (bool, error) {
	ch, ok := d.client.Caches.Channel(channelID)
	if !ok {
		return true, fmt.Errorf("channel %v is not cached", channelID)
	}

	member, ok := d.client.Caches.Member(ch.GuildID(), d.client.ID())
	if !ok {
		return true, fmt.Errorf("bot member in guild %v is not cached", ch.GuildID())
	}

	return d.client.Caches.MemberPermissionsInChannel(ch, member).Has(permissions), nil
}

func (d *DiscordSender) self(guildID snowflake.ID) (discord.Member, error) {
	if member, ok := d.client.Caches.Member(guildID, d.client.ID()); ok {
		return member, nil
	}

	member, err := d.client.Rest.GetMember(guildID, d.client.ID())
	if err != nil {
		return discord.Member{}, fmt.Errorf("failed to get bot member: %w", err)
	}

	return *member, nil
}

// retry repeats Discord's 5xx errors, which often pass on a second try.
// DisGo itself only retries rate limits.
func (d *DiscordSender) retry(do func() error) error {
	backoff := d.backoff

	for attempt := 0; ; attempt++ {
		err := do()
		if err == nil || attempt >= d.retries || !isServerError(err) {
			return err
		}

		time.Sleep(backoff)
		backoff *= 2
	}
}

// A failed attempt may have read files partway, and a retry would upload
// only what's left.
func rewind(files []*discord.File) {
	for _, f := range files {
		if s, ok := f.Reader.(io.Seeker); ok {
			_, _ = s.Seek(0, io.SeekStart)
		}
	}
}

func isNotFound(err error) bool {
	return statusCode(err) == http.StatusNotFound
}

func isServerError(err error) bool {
	return statusCode(err) >= http.StatusInternalServerError
}

// statusCode is the HTTP status of a Discord error, or 0 for other errors.
func statusCode(err error) int {
	var restErr *rest.Error
	if !errors.As(err, &restErr) || restErr.Response == nil {
		return 0
	}

	return restErr.Response.StatusCode
}
