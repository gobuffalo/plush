package plush_test

import (
	"testing"
	"time"

	rootplush "github.com/gobuffalo/plush/v5"
	_ "github.com/gobuffalo/plush/v5/VM/plush"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

func Test_Root_Render_Mode_VM_Uses_Registered_VM_Renderer(t *testing.T) {
	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)

	out, err := rootplush.Render(`<%= "vm" %>`, rootplush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "vm", out)
}

func Test_Root_Render_Mode_VM_Uses_Bytecode_Only_Cache(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)

	filename := "render-mode-vm.plush"
	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := rootplush.Render(`<%= "vm-cache" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "vm-cache", out)

	entry, ok := cache.Get(rootplush.GenerateASTKey(filename))
	require.True(t, ok)
	require.NotNil(t, entry.VMBytecode)
	require.Nil(t, entry.Program)
	require.Empty(t, entry.Input)
}

func Test_Root_Render_Mode_VM_Bytecode_Cache_Invalidates_When_Source_Changes(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)

	filename := "render-mode-vm-source-change.plush"
	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := rootplush.Render(`<%= "first" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "first", out)

	out, err = rootplush.Render(`<%= "second" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "second", out)

	diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, rootplush.VMBytecodeCacheMissStore, diagnostics.VMBytecodeCache)
}

func Test_Root_Render_Mode_VM_Bytecode_Cache_Uses_Full_Template_Path(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)

	first := "/templates/client-1/index.plush"
	second := "/templates/client-2/index.plush"

	firstCtx := rootplush.NewContext()
	firstCtx.Set(meta.TemplateFileKey, first)
	out, err := rootplush.Render(`<%= "one" %>`, firstCtx)
	require.NoError(t, err)
	require.Equal(t, "one", out)

	secondCtx := rootplush.NewContext()
	secondCtx.Set(meta.TemplateFileKey, second)
	out, err = rootplush.Render(`<%= "two" %>`, secondCtx)
	require.NoError(t, err)
	require.Equal(t, "two", out)

	firstKey := rootplush.GenerateASTKey(first)
	secondKey := rootplush.GenerateASTKey(second)
	require.NotEqual(t, firstKey, secondKey)
	_, ok := cache.Get(firstKey)
	require.True(t, ok)
	_, ok = cache.Get(secondKey)
	require.True(t, ok)
}

func Test_Root_Render_Mode_VM_Punch_Hole_Cache_Invalidates_When_Source_Changes(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)

	filename := "render-mode-vm-hole-source-change.plush"
	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := rootplush.Render(`A<%H "hole" %>B`, ctx)
	require.NoError(t, err)
	require.Equal(t, "AholeB", out)

	out, err = rootplush.Render(`C<%H "hole" %>D`, ctx)
	require.NoError(t, err)
	require.Equal(t, "CholeD", out)
}

func Test_Root_Render_Mode_VM_Writes_Diagnostics_To_Context(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)

	filename := "render-mode-vm-diagnostics.plush"
	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)
	ctx.Set("name", "mido")
	ctx.Set("slow", func() string {
		time.Sleep(20 * time.Millisecond)
		return ""
	})

	out, err := rootplush.Render(`<%= slow() %><%= name %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "mido", out)

	diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, rootplush.RenderModeNameVM, diagnostics.Mode)
	require.Equal(t, filename, diagnostics.TemplateFilename)
	require.Equal(t, rootplush.VMBytecodeCacheMissStore, diagnostics.VMBytecodeCache)
	require.NotZero(t, diagnostics.EngineDuration)

	out, err = rootplush.Render(`<%= slow() %><%= name %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "mido", out)

	diagnostics, ok = rootplush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, rootplush.RenderModeNameVM, diagnostics.Mode)
	require.Equal(t, filename, diagnostics.TemplateFilename)
	require.Equal(t, rootplush.VMBytecodeCacheHit, diagnostics.VMBytecodeCache)
	require.NotZero(t, diagnostics.EngineDuration)
}

func Test_Root_Render_Mode_VM_Generic_Fallback_Uses_Interpreter(t *testing.T) {
	previousMode := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previousMode)
	previousFallback := rootplush.SetVMGenericFallback(true)
	defer rootplush.SetVMGenericFallback(previousFallback)

	ctx := rootplush.NewContext()
	out, err := rootplush.Render(`<% let forceBytecode = fn() { return "x" } %><% let title = "Products" %><%= title %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "Products", out)

	diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, rootplush.RenderModeNameVM, diagnostics.Mode)
	require.Equal(t, rootplush.RenderFastPathInterpreterFallback, diagnostics.FastPath)
}

func Test_Root_Render_Mode_VM_Generic_Fallback_Keeps_Bytecode_Cache(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	previousMode := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previousMode)
	previousFallback := rootplush.SetVMGenericFallback(true)
	defer rootplush.SetVMGenericFallback(previousFallback)

	filename := "render-mode-vm-fallback-cache.plush"
	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	input := `<% let forceBytecode = fn() { return "x" } %><% let title = "Products" %><%= title %>`
	out, err := rootplush.Render(input, ctx)
	require.NoError(t, err)
	require.Equal(t, "Products", out)
	diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, rootplush.VMBytecodeCacheMissStore, diagnostics.VMBytecodeCache)

	out, err = rootplush.Render(input, ctx)
	require.NoError(t, err)
	require.Equal(t, "Products", out)
	diagnostics, ok = rootplush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, rootplush.VMBytecodeCacheHit, diagnostics.VMBytecodeCache)
	require.Equal(t, rootplush.RenderFastPathInterpreterFallback, diagnostics.FastPath)
}

func Test_Root_Render_Mode_VM_Generic_Fallback_Renders_Partials_With_Interpreter(t *testing.T) {
	previousMode := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previousMode)
	previousFallback := rootplush.SetVMGenericFallback(true)
	defer rootplush.SetVMGenericFallback(previousFallback)

	ctx := rootplush.NewContext()
	ctx.Set("partial", rootplush.PartialHelper)
	ctx.Set("partialFeeder", func(name string) (string, error) {
		require.Equal(t, "row.plush", name)
		return `<% let title = "Row" %><%= title %>`, nil
	})
	ctx.Set(meta.TemplateFileKey, "render-mode-vm-fallback-partial.plush")

	out, err := rootplush.Render(`<% let forceBytecode = fn() { return "x" } %><%= partial("row.plush", {}) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "Row", out)

	diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, rootplush.RenderFastPathInterpreterFallback, diagnostics.FastPath)
	require.Zero(t, diagnostics.VMHotspots.PartialCalls)
}

func Test_Root_Render_Mode_Interpreter_Still_Uses_AST_Cache(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	previous := rootplush.SetRenderMode(rootplush.RenderModeInterpreter)
	defer rootplush.SetRenderMode(previous)

	filename := "render-mode-interpreter.plush"
	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := rootplush.Render(`<%= "interpreter-cache" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "interpreter-cache", out)

	entry, ok := cache.Get(rootplush.GenerateASTKey(filename))
	require.True(t, ok)
	require.NotNil(t, entry.Program)
	require.Nil(t, entry.VMBytecode)
	require.Empty(t, entry.Input)
}
