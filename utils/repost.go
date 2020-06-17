package utils

import "time"

var (
	RepostCache = make(map[string]map[string]bool)
)

func IsRepost(channelID, post string) bool {
	_, ok := RepostCache[channelID][post]
	return ok
}

func NewRepostChecker(channelID, post string) {
	if _, ok := RepostCache[channelID]; !ok {
		RepostCache[channelID] = map[string]bool{}
	}

	RepostCache[channelID][post] = true
	go func() {
		time.Sleep(24 * time.Hour)
		delete(RepostCache[channelID], post)
	}()
}
