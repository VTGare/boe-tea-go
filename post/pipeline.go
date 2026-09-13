package post

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/artworks/render"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/sync/errgroup"
)

type fetchedItem struct {
	artwork  artworks.Artwork
	provider artworks.Provider
}

// fetchResults is one fetch run, in input-URL order.
type fetchResults struct {
	items         []fetchedItem
	reposts       []*repost.Repost
	matched       int
	originTwitter bool
}

type fetchJob struct {
	url      string
	id       string
	provider artworks.Provider
}

// fetchSlot is one job's outcome; rep and artwork can coexist.
type fetchSlot struct {
	artwork  artworks.Artwork
	provider artworks.Provider
	rep      *repost.Repost
	err      error
}

// fetch resolves URLs in input order; reposts record only after successful fetch.
func (r *Poster) fetch(ctx context.Context, guild *store.Guild, channelID string, urls []string, opts runOpts) (fetchResults, error) {
	log := r.log.With(
		"guild_id", guild.ID,
		"channel_id", channelID,
	)

	if ok, err := r.deps.Sender.HasChannelPerms(guild.ID, channelID, sender.SendPermissions); !ok {
		log.Warn("skipping fetch, missing send permissions")

		return fetchResults{}, nil
	} else if err != nil {
		log.With("error", err).Debug("permission lookup failed, attempting fetch")
	}

	matched := make(map[string]struct{})
	jobs := make([]fetchJob, 0, len(urls))

	for _, url := range urls {
		id, provider := r.match(url)
		if provider == nil {
			continue
		}

		if _, ok := matched[id]; ok {
			continue
		}

		matched[id] = struct{}{}

		jobs = append(jobs, fetchJob{
			url:      url,
			id:       id,
			provider: provider,
		})
	}

	slots := make([]fetchSlot, len(jobs))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(r.maxConc)

	for i := range jobs {
		g.Go(func() error {
			slots[i] = r.doFetch(ctx, guild, channelID, jobs[i], opts)

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fetchResults{}, err
	}

	results := fetchResults{
		items:   make([]fetchedItem, 0, len(jobs)),
		reposts: make([]*repost.Repost, 0),
		matched: len(matched),
	}

	errs := make([]error, 0)

	for _, slot := range slots {
		if slot.rep != nil {
			results.reposts = append(results.reposts, slot.rep)
		}

		switch {
		case slot.artwork != nil:
			results.items = append(results.items, fetchedItem{
				artwork:  slot.artwork,
				provider: slot.provider,
			})

			if _, ok := slot.provider.(*twitter.Twitter); ok && slot.artwork.Len() > 0 {
				results.originTwitter = true
			}
		case slot.err != nil:
			errs = append(errs, slot.err)
		}
	}

	return results, errors.Join(errs...)
}

func (r *Poster) match(url string) (string, artworks.Provider) {
	if r.deps.Match == nil {
		return "", nil
	}

	return r.deps.Match(url)
}

func (r *Poster) doFetch(ctx context.Context, guild *store.Guild, channelID string, job fetchJob, opts runOpts) fetchSlot {
	log := r.log.With(
		"guild_id", guild.ID,
		"channel_id", channelID,
		"provider", fmt.Sprintf("%T", job.provider),
		"url", job.url,
	)

	log.Debug("matched a url")

	var slot fetchSlot

	needsCreate := false

	if guild.Repost != store.GuildRepostDisabled {
		rep, err := r.deps.Reposts.Find(ctx, channelID, job.id)
		if err != nil && !errors.Is(err, repost.ErrNotFound) {
			log.With("error", err).Error("failed to find a repost")
		}

		if rep != nil {
			slot.rep = rep

			if opts.isCrosspost || guild.Repost == store.GuildRepostStrict {
				return slot
			}
		} else {
			needsCreate = true
		}
	}

	_, isTwitter := job.provider.(*twitter.Twitter)

	// Twitter crossposts bypass guild settings by design.
	if !job.provider.Enabled(guild) && !opts.isCommand && !(opts.isCrosspost && isTwitter) {
		return slot
	}

	artwork, err := r.getOrFetch(job.provider, job.id)
	if err != nil {
		slot.err = err

		return slot
	}

	// Auto-posts should drop imageless tweets.
	if !opts.isCommand {
		if tweet, ok := artwork.(*twitter.Artwork); ok && tweet.Len() == 0 {
			log.Debug("skipping imageless tweet outside commands")

			return slot
		}
	}

	slot.artwork = artwork
	slot.provider = job.provider

	if needsCreate {
		rep := &repost.Repost{
			ID:        job.id,
			URL:       job.url,
			GuildID:   guild.ID,
			ChannelID: channelID,
			MessageID: opts.messageID,
		}

		if err := r.deps.Reposts.Create(ctx, rep, guild.RepostExpiration); err != nil {
			log.With("error", err).Error("error creating a repost")
		}
	}

	return slot
}

func (r *Poster) getOrFetch(provider artworks.Provider, id string) (artworks.Artwork, error) {
	key := fmt.Sprintf("%T:%v", provider, id)

	if r.deps.ArtworkCache != nil {
		if cached, ok := r.deps.ArtworkCache.Get(key); ok {
			if artwork, ok := cached.(artworks.Artwork); ok && artwork != nil {
				return artwork, nil
			}
		}
	}

	artwork, err := provider.Find(id)
	if err != nil {
		return nil, err
	}

	if r.deps.ArtworkCache != nil {
		r.deps.ArtworkCache.Set(key, artwork, 0)
	}

	return artwork, nil
}

type sentPage struct {
	info     *cache.MessageInfo
	msg      *discordgo.Message
	embedURL string
}

// deliver renders and sends every page best-effort, failures join.
func (r *Poster) deliver(guild *store.Guild, channelID string, items []fetchedItem, run Post, opts runOpts) ([]sentPage, error) {
	artworks := make([]artworks.Artwork, 0, len(items))
	for _, item := range items {
		if item.artwork == nil {
			continue
		}

		artworks = append(artworks, item.artwork)
	}

	if len(artworks) == 0 {
		return nil, nil
	}

	bundles, err := r.generateMessages(guild, items, run, opts)
	if err != nil {
		return nil, &renderError{err}
	}

	if len(bundles) == 0 {
		return nil, nil
	}

	// Only the first bundle is filtered since this code path can only be reached through a command.
	bundles[0].Sends = applySkip(bundles[0].Sends, run.Skip)
	bundles = applyLimit(bundles, guild.Limit)

	if opts.isCrosspost && len(bundles) > 0 && len(bundles[0].Sends) > 0 {
		first := bundles[0].Sends[0]
		if first != nil && len(first.Embeds) > 0 && first.Embeds[0] != nil {
			first.Content = first.Embeds[0].URL + "\n" + first.Content
		}
	}

	log := r.log.With(
		"guild_id", guild.ID,
		"channel_id", channelID,
		"crosspost", opts.isCrosspost,
	)

	pages := make([]sentPage, 0)
	errs := make([]error, 0)

	for _, bundle := range bundles {
		for _, message := range bundle.Sends {
			if message == nil {
				continue
			}

			msg, err := r.deps.Sender.SendComplex(guild.ID, channelID, message)
			if err != nil {
				if errors.Is(err, sender.ErrSkipped) {
					continue
				}

				if kind := classify(err); kind == KindNoPerms {
					log.With("channel_id", channelID).Debug("skipping send, missing permissions")
				} else {
					log.With("error", err).Warn("failed to send artwork message")
				}

				errs = append(errs, &Error{Kind: classify(err), Cause: err})

				continue
			}

			var embedURL string
			if len(message.Embeds) > 0 && message.Embeds[0] != nil {
				embedURL = message.Embeds[0].URL
			}

			pages = append(pages, sentPage{
				info:     &cache.MessageInfo{MessageID: msg.ID, ChannelID: msg.ChannelID, ArtworkID: bundle.ID},
				msg:      msg,
				embedURL: embedURL,
			})
		}
	}

	return pages, errors.Join(errs...)
}

func (r *Poster) generateMessages(guild *store.Guild, items []fetchedItem, run Post, opts runOpts) ([]render.Bundle, error) {
	inputs := make([]render.Input, 0, len(items))

	for _, item := range items {
		if item.artwork == nil {
			continue
		}

		var quote string
		if guild.FlavorText && r.deps.RandomQuote != nil {
			quote = r.deps.RandomQuote(guild.NSFW)
		}

		rendered, err := item.artwork.Render()
		if err != nil {
			return nil, err
		}

		inputs = append(inputs, render.Input{
			ID:            item.artwork.ID(),
			Footer:        quote,
			Rendered:      rendered,
			SkipFirstPage: skipFirst(guild, item.artwork, opts),
		})
	}

	return render.Build(inputs, renderOptions(guild, run, opts)), nil
}

func renderOptions(guild *store.Guild, run Post, opts runOpts) render.Options {
	renderOpts := render.Options{
		TagsEnabled: guild.Tags,
		Crosspost:   opts.isCrosspost,
	}

	if opts.isCrosspost {
		renderOpts.AuthorName = messages.CrosspostBy(run.AuthorName)
		renderOpts.AuthorIconURL = run.AuthorAvatar
	} else {
		renderOpts.Reference = &discordgo.MessageReference{
			GuildID:   run.GuildID,
			ChannelID: run.ChannelID,
			MessageID: run.MessageID,
		}
	}

	return renderOpts
}

func skipFirst(guild *store.Guild, a artworks.Artwork, opts runOpts) bool {
	if !guild.SkipFirst {
		return false
	}

	if opts.isCommand {
		return false
	}

	tweet, isTwitter := a.(*twitter.Artwork)
	if !isTwitter {
		return false
	}

	if a.Len() == 0 {
		return true
	}

	if len(tweet.Videos) > 0 || opts.isCrosspost {
		return false
	}

	return true
}

func applySkip(sends []*discordgo.MessageSend, skip SkipFilter) []*discordgo.MessageSend {
	if skip.Mode == SkipModeNone || len(skip.Indices) == 0 {
		return sends
	}

	filtered := make([]*discordgo.MessageSend, 0, len(sends))

	switch skip.Mode {
	case SkipModeExclude:
		for ind, val := range sends {
			if _, ok := skip.Indices[ind+1]; !ok {
				filtered = append(filtered, val)
			}
		}
	case SkipModeInclude:
		for ind, val := range sends {
			if _, ok := skip.Indices[ind+1]; ok {
				filtered = append(filtered, val)
			}
		}
	default:
		return sends
	}

	return filtered
}

// applyLimit truncates over-limit albums.
func applyLimit(bundles []render.Bundle, limit int) []render.Bundle {
	count := 0
	for _, bundle := range bundles {
		count += len(bundle.Sends)
	}

	if count <= limit {
		return bundles
	}

	if len(bundles) == 0 || len(bundles[0].Sends) == 0 || bundles[0].Sends[0] == nil {
		return bundles
	}

	if limit < 0 {
		limit = 0
	}

	bundles[0].Sends[0].Content = messages.LimitExceeded(limit, len(bundles), count)

	if len(bundles) == 1 {
		if limit < len(bundles[0].Sends) {
			bundles[0].Sends = bundles[0].Sends[:limit]
		}

		return bundles
	}

	filtered := make([]render.Bundle, 0, limit)
	for _, bundle := range bundles {
		if len(bundle.Sends) > 0 {
			filtered = append(filtered, render.Bundle{ID: bundle.ID, Sends: []*discordgo.MessageSend{bundle.Sends[0]}})
		}
	}

	return filtered
}

func (r *Poster) notifyReposts(guild *store.Guild, run Post, reps []*repost.Repost, matched int) error {
	if len(reps) == 0 {
		return nil
	}

	log := r.log.With(
		"guild_id", guild.ID,
		"user_id", run.AuthorID,
	)

	errs := make([]error, 0)

	if guild.Repost == store.GuildRepostStrict {
		perm, err := r.deps.Sender.BotHasGuildPerms(
			guild.ID,
			discordgo.PermissionAdministrator|discordgo.PermissionManageMessages,
		)
		if err != nil {
			log.With("error", err).Warn("failed to check delete message perms")
		}

		if perm && matched == len(reps) {
			if err := r.deps.Sender.DeleteMessage(guild.ID, run.ChannelID, run.MessageID); err != nil {
				log.With(
					"channel_id", run.ChannelID,
					"message_id", run.MessageID,
				).Warn("failed to delete original repost message")

				errs = append(errs, fmt.Errorf("failed to delete original repost message: %w", err))
			}
		}
	}

	locale := messages.RepostEmbed()

	eb := embeds.NewBuilder()
	eb.Title(locale.Title)

	for ind, rep := range reps {
		eb.AddField(
			fmt.Sprintf("#%v | %v", ind+1, rep.ID),
			fmt.Sprintf(
				"**%v %v**\n**URL:** %v\n\n%v",
				locale.Expires, messages.RelativeTimestamp(rep.ExpiresAt),
				rep.URL,
				messages.NamedLink(
					locale.OriginalMessage,
					fmt.Sprintf("https://discord.com/channels/%v/%v/%v", rep.GuildID, rep.ChannelID, rep.MessageID),
				),
			),
		)
	}

	if ok, err := r.deps.Sender.HasChannelPerms(guild.ID, run.ChannelID, sender.SendPermissions); !ok {
		log.Warn("skipping repost message, missing send permissions")

		return errors.Join(errs...)
	} else if err != nil {
		log.With("error", err).Debug("permission lookup failed, attempting send")
	}

	repostMessage, err := r.deps.Sender.SendEmbed(guild.ID, run.ChannelID, eb.Finalize())
	if err != nil {
		log.With("error", err).Warn("failed to send repost message")

		return errors.Join(append(errs, fmt.Errorf("failed to send repost message: %w", err))...)
	}

	r.deps.Sender.Expire(repostMessage)

	return errors.Join(errs...)
}

// finalize runs the post-send side effects.
func (r *Poster) finalize(run Post, guild *store.Guild, pages []sentPage, fetched fetchResults, opts runOpts) error {
	if r.deps.RecordArtwork != nil {
		for _, item := range fetched.items {
			if item.provider == nil {
				continue
			}

			r.deps.RecordArtwork(item.provider)
		}
	}

	errs := make([]error, 0)

	if guild.Reactions && !opts.isCommand && !opts.isCrosspost && fetched.originTwitter {
		if err := r.addBookmarkReactions(run.GuildID, run.ChannelID, run.MessageID); err != nil {
			r.log.With("error", err).Debug("failed to add bookmark reactions")

			errs = append(errs, fmt.Errorf("failed to add bookmark reactions: %w", err))
		}
	}

	if guild.Reactions && len(pages) > 0 {
		mediaCount := 0
		for _, item := range fetched.items {
			if item.artwork == nil {
				continue
			}

			mediaCount += item.artwork.Len()
		}

		// Imageless pages (e.g. text-only tweets) can't be bookmarked.
		if mediaCount != 0 {
			for _, page := range pages {
				if page.msg == nil || page.embedURL == "" {
					continue
				}

				err := r.addBookmarkReactions(page.msg.GuildID, page.msg.ChannelID, page.msg.ID)
				if err != nil && !strings.Contains(err.Error(), "403") {
					wrapped := fmt.Errorf("failed to add reactions: %w", err)

					errs = append(errs, &Error{Kind: classify(wrapped), Cause: wrapped})
				}
			}
		}
	}

	return errors.Join(errs...)
}

func (r *Poster) addBookmarkReactions(guildID, channelID, messageID string) error {
	for _, reaction := range []string{"💖", "🤤"} {
		if err := r.deps.Sender.AddReaction(guildID, channelID, messageID, reaction); err != nil {
			return err
		}
	}

	return nil
}

type crossSlot struct {
	infos []*cache.MessageInfo
	err   error
}

func (r *Poster) crosspostOne(ctx context.Context, run Post, userID, groupName, channelID string) crossSlot {
	log := r.log.With(
		"user_id", userID,
		"group", groupName,
		"channel_id", channelID,
	)

	guildID, err := r.deps.Sender.ChannelGuildID(run.GuildID, channelID)
	if err != nil {
		log.With("error", err).Info("failed to crosspost")

		return crossSlot{}
	}

	member, err := r.deps.Sender.IsMember(guildID, userID)
	if err != nil {
		log.With("error", err).Info("failed to check membership, keeping crosspost channel")

		return crossSlot{}
	}

	if !member {
		log.Debug("member left the server, removing crosspost channel")

		if _, err := r.deps.Users.DeleteCrosspostChannel(ctx, userID, groupName, channelID); err != nil {
			log.With("error", err).Error("failed to remove a channel from user's group")
		}

		return crossSlot{}
	}

	guild, err := r.deps.Guilds.Guild(ctx, guildID)
	if err != nil {
		log.With("error", err).Info("failed to find guild")

		return crossSlot{}
	}

	if guild == nil {
		log.Info("failed to find guild")

		return crossSlot{}
	}

	if !guild.Crosspost {
		return crossSlot{}
	}

	if len(guild.ArtChannels) != 0 && !slices.Contains(guild.ArtChannels, channelID) {
		return crossSlot{}
	}

	opts := run.crosspostOpts()
	errs := make([]error, 0)

	fetched, err := r.fetch(ctx, guild, channelID, run.URLs, opts)
	if err != nil {
		log.With("error", err).Error("failed to fetch artworks")

		errs = append(errs, fmt.Errorf("failed to fetch artworks: %w", err))
	}

	pages, err := r.deliver(guild, channelID, fetched.items, run, opts)
	if err != nil {
		if _, ok := errors.AsType[*renderError](err); ok {
			log.With("error", err).Error("failed to render artworks")

			errs = append(errs, err)

			return crossSlot{err: errors.Join(errs...)}
		}

		log.With("error", err).Error("failed to send messages")

		errs = append(errs, err)
	}

	if err := r.finalize(run, guild, pages, fetched, opts); err != nil {
		errs = append(errs, err)
	}

	return crossSlot{
		infos: pagesToInfos(pages),
		err:   errors.Join(errs...),
	}
}
