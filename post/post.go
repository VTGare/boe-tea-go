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

// Post is one immutable artwork-posting run, safe to share across sends.
type Post struct {
	GuildID   string
	ChannelID string
	MessageID string

	AuthorID     string
	AuthorName   string
	AuthorAvatar string

	IsCommand bool

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
