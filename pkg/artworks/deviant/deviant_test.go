package deviant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeviantArt_Match(t *testing.T) {
	deviant := New()

	tests := []struct {
		name  string
		url   string
		want  string
		want1 bool
	}{
		{
			name:  "Arbor Vitae",
			url:   "https://www.deviantart.com/bengeigerart/art/Arbor-Vitae-877183179",
			want:  "Arbor-Vitae-877183179",
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
			url:   "https://deviantart.com/vt/art/boetea-69?width=9001",
			want:  "boetea-69",
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := deviant.Match(tt.url)
			assert.Equal(t, tt.want, id)
			assert.Equal(t, tt.want1, ok)
		})
	}
}
