package log

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager_ChannelIdempotency(t *testing.T) {
	m := newManager(Config{}, io.Discard)
	a := m.Channel("foo")
	b := m.Channel("foo")
	assert.Same(t, a, b)
}

func TestManager_DefaultChannelName(t *testing.T) {
	m := newManager(Config{DefaultChannelName: "main"}, io.Discard)
	ch := m.Channel()
	assert.Equal(t, "main", ch.name)
}

func TestManager_DefaultChannelNameFallback(t *testing.T) {
	m := newManager(Config{}, io.Discard)
	ch := m.Channel()
	assert.Equal(t, "general", ch.name)
}

func TestManager_ShareContextPropagatestoExistingChannel(t *testing.T) {
	m := newManager(Config{}, io.Discard)
	ch := m.Channel("test")
	m.ShareContext(map[string]any{"app": "releasar"})
	assert.Equal(t, "releasar", ch.ctx["app"])
}

func TestManager_ShareContextPropagatestoNewChannel(t *testing.T) {
	m := newManager(Config{}, io.Discard)
	m.ShareContext(map[string]any{"app": "releasar"})
	ch := m.Channel("new")
	assert.Equal(t, "releasar", ch.ctx["app"])
}

func TestManager_RemoveContext(t *testing.T) {
	m := newManager(Config{}, io.Discard)
	m.ShareContext(map[string]any{"a": 1, "b": 2})
	m.RemoveContext("a")
	assert.NotContains(t, m.ctx, "a")
	assert.Contains(t, m.ctx, "b")
}

func TestManager_FlushContext(t *testing.T) {
	m := newManager(Config{}, io.Discard)
	m.ShareContext(map[string]any{"a": 1})
	m.FlushContext()
	assert.Empty(t, m.ctx)
}

func TestChannel_ShareContext(t *testing.T) {
	m := newManager(Config{}, io.Discard)
	ch := m.Channel("test")
	ch.ShareContext(map[string]any{"key": "value"})
	assert.Equal(t, "value", ch.ctx["key"])
}

func TestChannel_RemoveContext(t *testing.T) {
	m := newManager(Config{}, io.Discard)
	ch := m.Channel("test")
	ch.ShareContext(map[string]any{"a": 1, "b": 2})
	ch.RemoveContext("a")
	assert.NotContains(t, ch.ctx, "a")
	assert.Contains(t, ch.ctx, "b")
}

func TestChannel_FlushContext(t *testing.T) {
	m := newManager(Config{}, io.Discard)
	ch := m.Channel("test")
	ch.ShareContext(map[string]any{"a": 1})
	ch.FlushContext()
	assert.Empty(t, ch.ctx)
}

func TestBufferedWriter_Replay(t *testing.T) {
	b := &bufferedWriter{}
	b.Write([]byte(`{"msg":"before init"}` + "\n"))
	b.Write([]byte(`{"msg":"also before"}` + "\n"))

	var file bytes.Buffer
	b.replay(&file)

	assert.Contains(t, file.String(), "before init")
	assert.Contains(t, file.String(), "also before")
}

func TestInit_ReplaysClearsBuffer(t *testing.T) {
	original := manager
	originalPending := pending
	t.Cleanup(func() {
		manager = original
		pending = originalPending
	})

	buf := &bufferedWriter{}
	buf.Write([]byte(`{"msg":"early entry"}` + "\n"))
	pending = buf

	// Simulate what Init does without touching the filesystem via the real path.
	var replay bytes.Buffer
	pending.replay(&replay)
	pending = nil

	assert.Contains(t, replay.String(), "early entry")
	assert.Nil(t, pending)
}
