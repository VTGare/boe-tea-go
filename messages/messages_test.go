package messages

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "10 seconds",
			duration: 10 * time.Second,
			want:     "10 seconds",
		},
		{
			name:     "1 hour 10 seconds",
			duration: 1*time.Hour + 10*time.Second,
			want:     "01 hours 10 seconds",
		},
		{
			name:     "3 hour 10 minutes 10 seconds",
			duration: 1*time.Hour + 10*time.Minute + 10*time.Second,
			want:     "01 hours 10 minutes 10 seconds",
		},
		{
			name:     "zero",
			duration: 0,
			want:     "00 seconds",
		},
	}
	for _, tt := range tests {
		res := FormatDuration(tt.duration)

		assert.Equal(t, tt.want, res, tt.name)
	}
}
