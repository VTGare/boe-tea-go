// Package sender owns every Discord write the artwork pipeline performs.
package sender

import (
	"errors"
	"time"

	"github.com/bwmarrin/discordgo"
)

// ErrSkipped reports that a send was skipped because the bot lacks the
// required channel permissions. It is not a failure: callers should
// drop the message and continue with the next one.
var ErrSkipped = errors.New("sender: skipped, missing permissions")

// SendPermissions is the permission set for posting artwork.
const SendPermissions int64 = discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks

type Sender interface {
	// SendComplex sends a message, skipping with ErrSkipped when the
	// bot lacks the required permissions.
	SendComplex(guildID, channelID string, message *discordgo.MessageSend) (*discordgo.Message, error)

	// SendEmbed sends an embed, skipping with ErrSkipped when the bot
	// lacks the required permissions.
	SendEmbed(guildID, channelID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error)

	// DeleteMessage deletes a message, used by the strict repost path.
	DeleteMessage(guildID, channelID, messageID string) error

	// AddReaction adds a bookmark reaction to a message.
	AddReaction(guildID, channelID, messageID, emoji string) error

	// EditEmbed replaces a message's embeds, used by the reaction
	// pagination widget.
	EditEmbed(guildID, channelID, messageID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error)

	// RemoveReaction removes one user's reaction, used by the reaction
	// pagination widget.
	RemoveReaction(guildID, channelID, messageID, emoji, userID string) error

	// RemoveAllReactions clears a message's reactions, used by the
	// reaction pagination widget's stop control.
	RemoveAllReactions(guildID, channelID, messageID string) error

	// ChannelGuildID resolves which guild a channel belongs to.
	ChannelGuildID(hintGuildID, channelID string) (string, error)

	// IsMember reports whether a user is in a guild.
	IsMember(guildID, userID string) (bool, error)

	// HasChannelPerms reports whether the bot holds permissions in a
	// channel.
	HasChannelPerms(guildID, channelID string, permissions int64) (bool, error)

	// BotHasGuildPerms reports whether the bot itself holds a guild
	// permission, used before deleting reposted messages.
	BotHasGuildPerms(guildID string, permission int64) (bool, error)

	// Expire schedules a message for deletion after the given
	// duration, defaulting to 15 seconds.
	Expire(message *discordgo.Message, after ...time.Duration)
}
