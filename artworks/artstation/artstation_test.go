package artstation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtstation_Match(t *testing.T) {
	artstation := New()

	tests := []struct {
		name  string
		url   string
		want  string
		want1 bool
	}{
		{
			name:  "Match",
			url:   "https://www.artstation.com/artwork/q98e9N",
			want:  "q98e9N",
			want1: true,
		},
		{
			name:  "Don't match",
			url:   "https://www.deviantart.com/art/Arbor-Vitae-877183179",
			want:  "",
			want1: false,
		},
		{
			name:  "With query parameters",
			url:   "https://www.artstation.com/artwork/1234?width=9001",
			want:  "1234",
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := artstation.Match(tt.url)
			assert.Equal(t, tt.want, id)
			assert.Equal(t, tt.want1, ok)
		})
	}
}
