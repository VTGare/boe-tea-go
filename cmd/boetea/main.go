package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VTGare/boe-tea-go/artworks/bluesky"
	"github.com/VTGare/boe-tea-go/artworks/deviant"
	"github.com/VTGare/boe-tea-go/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/commands"
	"github.com/VTGare/boe-tea-go/handlers"
	"github.com/VTGare/boe-tea-go/internal/config"
	"github.com/VTGare/boe-tea-go/internal/diag"
	"github.com/VTGare/boe-tea-go/internal/logger"
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/internal/spool"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/router"
	"github.com/VTGare/boe-tea-go/router/middleware"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/boe-tea-go/store/mongo"
	"github.com/VTGare/boe-tea-go/store/postgres"

	"github.com/bwmarrin/discordgo"
	"github.com/getsentry/sentry-go"
	cache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

func initStore(ctx context.Context, cfg *config.Config) (store.Store, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resolved, err := cfg.StoreBackend()
	if err != nil {
		return nil, err
	}

	var backend store.Store

	switch resolved.Backend {
	case "postgres":
		backend, err = postgres.New(ctx, resolved.Postgres.DSN)
		if err != nil {
			return nil, err
		}
	default:
		backend, err = mongo.New(ctx, resolved.Mongo.URI, resolved.Mongo.Database)
		if err != nil {
			return nil, err
		}
	}

	if err := backend.Init(ctx); err != nil {
		return nil, err
	}

	store := store.NewStatefulStore(backend, cache.New(30*time.Minute, 1*time.Hour))
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
	slogger := slog.New(zapslog.NewHandler(zapLogger.Core()))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer cancel()

	store, err := initStore(ctx, cfg)
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

	b.Sender = sender.NewDiscordSender(b.ShardManager, log, nil)

	b.AddProvider(twitter.New())
	b.AddProvider(deviant.New())
	b.AddProvider(bluesky.New())

	spool.Configure(cfg.Media.SpoolConfig())

	if cfg.Debug != nil && cfg.Debug.PprofPort > 0 {
		diag.Start(ctx, log, cfg.Debug.PprofPort, diag.DefaultDir())
	}

	if err := pixiv.LoadAuth(cfg.Pixiv.AuthToken, cfg.Pixiv.RefreshToken); err == nil {
		log.Info("Successfully logged into Pixiv.")
		b.AddProvider(pixiv.New(cfg.Pixiv.ProxyHost))
	}

	r := router.New(router.Config{
		PrefixResolver: handlers.PrefixResolver(b),
		OwnerIDs:       []string{cfg.Discord.AuthorID},
		Fallback:       handlers.OnMessage(b),
		ErrorHandler:   handlers.OnError(b),
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{
				discordgo.AllowedMentionTypeEveryone,
				discordgo.AllowedMentionTypeRoles,
				discordgo.AllowedMentionTypeUsers,
			},
		},
		DevGuildID: cfg.Discord.DevGuildID,
	})

	r.Use(
		middleware.Logging(slogger),
		middleware.Recover(),
		middleware.Timeout(2*time.Minute),
		handlers.ObserveStats(b),
	)

	b.AddRouter(r)

	handlers.RegisterHandlers(b)
	commands.RegisterCommands(b)

	if err := b.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
