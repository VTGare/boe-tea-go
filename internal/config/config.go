package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/internal/spool"
	"github.com/julien040/go-ternary"
)

// Config is an application configuration struct.
type Config struct {
	Discord    *Discord     `json:"discord"`
	Mongo      *Mongo       `json:"mongo"`
	Store      *StoreConfig `json:"store"`
	Repost     *Repost      `json:"repost"`
	Pixiv      *Pixiv       `json:"pixiv"`
	SauceNAO   string       `json:"saucenao"`
	Encryption string       `json:"encryption"`
	Sentry     string       `json:"sentry"`
	Media      *Media       `json:"media"`
	Debug      *Debug       `json:"debug"`
	Quotes     []*Quote     `json:"quotes"`

	safeQuotes []*Quote
}

// Discord stores Discord bot configuration. Acquire bot token on Discord's Developer Portal. Prefixes must be below 5 characters each.
// AuthorID is required to enable developer commands. Empty AuthorID may lead to undefined behavior.
type Discord struct {
	Token      string `json:"token"`
	AuthorID   string `json:"author_id"`
	DevGuildID string `json:"dev_guild_id"`
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

// Media stores media download limits. Every field is optional;
// zero values select built-in defaults, so the block can be omitted entirely.
type Media struct {
	MaxConcurrent int    `json:"max_concurrent"`
	SpoolDir      string `json:"spool_dir"`
}

// SpoolConfig converts Media to a spool.Config. A nil Media gives a zero
// config, which means defaults.
func (m *Media) SpoolConfig() spool.Config {
	if m == nil {
		return spool.Config{}
	}

	return spool.Config{
		MaxConcurrent: m.MaxConcurrent,
		Dir:           m.SpoolDir,
	}
}

// Debug holds temporary switches for the OOM investigation; remove it
// once the leak is found. A zero PprofPort turns diagnostics off.
type Debug struct {
	PprofPort int `json:"pprof_port"`
}

// Quote is a message shown in Boe Tea's embeds, selected randomly. If empty, footer will always be empty.
type Quote struct {
	Content string `json:"content"`
	NSFW    bool   `json:"nsfw"`
}

// Load reads the config file at path, then applies BOETEA_* environment
// variables on top of it. A missing file is fine when the environment has
// everything. Quotes come from the file and from the quotes file.
func Load(path string) (*Config, error) {
	var cfg Config

	file, err := readFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(file, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}

	cfg.fillSections()
	cfg.expandStoreSecrets()

	if err := cfg.applyEnv(); err != nil {
		return nil, err
	}

	quotesPath, explicit := os.LookupEnv("BOETEA_QUOTES_FILE")
	if !explicit {
		quotesPath = "quotes.json"
	}

	if err := cfg.loadQuotes(quotesPath, explicit); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// readFile falls back to the executable's directory for relative paths.
func readFile(path string) ([]byte, error) {
	file, err := os.ReadFile(path)
	if err == nil || filepath.IsAbs(path) {
		return file, err
	}

	exePath, exeErr := os.Executable()
	if exeErr != nil {
		return nil, err
	}

	return os.ReadFile(filepath.Join(filepath.Dir(exePath), path))
}

// fillSections allocates the sections main reads without nil checks.
func (c *Config) fillSections() {
	if c.Discord == nil {
		c.Discord = &Discord{}
	}

	if c.Repost == nil {
		c.Repost = &Repost{}
	}

	if c.Pixiv == nil {
		c.Pixiv = &Pixiv{}
	}

	if c.Media == nil {
		c.Media = &Media{}
	}

	if c.Debug == nil {
		c.Debug = &Debug{}
	}
}

// applyEnv lets set variables win over the file, even when they're empty,
// so an empty BOETEA_DISCORD_DEV_GUILD_ID turns a file's dev guild off.
func (c *Config) applyEnv() error {
	strs := []struct {
		key string
		dst *string
	}{
		{"BOETEA_DISCORD_TOKEN", &c.Discord.Token},
		{"BOETEA_DISCORD_AUTHOR_ID", &c.Discord.AuthorID},
		{"BOETEA_DISCORD_DEV_GUILD_ID", &c.Discord.DevGuildID},
		{"BOETEA_REPOST_TYPE", &c.Repost.Type},
		{"BOETEA_REDIS_URI", &c.Repost.RedisURI},
		{"BOETEA_PIXIV_AUTH_TOKEN", &c.Pixiv.AuthToken},
		{"BOETEA_PIXIV_REFRESH_TOKEN", &c.Pixiv.RefreshToken},
		{"BOETEA_PIXIV_PROXY_HOST", &c.Pixiv.ProxyHost},
		{"BOETEA_SAUCENAO_KEY", &c.SauceNAO},
		{"BOETEA_SENTRY_DSN", &c.Sentry},
		{"BOETEA_MEDIA_SPOOL_DIR", &c.Media.SpoolDir},
	}

	for _, s := range strs {
		if v, ok := os.LookupEnv(s.key); ok {
			*s.dst = v
		}
	}

	ints := []struct {
		key string
		dst *int
	}{
		{"BOETEA_MEDIA_MAX_CONCURRENT", &c.Media.MaxConcurrent},
		{"BOETEA_PPROF_PORT", &c.Debug.PprofPort},
	}

	for _, i := range ints {
		v, ok := os.LookupEnv(i.key)
		if !ok || v == "" {
			continue
		}

		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %w", i.key, err)
		}

		*i.dst = n
	}

	if v, ok := os.LookupEnv("BOETEA_STORE_BACKEND"); ok {
		if c.Store == nil {
			c.Store = &StoreConfig{}
		}

		c.Store.Backend = v
	}

	if v := os.Getenv("BOETEA_POSTGRES_DSN"); v != "" {
		c.setPostgresDSN(v)
	}

	// The top-level mongo block wins over store.mongo, so the variables go there.
	uri, db := os.Getenv("BOETEA_MONGO_URI"), os.Getenv("BOETEA_MONGO_DATABASE")
	if uri != "" || db != "" {
		if c.Mongo == nil {
			c.Mongo = &Mongo{}
		}

		if uri != "" {
			c.Mongo.URI = uri
		}

		if db != "" {
			c.Mongo.Database = db
		}
	}

	return nil
}

// loadQuotes adds the quotes file's quotes to the config's. A missing file
// is only an error when BOETEA_QUOTES_FILE names it.
func (c *Config) loadQuotes(path string, required bool) error {
	file, err := readFile(path)
	switch {
	case err == nil:
		var quotes []*Quote
		if err := json.Unmarshal(file, &quotes); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		c.Quotes = append(c.Quotes, quotes...)
	case required || !errors.Is(err, fs.ErrNotExist):
		return err
	}

	c.filterSafeQuotes()

	return nil
}

func (c *Config) filterSafeQuotes() {
	c.safeQuotes = make([]*Quote, 0, len(c.Quotes))

	for _, quote := range c.Quotes {
		if !quote.NSFW {
			c.safeQuotes = append(c.safeQuotes, quote)
		}
	}
}

func (c *Config) setPostgresDSN(dsn string) {
	if c.Store == nil {
		c.Store = &StoreConfig{}
	}

	if c.Store.Postgres == nil {
		c.Store.Postgres = &Postgres{}
	}

	c.Store.Postgres.DSN = dsn
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
		c.setPostgresDSN(dsn)
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
	quotes := ternary.If(
		nsfw,
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
