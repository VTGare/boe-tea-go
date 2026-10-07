package spool

import (
	"io"
	"os"
	"path/filepath"

	"github.com/disgoorg/disgo/discord"
)

// Config limits how many media payloads are in flight and where they're
// spooled on disk. Files are deleted right after sending, so nothing
// piles up. Zero values mean defaults.
type Config struct {
	// MaxConcurrent caps how many payloads are in flight at once.
	MaxConcurrent int

	Dir string
}

// DefaultMaxConcurrent is the default Config.MaxConcurrent.
const DefaultMaxConcurrent = 5

// DefaultDir returns the built-in spool directory.
func DefaultDir() string {
	return filepath.Join(os.TempDir(), "boe-tea-spool")
}

// DefaultConfig returns the built-in config.
func DefaultConfig() Config {
	return Config{
		MaxConcurrent: DefaultMaxConcurrent,
		Dir:           DefaultDir(),
	}
}

var (
	slots = make(chan struct{}, DefaultMaxConcurrent)
	dir   = DefaultDir()
)

// Configure applies the config, using defaults for zero values, creates
// the directory and deletes files left over from earlier runs. Call it
// once at startup. It returns the config actually in use.
func Configure(c Config) Config {
	defaults := DefaultConfig()

	if c.MaxConcurrent > 0 {
		defaults.MaxConcurrent = c.MaxConcurrent
	}

	if c.Dir != "" {
		defaults.Dir = c.Dir
	}

	slots = make(chan struct{}, defaults.MaxConcurrent)
	dir = defaults.Dir

	_ = os.MkdirAll(dir, 0o755)
	Sweep()

	return defaults
}

// Acquire waits for a free payload slot, so big uploads queue up instead
// of piling up in memory.
func Acquire() {
	slots <- struct{}{}
}

// Release frees a payload slot acquired with Acquire.
func Release() {
	<-slots
}

// TempFile creates a spooled temp file.
func TempFile(pattern string) (*os.File, error) {
	_ = os.MkdirAll(dir, 0o755)

	return os.CreateTemp(dir, pattern)
}

// Download streams src to a spooled temp file and rewinds it for
// reading.
func Download(pattern string, src io.Reader) (*os.File, error) {
	tmp, err := TempFile(pattern)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		Remove(tmp.Name())
		return nil, err
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		Remove(tmp.Name())
		return nil, err
	}

	return tmp, nil
}

// Remove deletes path. It is a no-op for paths outside the spool directory.
func Remove(path string) {
	if filepath.Dir(path) != dir {
		return
	}

	_ = os.Remove(path)
}

// RemoveFiles closes and deletes the spooled files backing the given
// discord files. Files that were not spooled are left alone.
func RemoveFiles(files []*discord.File) {
	for _, file := range files {
		if file == nil {
			continue
		}

		spooled, ok := file.Reader.(*os.File)
		if !ok {
			continue
		}

		spooled.Close()
		Remove(spooled.Name())
	}
}

// Sweep deletes everything in the spool directory. It runs at startup,
// when nothing there can still be in use.
func Sweep() {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}
