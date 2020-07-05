package nhentai

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/VTGare/boe-tea-go/services"
)

var (
	baseNHentai = "https://nhentai.net"
)

type rawNHBook struct {
	ID         interface{} `json:"id"`
	MediaID    string      `json:"media_id"`
	Titles     NHTitle     `json:"title"`
	Tags       []rawNHTag  `json:"tags"`
	Pages      int         `json:"num_pages"`
	Favourites int         `json:"num_favorites"`
}

type NHTitle struct {
	Japanese string `json:"japanese"`
	English  string `json:"english"`
	Pretty   string `json:"pretty"`
}

type rawNHTag struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Count int    `json:"count"`
}

type NHBook struct {
	ID         int
	URL        string
	Titles     NHTitle
	Artists    []string
	Tags       []string
	Cover      string
	Pages      int
	Favourites int
}

func getRawBook(id string) (*rawNHBook, error) {
	resp, err := services.FasthttpGet(baseNHentai + "/api/gallery/" + id)
	if err != nil {
		return nil, err
	}

	var book rawNHBook
	err = json.Unmarshal(resp, &book)
	if err != nil {
		return nil, err
	}

	return &book, nil
}

func sortTags(book *rawNHBook) (artists []string, tags []string) {
	artists = make([]string, 0)
	tags = make([]string, 0)

	for _, tag := range book.Tags {
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

func GetNHentai(id string) (*NHBook, error) {
	raw, err := getRawBook(id)
	if err != nil {
		return nil, err
	}

	book := &NHBook{}
	if id, ok := raw.ID.(int); ok {
		book.ID = id
	} else if id, ok := raw.ID.(string); ok {
		book.ID, _ = strconv.Atoi(id)
	} else {
		book.ID = 0
	}

	book.Titles = raw.Titles
	book.Cover = fmt.Sprintf("https://t.nhentai.net/galleries/%v/cover.jpg", raw.MediaID)
	book.Artists, book.Tags = sortTags(raw)
	book.Favourites = raw.Favourites
	book.Pages = raw.Pages
	book.URL = fmt.Sprintf("https://nhentai.net/g/%v/", id)

	return book, nil
}
