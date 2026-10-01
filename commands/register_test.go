package commands

import (
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/router"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RegisterCommands", func() {
	It("registers every command without validation errors", func() {
		b := &bot.Bot{Router: router.New(router.Config{})}

		Expect(func() { RegisterCommands(b) }).NotTo(Panic())
		Expect(b.Router.Commands()).NotTo(BeEmpty())
	})
})
