package handlers

import (
	"testing"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHandlers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handlers Suite")
}

var _ = Describe("OnPanic", func() {
	var b *bot.Bot

	BeforeEach(func() {
		logger := zap.NewExample().Sugar()
		b = &bot.Bot{Log: logger}
	})

	It("survives an empty context", func() {
		Expect(func() {
			OnPanic(b)(&gumi.Ctx{}, "boom")
		}).NotTo(Panic())
	})

	It("logs event context when present", func() {
		gctx := &gumi.Ctx{
			Event: &discordgo.MessageCreate{
				Message: &discordgo.Message{
					ID:        "m",
					ChannelID: "c",
					GuildID:   "g",
				},
			},
		}

		Expect(func() {
			OnPanic(b)(gctx, "boom")
		}).NotTo(Panic())
	})
})

var _ = Describe("OnMessage", func() {
	var b *bot.Bot

	BeforeEach(func() {
		logger := zap.NewExample().Sugar()
		b = &bot.Bot{Log: logger}
	})

	It("drops nil contexts and malformed events", func() {
		Expect(OnMessage(b)(nil)).To(BeNil())
		Expect(OnMessage(b)(&gumi.Ctx{})).To(BeNil())
		Expect(OnMessage(b)(&gumi.Ctx{Event: &discordgo.MessageCreate{}})).To(BeNil())
	})
})
