package chotto

import "testing"

func TestSecondsToReadable(t *testing.T) {
	type args struct {
		sec int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"1 minute", args{60}, "1m0s"},
		{"2", args{3600}, "1h0m0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := secondsToReadable(tt.args.sec); got != tt.want {
				t.Errorf("secondsToReadable() = %v, want %v", got, tt.want)
			}
		})
	}
}
