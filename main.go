package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/bwmarrin/discordgo"
)

var (
	dg *discordgo.Session
)

func main() {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatalln("BOT_TOKEN env variable doesn't exit")
	}

	var err error
	dg, err = discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Error creating a session: ", err)
	}

	dg.AddHandler(onReady)
	dg.AddHandler(messageCreated)
	dg.AddHandler(guildCreated)
	dg.AddHandler(reactCreated)
	dg.AddHandler(guildDeleted)

	if err := dg.Open(); err != nil {
		log.Fatalln("Error opening connection,", err)
	}
	defer dg.Close()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGSEGV, syscall.SIGHUP)
	<-sc

	database.Client.Disconnect(context.Background())
}
