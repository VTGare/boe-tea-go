package messages

import (
	"testing"
	"time"
)

func TestFormatBool(t *testing.T) {
	type args struct {
		b bool
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{"when is true", args{true}, "enabled"},
		{"when is false", args{false}, "disabled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatBool(tt.args.b); got != tt.want {
				t.Errorf("FormatBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClickHere(t *testing.T) {
	type args struct {
		url string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"with full URL", args{"https://example.com"}, "[Click here](https://example.com)"},
		{"with short URL", args{"example.com"}, "[Click here](example.com)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClickHere(tt.args.url); got != tt.want {
				t.Errorf("ClickHere() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLimitExceeded(t *testing.T) {
	type args struct {
		limit    int
		artworks int
		count    int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"with a single artwork", args{10, 1, 5}, "Album size `(5)` exceeds the server's limit `(10)`, album has been cut."},
		{"with multiple artworks", args{10, 3, 5}, "Album size `(5)` exceeds the server's limit `(10)`, only the first image of every artwork has been sent."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LimitExceeded(tt.args.limit, tt.args.artworks, tt.args.count); got != tt.want {
				t.Errorf("LimitExceeded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCrosspostBy(t *testing.T) {
	type args struct {
		author string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"with some author name", args{"nabi"}, "CrossPost requested by nabi"},
		{"with empty author name", args{""}, "CrossPost requested by anonymous"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CrosspostBy(tt.args.author); got != tt.want {
				t.Errorf("CrosspostBy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRateLimit(t *testing.T) {
	type args struct {
		duration time.Duration
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"with limit of 0", args{time.Second * 0}, "Calm down, you're getting rate limited. Try again in **0s**"},
		{"with limit of 1 second", args{time.Second}, "Calm down, you're getting rate limited. Try again in **1s**"},
		{"with limit of 1 minute", args{time.Minute}, "Calm down, you're getting rate limited. Try again in **1m0s**"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RateLimit(tt.args.duration); got != tt.want {
				t.Errorf("RateLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListChannels(t *testing.T) {
	type args struct {
		channels []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"with single channel", args{[]string{"theoffice"}}, "<#theoffice> | `theoffice`"},
		{"with multiple channels", args{[]string{"theoffice", "lounge", "memes"}}, "<#theoffice> | `theoffice` • <#lounge> | `lounge` • <#memes> | `memes`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ListChannels(tt.args.channels); got != tt.want {
				t.Errorf("ListChannels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	type args struct {
		d time.Duration
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"10 seconds", args{10 * time.Second}, "10 seconds"},
		{"1 hour 10 seconds", args{1*time.Hour + 10*time.Second}, "01 hours 10 seconds"},
		{"3 hours 10 minutes 10 seconds", args{3*time.Hour + 10*time.Minute + 10*time.Second}, "03 hours 10 minutes 10 seconds"},
		{"zero", args{0}, "00 seconds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDuration(tt.args.d); got != tt.want {
				t.Errorf("FormatDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
