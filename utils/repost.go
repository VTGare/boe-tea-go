package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

func errRepostDetection(err error) error {
	return fmt.Errorf("Repost detection mechanism has failed. Please report this error to a dev and disable repost detection if problem remains.\n%v", err)
}

//IsRepost checks if something has been cached in repost cache
func IsRepost(channelID, post string) (*database.ImagePost, error) {
	rep, err := database.IsRepost(channelID, post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, errRepostDetection(err)
	}

	return rep, nil
}

//NewRepostDetection caches post info per channel.
func NewRepostDetection(author, guildID, channelID, messageID, post string) error {
	err := database.InsertOnePost(database.NewImagePost(author, guildID, channelID, messageID, post))
	if err != nil {
		return errRepostDetection(err)
	}
	return nil
}

func RemoveReposts(s *discordgo.Session, m *discordgo.MessageCreate) []*database.ImagePost {
	var (
		ips         = make([]*database.ImagePost, 0)
		totalCount  = 0
		repostCount = 0
	)

	pixiv := PixivRegex.FindAllStringSubmatch(m.Content, len(m.Content)+1)
	if pixiv != nil {
		ids := make([]string, 0)
		for _, match := range pixiv {
			ids = append(ids, match[1])
		}
		totalCount += len(ids)

		for _, id := range ids {
			ip, err := database.IsRepost(m.ChannelID, id)
			if err != nil {
				log.Warnln(err)
			}
			if ip != nil {
				repostCount++
				ips = append(ips, ip)
				m.Content = strings.ReplaceAll(m.Content, id, "")
			}
		}
	}

	if tweets := services.TwitterRegex.FindAllString(m.Content, len(m.Content)+1); tweets != nil {
		totalCount += len(tweets)
		for _, tweet := range tweets {
			ip, err := IsRepost(m.ChannelID, tweet)
			if err != nil {
				log.Warnln(err)
			}
			if ip != nil {
				repostCount++
				ips = append(ips, ip)
				m.Content = strings.ReplaceAll(m.Content, tweet, "")
			} else {
				NewRepostDetection(m.Author.Username, m.GuildID, m.ChannelID, m.ID, tweet)
			}
		}
	}

	f, _ := MemberHasPermission(s, m.GuildID, s.State.User.ID, discordgo.PermissionManageMessages|discordgo.PermissionAdministrator)
	if f && totalCount == repostCount && totalCount > 0 {
		err := s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
			log.Warn(err)
		}
	}

	return ips
}

func RepostsToEmbed(reposts ...*database.ImagePost) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: "General Reposti!",
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "https://i.imgur.com/OZ1Al5h.png",
		},
		Timestamp: EmbedTimestamp(),
		Color:     EmbedColor,
	}

	for _, rep := range reposts {
		dur := rep.CreatedAt.Add(86400 * time.Second).Sub(time.Now())
		content := &discordgo.MessageEmbedField{
			Name:   "Content",
			Value:  rep.Content,
			Inline: true,
		}
		link := &discordgo.MessageEmbedField{
			Name:   "Link to post",
			Value:  fmt.Sprintf("[Press here desu~](https://discord.com/channels/%v/%v/%v)", rep.GuildID, rep.ChannelID, rep.MessageID),
			Inline: true,
		}
		expires := &discordgo.MessageEmbedField{
			Name:   "Expires",
			Value:  dur.Round(time.Second).String(),
			Inline: true,
		}
		embed.Fields = append(embed.Fields, content, link, expires)
	}

	return embed
}
