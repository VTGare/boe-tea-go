package stats

import "time"

var (
	lastStats *Stats
)

type Stats struct {
	Guilds     int
	Channels   int
	TotalPosts int
	LaunchTime time.Time
}

func init() {
	lastStats = &Stats{0, 0, 0, time.Now()}
}
