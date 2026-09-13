package post

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi"
	goCache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const defaultMaxConcurrency = 8

type GuildStore interface {
	Guild(ctx context.Context, guildID string) (*store.Guild, error)
}

type UserStore interface {
	User(ctx context.Context, userID string) (*store.User, error)
	DeleteCrosspostChannel(ctx context.Context, userID, group, channel string) (*store.User, error)
}

type Deps struct {
	Guilds GuildStore
	Users  UserStore

	Match func(url string) (string, artworks.Provider)

	Reposts      repost.Detector
	ArtworkCache *goCache.Cache

	Sender sender.Sender

	Log *zap.SugaredLogger

	RandomQuote    func(nsfw bool) string
	RecordArtwork  func(artworks.Provider)
	MaxConcurrency int
}

type Poster struct {
	deps    Deps
	log     *zap.SugaredLogger
	maxConc int
}

func NewPoster(deps Deps) *Poster {
	log := deps.Log
	if log == nil {
		log = zap.NewNop().Sugar()
	}

	maxConc := deps.MaxConcurrency
	if maxConc < 1 {
		maxConc = defaultMaxConcurrency
	}

	return &Poster{
		deps:    deps,
		log:     log,
		maxConc: maxConc,
	}
}

// Send runs the pipeline.
func (r *Poster) Send(ctx context.Context, run Post) ([]*cache.MessageInfo, error) {
	guild, err := r.deps.Guilds.Guild(ctx, run.GuildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get a guild: %w", err)
	}

	if guild == nil {
		return nil, fmt.Errorf("failed to get a guild: not found")
	}

	user, err := r.deps.Users.User(ctx, run.AuthorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get a user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("failed to get a user: not found")
	}

	if user.Ignore && !run.IsCommand {
		return nil, nil
	}

	opts := run.optsFor()
	errs := make([]error, 0)

	fetched, err := r.fetch(ctx, guild, run.ChannelID, run.URLs, opts)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to fetch artworks: %w", err))
	}

	pages, err := r.deliver(guild, run.ChannelID, fetched.items, run, opts)
	if err != nil {
		if _, ok := errors.AsType[*renderError](err); ok {
			errs = append(errs, err)

			return nil, errors.Join(errs...)
		}

		errs = append(errs, err)
	}

	if err := r.finalize(run, guild, pages, fetched, opts); err != nil {
		errs = append(errs, err)
	}

	// Repost notices are auxiliary. Failures stay in the logs, never fail the run.
	_ = r.notifyReposts(guild, run, fetched.reposts, fetched.matched)

	sent := pagesToInfos(pages)

	if group, ok := user.FindGroup(run.ChannelID); user.Crosspost && ok {
		cross, err := r.Crosspost(ctx, run, user.ID, group)
		if err != nil {
			errs = append(errs, err)
		}

		sent = append(sent, cross...)
	}

	return sent, errors.Join(errs...)
}

// Crosspost fans out without mutating group; per-channel failures join.
func (r *Poster) Crosspost(ctx context.Context, run Post, userID string, group *store.Group) ([]*cache.MessageInfo, error) {
	user, err := r.deps.Users.User(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fmt.Errorf("failed to get a user: not found")
	}

	if user.Ignore && !run.IsCommand {
		return []*cache.MessageInfo{}, nil
	}

	children := append([]string(nil), group.Children...)

	if len(run.ExcludedChannels) > 0 {
		excluded := make(map[string]struct{}, len(run.ExcludedChannels))
		for _, channelID := range run.ExcludedChannels {
			excluded[channelID] = struct{}{}
		}

		children = slices.DeleteFunc(children, func(channelID string) bool {
			_, ok := excluded[channelID]

			return ok
		})
	}

	if group.IsPair {
		children = slices.DeleteFunc(children, func(channelID string) bool {
			return channelID == run.ChannelID
		})
	}

	slots := make([]crossSlot, len(children))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(r.maxConc)

	for i := range children {
		g.Go(func() error {
			slots[i] = r.crosspostOne(ctx, run, userID, group.Name, children[i])

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	sent := make([]*cache.MessageInfo, 0)
	errs := make([]error, 0)

	for _, slot := range slots {
		sent = append(sent, slot.infos...)

		if slot.err != nil {
			errs = append(errs, slot.err)
		}
	}

	return sent, errors.Join(errs...)
}

func CacheResult(ec *cache.EmbedCache, authorID, channelID, messageID string, sent []*cache.MessageInfo) {
	if ec == nil || len(sent) == 0 {
		return
	}

	ec.Set(authorID, channelID, messageID, true, sent...)

	for _, msg := range sent {
		if msg == nil {
			continue
		}

		ec.Set(authorID, msg.ChannelID, msg.MessageID, false)
	}
}

func pagesToInfos(pages []sentPage) []*cache.MessageInfo {
	sent := make([]*cache.MessageInfo, 0, len(pages))

	for _, page := range pages {
		if page.info == nil {
			continue
		}

		sent = append(sent, page.info)
	}

	return sent
}

// DepsFromBot wires a Poster from a running bot.
func DepsFromBot(b *bot.Bot) Deps {
	deps := Deps{
		Guilds: b.Store,
		Users:  b.Store,
		Match:  b.Match,
	}

	deps.Reposts = b.RepostDetector
	deps.ArtworkCache = b.ArtworkCache
	deps.Sender = b.Sender
	deps.Log = b.Log

	if b.Config != nil {
		deps.RandomQuote = b.Config.RandomQuote
	}

	if b.Stats != nil {
		deps.RecordArtwork = b.Stats.IncrementArtwork
	}

	return deps
}

// RunFromEvent builds the immutable run for one triggering message.
// Skip filters and channel exclusions are set by the caller.
func RunFromEvent(gctx *gumi.Ctx, urls []string) Post {
	run := Post{
		URLs: append([]string(nil), urls...),
		Skip: SkipFilter{Indices: make(map[int]struct{})},
	}

	if gctx == nil || gctx.Event == nil || gctx.Event.Message == nil {
		return run
	}

	run.GuildID = gctx.Event.GuildID
	run.ChannelID = gctx.Event.ChannelID
	run.MessageID = gctx.Event.ID
	run.IsCommand = gctx.Command != nil

	if author := gctx.Event.Message.Author; author != nil {
		run.AuthorID = author.ID
		run.AuthorName = author.Username
		run.AuthorAvatar = author.AvatarURL("")
	}

	return run
}
