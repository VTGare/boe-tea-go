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
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/boe-tea-go/store/mongo"
	"github.com/VTGare/boe-tea-go/store/postgres"
	"github.com/VTGare/gumi/v2"
	"github.com/VTGare/gumi/v2/middleware"

	"github.com/disgoorg/disgo"
	disgobot "github.com/disgoorg/disgo/bot"
	disgocache "github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/sharding"
	"github.com/disgoorg/snowflake/v2"
	"github.com/getsentry/sentry-go"
	cache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

func initStore(ctx context.Context, cfg *config.Config) (store.Store, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resolved, err := cfg.StoreBackend()
	if err != nil {
		return nil, err
	}

	var backend store.Store

	switch resolved.Backend {
	case "postgres":
		backend, err = postgres.New(connectCtx, resolved.Postgres.DSN)
		if err != nil {
			return nil, err
		}
	default:
		backend, err = mongo.New(connectCtx, resolved.Mongo.URI, resolved.Mongo.Database)
		if err != nil {
			return nil, err
		}
	}

	// Init applies migrations, which can rewrite whole tables and take
	// minutes, so it gets far longer than connecting does.
	initCtx, cancelInit := context.WithTimeout(ctx, 30*time.Minute)
	defer cancelInit()

	if err := backend.Init(initCtx); err != nil {
		return nil, err
	}

	store := store.NewStatefulStore(backend, cache.New(30*time.Minute, 1*time.Hour))
	return store, nil
}

func main() {
	cfg, err := config.Load(configPath())
	if err != nil {
		fmt.Println("Failed to load the config: ", err)
		os.Exit(1)
	}

	if cfg.Discord.Token == "" {
		fmt.Println("No Discord token. Set BOETEA_DISCORD_TOKEN or discord.token in config.json.")
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

	ownerID, err := configID(cfg.Discord.AuthorID)
	if err != nil {
		log.Fatalf("invalid discord.author_id: %v", err)
	}

	devGuildID, err := configID(cfg.Discord.DevGuildID)
	if err != nil {
		log.Fatalf("invalid discord.dev_guild_id: %v", err)
	}

	client, err := newClient(cfg.Discord.Token, slogger)
	if err != nil {
		log.Fatal(err)
	}

	b := bot.New(cfg, client, store, log, repostDetector)
	b.Sender = sender.NewDiscordSender(client, log)

	b.AddProvider(twitter.New())
	b.AddProvider(deviant.New())
	b.AddProvider(bluesky.New())

	spool.Configure(cfg.Media.SpoolConfig())

	if cfg.Debug.PprofPort > 0 {
		diag.Start(ctx, log, cfg.Debug.PprofPort, diag.DefaultDir())
	}

	if err := pixiv.LoadAuth(cfg.Pixiv.AuthToken, cfg.Pixiv.RefreshToken); err == nil {
		log.Info("Successfully logged into Pixiv.")
		b.AddProvider(pixiv.New(cfg.Pixiv.ProxyHost))
	}

	r := gumi.New(gumi.Config{
		PrefixResolver: handlers.PrefixResolver(b),
		OwnerIDs:       []snowflake.ID{ownerID},
		Fallback:       handlers.OnMessage(b),
		ErrorHandler:   handlers.OnError(b),
		AllowedMentions: &discord.AllowedMentions{
			Parse: []discord.AllowedMentionType{
				discord.AllowedMentionTypeEveryone,
				discord.AllowedMentionTypeRoles,
				discord.AllowedMentionTypeUsers,
			},
		},
		DevGuildID: devGuildID,
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

// newClient asks Discord for the recommended shard count, so it needs a
// real token and a connection.
func newClient(token string, log *slog.Logger) (*disgobot.Client, error) {
	var (
		client *disgobot.Client
		err    error
	)

	client, err = disgo.New(token,
		disgobot.WithLogger(log),
		disgobot.WithShardManagerConfigOpts(
			sharding.WithAutoScaling(true),
			sharding.WithGatewayConfigOpts(
				gateway.WithIntents(gateway.IntentsNonPrivileged, gateway.IntentMessageContent),
			),
		),
		disgobot.WithCacheConfigOpts(
			disgocache.WithCaches(disgocache.FlagGuilds, disgocache.FlagChannels, disgocache.FlagRoles, disgocache.FlagMembers),
			// Permission checks only need the bot's own member.
			disgocache.WithMemberCachePolicy(func(m discord.Member) bool {
				return m.User.ID == client.ID()
			}),
		),
		// Listeners run on the shard's read loop otherwise, and one slow
		// handler would stall the whole shard.
		disgobot.WithEventManagerConfigOpts(disgobot.WithAsyncEventsEnabled()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a Discord client: %w", err)
	}

	return client, nil
}

func configPath() string {
	if path := os.Getenv("BOETEA_CONFIG"); path != "" {
		return path
	}

	return "config.json"
}

// configID is 0 for "", which the config uses for no ID.
func configID(raw string) (snowflake.ID, error) {
	if raw == "" {
		return 0, nil
	}

	return snowflake.Parse(raw)
}
