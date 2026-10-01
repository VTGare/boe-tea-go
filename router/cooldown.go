package router

import (
	"fmt"
	"sync"
	"time"
)

// CooldownScope determines which key a cooldown bucket is tied to.
type CooldownScope int

const (
	// CooldownUser limits each user independently (default).
	CooldownUser CooldownScope = iota
	// CooldownChannel limits each channel independently.
	CooldownChannel
	// CooldownGuild limits each guild independently (DMs are keyed by channel).
	CooldownGuild
	// CooldownGlobal limits the command for everyone.
	CooldownGlobal
)

func (s CooldownScope) String() string {
	switch s {
	case CooldownUser:
		return "user"
	case CooldownChannel:
		return "channel"
	case CooldownGuild:
		return "guild"
	case CooldownGlobal:
		return "global"
	}
	return "unknown"
}

// Cooldown is a fixed-window rate limiter: Uses invocations per Per for
// every key in Scope (Uses defaults to 1). Safe for concurrent use.
// Set it on a group to cover its subcommands:
//
//	Cooldown: &router.Cooldown{Uses: 2, Per: 10 * time.Second}
type Cooldown struct {
	Scope CooldownScope
	Uses  int
	Per   time.Duration

	mu        sync.Mutex
	buckets   map[string]*cooldownBucket
	lastSweep time.Time
}

type cooldownBucket struct {
	count int
	reset time.Time
}

func NewCooldown(scope CooldownScope, uses int, per time.Duration) *Cooldown {
	return &Cooldown{Scope: scope, Uses: uses, Per: per}
}

func (c *Cooldown) Key(ctx *Context) string {
	switch c.Scope {
	case CooldownChannel:
		return ctx.ChannelID()
	case CooldownGuild:
		if g := ctx.GuildID(); g != "" {
			return g
		}
		return ctx.ChannelID()
	case CooldownGlobal:
		return ""
	default:
		return ctx.AuthorID()
	}
}

// Take spends one use; it reports how long to wait when limited.
func (c *Cooldown) Take(key string) (time.Duration, bool) {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.buckets == nil {
		c.buckets = make(map[string]*cooldownBucket)
	}

	c.sweep(now)

	b, ok := c.buckets[key]
	if !ok || now.After(b.reset) {
		c.buckets[key] = &cooldownBucket{count: 1, reset: now.Add(c.Per)}
		return 0, true
	}

	if b.count >= c.uses() {
		return b.reset.Sub(now), false
	}

	b.count++
	return 0, true
}

func (c *Cooldown) Reset(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.buckets, key)
}

func (c *Cooldown) sweep(now time.Time) {
	if now.Sub(c.lastSweep) < time.Minute && len(c.buckets) < 4096 {
		return
	}

	c.lastSweep = now
	for k, b := range c.buckets {
		if now.After(b.reset) {
			delete(c.buckets, k)
		}
	}
}

// uses is Uses, defaulting to 1.
func (c *Cooldown) uses() int {
	return max(c.Uses, 1)
}

func (c *Cooldown) String() string {
	uses := c.uses()
	noun := "uses"
	if uses == 1 {
		noun = "use"
	}

	return fmt.Sprintf("%d %s per %s (per %s)", uses, noun, c.Per, c.Scope)
}
