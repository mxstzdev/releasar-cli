package log

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config controls the global manager's output and minimum level.
type Config struct {
	Level              *zerolog.Level // log level; default: info
	Directory          string         // location of the log directory; default: "var/log/releasar"
	Filename           string         // name of the log file; default: "releasar.log"
	Rotating           *bool          // rotates log files every day; default: true
	MaxBackups         int            // number of old files to keep; default: 7
	DefaultChannelName string         // name of the default channel; default: "general"
}

// Manager owns the root logger and all named channels.
type Manager struct {
	mu       sync.RWMutex
	cfg      Config
	ctx      map[string]any
	logger   zerolog.Logger
	channels map[string]*Channel
}

// Channel is a named, scoped logger backed by a zerolog.Logger.
type Channel struct {
	mu     sync.Mutex
	name   string
	ctx    map[string]any
	logger zerolog.Logger
}

var manager = newManager(Config{})

// Prepares the writer for a zerlog logger instance
func makeFileWriter(cfg Config) (io.Writer, error) {
	path := filepath.Join(cfg.Directory, cfg.Filename)

	if cfg.Rotating == nil || *cfg.Rotating {
		return io.MultiWriter(os.Stderr, &lumberjack.Logger{
			Filename:   path,
			MaxSize:    100,
			MaxAge:     30,
			MaxBackups: max(cfg.MaxBackups, 7),
		}), nil
	}

	if err := os.MkdirAll(cfg.Directory, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}

	return io.MultiWriter(os.Stderr, f), nil
}

func newManager(cfg Config) *Manager {
	level := zerolog.InfoLevel

	if cfg.Level != nil {
		level = *cfg.Level
	}

	if cfg.DefaultChannelName == "" {
		cfg.DefaultChannelName = "general"
	}

	if cfg.Directory == "" {
		cfg.Directory = "var/log/releasar"
	}

	if cfg.Filename == "" {
		cfg.Filename = "releasar.log"
	}

	w, err := makeFileWriter(cfg)
	if err != nil {
		w = os.Stderr
	}

	return &Manager{
		cfg:      cfg,
		ctx:      make(map[string]any),
		logger:   zerolog.New(w).Level(level).With().Timestamp().Logger(),
		channels: make(map[string]*Channel),
	}
}

// Init replaces the manager.
func Init(cfg Config) {
	manager = newManager(cfg)
}

// Nop returns a Channel that silently discards all log output at zero allocation cost.
// Intended for tests and callers that do not need logging.
func Nop() *Channel {
	return &Channel{
		logger: zerolog.Nop(),
		ctx:    make(map[string]any),
	}
}

// Get returns the named channel, creating it lazily on first call.
func Get(name string) *Channel {
	return manager.Channel(name)
}

func (m *Manager) defaultChannelName() string {
	return m.cfg.DefaultChannelName
}

// Channel returns a named channel, or the default channel if no name is given.
func (m *Manager) Channel(name ...string) *Channel {
	n := m.defaultChannelName()
	if len(name) > 0 {
		n = name[0]
	}

	m.mu.RLock()
	ch, ok := m.channels[n]
	m.mu.RUnlock()

	if ok {
		return ch
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if ch, ok = m.channels[n]; ok {
		return ch
	}

	ctx := maps.Clone(m.ctx)

	ch = &Channel{
		name:   n,
		ctx:    ctx,
		logger: m.logger.With().Str("channel", n).Logger(),
	}

	m.channels[n] = ch

	return ch
}

// Adds fields to all future log messages across all channels.
func (m *Manager) ShareContext(fields map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	maps.Copy(m.ctx, fields)

	for _, c := range m.channels {
		c.ShareContext(fields)
	}
}

// Removes individual shared context fields from the manager and all channels
func (m *Manager) RemoveContext(keys ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.ctx, k)
	}
	for _, c := range m.channels {
		c.RemoveContext(keys...)
	}
}

// Removes all shared context from the manager and all channels.
func (m *Manager) FlushContext() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ctx = make(map[string]any)

	for _, c := range m.channels {
		c.FlushContext()
	}
}

func (m *Manager) Trace(msg string, ctx ...map[string]any) { m.Channel().Trace(msg, ctx...) }
func (m *Manager) Debug(msg string, ctx ...map[string]any) { m.Channel().Debug(msg, ctx...) }
func (m *Manager) Info(msg string, ctx ...map[string]any)  { m.Channel().Info(msg, ctx...) }
func (m *Manager) Warn(msg string, ctx ...map[string]any)  { m.Channel().Warn(msg, ctx...) }
func (m *Manager) Error(msg string, ctx ...map[string]any) { m.Channel().Error(msg, ctx...) }
func (m *Manager) Fatal(msg string, ctx ...map[string]any) { m.Channel().Fatal(msg, ctx...) }

// Logger returns the underlying zerolog.Logger for full API access.
func (c *Channel) Logger() zerolog.Logger {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.logger
}

// Adds fields to all future log messages of the channel.
func (c *Channel) ShareContext(fields map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	maps.Copy(c.ctx, fields)
}

// Removes individual shared context fields from the channel.
func (c *Channel) RemoveContext(keys ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range keys {
		delete(c.ctx, k)
	}
}

// Removes all shared context from the channel.
func (c *Channel) FlushContext() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ctx = make(map[string]any)
}

// Logs with an arbitrary event/level
func (c *Channel) log(e *zerolog.Event, msg string, ctx []map[string]any) {
	c.mu.Lock()
	merged := maps.Clone(c.ctx)
	c.mu.Unlock()

	for _, m := range ctx {
		maps.Copy(merged, m)
	}

	if len(merged) > 0 {
		e = e.Interface("context", merged)
	}
	e.Msg(msg)
}

func (c *Channel) Trace(msg string, ctx ...map[string]any) { c.log(c.logger.Trace(), msg, ctx) }
func (c *Channel) Debug(msg string, ctx ...map[string]any) { c.log(c.logger.Debug(), msg, ctx) }
func (c *Channel) Info(msg string, ctx ...map[string]any)  { c.log(c.logger.Info(), msg, ctx) }
func (c *Channel) Warn(msg string, ctx ...map[string]any)  { c.log(c.logger.Warn(), msg, ctx) }
func (c *Channel) Error(msg string, ctx ...map[string]any) { c.log(c.logger.Error(), msg, ctx) }
func (c *Channel) Fatal(msg string, ctx ...map[string]any) { c.log(c.logger.Fatal(), msg, ctx) }
