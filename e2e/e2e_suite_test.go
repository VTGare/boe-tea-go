//go:build e2e

// Package e2e drives the artwork pipeline against a real Discord server.
//
// Required environment:
//
//	E2E_DISCORD_TOKEN  bot token with send/embed/attach/react/manage-messages
//	E2E_GUILD_ID       test guild ID
//	E2E_CHANNEL_ID     test channel ID (bot must see it)
//
// Optional:
//
//	E2E_XPOST_CHANNEL_ID second channel enabling the crosspost spec
//	E2E_POSTGRES_DSN     defaults to the testpg DSN from docker-compose.test.yml
//	E2E_TEST_USER_ID     store author ID, defaults to the bot's own user ID
//	                     (the bot is always a guild member, which the
//	                     crosspost membership check requires)
//	E2E_LIVE_ARTWORKS=1  runs live provider coverage from E2E_TWITTER_URL,
//	                     E2E_DEVIANT_URL, E2E_BLUESKY_URL and E2E_PIXIV_URL
//	                     (each missing URL skips its part, pixiv also needs
//	                     E2E_PIXIV_REFRESH_TOKEN). E2E_TWITTER_TEXT_URL pins
//	                     a text-only tweet.
//
// Run with: task test:e2e
package e2e

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Discord E2E Suite")
}
