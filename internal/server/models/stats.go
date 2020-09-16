package models

import "github.com/bwmarrin/discordgo"

type Stats struct {
	Guilds   int `json:"guilds,omitempty"`
	Channels int `json:"channels,omitempty"`
}

func NewStats(s *discordgo.Session) *Stats {
	count := 0
	for _, g := range s.State.Guilds {
		count += len(g.Channels)
	}

	return &Stats{len(s.State.Guilds), count}
}
