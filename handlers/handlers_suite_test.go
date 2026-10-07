package handlers

import (
	"errors"
	"testing"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/gumi/v2"
	"github.com/VTGare/gumi/v2/middleware"
	"go.uber.org/zap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHandlers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handlers Suite")
}

var _ = Describe("Recover middleware", func() {
	It("converts panics into PanicError", func() {
		mw := middleware.Recover()

		err := mw(func(*gumi.Context) error {
			panic("boom")
		})(&gumi.Context{})

		var panicErr *gumi.PanicError

		Expect(errors.As(err, &panicErr)).To(BeTrue())
		Expect(panicErr.Value).To(Equal("boom"))
	})

	It("passes handler results through", func() {
		mw := middleware.Recover()

		Expect(mw(func(*gumi.Context) error {
			return nil
		})(&gumi.Context{})).To(BeNil())
	})
})

var _ = Describe("OnError", func() {
	var b *bot.Bot

	BeforeEach(func() {
		logger := zap.NewExample().Sugar()
		b = &bot.Bot{Log: logger}
	})

	It("survives a nil context", func() {
		Expect(func() {
			OnError(b)(nil, errors.New("boom"))
		}).NotTo(Panic())
	})

	It("logs panics without replying", func() {
		Expect(func() {
			OnError(b)(&gumi.Context{}, &gumi.PanicError{Value: "boom"})
		}).NotTo(Panic())
	})

	It("swallows silent check errors", func() {
		Expect(func() {
			OnError(b)(&gumi.Context{}, &gumi.CheckError{Check: "owner_only", Silent: true})
		}).NotTo(Panic())
	})
})
