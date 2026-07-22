package vm

import (
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

func Test_Render_Interpreter_Fallback_With_Partial_Overlay_Context(t *testing.T) {
	parent := plush.NewContext()
	parent.Set("partial", vmPartialHelper)

	ctx := newPartialOverlayContext(parent)
	ctx.Set("show", true)
	require.True(t, ctx.Has("show"))
	require.Equal(t, true, ctx.Value("show"))

	out, err := renderInterpreterFallback(`<%= if (show) { %>Mido<% } %>`, ctx, "partial.plush.html")
	require.NoError(t, err)
	require.Equal(t, "Mido", out)
}

func Test_VM_Partial_Helper_Delegates_During_Interpreter_Render(t *testing.T) {
	ctx := plush.NewContext()
	ctx.Set("partial", vmPartialHelper)
	ctx.Set("partialFeeder", func(name string) (string, error) {
		require.Equal(t, "row.plush", name)
		return `<% let title = "Row" %><%= title %>`, nil
	})
	plush.SetRenderDiagnostics(ctx, plush.RenderDiagnostics{})

	out, err := plush.RenderInterpreter(`<%= partial("row.plush", {}) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "Row", out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Zero(t, diagnostics.VMHotspots.PartialCalls)
}

func Test_VM_Partial_Helper_Interpreter_Render_With_Overlay_If(t *testing.T) {
	parent := plush.NewContext()
	parent.Set("partial", vmPartialHelper)
	parent.Set("partialFeeder", func(name string) (string, error) {
		require.Equal(t, "row.plush", name)
		return `<%= if (show) { %><%= name %><% } else { %>Hidden<% } %>`, nil
	})

	ctx := newPartialOverlayContext(parent)
	ctx.Set("show", true)
	ctx.Set("name", "Mido")

	out, err := plush.RenderInterpreter(`<%= partial("row.plush", {}) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "Mido", out)
}
