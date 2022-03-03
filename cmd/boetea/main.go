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
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/session/shard"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

func initialiseShardManager(b *bot.Bot, token string) error {
	newShard := state.NewShardFunc(func(m *shard.Manager, s *state.State) {
		s.AddIntents(gateway.IntentGuilds)
		s.AddIntents(gateway.IntentGuildBans)
		s.AddIntents(gateway.IntentGuildMessages)
		s.AddIntents(gateway.IntentGuildMessageReactions)
		s.AddIntents(gateway.IntentDirectMessages)
		s.AddIntents(gateway.IntentDirectMessageReactions)

		for _, handler := range handlers.All(b, s) {
			s.AddHandler(handler)
		}
	})

	mgr, err := shard.NewManager("Bot "+token, newShard)
	if err != nil {
		return err
	}

	b.WithShardManager(mgr)
	return nil
}

func newLogger(sentryToken string) (*zap.SugaredLogger, error) {
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	if sentryToken != "" {
		sentryOption, err := logger.Sentry(sentryToken)
		if err != nil {
			return nil, err
		}
		defer sentry.Flush(10 * time.Second)

		zapLogger = zapLogger.WithOptions(sentryOption)
	}

	return zapLogger.Sugar(), nil
}

func newRepostDetector(t string, redisURI ...string) (repost.Detector, error) {
	if t == "redis" {
		detector, err := repost.NewRedis(redisURI[0])
		if err != nil {
			return nil, err
		}

		return detector, nil
	}

	return repost.NewMemory(), nil
}

func initialiseDatabase(log *zap.SugaredLogger, mongoURI, database string) (*models.Models, error) {
	db, err := mongodb.New(mongoURI, database)
	if err != nil {
		return nil, err
	}

	err = db.CreateCollections()
	if err != nil {
		return nil, err
	}

	return models.New(db, log), nil
}

func main() {
	cfg, err := config.FromFile("config.json")
	if err != nil {
		fmt.Println("config not found: ", err)
		os.Exit(1)
	}

	log, err := newLogger(cfg.Sentry)
	if err != nil {
		fmt.Println("failed to initialise logger: ", err)
		os.Exit(1)
	}

	rep, err := newRepostDetector(cfg.Repost.Type, cfg.Repost.RedisURI)
	if err != nil {
		log.Fatalf("failed to initialise a repost detector: %v", err)
	}

	m, err := initialiseDatabase(log, cfg.Mongo.URI, cfg.Mongo.Database)
	if err != nil {
		log.Fatalf("failed to initialise a database: %v", err)
	}

	b, err := bot.New(cfg, m, log, rep)
	if err != nil {
		log.Fatalf("failed to create a new bot: %v", err)
	}

	b.WithProvider(twitter.New())
	b.WithProvider(deviant.New())
	b.WithProvider(artstation.New())
	if pixiv, err := pixiv.New(cfg.Pixiv.AuthToken, cfg.Pixiv.RefreshToken); err == nil {
		log.Info("successfully logged into pixiv")
		b.WithProvider(pixiv)
	}

	if err := initialiseShardManager(b, cfg.Discord.Token); err != nil {
		log.Fatalf("failed to initialise a shard manager: %v", err)
	}

	if err := b.Open(); err != nil {
		log.Fatalf("failed to open a session: %v", err)
	}

	if err := b.WithCommands(commands.CreateData()); err != nil {
		log.Fatalf("failed to register commands: %v", err)
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	m.DB.Close()
	rep.Close()
	b.Close()
}
