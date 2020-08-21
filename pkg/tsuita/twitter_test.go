package tsuita

import "testing"

func TestTwitter(t *testing.T) {
	tweet, err := GetTweet("https://twitter.com/FrostHAHA/status/1258802153168273409")
	if err != nil {
		t.Fatal(err)
	}

	println(tweet.Gallery)
}
