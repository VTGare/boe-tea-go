package stats

import (
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/gumi"
	"go.uber.org/atomic"
)

type Stats struct {
	Commands map[string]*atomic.Int64
	Artworks map[string]*atomic.Int64

	mut sync.RWMutex
}

type Item struct {
	Name  string
	Count int64
}

func New(router *gumi.Router, providers []artworks.Provider) *Stats {
	stats := &Stats{
		Commands: map[string]*atomic.Int64{},
		Artworks: map[string]*atomic.Int64{},
	}

	for command := range router.Commands {
		stats.Commands[command] = atomic.NewInt64(0)
	}

	for _, provider := range providers {
		t := reflect.TypeOf(provider).String()
		t = strings.Split(t, ".")[1]
		stats.Artworks[t] = atomic.NewInt64(0)
	}

	return stats
}

func (m *Stats) IncrementCommand(cmd string) {
	m.mut.Lock()
	defer m.mut.Unlock()

	count, ok := m.Commands[cmd]
	if !ok {
		count = atomic.NewInt64(0)
		m.Commands[cmd] = count
	}

	count.Add(1)
}

func (m *Stats) IncrementArtwork(provider artworks.Provider) {
	m.mut.Lock()
	defer m.mut.Unlock()

	t := reflect.TypeOf(provider).String()
	t = strings.Split(t, ".")[1]

	count, ok := m.Artworks[t]
	if !ok {
		count = atomic.NewInt64(0)
		m.Commands[t] = count
	}

	count.Add(1)
}

func (m *Stats) CommandStats() ([]Item, int64) {
	m.mut.RLock()
	defer m.mut.RUnlock()

	return stats(m.Commands)
}

func (m *Stats) ArtworkStats() ([]Item, int64) {
	m.mut.RLock()
	defer m.mut.RUnlock()

	return stats(m.Artworks)
}

func stats(m map[string]*atomic.Int64) ([]Item, int64) {
	var (
		items = make([]Item, 0, len(m))
		total int64
	)

	for name, count := range m {
		c := count.Load()
		items = append(items, Item{
			Name:  name,
			Count: c,
		})

		total += c
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})

	return items, total
}
