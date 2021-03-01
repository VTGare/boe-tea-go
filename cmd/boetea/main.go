package main

import (
	"os"

	"github.com/VTGare/boe-tea-go/internal/bot"
	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/ugoira"
	"github.com/VTGare/boe-tea-go/pkg/tsuita"
	log "github.com/sirupsen/logrus"
)

var (
	token        = os.Getenv("BOT_TOKEN")
	authToken    = os.Getenv("AUTH_TOKEN")
	refreshToken = os.Getenv("REFRESH_TOKEN")
	authorID     = os.Getenv("AUTHOR_ID")
)

func main() {
	switch {
	case token == "":
		log.Fatalln("BOT_TOKEN env variable doesn't exist")
	case authToken == "":
		log.Fatalln("AUTH_TOKEN env variable doesn't exist")
	case refreshToken == "":
		log.Fatalln("REFRESH_TOKEN env variable doesn't exist")
	case authorID == "":
		log.Fatalln("AUTHOR_ID env variable doesn't exist'")
	}

	bot, err := bot.NewBot(token)
	px, err := ugoira.NewApp(authToken, refreshToken)
	if px != nil {
		bot.Router.Storage.Set("pixiv", px)
	}

	bot.Router.Storage.Set("twitter", tsuita.NewTsuita())
	bot.Router.AuthorID = authorID

	err = bot.Run()
	if err != nil {
		log.Fatalln(err)
	}

	database.DB.Close()
}
