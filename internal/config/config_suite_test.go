package config

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfigUnit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Unit Suite")
}

var _ = BeforeSuite(func() {
	keys := []string{
		"POSTGRES_DSN", "MONGO_URI", "PGHOST_X",
		"BOETEA_DISCORD_TOKEN", "BOETEA_DISCORD_AUTHOR_ID", "BOETEA_DISCORD_DEV_GUILD_ID",
		"BOETEA_STORE_BACKEND", "BOETEA_POSTGRES_DSN", "BOETEA_MONGO_URI", "BOETEA_MONGO_DATABASE",
		"BOETEA_REPOST_TYPE", "BOETEA_REDIS_URI",
		"BOETEA_PIXIV_AUTH_TOKEN", "BOETEA_PIXIV_REFRESH_TOKEN", "BOETEA_PIXIV_PROXY_HOST",
		"BOETEA_SAUCENAO_KEY", "BOETEA_SENTRY_DSN",
		"BOETEA_MEDIA_MAX_CONCURRENT", "BOETEA_MEDIA_SPOOL_DIR", "BOETEA_PPROF_PORT",
		"BOETEA_QUOTES_FILE",
	}

	for _, key := range keys {
		if val, ok := os.LookupEnv(key); ok {
			DeferCleanup(os.Setenv, key, val)
			Expect(os.Unsetenv(key)).To(Succeed())
		} else {
			DeferCleanup(os.Unsetenv, key)
		}
	}
})

func writeTempConfig(content string) string {
	f, err := os.CreateTemp("", "config-*.json")
	Expect(err).NotTo(HaveOccurred())

	_, err = f.WriteString(content)
	Expect(err).NotTo(HaveOccurred())
	Expect(f.Close()).To(Succeed())

	DeferCleanup(os.Remove, f.Name())

	return f.Name()
}

func setenv(key, value string) {
	Expect(os.Setenv(key, value)).To(Succeed())
	DeferCleanup(os.Unsetenv, key)
}

var _ = Describe("Environment", func() {
	It("loads everything from the environment without a config file", func() {
		setenv("BOETEA_DISCORD_TOKEN", "token")
		setenv("BOETEA_DISCORD_AUTHOR_ID", "1")
		setenv("BOETEA_STORE_BACKEND", "postgres")
		setenv("BOETEA_POSTGRES_DSN", "postgres://env/db")
		setenv("BOETEA_REPOST_TYPE", "redis")
		setenv("BOETEA_REDIS_URI", "redis://localhost:6379")
		setenv("BOETEA_PIXIV_AUTH_TOKEN", "auth")
		setenv("BOETEA_PIXIV_REFRESH_TOKEN", "refresh")
		setenv("BOETEA_PIXIV_PROXY_HOST", "https://proxy")
		setenv("BOETEA_SAUCENAO_KEY", "sauce")
		setenv("BOETEA_SENTRY_DSN", "sentry")
		setenv("BOETEA_MEDIA_MAX_CONCURRENT", "3")
		setenv("BOETEA_MEDIA_SPOOL_DIR", "/spool")
		setenv("BOETEA_PPROF_PORT", "16060")

		cfg, err := Load(filepath.Join(GinkgoT().TempDir(), "missing.json"))
		Expect(err).NotTo(HaveOccurred())

		Expect(*cfg.Discord).To(Equal(Discord{Token: "token", AuthorID: "1"}))
		Expect(*cfg.Repost).To(Equal(Repost{Type: "redis", RedisURI: "redis://localhost:6379"}))
		Expect(*cfg.Pixiv).To(Equal(Pixiv{AuthToken: "auth", RefreshToken: "refresh", ProxyHost: "https://proxy"}))
		Expect(cfg.SauceNAO).To(Equal("sauce"))
		Expect(cfg.Sentry).To(Equal("sentry"))
		Expect(*cfg.Media).To(Equal(Media{MaxConcurrent: 3, SpoolDir: "/spool"}))
		Expect(cfg.Debug.PprofPort).To(Equal(16060))

		resolved, err := cfg.StoreBackend()
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Backend).To(Equal("postgres"))
		Expect(resolved.Postgres.DSN).To(Equal("postgres://env/db"))
	})

	It("lets set variables win over the file, even empty ones", func() {
		setenv("BOETEA_DISCORD_TOKEN", "env-token")
		setenv("BOETEA_DISCORD_DEV_GUILD_ID", "")

		cfg, err := Load(writeTempConfig(`{"discord": {"token": "file-token", "author_id": "1", "dev_guild_id": "123"}}`))
		Expect(err).NotTo(HaveOccurred())

		Expect(*cfg.Discord).To(Equal(Discord{Token: "env-token", AuthorID: "1"}))
	})

	It("puts the Mongo variables in the block that wins", func() {
		setenv("BOETEA_MONGO_URI", "mongodb://env:27017")
		setenv("BOETEA_MONGO_DATABASE", "env-db")

		cfg, err := Load(writeTempConfig(`{"store": {"backend": "mongo",
			"mongo": {"uri": "mongodb://nested:27017", "default_db": "boe"}}}`))
		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()
		Expect(err).NotTo(HaveOccurred())
		Expect(*resolved.Mongo).To(Equal(Mongo{URI: "mongodb://env:27017", Database: "env-db"}))
	})

	It("rejects numbers that don't parse", func() {
		setenv("BOETEA_PPROF_PORT", "abc")

		_, err := Load(writeTempConfig(`{}`))
		Expect(err).To(MatchError(ContainSubstring("BOETEA_PPROF_PORT")))
	})

	It("fails on a config file that doesn't parse", func() {
		_, err := Load(writeTempConfig(`{`))
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Quotes", func() {
	writeQuotes := func(content string) string {
		path := filepath.Join(GinkgoT().TempDir(), "quotes.json")
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())

		return path
	}

	It("adds the quotes file to the config's quotes", func() {
		setenv("BOETEA_QUOTES_FILE", writeQuotes(`[{"content": "from file", "nsfw": true}]`))

		cfg, err := Load(writeTempConfig(`{"quotes": [{"content": "from config"}]}`))
		Expect(err).NotTo(HaveOccurred())

		Expect(cfg.Quotes).To(Equal([]*Quote{{Content: "from config"}, {Content: "from file", NSFW: true}}))
		Expect(cfg.RandomQuote(false)).To(Equal("from config"))
	})

	It("fails when BOETEA_QUOTES_FILE names a missing file", func() {
		setenv("BOETEA_QUOTES_FILE", filepath.Join(GinkgoT().TempDir(), "missing.json"))

		_, err := Load(writeTempConfig(`{}`))
		Expect(err).To(HaveOccurred())
	})

	It("has no quotes without either source", func() {
		cfg, err := Load(writeTempConfig(`{}`))
		Expect(err).NotTo(HaveOccurred())

		Expect(cfg.RandomQuote(true)).To(BeEmpty())
	})
})

