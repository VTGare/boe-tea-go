package twitter

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
)

func TestTwitterMatch(t *testing.T) {
	tests := []struct {
		name              string
		tr                tsuita
		url               string
		expectedBool      bool
		expectedSnowflake string
	}{
		{
			name:              "Standard Twitter URL",
			tr:                tsuita{},
			url:               "https://twitter.com/watsonameliaEN/status/1371674594675937282",
			expectedSnowflake: "1371674594675937282",
			expectedBool:      true,
		},
		{
			name:              "Mobile Twitter URL",
			tr:                tsuita{},
			url:               "https://mobile.twitter.com/watsonameliaEN/status/1371674594675937282",
			expectedBool:      true,
			expectedSnowflake: "1371674594675937282",
		},
		{
			name:              "Twitter URL without username",
			tr:                tsuita{},
			url:               "https://twitter.com/i/status/1371674594675937282",
			expectedBool:      true,
			expectedSnowflake: "1371674594675937282",
		},
		{
			name:              "Twitter URL i/web",
			tr:                tsuita{},
			url:               "https://twitter.com/i/web/status/1371674594675937282",
			expectedBool:      true,
			expectedSnowflake: "1371674594675937282",
		},
		{
			name:              "Invalid snowflake",
			tr:                tsuita{},
			url:               "https://twitter.com/i/web/status/13716745235f",
			expectedBool:      false,
			expectedSnowflake: "",
		},
		{
			name:              "With query parameters",
			tr:                tsuita{},
			url:               "https://twitter.com/i/web/status/12345678?width=120",
			expectedBool:      true,
			expectedSnowflake: "12345678",
		},
		{
			name:              "Profile URL",
			tr:                tsuita{},
			url:               "https://twitter.com/vtgare",
			expectedBool:      false,
			expectedSnowflake: "",
		},
		{
			name:              "Not Twitter URL",
			tr:                tsuita{},
			url:               "https://google.com",
			expectedBool:      false,
			expectedSnowflake: "",
		},
		{
			name:              "Not URL",
			tr:                tsuita{},
			url:               "google",
			expectedBool:      false,
			expectedSnowflake: "",
		},
		{
			name:              "Invalid Twitter URL",
			tr:                tsuita{},
			url:               "https://twitter.com/i/web/",
			expectedBool:      false,
			expectedSnowflake: "",
		},
	}

	for _, tt := range tests {
		snowflake, ok := tt.tr.Match(tt.url)

		assert.Equal(t, tt.expectedBool, ok, tt.name)
		assert.Equal(t, tt.expectedSnowflake, snowflake, tt.name)
	}
}

func TestArtworkEmbeds(t *testing.T) {
	tests := []struct {
		name   string
		a      Artwork
		footer string
		want   []*discordgo.MessageEmbed
	}{
		{
			name: "No media files",
			a: Artwork{
				FullName:  "VTGare",
				Username:  "vt",
				Content:   "No media files",
				Timestamp: "2014-04-15T18:00:15-07:00",
				Likes:     10,
				Retweets:  1,
				Gallery:   nil,
			},
			footer: "test",
			want: []*discordgo.MessageEmbed{
				{
					Title:       "VTGare (vt)",
					Description: "No media files",
					Timestamp:   "2014-04-15T18:00:15-07:00",
					Fields: []*discordgo.MessageEmbedField{
						{Name: "Retweets", Value: "1", Inline: true}, {Name: "Likes", Value: "10", Inline: true},
					},
					Color:  4431601,
					Footer: &discordgo.MessageEmbedFooter{Text: "test"},
				},
			},
		},
		{
			name: "One media file",
			a: Artwork{
				FullName:  "VTGare",
				Username:  "vt",
				Content:   "One media file",
				Timestamp: "2014-04-15T18:00:15-07:00",
				Likes:     10,
				Retweets:  1,
				Gallery: []*Media{
					{
						URL: "https://google.com",
					},
				},
			},
			footer: "test",
			want: []*discordgo.MessageEmbed{
				{
					Title:       "VTGare (vt)",
					Description: "One media file",
					Timestamp:   "2014-04-15T18:00:15-07:00",
					Image:       &discordgo.MessageEmbedImage{URL: "https://google.com"},
					Fields: []*discordgo.MessageEmbedField{
						{Name: "Retweets", Value: "1", Inline: true}, {Name: "Likes", Value: "10", Inline: true},
					},
					Color:  4431601,
					Footer: &discordgo.MessageEmbedFooter{Text: "test"},
				},
			},
		},
		{
			name: "Two media files",
			a: Artwork{
				FullName:  "VTGare",
				Username:  "vt",
				Content:   "Two media files",
				Timestamp: "2014-04-15T18:00:15-07:00",
				Likes:     10,
				Retweets:  1,
				Gallery: []*Media{
					{
						URL: "https://google.com",
					},
					{
						URL: "https://twitter.com",
					},
				},
			},
			footer: "test",
			want: []*discordgo.MessageEmbed{
				{
					Title:       "VTGare (vt) | Page 1 / 2",
					Description: "Two media files",
					Timestamp:   "2014-04-15T18:00:15-07:00",
					Image:       &discordgo.MessageEmbedImage{URL: "https://google.com"},
					Fields: []*discordgo.MessageEmbedField{
						{Name: "Retweets", Value: "1", Inline: true}, {Name: "Likes", Value: "10", Inline: true},
					},
					Color:  4431601,
					Footer: &discordgo.MessageEmbedFooter{Text: "test"},
				},
				{
					Title:     "VTGare (vt) | Page 2 / 2",
					Timestamp: "2014-04-15T18:00:15-07:00",
					Image:     &discordgo.MessageEmbedImage{URL: "https://twitter.com"},
					Color:     4431601,
					Footer:    &discordgo.MessageEmbedFooter{Text: "test"},
				},
			},
		},
		{
			name: "One animated media file",
			a: Artwork{
				FullName:  "VTGare",
				Username:  "vt",
				Content:   "One animated media file",
				Timestamp: "2014-04-15T18:00:15-07:00",
				Likes:     10,
				Retweets:  1,
				Gallery: []*Media{
					{
						URL:      "https://google.com",
						Animated: true,
					},
				},
			},
			footer: "test",
			want: []*discordgo.MessageEmbed{
				{
					Title:       "VTGare (vt)",
					Description: "One animated media file",
					Timestamp:   "2014-04-15T18:00:15-07:00",
					Fields: []*discordgo.MessageEmbedField{
						{Name: "Retweets", Value: "1", Inline: true}, {Name: "Likes", Value: "10", Inline: true},
						{Name: "Video", Value: "https://google.com"},
					},
					Color:  4431601,
					Footer: &discordgo.MessageEmbedFooter{Text: "test"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.a.Embeds(tt.footer))
		})
	}
}
