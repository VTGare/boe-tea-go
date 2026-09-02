package messages

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMessages(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Messages Suite")
}

var _ = Describe("FormatBool", func() {
	DescribeTable("formats a bool as a human-readable string",
		func(input bool, expected string) {
			Expect(FormatBool(input)).To(Equal(expected))
		},
		Entry("when is true", true, "enabled"),
		Entry("when is false", false, "disabled"),
	)
})

var _ = Describe("ClickHere", func() {
	DescribeTable("wraps a URL in a Discord clickable link",
		func(input, expected string) {
			Expect(ClickHere(input)).To(Equal(expected))
		},
		Entry("with full URL", "https://example.com", "[Click here](https://example.com)"),
		Entry("with short URL", "example.com", "[Click here](example.com)"),
	)
})

var _ = Describe("LimitExceeded", func() {
	DescribeTable("returns the right error message based on artwork count",
		func(limit, artworks, count int, expected string) {
			Expect(LimitExceeded(limit, artworks, count)).To(Equal(expected))
		},
		Entry("with a single artwork",
			10, 1, 5,
			"Album size `(5)` exceeds the server's limit `(10)`, album has been cut."),
		Entry("with multiple artworks",
			10, 3, 5,
			"Album size `(5)` exceeds the server's limit `(10)`, only the first image of every artwork has been sent."),
	)
})

var _ = Describe("CrosspostBy", func() {
	DescribeTable("formats the crosspost requested-by line",
		func(author, expected string) {
			Expect(CrosspostBy(author)).To(Equal(expected))
		},
		Entry("with some author name", "nabi", "Crosspost requested by nabi"),
		Entry("with empty author name", "", "Crosspost requested by anonymous"),
	)
})

var _ = Describe("RateLimit", func() {
	DescribeTable("formats the rate-limit warning for a given duration",
		func(d time.Duration, expected string) {
			Expect(RateLimit(d)).To(Equal(expected))
		},
		Entry("with limit of 0", time.Second*0, "Calm down, you're getting rate limited. Try again in **0s**"),
		Entry("with limit of 1 second", time.Second, "Calm down, you're getting rate limited. Try again in **1s**"),
		Entry("with limit of 1 minute", time.Minute, "Calm down, you're getting rate limited. Try again in **1m0s**"),
	)
})

var _ = Describe("ListChannels", func() {
	DescribeTable("joins channels into a single Discord-formatted string",
		func(channels []string, expected string) {
			Expect(ListChannels(channels)).To(Equal(expected))
		},
		Entry("with single channel",
			[]string{"theoffice"},
			"<#theoffice> | `theoffice`"),
		Entry("with multiple channels",
			[]string{"theoffice", "lounge", "memes"},
			"<#theoffice> | `theoffice` • <#lounge> | `lounge` • <#memes> | `memes`"),
	)
})

var _ = Describe("FormatDuration", func() {
	DescribeTable("formats a duration as a human-readable string",
		func(d time.Duration, expected string) {
			Expect(FormatDuration(d)).To(Equal(expected))
		},
		Entry("10 seconds", 10*time.Second, "10 seconds"),
		Entry("1 hour 10 seconds", 1*time.Hour+10*time.Second, "01 hours 10 seconds"),
		Entry("3 hours 10 minutes 10 seconds", 3*time.Hour+10*time.Minute+10*time.Second, "03 hours 10 minutes 10 seconds"),
		Entry("zero", time.Duration(0), "00 seconds"),
	)
})
