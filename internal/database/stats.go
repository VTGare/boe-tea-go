package database

import "time"

var (
	lastCachedResult *Stats
)

type Stats struct {
	Guilds     int       `json:"guilds" bson:"guilds"`
	Channels   int       `json:"channels" bson:"channels"`
	TotalPosts int       `json:"total_posts" bson:"total_posts"`
	StartTime  time.Time `json:"start_time" bson:"start_time"`
}
