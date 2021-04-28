package metrics

import (
	"sync/atomic"
	"time"
)

type Metrics struct {
	CommandsExecuted int64
	ArtworksSent     int64
	Uptime           time.Duration
}

func New() *Metrics {
	metrics := &Metrics{}
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for range ticker.C {
			metrics.Uptime += 1 * time.Second
		}
	}()

	return metrics
}

func (m *Metrics) IncrementCommand() {
	atomic.AddInt64(&m.CommandsExecuted, 1)
}

func (m *Metrics) IncrementArtwork() {
	atomic.AddInt64(&m.ArtworksSent, 1)
}
