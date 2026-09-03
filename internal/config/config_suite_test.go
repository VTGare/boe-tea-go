package config

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfigUnit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Unit Suite")
}

var _ = BeforeSuite(func() {
	for _, key := range []string{"POSTGRES_DSN", "MONGO_URI", "PGHOST_X"} {
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

var _ = Describe("StoreBackend", func() {
	const legacy = `{"mongo": {"uri": "mongodb://legacy:27017", "default_db": "boe"}}`

	It("defaults to legacy top-level mongo when store block is missing", func() {
		cfg, err := FromFile(writeTempConfig(legacy))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Backend).To(Equal("mongo"))
		Expect(resolved.Mongo.URI).To(Equal("mongodb://legacy:27017"))
	})

	It("prefers legacy mongo over store.mongo", func() {
		cfg, err := FromFile(writeTempConfig(`{"mongo": {"uri": "mongodb://legacy:27017", "default_db": "boe"},
			"store": {"backend": "mongo", "mongo": {"uri": "mongodb://nested:27017", "default_db": "boe"}}}`))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Mongo.URI).To(Equal("mongodb://legacy:27017"))
	})

	It("uses store.mongo when no legacy block exists", func() {
		cfg, err := FromFile(writeTempConfig(`{"store": {"backend": "mongo",
			"mongo": {"uri": "mongodb://nested:27017", "default_db": "boe"}}}`))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Mongo.URI).To(Equal("mongodb://nested:27017"))
	})

	It("resolves postgres from the store block", func() {
		cfg, err := FromFile(writeTempConfig(`{"mongo": {"uri": "mongodb://legacy:27017", "default_db": "boe"},
			"store": {"backend": "postgres", "postgres": {"dsn": "postgres://u:p@h/db", "database": "db"}}}`))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Backend).To(Equal("postgres"))
		Expect(resolved.Postgres.DSN).To(Equal("postgres://u:p@h/db"))
	})

	It("fails on unknown backends and missing configs", func() {
		cfg, err := FromFile(writeTempConfig(`{"store": {"backend": "sqlite"}}`))
		Expect(err).NotTo(HaveOccurred())
		_, err = cfg.StoreBackend()
		Expect(err).To(HaveOccurred())

		cfg, err = FromFile(writeTempConfig(`{"store": {"backend": "postgres"}}`))
		Expect(err).NotTo(HaveOccurred())
		_, err = cfg.StoreBackend()
		Expect(err).To(HaveOccurred())

		cfg, err = FromFile(writeTempConfig(`{}`))
		Expect(err).NotTo(HaveOccurred())
		_, err = cfg.StoreBackend()
		Expect(err).To(HaveOccurred())
	})

	It("lets POSTGRES_DSN env win over the file", func() {
		Expect(os.Setenv("POSTGRES_DSN", "postgres://env/x")).To(Succeed())
		DeferCleanup(os.Unsetenv, "POSTGRES_DSN")

		cfg, err := FromFile(writeTempConfig(`{"store": {"backend": "postgres",
			"postgres": {"dsn": "postgres://file/x", "database": "x"}}}`))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Postgres.DSN).To(Equal("postgres://env/x"))
	})

	It("expands ${ENV} placeholders in DSNs", func() {
		Expect(os.Setenv("PGHOST_X", "dbhost")).To(Succeed())
		DeferCleanup(os.Unsetenv, "PGHOST_X")

		cfg, err := FromFile(writeTempConfig(`{"store": {"backend": "postgres",
			"postgres": {"dsn": "postgres://u:p@${PGHOST_X}/db", "database": "db"}}}`))

		Expect(err).NotTo(HaveOccurred())

		resolved, err := cfg.StoreBackend()

		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Postgres.DSN).To(Equal("postgres://u:p@dbhost/db"))
	})
})
