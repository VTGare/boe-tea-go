// Package sender does all the Discord writes for posting artwork, so the
// pipeline can be tested without Discord.
package sender

import (
	"errors"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// ErrSkipped means a send was skipped because the bot lacks channel
// permissions. It isn't a failure: drop the message and move on.
var ErrSkipped = errors.New("sender: skipped, missing permissions")

// SendPermissions is the permission set for posting artwork.
const SendPermissions = discord.PermissionSendMessages | discord.PermissionEmbedLinks

type Sender interface {
	// SendComplex sends a message, skipping with ErrSkipped when the
	// bot lacks the required permissions.
	SendComplex(channelID snowflake.ID, message discord.MessageCreate) (*discord.Message, error)

	// SendEmbed sends an embed, skipping with ErrSkipped when the bot
	// lacks the required permissions.
	SendEmbed(channelID snowflake.ID, embed discord.Embed) (*discord.Message, error)

	// DeleteMessage deletes a message, e.g. a repost in strict mode.
	DeleteMessage(channelID, messageID snowflake.ID) error

	// AddReaction adds a bookmark reaction to a message.
	AddReaction(channelID, messageID snowflake.ID, emoji string) error

	// ChannelGuildID resolves which guild a channel belongs to.
	ChannelGuildID(channelID snowflake.ID) (snowflake.ID, error)

	// IsMember reports whether a user is in a guild.
	IsMember(guildID, userID snowflake.ID) (bool, error)

	// HasChannelPerms reports whether the bot holds permissions in a
	// channel.
	HasChannelPerms(guildID, channelID snowflake.ID, permissions discord.Permissions) (bool, error)

	// BotHasGuildPerms reports whether the bot itself holds a guild
	// permission, used before deleting reposted messages.
	BotHasGuildPerms(guildID snowflake.ID, permission discord.Permissions) (bool, error)

	// Expire schedules a message for deletion after the given
	// duration, defaulting to 15 seconds.
	Expire(message *discord.Message, after ...time.Duration)
}
