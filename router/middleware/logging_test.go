package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/VTGare/boe-tea-go/router"
	"github.com/VTGare/boe-tea-go/router/middleware"
	"github.com/bwmarrin/discordgo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logging", func() {
	run := func(err error) map[string]any {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))

		ctx := &router.Context{
			Command: &router.Command{Name: "ping"},
			Message: &discordgo.Message{GuildID: "g", ChannelID: "c", Author: &discordgo.User{ID: "u", Username: "vt"}},
		}

		got := middleware.Logging(logger)(func(*router.Context) error { return err })(ctx)
		if err == nil {
			Expect(got).NotTo(HaveOccurred())
		} else {
			Expect(got).To(MatchError(err))
		}

		var entry map[string]any
		Expect(json.Unmarshal(buf.Bytes(), &entry)).To(Succeed())
		return entry
	}

	It("logs successful commands at info with invocation fields", func() {
		entry := run(nil)

		Expect(entry).To(HaveKeyWithValue("level", "INFO"))
		Expect(entry).To(HaveKeyWithValue("msg", "command executed"))
		Expect(entry).To(HaveKeyWithValue("command", "ping"))
		Expect(entry).To(HaveKeyWithValue("source", "message"))
		Expect(entry).To(HaveKeyWithValue("user_id", "u"))
		Expect(entry).To(HaveKeyWithValue("guild_id", "g"))
		Expect(entry).To(HaveKeyWithValue("channel_id", "c"))
		Expect(entry).To(HaveKeyWithValue("user", "vt"))
		Expect(entry).To(HaveKey("duration"))
	})

	It("logs user-facing rejections at info with the reason", func() {
		entry := run(router.Errorf("Command not found."))

		Expect(entry).To(HaveKeyWithValue("level", "INFO"))
		Expect(entry).To(HaveKeyWithValue("msg", "command rejected"))
		Expect(entry).To(HaveKeyWithValue("reason", "Command not found."))
	})

	It("logs other failures at error", func() {
		entry := run(errors.New("boom"))

		Expect(entry).To(HaveKeyWithValue("level", "ERROR"))
		Expect(entry).To(HaveKeyWithValue("msg", "command failed"))
		Expect(entry).To(HaveKeyWithValue("error", "boom"))
	})

	It("discards everything with a nil logger", func() {
		h := middleware.Logging(nil)(func(*router.Context) error { return nil })
		Expect(h(&router.Context{Command: &router.Command{Name: "ping"}})).To(Succeed())
	})
})
