package services

import (
	"testing"
)

func TestAscii2DResponse(t *testing.T) {
	_, err := getASCII2DPage("https://i.pinimg.com/originals/bf/85/da/bf85daf12641323bc231a526c71c7a57.png")
	if err != nil {
		t.Fatal(err)
	}
}
