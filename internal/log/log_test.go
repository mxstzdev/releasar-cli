package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager_ChannelIdempotency(t *testing.T) {
	m := newManager(Config{})
	a := m.Channel("foo")
	b := m.Channel("foo")
	assert.Same(t, a, b)
}

func TestManager_DefaultChannelName(t *testing.T) {
	m := newManager(Config{DefaultChannelName: "main"})
	ch := m.Channel()
	assert.Equal(t, "main", ch.name)
}

func TestManager_DefaultChannelNameFallback(t *testing.T) {
	m := newManager(Config{})
	ch := m.Channel()
	assert.Equal(t, "general", ch.name)
}

func TestManager_ShareContextPropagatestoExistingChannel(t *testing.T) {
	m := newManager(Config{})
	ch := m.Channel("test")
	m.ShareContext(map[string]any{"app": "releasar"})
	assert.Equal(t, "releasar", ch.ctx["app"])
}

func TestManager_ShareContextPropagatestoNewChannel(t *testing.T) {
	m := newManager(Config{})
	m.ShareContext(map[string]any{"app": "releasar"})
	ch := m.Channel("new")
	assert.Equal(t, "releasar", ch.ctx["app"])
}

func TestManager_RemoveContext(t *testing.T) {
	m := newManager(Config{})
	m.ShareContext(map[string]any{"a": 1, "b": 2})
	m.RemoveContext("a")
	assert.NotContains(t, m.ctx, "a")
	assert.Contains(t, m.ctx, "b")
}

func TestManager_FlushContext(t *testing.T) {
	m := newManager(Config{})
	m.ShareContext(map[string]any{"a": 1})
	m.FlushContext()
	assert.Empty(t, m.ctx)
}

func TestChannel_ShareContext(t *testing.T) {
	m := newManager(Config{})
	ch := m.Channel("test")
	ch.ShareContext(map[string]any{"key": "value"})
	assert.Equal(t, "value", ch.ctx["key"])
}

func TestChannel_RemoveContext(t *testing.T) {
	m := newManager(Config{})
	ch := m.Channel("test")
	ch.ShareContext(map[string]any{"a": 1, "b": 2})
	ch.RemoveContext("a")
	assert.NotContains(t, ch.ctx, "a")
	assert.Contains(t, ch.ctx, "b")
}

func TestChannel_FlushContext(t *testing.T) {
	m := newManager(Config{})
	ch := m.Channel("test")
	ch.ShareContext(map[string]any{"a": 1})
	ch.FlushContext()
	assert.Empty(t, ch.ctx)
}
