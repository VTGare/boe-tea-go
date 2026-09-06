package sender

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

// ExpireMessage deletes a message after the given duration, defaulting
// to 15 seconds. It never blocks: the delete runs in the background.
func ExpireMessage(log *zap.SugaredLogger, s *discordgo.Session, msg *discordgo.Message, after ...time.Duration) {
	expireDuration := time.Second * 15
	if len(after) > 0 {
		expireDuration = after[0]
	}

	if msg == nil || s == nil {
		return
	}

	go func() {
		time.Sleep(expireDuration)
		if s == nil {
			return
		}

		if err := s.ChannelMessageDelete(msg.ChannelID, msg.ID); err != nil {
			log.With(
				"guild_id", msg.GuildID,
				"channel_id", msg.ChannelID,
				"message_id", msg.ID,
				"error", err,
			).Warn("failed to expire message")
		}
	}()
}
