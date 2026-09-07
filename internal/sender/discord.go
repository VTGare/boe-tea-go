package sender

import (
	"fmt"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/internal/spool"
	"github.com/bwmarrin/discordgo"
	"github.com/servusdei2018/shards/v2"
	"go.uber.org/zap"
)

// DiscordSender is the production Sender adapter. It resolves the
// shard session owning each guild, so callers pass guild IDs instead
// of sessions.
type DiscordSender struct {
	manager  *shards.Manager
	fallback *discordgo.Session
	log      *zap.SugaredLogger
}

func NewDiscordSender(manager *shards.Manager, log *zap.SugaredLogger, fallback *discordgo.Session) *DiscordSender {
	if log == nil {
		log = zap.NewNop().Sugar()
	}

	return &DiscordSender{
		manager:  manager,
		fallback: fallback,
		log:      log,
	}
}

func (d *DiscordSender) sessionFor(guildID string) (*discordgo.Session, error) {
	if d.manager != nil {
		id, err := strconv.ParseInt(guildID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse guild id: %w", err)
		}

		if s := d.manager.SessionForGuild(id); s != nil {
			return s, nil
		}
	}

	if d.fallback != nil {
		return d.fallback, nil
	}

	return nil, fmt.Errorf("no session for guild")
}

func (d *DiscordSender) SendComplex(guildID, channelID string, message *discordgo.MessageSend) (*discordgo.Message, error) {
	if message == nil {
		return nil, fmt.Errorf("nil message")
	}

	s, err := d.sessionFor(guildID)
	if err != nil {
		return nil, err
	}

	required := SendPermissions
	if len(message.Files) > 0 {
		required |= discordgo.PermissionAttachFiles
	}

	if ok, err := CheckChannelPerms(s, channelID, required); !ok {
		d.log.With("guild_id", guildID, "channel_id", channelID).Warn("skipping send, missing permissions")

		return nil, ErrSkipped
	} else if err != nil {
		d.log.With("error", err).Debug("permission lookup failed, attempting send")
	}

	if len(message.Files) > 0 {
		spool.Acquire()
		defer spool.Release()
		defer spool.RemoveFiles(message.Files)
	}

	msg, err := s.ChannelMessageSendComplex(channelID, message)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	if msg.GuildID == "" {
		msg.GuildID = guildID
	}

	return msg, nil
}

func (d *DiscordSender) SendEmbed(guildID, channelID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	s, err := d.sessionFor(guildID)
	if err != nil {
		return nil, err
	}

	if ok, err := CheckChannelPerms(s, channelID, SendPermissions); !ok {
		d.log.With("guild_id", guildID, "channel_id", channelID).Warn("skipping send, missing permissions")

		return nil, ErrSkipped
	} else if err != nil {
		d.log.With("error", err).Debug("permission lookup failed, attempting send")
	}

	msg, err := s.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	if msg.GuildID == "" {
		msg.GuildID = guildID
	}

	return msg, nil
}

func (d *DiscordSender) DeleteMessage(guildID, channelID, messageID string) error {
	s, err := d.sessionFor(guildID)
	if err != nil {
		return err
	}

	if err := s.ChannelMessageDelete(channelID, messageID); err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

func (d *DiscordSender) AddReaction(guildID, channelID, messageID, emoji string) error {
	s, err := d.sessionFor(guildID)
	if err != nil {
		return err
	}

	if err := s.MessageReactionAdd(channelID, messageID, emoji); err != nil {
		return fmt.Errorf("failed to add reaction: %w", err)
	}

	return nil
}

func (d *DiscordSender) EditEmbed(guildID, channelID, messageID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	s, err := d.sessionFor(guildID)
	if err != nil {
		return nil, err
	}

	msg, err := s.ChannelMessageEditEmbed(channelID, messageID, embed)
	if err != nil {
		return nil, fmt.Errorf("failed to edit message: %w", err)
	}

	if msg.GuildID == "" {
		msg.GuildID = guildID
	}

	return msg, nil
}

func (d *DiscordSender) RemoveReaction(guildID, channelID, messageID, emoji, userID string) error {
	s, err := d.sessionFor(guildID)
	if err != nil {
		return err
	}

	if err := s.MessageReactionRemove(channelID, messageID, emoji, userID); err != nil {
		return fmt.Errorf("failed to remove reaction: %w", err)
	}

	return nil
}

func (d *DiscordSender) RemoveAllReactions(guildID, channelID, messageID string) error {
	s, err := d.sessionFor(guildID)
	if err != nil {
		return err
	}

	if err := s.MessageReactionsRemoveAll(channelID, messageID); err != nil {
		return fmt.Errorf("failed to remove reactions: %w", err)
	}

	return nil
}

func (d *DiscordSender) ChannelGuildID(hintGuildID, channelID string) (string, error) {
	s, err := d.sessionFor(hintGuildID)
	if err != nil {
		return "", err
	}

	channel, err := s.Channel(channelID)
	if err != nil {
		return "", fmt.Errorf("failed to get channel: %w", err)
	}

	return channel.GuildID, nil
}

func (d *DiscordSender) IsMember(guildID, userID string) (bool, error) {
	s, err := d.sessionFor(guildID)
	if err != nil {
		return false, err
	}

	if _, err := s.GuildMember(guildID, userID); err != nil {
		if isNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get member: %w", err)
	}

	return true, nil
}

func (d *DiscordSender) HasChannelPerms(guildID, channelID string, permissions int64) (bool, error) {
	s, err := d.sessionFor(guildID)
	if err != nil {
		return true, err
	}

	return CheckChannelPerms(s, channelID, permissions)
}

func (d *DiscordSender) BotHasGuildPerms(guildID string, permission int64) (bool, error) {
	s, err := d.sessionFor(guildID)
	if err != nil {
		return false, err
	}

	if s.State == nil || s.State.User == nil {
		return false, fmt.Errorf("discord session not ready")
	}

	return CheckGuildPerms(s, guildID, s.State.User.ID, permission)
}

func (d *DiscordSender) Expire(message *discordgo.Message, after ...time.Duration) {
	if message == nil {
		return
	}

	s, err := d.sessionFor(message.GuildID)
	if err != nil {
		d.log.With("error", err).Warn("failed to resolve session to expire message")

		return
	}

	ExpireMessage(d.log, s, message, after...)
}
