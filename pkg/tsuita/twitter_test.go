package tsuita

import "testing"

func TestTwitter(t *testing.T) {
	_, err := GetTweet("https://twitter.com/moshimoshibe/status/1300448268178976768")
	if err != nil {
		t.Fatal(err)
	}

	_, err = GetTweet("https://mobile.twitter.com/i/web/status/1300448268178976768")
	if err != nil {
		t.Fatal(err)
	}

	_, err = GetTweet("https://twitter.com/i/web/status/1300448268178976768")
	if err != nil {
		t.Fatal(err)
	}
}
