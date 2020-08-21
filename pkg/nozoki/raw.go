package nozoki

import (
	"fmt"
	"strconv"
)

type rawNHBook struct {
	ID         interface{} `json:"id"`
	MediaID    string      `json:"media_id"`
	Titles     NHTitle     `json:"title"`
	Tags       []rawNHTag  `json:"tags"`
	Pages      int         `json:"num_pages"`
	Favourites int         `json:"num_favorites"`
}

type rawNHTag struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Count int    `json:"count"`
}

//NHTitle contains three translations of the doujinshi title
type NHTitle struct {
	Japanese string `json:"japanese"`
	English  string `json:"english"`
	Pretty   string `json:"pretty"`
}

func (raw *rawNHBook) toBook() *NHBook {
	artists, tags := raw.sortTags()
	return &NHBook{
		ID:         raw.id(),
		Titles:     raw.Titles,
		Artists:    artists,
		Tags:       tags,
		URL:        fmt.Sprintf("https://nhentai.net/g/%v/", raw.id()),
		Cover:      fmt.Sprintf("https://t.nhentai.net/galleries/%v/cover.jpg", raw.MediaID),
		Pages:      raw.Pages,
		Favourites: raw.Favourites,
	}
}

func (raw *rawNHBook) sortTags() (artists []string, tags []string) {
	artists = make([]string, 0)
	tags = make([]string, 0)

	for _, tag := range raw.Tags {
		if tag.Type == "artist" {
			artists = append(artists, tag.Name)
		} else if tag.Type == "group" {
			artists = append(artists, tag.Name+" (group)")
		} else {
			tags = append(tags, tag.Name)
		}
	}

	return
}

func (raw *rawNHBook) id() int {
	if id, ok := raw.ID.(int); ok {
		return id
	} else if id, ok := raw.ID.(string); ok {
		intID, _ := strconv.Atoi(id)
		return intID
	} else {
		return 0
	}
}
