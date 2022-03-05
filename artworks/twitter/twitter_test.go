package twitter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTwitterMatch(t *testing.T) {
	tests := []struct {
		name              string
		tr                Twitter
		url               string
		expectedBool      bool
		expectedSnowflake string
	}{
		{
			name:              "Standard Twitter URL",
			tr:                Twitter{},
			url:               "https://twitter.com/watsonameliaEN/status/1371674594675937282",
			expectedSnowflake: "1371674594675937282",
			expectedBool:      true,
		},
		{
			name:              "Mobile Twitter URL",
			tr:                Twitter{},
			url:               "https://mobile.twitter.com/watsonameliaEN/status/1371674594675937282",
			expectedBool:      true,
			expectedSnowflake: "1371674594675937282",
		},
		{
			name:              "Twitter URL without username",
			tr:                Twitter{},
			url:               "https://twitter.com/i/status/1371674594675937282",
			expectedBool:      true,
			expectedSnowflake: "1371674594675937282",
		},
		{
			name:              "Twitter URL i/web",
			tr:                Twitter{},
			url:               "https://twitter.com/i/web/status/1371674594675937282",
			expectedBool:      true,
			expectedSnowflake: "1371674594675937282",
		},
		{
			name:              "Invalid snowflake",
			tr:                Twitter{},
			url:               "https://twitter.com/i/web/status/13716745235f",
			expectedBool:      false,
			expectedSnowflake: "",
		},
		{
			name:              "With query parameters",
			tr:                Twitter{},
			url:               "https://twitter.com/i/web/status/12345678?width=120",
			expectedBool:      true,
			expectedSnowflake: "12345678",
		},
		{
			name:              "With /photo/1 suffix",
			tr:                Twitter{},
			url:               "https://twitter.com/i/web/status/1371674594675937282/photo/1",
			expectedBool:      true,
			expectedSnowflake: "1371674594675937282",
		},
		{
			name:              "Profile URL",
			tr:                Twitter{},
			url:               "https://twitter.com/vtgare",
			expectedBool:      false,
			expectedSnowflake: "",
		},
		{
			name:              "Not Twitter URL",
			tr:                Twitter{},
			url:               "https://google.com",
			expectedBool:      false,
			expectedSnowflake: "",
		},
		{
			name:              "Not URL",
			tr:                Twitter{},
			url:               "google",
			expectedBool:      false,
			expectedSnowflake: "",
		},
		{
			name:              "Invalid Twitter URL",
			tr:                Twitter{},
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

func TestTwitterFind(t *testing.T) {
	client := New()
	tests := []struct {
		name      string
		snowflake string
		want      func(*Artwork) bool
		wantErr   error
	}{
		{
			name:      "Twitter GIF",
			snowflake: "1408344696980529153",
			want: func(a *Artwork) bool {
				if len(a.Gallery) == 0 {
					return false
				}

				gif := a.Gallery[0]
				return gif.Type == MediaTypeGIF && gif.URL == "https://video.twimg.com/tweet_video/E4tx4P5VcAMGP4F.mp4"
			},
		},
		{
			name:      "Twitter video",
			snowflake: "1399456145236848640",
			want: func(a *Artwork) bool {
				if len(a.Gallery) == 0 {
					return false
				}

				video := a.Gallery[0]
				return video.Type == MediaTypeVideo && video.URL == "https://pbs.twimg.com/ext_tw_video_thumb/1399377616600141833/pu/img/q1nnSghkCfGlD7mM.jpg"
			},
		},
	}

	for _, test := range tests {
		artwork, err := client.Find(test.snowflake)

		assert.True(t, test.want(artwork.(*Artwork)), test.name)
		assert.Equal(t, test.wantErr, err, test.name)
	}
}
