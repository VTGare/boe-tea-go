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
	token         = os.Getenv("BOT_TOKEN")
	pixivEmail    = os.Getenv("PIXIV_EMAIL")
	pixivPassword = os.Getenv("PIXIV_PASSWORD")
	authorID      = os.Getenv("AUTHOR_ID")
)

func main() {
	switch {
	case token == "":
		log.Fatalln("BOT_TOKEN env variable doesn't exist")
	case pixivEmail == "":
		log.Fatalln("PIXIV_EMAIL env variable doesn't exist")
	case pixivPassword == "":
		log.Fatalln("PIXIV_PASSWORD env variable doesn't exist")
	case authorID == "":
		log.Fatalln("AUTHOR_ID env variable doesn't exist'")
	}

	bot, err := bot.NewBot(token)
	px, err := ugoira.NewApp(pixivEmail, pixivPassword)
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
