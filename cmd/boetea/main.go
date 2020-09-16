package main

import (
	"os"

	"github.com/VTGare/boe-tea-go/internal/bot"
	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/server"
	log "github.com/sirupsen/logrus"
)

var (
	token = os.Getenv("BOT_TOKEN")
	pwd   = os.Getenv("SERVER_PASSWORD")
)

func main() {
	switch {
	case token == "":
		log.Fatalln("BOT_TOKEN env variable doesn't exist")
	}

	b, err := bot.NewBot(token)

	go func() {
		server.StartServer(pwd)
	}()

	err = b.Run()
	if err != nil {
		log.Fatalln(err)
	}

	database.DB.Close()
}
