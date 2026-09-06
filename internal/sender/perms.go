package sender

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// ChannelPermissions returns the bot's permissions in a channel for
// the given session.
func ChannelPermissions(s *discordgo.Session, channelID string) (int64, error) {
	if s == nil || s.State == nil || s.State.User == nil {
		return 0, fmt.Errorf("discord session not ready")
	}

	return s.State.UserChannelPermissions(s.State.User.ID, channelID)
}

// CheckChannelPerms reports whether the session's user holds the given
// permissions in a channel. It fails open: lookup errors return true
// with the error so the caller can attempt the send anyway.
func CheckChannelPerms(s *discordgo.Session, channelID string, permissions int64) (bool, error) {
	perms, err := ChannelPermissions(s, channelID)
	if err != nil {
		return true, err
	}

	return perms&permissions == permissions, nil
}

// CheckGuildPerms reports whether a user holds a guild permission,
// falling back to a live member fetch when the state cache misses and
// granting owners every permission.
func CheckGuildPerms(s *discordgo.Session, guildID string, userID string, permission int64) (bool, error) {
	if s == nil || s.State == nil {
		return false, fmt.Errorf("discord session not ready")
	}

	member, err := s.State.Member(guildID, userID)
	if err != nil {
		if member, err = s.GuildMember(guildID, userID); err != nil {
			return false, err
		}
	}

	if member == nil {
		return false, fmt.Errorf("failed to get member")
	}

	for _, roleID := range member.Roles {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			return false, err
		}

		if role == nil {
			continue
		}

		if role.Permissions&permission != 0 {
			return true, nil
		}
	}

	g, err := s.State.Guild(guildID)
	if err != nil {
		return false, fmt.Errorf("failed to get guild: %w", err)
	}

	if g == nil {
		return false, nil
	}

	if g.OwnerID == userID {
		return true, nil
	}

	return false, nil
}
