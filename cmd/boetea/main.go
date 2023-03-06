package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VTGare/boe-tea-go/artworks/artstation"
	"github.com/VTGare/boe-tea-go/artworks/deviant"
	"github.com/VTGare/boe-tea-go/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/commands"
	"github.com/VTGare/boe-tea-go/handlers"
	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/internal/logger"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/boe-tea-go/store/mongo"
	"github.com/VTGare/gumi"

	"github.com/getsentry/sentry-go"
	cache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

func initStore(mongoURI, database string) (store.Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mongo, err := mongo.New(ctx, mongoURI, database)
	if err != nil {
		return nil, err
	}

	if err := mongo.Init(ctx); err != nil {
		return nil, err
	}

	store := store.NewStatefulStore(mongo, cache.New(30*time.Minute, 1*time.Hour))
	return store, nil
}

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

	store, err := initStore(cfg.Mongo.URI, cfg.Mongo.Database)
	if err != nil {
		log.Fatal(err)
	}

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

	b, err := bot.New(cfg, store, log, repostDetector)
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

	store.Close(context.Background())
	repostDetector.Close()
	b.Close()
}
