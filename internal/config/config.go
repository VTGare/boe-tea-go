package config

import (
	"encoding/json"
	"os"
)

//Config is an application configuration struct.
type Config struct {
	Discord    *Discord `json:"discord"`
	Mongo      *Mongo   `json:"mongo"`
	Repost     *Repost  `json:"repost"`
	Pixiv      *Pixiv   `json:"pixiv"`
	SauceNAO   string   `json:"saucenao"`
	Sentry     string   `json:"sentry"`
	Encryption string   `json:"encryption"`
	Quotes     []*Quote `json:"quotes"`
}

//Discord stores Discord bot configuration. Acquire bot token on Discord's Developer Portal. Prefixes must be below 5 characters each.
//AuthorID is required to enable developer commands. Empty AuthorID may lead to undefined behavior.
type Discord struct {
	Token    string `json:"token"`
	AuthorID string `json:"author_id"`
}

//Pixiv stores Pixiv login information. Guide how to acquire auth and refresh tokens: https://gist.github.com/upbit/6edda27cb1644e94183291109b8a5fde
type Pixiv struct {
	AuthToken    string `json:"auth_token"`
	RefreshToken string `json:"refresh_token"`
}

//Mongo stores Mongo connection configuration. Required.
type Mongo struct {
	URI      string `json:"uri"`
	Database string `json:"default_db"`
}

//Repost stores repost detector configuration. Supported types: "memory", "redis". RedisURI is not required for in-memory storage.
type Repost struct {
	Type     string `json:"type"`
	RedisURI string `json:"redis_uri"`
}

//Quote is a message shown in Boe Tea's embeds, selected randomly. If empty, footer will always be empty.
type Quote struct {
	Content string `json:"content"`
	NSFW    bool   `json:"nsfw"`
}

func FromFile(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = json.Unmarshal(file, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