var _ = Describe("Discord", func() {
	It("parses the dev guild ID", func() {
		cfg, err := Load(writeTempConfig(`{"discord": {"token": "t", "author_id": "a", "dev_guild_id": "123"}}`))

		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Discord.DevGuildID).To(Equal("123"))
	})

	It("leaves the dev guild ID empty when unset", func() {
		cfg, err := Load(writeTempConfig(`{"discord": {"token": "t", "author_id": "a"}}`))

		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Discord.DevGuildID).To(BeEmpty())
	})
})

var _ = Describe("StoreBackend", func() {
	const legacy = `{"mongo": {"uri": "mongodb://legacy:27017", "default_db": "boe"}}`

	It("defaults to legacy top-level mongo when store block is missing", func() {
		cfg, err := Load(writeTempConfig(legacy))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Backend).To(Equal("mongo"))
		Expect(resolved.Mongo.URI).To(Equal("mongodb://legacy:27017"))
	})

	It("prefers legacy mongo over store.mongo", func() {
		cfg, err := Load(writeTempConfig(`{"mongo": {"uri": "mongodb://legacy:27017", "default_db": "boe"},
			"store": {"backend": "mongo", "mongo": {"uri": "mongodb://nested:27017", "default_db": "boe"}}}`))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Mongo.URI).To(Equal("mongodb://legacy:27017"))
	})

	It("uses store.mongo when no legacy block exists", func() {
		cfg, err := Load(writeTempConfig(`{"store": {"backend": "mongo",
			"mongo": {"uri": "mongodb://nested:27017", "default_db": "boe"}}}`))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Mongo.URI).To(Equal("mongodb://nested:27017"))
	})

	It("resolves postgres from the store block", func() {
		cfg, err := Load(writeTempConfig(`{"mongo": {"uri": "mongodb://legacy:27017", "default_db": "boe"},
			"store": {"backend": "postgres", "postgres": {"dsn": "postgres://u:p@h/db", "database": "db"}}}`))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Backend).To(Equal("postgres"))
		Expect(resolved.Postgres.DSN).To(Equal("postgres://u:p@h/db"))
	})

	It("fails on unknown backends and missing configs", func() {
		cfg, err := Load(writeTempConfig(`{"store": {"backend": "sqlite"}}`))
		Expect(err).NotTo(HaveOccurred())
		_, err = cfg.StoreBackend()
		Expect(err).To(HaveOccurred())

		cfg, err = Load(writeTempConfig(`{"store": {"backend": "postgres"}}`))
		Expect(err).NotTo(HaveOccurred())
		_, err = cfg.StoreBackend()
		Expect(err).To(HaveOccurred())

		cfg, err = Load(writeTempConfig(`{}`))
		Expect(err).NotTo(HaveOccurred())
		_, err = cfg.StoreBackend()
		Expect(err).To(HaveOccurred())
	})

	It("lets POSTGRES_DSN env win over the file", func() {
		Expect(os.Setenv("POSTGRES_DSN", "postgres://env/x")).To(Succeed())
		DeferCleanup(os.Unsetenv, "POSTGRES_DSN")

		cfg, err := Load(writeTempConfig(`{"store": {"backend": "postgres",
			"postgres": {"dsn": "postgres://file/x", "database": "x"}}}`))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Postgres.DSN).To(Equal("postgres://env/x"))
	})

	It("expands ${ENV} placeholders in DSNs", func() {
		Expect(os.Setenv("PGHOST_X", "dbhost")).To(Succeed())
		DeferCleanup(os.Unsetenv, "PGHOST_X")

		cfg, err := Load(writeTempConfig(`{"store": {"backend": "postgres",
			"postgres": {"dsn": "postgres://u:p@${PGHOST_X}/db", "database": "db"}}}`))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Postgres.DSN).To(Equal("postgres://u:p@dbhost/db"))
	})
})
