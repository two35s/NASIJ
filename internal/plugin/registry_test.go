package plugin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nasij/nasij/internal/plugin"
)

// fakePlugin is a test double satisfying the Plugin interface.
type fakePlugin struct {
	name    string
	version string
	kind    plugin.Kind
}

func (f *fakePlugin) Name() string    { return f.name }
func (f *fakePlugin) Version() string { return f.version }
func (f *fakePlugin) Kind() plugin.Kind { return f.kind }
func (f *fakePlugin) Init(_ context.Context, _ map[string]any) error { return nil }

func newFake(name string, kind plugin.Kind) *fakePlugin {
	return &fakePlugin{name: name, version: "1.0.0", kind: kind}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := plugin.NewRegistry()
	p := newFake("test-plugin", plugin.KindAnalyzer)

	require.NoError(t, r.Register(p))

	got, ok := r.Get("test-plugin")
	assert.True(t, ok)
	assert.Equal(t, p, got)
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := plugin.NewRegistry()
	p := newFake("dup", plugin.KindAnalyzer)

	require.NoError(t, r.Register(p))
	err := r.Register(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dup")
}

func TestRegistry_GetMissing(t *testing.T) {
	r := plugin.NewRegistry()
	_, ok := r.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_Count(t *testing.T) {
	r := plugin.NewRegistry()
	assert.Equal(t, 0, r.Count())
	require.NoError(t, r.Register(newFake("a", plugin.KindAnalyzer)))
	assert.Equal(t, 1, r.Count())
}

func TestRegistry_List_SortedAlphabetically(t *testing.T) {
	r := plugin.NewRegistry()
	require.NoError(t, r.Register(newFake("zzz", plugin.KindReporter)))
	require.NoError(t, r.Register(newFake("aaa", plugin.KindAnalyzer)))
	require.NoError(t, r.Register(newFake("mmm", plugin.KindExporter)))

	list := r.List()
	require.Len(t, list, 3)
	assert.Equal(t, "aaa", list[0].Name())
	assert.Equal(t, "mmm", list[1].Name())
	assert.Equal(t, "zzz", list[2].Name())
}

func TestRegistry_Unregister(t *testing.T) {
	r := plugin.NewRegistry()
	p := newFake("to-remove", plugin.KindAnalyzer)
	require.NoError(t, r.Register(p))
	require.NoError(t, r.Unregister("to-remove"))
	assert.Equal(t, 0, r.Count())
}

func TestRegistry_UnregisterMissing(t *testing.T) {
	r := plugin.NewRegistry()
	err := r.Unregister("ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

func TestRegistry_ByKind(t *testing.T) {
	r := plugin.NewRegistry()
	require.NoError(t, r.Register(newFake("analyzer-1", plugin.KindAnalyzer)))
	require.NoError(t, r.Register(newFake("analyzer-2", plugin.KindAnalyzer)))
	require.NoError(t, r.Register(newFake("reporter-1", plugin.KindReporter)))

	analyzers := r.ByKind(plugin.KindAnalyzer)
	assert.Len(t, analyzers, 2)

	reporters := r.ByKind(plugin.KindReporter)
	assert.Len(t, reporters, 1)

	exporters := r.ByKind(plugin.KindExporter)
	assert.Len(t, exporters, 0)
}

func TestRegistry_RegisterNil(t *testing.T) {
	r := plugin.NewRegistry()
	err := r.Register(nil)
	require.Error(t, err)
}
