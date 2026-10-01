package post

type SkipMode int

const (
	SkipModeNone SkipMode = iota
	SkipModeInclude
	SkipModeExclude
)

type SkipFilter struct {
	Mode    SkipMode
	Indices map[int]struct{}
}

// Post describes one artwork-posting run. It's never modified, so it's
// safe to share between sends.
type Post struct {
	GuildID   string
	ChannelID string
	MessageID string

	AuthorID     string
	AuthorName   string
	AuthorAvatar string

	IsCommand bool

	// IsInteraction marks runs triggered by a slash command, where there
	// is no message to reference or delete.
	IsInteraction bool

	URLs []string
	Skip SkipFilter

	ExcludedChannels []string
}

type runOpts struct {
	isCommand   bool
	isCrosspost bool
	messageID   string
}

func (p Post) optsFor() runOpts {
	return runOpts{
		isCommand: p.IsCommand,
		messageID: p.MessageID,
	}
}

func (p Post) crosspostOpts() runOpts {
	return runOpts{
		isCommand:   p.IsCommand,
		isCrosspost: true,
		messageID:   p.MessageID,
	}
}
