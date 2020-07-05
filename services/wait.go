package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	//ErrRateLimited ...
	ErrRateLimited = errors.New("trace.moe is rate limited, try again later")
	waitURL        = "https://trace.moe/api/search?url="
)

type WaitResult struct {
	Limit     int            `json:"limit"`
	LimitTTL  int            `json:"limit_ttl"`
	Quota     int            `json:"quota"`
	QuotaTTL  int            `json:"quota_ttl"`
	Documents []WaitDocument `json:"docs"`
}

type WaitDocument struct {
	AnilistID    int         `json:"anilist_id"`
	MalID        int         `json:"mal_id"`
	Anime        string      `json:"anime"`
	Episode      interface{} `json:"episode"`
	From         float64     `json:"from"`
	To           float64     `json:"to"`
	At           float64     `json:"at"`
	Similarity   float64     `json:"similarity"`
	Title        string      `json:"title"`
	TitleNative  string      `json:"title_native"`
	TitleChinese string      `json:"title_chinese"`
	TitleEnglish string      `json:"title_english"`
	TitleRomaji  string      `json:"title_romaji"`
	Synonyms     []string    `json:"synonyms"`
	IsAdult      bool        `json:"is_adult"`
}

func ErrOutOfQuota(quotaTTL int) error {
	return fmt.Errorf("boe tea has ran out of today's trace.moe quota. It'll reset in %v", secondsToReadable(quotaTTL))
}

func secondsToReadable(sec int) string {
	t := time.Second * time.Duration(sec)
	return t.String()
}

func SearchWait(image string) (*WaitResult, error) {
	image = url.QueryEscape(image)
	uri := waitURL + image

	body, err := FasthttpGet(uri)
	if err != nil {
		return nil, err
	}

	if strings.Contains(string(body), "Search limit exceeded") {
		return nil, ErrRateLimited
	}

	var res WaitResult
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}
