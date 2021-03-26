package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Discord *Discord `json:"discord"`
	Mongo   *Mongo   `json:"mongo"`
	Pixiv   *Pixiv   `json:"pixiv"`
	Quotes  []*Quote `json:"quotes"`
}

type Discord struct {
	Token    string   `json:"token"`
	Prefixes []string `json:"prefix"`
	AuthorID string   `json:"author_id"`
}

type Pixiv struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	AuthToken    string `json:"auth_token"`
	RefreshToken string `json:"refresh_token"`
}

type Mongo struct {
	URI      string `json:"uri"`
	Database string `json:"default_db"`
}

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
