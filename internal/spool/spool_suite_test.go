package spool

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSpool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Spool Suite")
}

var _ = Describe("Temp files", func() {
	It("creates files inside the configured directory", func() {
		dir := GinkgoT().TempDir()
		Configure(Config{Dir: dir})
		DeferCleanup(func() { Configure(DefaultConfig()) })

		file, err := TempFile("bt-video-*.mp4")

		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Dir(file.Name())).To(Equal(dir))

		file.Close()
		Remove(file.Name())

		Expect(file.Name()).NotTo(BeAnExistingFile())
	})

	It("ignores paths outside the spool directory", func() {
		dir := GinkgoT().TempDir()
		Configure(Config{Dir: dir})
		DeferCleanup(func() { Configure(DefaultConfig()) })

		outside := filepath.Join(GinkgoT().TempDir(), "outside.mp4")
		Expect(os.WriteFile(outside, []byte("data"), 0o600)).To(Succeed())

		Remove(outside)

		Expect(outside).To(BeAnExistingFile())
	})
})

var _ = Describe("RemoveFiles", func() {
	It("closes and deletes spooled files, leaving others alone", func() {
		dir := GinkgoT().TempDir()
		Configure(Config{Dir: dir})
		DeferCleanup(func() { Configure(DefaultConfig()) })

		spooled, err := TempFile("bt-video-*.mp4")
		Expect(err).NotTo(HaveOccurred())

		plain := filepath.Join(GinkgoT().TempDir(), "plain.mp4")
		Expect(os.WriteFile(plain, []byte("data"), 0o600)).To(Succeed())

		disk, err := os.Open(plain)
		Expect(err).NotTo(HaveOccurred())

		RemoveFiles([]*discordgo.File{
			{Reader: spooled},
			{Reader: disk},
			nil,
		})

		Expect(spooled.Name()).NotTo(BeAnExistingFile())
		Expect(plain).To(BeAnExistingFile())

		disk.Close()
	})
})

var _ = Describe("Sweep", func() {
	It("deletes everything left behind by previous runs", func() {
		dir := GinkgoT().TempDir()
		Configure(Config{Dir: dir})
		DeferCleanup(func() { Configure(DefaultConfig()) })

		stale := filepath.Join(dir, "stale.mp4")
		Expect(os.WriteFile(stale, []byte("data"), 0o600)).To(Succeed())

		Sweep()

		Expect(stale).NotTo(BeAnExistingFile())
	})
})

var _ = Describe("Download", func() {
	dir := ""
	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		Configure(Config{Dir: dir})
		DeferCleanup(func() { Configure(DefaultConfig()) })
	})

	It("streams content to a rewound file", func() {
		file, err := Download("bt-video-*.mp4", strings.NewReader("data"))

		Expect(err).NotTo(HaveOccurred())

		content, err := os.ReadFile(file.Name())

		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("data"))

		file.Close()
		Remove(file.Name())

		Expect(file.Name()).NotTo(BeAnExistingFile())
	})

	It("leaves no file behind when streaming fails", func() {
		_, err := Download("bt-video-*.mp4", &failingReader{})

		Expect(err).To(HaveOccurred())

		entries, err := os.ReadDir(dir)

		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})
})

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("boom")
}

var _ = Describe("Configure", func() {
	It("fills zero values with defaults", func() {
		cfg := Configure(Config{})

		Expect(cfg).To(Equal(DefaultConfig()))

		Configure(DefaultConfig())
	})

	It("keeps explicit values", func() {
		dir := GinkgoT().TempDir()
		cfg := Configure(Config{
			MaxConcurrent: 2,
			Dir:           dir,
		})

		Expect(cfg.MaxConcurrent).To(Equal(2))
		Expect(cfg.Dir).To(Equal(dir))

		Configure(DefaultConfig())
	})
})
