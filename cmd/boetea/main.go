package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/internal/database/mongodb"
	"github.com/VTGare/boe-tea-go/pkg/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/pkg/artworks/twitter"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/handlers"
	"github.com/VTGare/boe-tea-go/pkg/models"
	"github.com/VTGare/boe-tea-go/pkg/repost"
	"github.com/VTGare/gumi"
	"go.uber.org/zap"
)

func main() {
	prod, err := zap.NewProduction()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	log := prod.Sugar()

	cfg, err := config.FromFile("config.json")
	if err != nil {
		log.Fatal(err)
	}

	db, err := mongodb.New(cfg.Mongo.URI, cfg.Mongo.Database)
	if err != nil {
		log.Fatal(err)
	}

	m := models.New(db, log)

	var dec repost.Detector
	switch cfg.Repost.Type {
	case "redis":
		//TODO: add redis
		dec = repost.NewRedis(nil)
	default:
		dec = repost.NewMemory()
	}

	b, err := bot.New(cfg, m, log, dec)
	b.AddProvider(twitter.New())
	if pixiv, err := pixiv.New(cfg.Pixiv.AuthToken, cfg.Pixiv.RefreshToken); err == nil {
		log.Info("Successfully logged into Pixiv.")
		b.AddProvider(pixiv)
	}

	b.AddRouter(&gumi.Router{
		AuthorID:           cfg.Discord.AuthorID,
		PrefixResolver:     handlers.PrefixResolver(b),
		NotCommandCallback: handlers.NotCommand(b),
	})

	b.AddHandler(handlers.OnReady(b))
	b.AddHandler(handlers.OnGuildCreated(b))

	if err := b.Open(); err != nil {
		log.Fatal(err)
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	db.Close()
	b.Close()
}
