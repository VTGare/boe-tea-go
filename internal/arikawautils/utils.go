package arikawautils

import (
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

func UserID(sf string) (discord.UserID, error) {
	snowflake, err := discord.ParseSnowflake(sf)
	if err != nil {
		return 0, err
	}

	return discord.UserID(snowflake), nil
}

func MessageID(sf string) (discord.MessageID, error) {
	snowflake, err := discord.ParseSnowflake(sf)
	if err != nil {
		return 0, err
	}

	return discord.MessageID(snowflake), nil
}

func ChannelID(sf string) (discord.ChannelID, error) {
	snowflake, err := discord.ParseSnowflake(sf)
	if err != nil {
		return 0, err
	}

	return discord.ChannelID(snowflake), nil
}

func GuildID(sf string) (discord.GuildID, error) {
	snowflake, err := discord.ParseSnowflake(sf)
	if err != nil {
		return 0, err
	}

	return discord.GuildID(snowflake), nil
}

func MemberHasPermission(s *state.State, guildID discord.GuildID, userID discord.UserID, permission discord.Permissions) (bool, error) {
	member, err := s.Member(guildID, userID)
	if err != nil {
		return false, err
	}

	for _, roleID := range member.RoleIDs {
		role, err := s.Role(guildID, roleID)
		if err != nil {
			return false, err
		}

		if role.Permissions.Has(permission) {
			return true, nil
		}
	}

	guild, err := s.Guild(guildID)
	if err != nil {
		return false, err
	}

	if member.User.ID == guild.OwnerID {
		return true, nil
	}

	return false, nil
}
