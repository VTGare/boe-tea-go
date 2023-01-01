package nhentai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type API struct {
	client *http.Client
}

type nhentaiResult struct {
	ID         interface{} `json:"id,omitempty"`
	MediaID    string      `json:"media_id,omitempty"`
	Titles     *Titles     `json:"title,omitempty"`
	UploadDate int64       `json:"upload_date"`
	Tags       []struct {
		ID    int    `json:"id,omitempty"`
		Type  string `json:"type,omitempty"`
		Name  string `json:"name,omitempty"`
		URL   string `json:"url,omitempty"`
		Count int    `json:"count,omitempty"`
	} `json:"tags,omitempty"`
	Pages      interface{} `json:"num_pages,omitempty"`
	Favourites interface{} `json:"num_favorites,omitempty"`
}

type Hentai struct {
	ID         int
	URL        string
	Cover      string
	MediaID    string
	Titles     *Titles
	Tags       []*Tag
	Pages      int
	Favorites  int
	UploadedAt time.Time
}

type Titles struct {
	Japanese string `json:"japanese,omitempty"`
	English  string `json:"english,omitempty"`
	Pretty   string `json:"pretty,omitempty"`
}

type Tag struct {
	ID   int
	Type TagType
	Name string
	URL  string
}

type TagType int

const (
	GenreTag TagType = iota
	ArtistTag
	GroupTag
	LanguageTag
	CharacterTag
	ParodyTag
)

var (
	ErrNotFound             = errors.New("doujin not found")
	ErrCloudflareProtection = errors.New("failed to bypass cloudflare")
)

func New() (*API, error) {
	client := &http.Client{}
	return &API{
		client: client,
	}, nil
}

func (n *API) FindHentai(id string) (*Hentai, error) {
	res, err := n.get(id)
	if err != nil {
		return nil, err
	}

	tags := make([]*Tag, 0, len(res.Tags))
	for _, tag := range res.Tags {
		prettyTag := &Tag{
			ID:   tag.ID,
			Name: tag.Name,
			URL:  "https://nhentai.net" + tag.URL,
		}

		switch tag.Type {
		case "artist":
			prettyTag.Type = ArtistTag
		case "group":
			prettyTag.Type = GroupTag
		case "language":
			prettyTag.Type = LanguageTag
		case "parody":
			prettyTag.Type = ParodyTag
		case "character":
			prettyTag.Type = CharacterTag
		default:
			prettyTag.Type = GenreTag
		}

		tags = append(tags, prettyTag)
	}

	return &Hentai{
		Titles:     res.Titles,
		MediaID:    res.MediaID,
		URL:        fmt.Sprintf("https://nhentai.net/g/%v", res.ID),
		Tags:       tags,
		Cover:      fmt.Sprintf("https://t.nhentai.net/galleries/%v/cover.jpg", res.MediaID),
		ID:         interfaceToInt(res.ID),
		Pages:      interfaceToInt(res.Pages),
		Favorites:  interfaceToInt(res.Favourites),
		UploadedAt: time.Unix(res.UploadDate, 0),
	}, nil
}

func (h *Hentai) Genres() []*Tag {
	genres := make([]*Tag, 0)
	for _, tag := range h.Tags {
		if tag.Type == GenreTag {
			genres = append(genres, tag)
		}
	}

	return genres
}

func (h *Hentai) Language() (*Tag, bool) {
	for _, tag := range h.Tags {
		if tag.Type == LanguageTag {
			return tag, true
		}
	}

	return nil, false
}

func (h *Hentai) Parodies() []*Tag {
	parodies := make([]*Tag, 0)
	for _, tag := range h.Tags {
		if tag.Type == ParodyTag {
			parodies = append(parodies, tag)
		}
	}

	return parodies
}

func (h *Hentai) Characters() []*Tag {
	characters := make([]*Tag, 0)
	for _, tag := range h.Tags {
		if tag.Type == CharacterTag {
			characters = append(characters, tag)
		}
	}

	return characters
}

func (h *Hentai) Artists() []*Tag {
	artists := make([]*Tag, 0)
	for _, tag := range h.Tags {
		if tag.Type == GroupTag || tag.Type == ArtistTag {
			artists = append(artists, tag)
		}
	}

	return artists
}

func (t *Tag) String() string {
	return t.Name
}

func (n *API) get(id string) (*nhentaiResult, error) {
	req, err := http.NewRequest("GET", "https://nhentai.net/api/gallery/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create a request to nhentai: %w", err)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		var res nhentaiResult
		err = json.NewDecoder(resp.Body).Decode(&res)
		if err != nil {
			return nil, err
		}

		return &res, nil

	case 503:
		return nil, ErrCloudflareProtection
	case 404:
		return nil, ErrNotFound
	}

	return nil, fmt.Errorf("unexpected api response: %v", resp.Status)
}

func interfaceToInt(i interface{}) int {
	var res int

	switch v := i.(type) {
	case int:
		res = v
	case int64:
		res = int(v)
	case float64:
		res = int(v)
	case string:
		res, _ = strconv.Atoi(v)
	}

	return res
}
