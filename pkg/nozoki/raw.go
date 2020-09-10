package nozoki

import (
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
)

type rawNHBook struct {
	ID         interface{} `json:"id"`
	MediaID    string      `json:"media_id"`
	Titles     NHTitle     `json:"title"`
	Tags       []rawNHTag  `json:"tags"`
	Pages      interface{} `json:"num_pages"`
	Favourites interface{} `json:"num_favorites"`
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

func (raw *rawNHBook) toBook() (*NHBook, error) {
	artists, tags := raw.sortTags()

	id, err := interfaceToInt(raw.ID)
	if err != nil {
		logrus.Warnf("nozoki.toBook(): %v", err)
	}
	favourites, err := interfaceToInt(raw.Favourites)
	if err != nil {
		logrus.Warnf("nozoki.toBook(): %v", err)
	}
	pages, err := interfaceToInt(raw.Pages)
	if err != nil {
		logrus.Warnf("nozoki.toBook(): %v", err)
	}

	return &NHBook{
		ID:         id,
		Titles:     raw.Titles,
		Artists:    artists,
		Tags:       tags,
		URL:        fmt.Sprintf("https://nhentai.net/g/%v/", id),
		Cover:      fmt.Sprintf("https://t.nhentai.net/galleries/%v/cover.jpg", raw.MediaID),
		Pages:      pages,
		Favourites: favourites,
	}, nil
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

func interfaceToInt(i interface{}) (int, error) {
	switch v := i.(type) {
	case int:
		return v, nil
	case uint:
		return int(v), nil
	case int8:
		return int(v), nil
	case uint8:
		return int(v), nil
	case int16:
		return int(v), nil
	case uint16:
		return int(v), nil
	case int32:
		return int(v), nil
	case uint32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("failed to convert an ID: %v", v)
	}
}
