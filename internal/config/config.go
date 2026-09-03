package config

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/julien040/go-ternary"
)

// Config is an application configuration struct.
type Config struct {
	Discord  *Discord     `json:"discord"`
	Mongo    *Mongo       `json:"mongo"`
	Store    *StoreConfig `json:"store"`
	Repost   *Repost      `json:"repost"`
	Pixiv    *Pixiv       `json:"pixiv"`
	SauceNAO string       `json:"saucenao"`
	Sentry   string       `json:"sentry"`
	Quotes   []*Quote     `json:"quotes"`

	safeQuotes []*Quote
}

// Discord stores Discord bot configuration. Acquire bot token on Discord's Developer Portal. Prefixes must be below 5 characters each.
// AuthorID is required to enable developer commands. Empty AuthorID may lead to undefined behavior.
type Discord struct {
	Token    string `json:"token"`
	AuthorID string `json:"author_id"`
}

// Pixiv stores Pixiv login information. Guide how to acquire auth and refresh tokens: https://gist.github.com/upbit/6edda27cb1644e94183291109b8a5fde
type Pixiv struct {
	AuthToken    string `json:"auth_token"`
	RefreshToken string `json:"refresh_token"`
	ProxyHost    string `json:"proxy_host"`
}

// Mongo stores Mongo connection configuration. Required when store backend is mongo.
// Kept at top level for backwards compatibility with configs that predate the store selector.
type Mongo struct {
	URI      string `json:"uri"`
	Database string `json:"default_db"`
}

// Postgres stores Postgres connection configuration. Required when store backend is postgres.
type Postgres struct {
	DSN      string `json:"dsn"`
	Database string `json:"database"`
}

// StoreConfig selects the store backend. Backend must be "mongo" or "postgres".
// When nil or Backend is empty, backend defaults to "mongo" using the legacy top-level Mongo block.
type StoreConfig struct {
	Backend  string    `json:"backend"`
	Mongo    *Mongo    `json:"mongo"`
	Postgres *Postgres `json:"postgres"`
}

// Repost stores repost detector configuration. Supported types: "memory", "redis". RedisURI is not required for in-memory storage.
type Repost struct {
	Type     string `json:"type"`
	RedisURI string `json:"redis_uri"`
}

// Quote is a message shown in Boe Tea's embeds, selected randomly. If empty, footer will always be empty.
type Quote struct {
	Content string `json:"content"`
	NSFW    bool   `json:"nsfw"`
}

func FromFile(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		exePath, _ := os.Executable()
		file, err = os.ReadFile(fmt.Sprintf("%s/%s", filepath.Dir(exePath), path))
		if err != nil {
			return nil, err
		}
	}

	var cfg Config
	err = json.Unmarshal(file, &cfg)
	if err != nil {
		return nil, err
	}

	cfg.expandStoreSecrets()

	if len(cfg.Quotes) > 0 {
		cfg.safeQuotes = make([]*Quote, 0)

		for _, quote := range cfg.Quotes {
			if !quote.NSFW {
				cfg.safeQuotes = append(cfg.safeQuotes, quote)
			}
		}
	}

	return &cfg, nil
}

// expandStoreSecrets expands ${ENV} placeholders in store DSNs and lets
// explicit env vars win over file values.
func (c *Config) expandStoreSecrets() {
	if c.Mongo != nil {
		c.Mongo.URI = os.ExpandEnv(c.Mongo.URI)
	}

	if c.Store != nil {
		if c.Store.Mongo != nil {
			c.Store.Mongo.URI = os.ExpandEnv(c.Store.Mongo.URI)
		}

		if c.Store.Postgres != nil {
			c.Store.Postgres.DSN = os.ExpandEnv(c.Store.Postgres.DSN)
		}
	}

	if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
		if c.Store == nil {
			c.Store = &StoreConfig{}
		}

		if c.Store.Postgres == nil {
			c.Store.Postgres = &Postgres{}
		}

		c.Store.Postgres.DSN = dsn
	}

	if uri := os.Getenv("MONGO_URI"); uri != "" && c.Mongo != nil {
		c.Mongo.URI = uri
	}
}

// ResolvedStore is the effective store backend after applying backwards-compatibility rules.
type ResolvedStore struct {
	Backend  string
	Mongo    *Mongo
	Postgres *Postgres
}

// StoreBackend resolves the effective store backend with backwards compatibility.
//
//   - store block missing or backend empty -> "mongo" using legacy top-level mongo.
//   - backend "mongo" -> legacy top-level mongo wins if present, else store.mongo.
//   - backend "postgres" -> store.postgres (POSTGRES_DSN env wins).
func (c *Config) StoreBackend() (ResolvedStore, error) {
	backend := "mongo"
	if c.Store != nil && c.Store.Backend != "" {
		backend = c.Store.Backend
	}

	switch backend {
	case "mongo":
		mongo := c.Mongo
		if mongo == nil && c.Store != nil {
			mongo = c.Store.Mongo
		}

		if mongo == nil || mongo.URI == "" {
			return ResolvedStore{}, fmt.Errorf("mongo backend selected but no mongo config found")
		}

		return ResolvedStore{Backend: "mongo", Mongo: mongo}, nil
	case "postgres":
		var pg *Postgres
		if c.Store != nil {
			pg = c.Store.Postgres
		}

		if pg == nil || pg.DSN == "" {
			return ResolvedStore{}, fmt.Errorf("postgres backend selected but no postgres.dsn found")
		}

		return ResolvedStore{Backend: "postgres", Postgres: pg}, nil
	default:
		return ResolvedStore{}, fmt.Errorf("unknown store backend %q: must be \"mongo\" or \"postgres\"", backend)
	}
}

func (c *Config) RandomQuote(nsfw bool) string {
	quotes := ternary.If(nsfw,
		c.Quotes,
		c.safeQuotes,
	)

	if l := len(quotes); l > 0 {
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)

		return quotes[r.Intn(l)].Content
	}

	return ""
}
