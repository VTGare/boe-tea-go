package main

import (
	"os"

	"github.com/VTGare/boe-tea-go/internal/bot"
	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/ugoira"
	"github.com/VTGare/boe-tea-go/utils"
	log "github.com/sirupsen/logrus"
)

var (
	token         = os.Getenv("BOT_TOKEN")
	pixivEmail    = os.Getenv("PIXIV_EMAIL")
	pixivPassword = os.Getenv("PIXIV_PASSWORD")
)

func main() {
	switch {
	case token == "":
		log.Fatalln("BOT_TOKEN env variable doesn't exist")
	case pixivEmail == "":
		log.Fatalln("PIXIV_EMAIL env variable doesn't exist")
	case pixivPassword == "":
		log.Fatalln("PIXIV_PASSWORD env variable doesn't exist")
	}

	b, err := bot.NewBot(token)
	px, err := ugoira.NewApp(pixivEmail, pixivPassword)
	if err != nil {
		log.Warnln("ugoira.NewApp(): ", err)
		utils.PixivDown = true
	}

	ugoira.PixivApp = px
	err = b.Run()
	if err != nil {
		log.Fatalln(err)
	}

	database.DB.Close()
}
