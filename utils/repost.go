package utils

import "time"

var (
	//RepostCache is in-memory cached repost checker.
	RepostCache = make(map[string]map[string]bool)
)

//IsRepost checks if something has been cached in repost cache
func IsRepost(channelID, post string) bool {
	_, ok := RepostCache[channelID][post]
	return ok
}

//NewRepostChecker caches post info per channel.
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
