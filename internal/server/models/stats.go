package models

import (
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type Stats struct {
	Guilds    int `json:"guilds,omitempty"`
	Channels  int `json:"channels,omitempty"`
	PostCount int `json:"post_count,omitempty"`
}

var (
	cachedStats = &Stats{0, 0, 0}
)

func init() {
	postCount, err := database.DB.CountPosts()
	if err != nil {
		logrus.Warnln("CountPosts(): ", err)
	}
	cachedStats.PostCount = postCount

	for range time.NewTicker(15 * time.Second).C {
		postCount, err := database.DB.CountPosts()
		if err != nil {
			logrus.Warnln("CountPosts(): ", err)
		} else {
			cachedStats.PostCount = postCount
		}
	}
}

func NewStats(s *discordgo.Session) *Stats {
	channelCount := 0
	for _, g := range s.State.Guilds {
		channelCount += len(g.Channels)
	}

	cachedStats.Guilds = len(s.State.Guilds)
	cachedStats.Channels = channelCount

	return cachedStats
}
