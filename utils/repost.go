package utils

import "time"

var (
	RepostCache = make(map[string]map[string]bool)
)

func IsRepost(guildID, post string) bool {
	_, ok := RepostCache[guildID][post]
	return ok
}

func NewRepostChecker(guildID, post string) {
	if _, ok := RepostCache[guildID]; !ok {
		RepostCache[guildID] = map[string]bool{}
	}

	RepostCache[guildID][post] = true
	go func() {
		time.Sleep(24 * time.Hour)
		delete(RepostCache[guildID], post)
	}()
}
