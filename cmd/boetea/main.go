package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/internal/database/mongodb"
	"github.com/VTGare/boe-tea-go/internal/logger"
	"github.com/VTGare/boe-tea-go/pkg/artworks/artstation"
	"github.com/VTGare/boe-tea-go/pkg/artworks/deviant"
	"github.com/VTGare/boe-tea-go/pkg/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/pkg/artworks/twitter"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/commands"
	"github.com/VTGare/boe-tea-go/pkg/handlers"
	"github.com/VTGare/boe-tea-go/pkg/models"
	"github.com/VTGare/boe-tea-go/pkg/repost"
	"github.com/VTGare/gumi"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.FromFile("config.json")
	if err != nil {
		fmt.Println("Config not found: ", err)
		os.Exit(1)
	}

	zapLogger, err := zap.NewProduction()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if cfg.Sentry != "" {
		sentryOption, err := logger.Sentry(cfg.Sentry)
		if err != nil {
			fmt.Println("Error initializing Sentry: ", err)
			os.Exit(1)
		}
		defer sentry.Flush(10 * time.Second)

		zapLogger = zapLogger.WithOptions(sentryOption)
	}

	log := zapLogger.Sugar()

	db, err := mongodb.New(cfg.Mongo.URI, cfg.Mongo.Database)
	if err != nil {
		log.Fatal(err)
	}

	err = db.CreateCollections()
	if err != nil {
		log.Fatal(err)
	}

	m := models.New(db, log)

	var repostDetector repost.Detector
	switch cfg.Repost.Type {
	case "redis":
		repostDetector, err = repost.NewRedis(cfg.Repost.RedisURI)
		if err != nil {
			log.Fatal(err)
		}
	default:
		repostDetector = repost.NewMemory()
	}

	b, err := bot.New(cfg, m, log, repostDetector)
	if err != nil {
		log.Fatal(err)
	}

	b.AddProvider(twitter.New())
	b.AddProvider(deviant.New())
	b.AddProvider(artstation.New())
	if pixiv, err := pixiv.New(cfg.Pixiv.AuthToken, cfg.Pixiv.RefreshToken); err == nil {
		log.Info("Successfully logged into Pixiv.")
		b.AddProvider(pixiv)
	}

	b.AddRouter(&gumi.Router{
		Commands:                make(map[string]*gumi.Command),
		AuthorID:                cfg.Discord.AuthorID,
		PrefixResolver:          handlers.PrefixResolver(b),
		NotCommandCallback:      handlers.NotCommand(b),
		OnErrorCallback:         handlers.OnError(b),
		OnRateLimitCallback:     handlers.OnRateLimit(b),
		OnNSFWCallback:          handlers.OnNSFW(b),
		OnExecuteCallback:       handlers.OnExecute(b),
		OnNoPermissionsCallback: handlers.OnNoPerms(b),
		OnPanicCallBack:         handlers.OnPanic(b),
	})

	handlers.RegisterHandlers(b)
	commands.RegisterCommands(b)

	if err := b.Open(); err != nil {
		log.Fatal(err)
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	db.Close()
	repostDetector.Close()
	b.Close()
}
