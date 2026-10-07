package post

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
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

// Send posts the artworks for run.
func (r *Poster) Send(ctx context.Context, run Post) ([]*cache.MessageInfo, error) {
	guild, err := r.deps.Guilds.Guild(ctx, dgoutils.IDString(run.GuildID))
	if err != nil {
		return nil, fmt.Errorf("failed to get a guild: %w", err)
	}

	if guild == nil {
		return nil, fmt.Errorf("failed to get a guild: not found")
	}

	user, err := r.deps.Users.User(ctx, run.AuthorID.String())
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

	// Repost notices are extra: if one fails, log it and carry on.
	_ = r.notifyReposts(guild, run, fetched.reposts, fetched.matched)

	sent := pagesToInfos(pages)

	if group, ok := user.FindGroup(run.ChannelID.String()); user.Crosspost && ok {
		cross, err := r.Crosspost(ctx, run, run.AuthorID, group)
		if err != nil {
			errs = append(errs, err)
		}

		sent = append(sent, cross...)
	}

	return sent, errors.Join(errs...)
}

// Crosspost posts to every channel in group. A failed channel doesn't
// stop the others; all errors are returned together.
func (r *Poster) Crosspost(ctx context.Context, run Post, userID snowflake.ID, group *store.Group) ([]*cache.MessageInfo, error) {
	user, err := r.deps.Users.User(ctx, userID.String())
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fmt.Errorf("failed to get a user: not found")
	}

	if user.Ignore && !run.IsCommand {
		return []*cache.MessageInfo{}, nil
	}

	children := make([]snowflake.ID, 0, len(group.Children))
	for _, child := range group.Children {
		if id := dgoutils.ParseID(child); id != 0 {
			children = append(children, id)
		}
	}

	children = slices.DeleteFunc(children, func(channelID snowflake.ID) bool {
		return slices.Contains(run.ExcludedChannels, channelID) || (group.IsPair && channelID == run.ChannelID)
	})

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

func CacheResult(ec *cache.EmbedCache, authorID, channelID, messageID snowflake.ID, sent []*cache.MessageInfo) {
	if ec == nil || len(sent) == 0 {
		return
	}

	ec.Set(authorID.String(), channelID.String(), messageID.String(), true, sent...)

	for _, msg := range sent {
		if msg == nil {
			continue
		}

		ec.Set(authorID.String(), msg.ChannelID, msg.MessageID, false)
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

// RunFromMessage builds the immutable run for one triggering message.
// Skip filters and channel exclusions are set by the caller.
func RunFromMessage(msg *discord.Message, urls []string, isCommand bool) Post {
	run := Post{
		URLs: append([]string(nil), urls...),
		Skip: SkipFilter{Indices: make(map[int]struct{})},
	}

	if msg == nil {
		return run
	}

	if msg.GuildID != nil {
		run.GuildID = *msg.GuildID
	}

	run.ChannelID = msg.ChannelID
	run.MessageID = msg.ID
	run.IsCommand = isCommand
	run.AuthorID = msg.Author.ID
	run.AuthorName = msg.Author.Username
	run.AuthorAvatar = msg.Author.EffectiveAvatarURL()

	return run
}
