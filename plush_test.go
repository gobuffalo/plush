package plush_test

import (
	"sync"
	"testing"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

func Test_Render_Simple_HTML(t *testing.T) {
	r := require.New(t)

	input := `<p>Hi</p>`
	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal(input, s)
}

func Test_Render_Keeps_Spacing(t *testing.T) {
	r := require.New(t)
	input := `<%= greet %> <%= name %>`

	ctx := plush.NewContext()
	ctx.Set("greet", "hi")
	ctx.Set("name", "mark")

	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("hi mark", s)
}

func Test_Render_HTML_Injected_String(t *testing.T) {
	r := require.New(t)

	input := `<p><%= "mark" %></p>`
	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal("<p>mark</p>", s)
}

func Test_Render_Injected_Variable(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	s, err := plush.Render(input, plush.NewContextWith(map[string]interface{}{
		"name": "Mark",
	}))
	r.NoError(err)
	r.Equal("<p>Mark</p>", s)
}

func Test_Render_Missing_Variable(t *testing.T) {
	r := require.New(t)

	input := `<p><%= name %></p>`
	_, err := plush.Render(input, plush.NewContext())
	r.Error(err)
}

func Test_Render_Show_No_Show(t *testing.T) {
	r := require.New(t)
	input := `<%= "shown" %><% "notshown" %>`
	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal("shown", s)
}

func Test_Render_Script_Function(t *testing.T) {
	r := require.New(t)

	input := `<% let add = fn(x) { return x + 2; }; %><%= add(2) %>`

	s, err := plush.Render(input, plush.NewContext())
	r.NoError(err)
	r.Equal("4", s)
}

func Test_Render_Mode_Default_Interpreter(t *testing.T) {
	r := require.New(t)
	previous := plush.SetRenderMode(plush.RenderModeInterpreter)
	defer plush.SetRenderMode(previous)

	s, err := plush.Render(`<%= "interpreter" %>`, plush.NewContext())
	r.NoError(err)
	r.Equal("interpreter", s)
}

func Test_Render_Diagnostics_Interpreter_Context(t *testing.T) {
	r := require.New(t)
	previous := plush.SetRenderMode(plush.RenderModeInterpreter)
	defer plush.SetRenderMode(previous)

	ctx := plush.NewContext()
	ctx.Set(meta.TemplateFileKey, "products/show.plush")

	s, err := plush.Render(`<%= "interpreter" %>`, ctx)
	r.NoError(err)
	r.Equal("interpreter", s)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	r.True(ok)
	r.Equal(plush.RenderModeNameInterpreter, diagnostics.Mode)
	r.Equal("products/show.plush", diagnostics.TemplateFilename)
	r.Equal(plush.VMBytecodeCacheDisabled, diagnostics.VMBytecodeCache)
	r.NotZero(diagnostics.EngineDuration)
}

func Test_Render_Diagnostics_From_Data_After_Buffalo_Renderer(t *testing.T) {
	r := require.New(t)
	previous := plush.SetRenderMode(plush.RenderModeInterpreter)
	defer plush.SetRenderMode(previous)

	data := map[string]interface{}{
		meta.TemplateFileKey: "products/show.plush",
	}
	rendered, err := plush.BuffaloRenderer(`<%= "interpreter" %>`, data, nil)
	r.NoError(err)
	r.Equal("interpreter", rendered)

	diagnostics, ok := plush.RenderDiagnosticsFromData(data)
	r.True(ok)
	r.Equal(plush.RenderModeNameInterpreter, diagnostics.Mode)
	r.Equal("products/show.plush", diagnostics.TemplateFilename)
	r.Equal(plush.VMBytecodeCacheDisabled, diagnostics.VMBytecodeCache)
	r.NotZero(diagnostics.EngineDuration)
}

func Test_Render_Diagnostics_Accumulates_Sequential_Template_Durations(t *testing.T) {
	r := require.New(t)
	previous := plush.SetRenderMode(plush.RenderModeInterpreter)
	defer plush.SetRenderMode(previous)

	ctx := plush.NewContext()
	ctx.Set(meta.TemplateFileKey, "products/show.plush")

	_, err := plush.Render(`<%= "body" %>`, ctx)
	r.NoError(err)
	first, ok := plush.RenderDiagnosticsFromContext(ctx)
	r.True(ok)
	r.Equal("products/show.plush", first.TemplateFilename)
	r.NotZero(first.EngineDuration)

	ctx.Set(meta.TemplateFileKey, "application.plush")
	_, err = plush.Render(`<%= "layout" %>`, ctx)
	r.NoError(err)
	second, ok := plush.RenderDiagnosticsFromContext(ctx)
	r.True(ok)
	r.Equal("products/show.plush", second.TemplateFilename)
	r.Greater(second.EngineDuration, first.EngineDuration)
}

func Test_Render_Interpreter_AST_Cache_Invalidates_When_Source_Changes(t *testing.T) {
	r := require.New(t)
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer plush.ClearTemplateCache()

	previous := plush.SetRenderMode(plush.RenderModeInterpreter)
	defer plush.SetRenderMode(previous)

	ctx := plush.NewContext()
	ctx.Set(meta.TemplateFileKey, "interpreter-source-change.plush")

	out, err := plush.Render(`<%= "first" %>`, ctx)
	r.NoError(err)
	r.Equal("first", out)

	out, err = plush.Render(`<%= "second" %>`, ctx)
	r.NoError(err)
	r.Equal("second", out)
}

func Test_Buffalo_Renderer_With_Context_Configures_Context(t *testing.T) {
	r := require.New(t)
	previous := plush.SetRenderMode(plush.RenderModeInterpreter)
	defer plush.SetRenderMode(previous)

	configured := false
	data := map[string]interface{}{
		meta.TemplateFileKey: "products/show.plush",
	}
	rendered, err := plush.BuffaloRendererWithContext(`<%= marker %>`, data, nil, func(ctx *plush.Context) {
		configured = true
		ctx.Set("marker", "configured")
	})
	r.NoError(err)
	r.True(configured)
	r.Equal("configured", rendered)
	r.Equal("configured", data["marker"])

	diagnostics, ok := plush.RenderDiagnosticsFromData(data)
	r.True(ok)
	r.Equal(plush.RenderModeNameInterpreter, diagnostics.Mode)
}

func Test_Render_Diagnostics_VM_Hotspots_Header(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	plush.EnableRenderVMHotspotDiagnostics(ctx)

	plush.AddRenderDiagnosticVMHelperTiming(ctx, "slow:helper", 3*time.Millisecond)
	plush.AddRenderDiagnosticVMHelperTiming(ctx, "slow:helper", 2*time.Millisecond)
	plush.AddRenderDiagnosticVMHelperTiming(ctx, "fast", time.Millisecond)
	plush.AddRenderDiagnosticVMPartialTiming(ctx, "row,card", 4*time.Millisecond)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	r.True(ok)
	r.Equal(3, diagnostics.VMHotspots.HelperCalls)
	r.Equal(1, diagnostics.VMHotspots.PartialCalls)
	r.InDelta(6.0, diagnostics.VMHelperDurationMilliseconds(), 0.001)
	r.InDelta(4.0, diagnostics.VMPartialDurationMilliseconds(), 0.001)
	r.Equal("slow_helper:2:5.000;fast:1:1.000", diagnostics.VMHelperHotspotsHeader())
	r.Equal("row_card:1:4.000", diagnostics.VMPartialHotspotsHeader())
}

func Test_Render_Diagnostics_VM_Hotspots_Default_Off(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()

	plush.AddRenderDiagnosticVMHelperTiming(ctx, "helper", time.Millisecond)
	plush.AddRenderDiagnosticVMPartialTiming(ctx, "partial", time.Millisecond)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	r.False(ok)
	r.Zero(diagnostics.VMHotspots.HelperCalls)
	r.Zero(diagnostics.VMHotspots.PartialCalls)
}

func Test_Render_Diagnostics_VM_Hotspots_Concurrent_Update(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	plush.EnableRenderVMHotspotDiagnostics(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				plush.AddRenderDiagnosticVMHelperTiming(ctx, "helper", time.Microsecond)
				plush.AddRenderDiagnosticVMPartialTiming(ctx, "partial", time.Microsecond)
			}
		}()
	}
	wg.Wait()

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	r.True(ok)
	r.Equal(400, diagnostics.VMHotspots.HelperCalls)
	r.Equal(400, diagnostics.VMHotspots.PartialCalls)
}

func Test_Render_Mode_VM_Requires_Registered_Renderer(t *testing.T) {
	r := require.New(t)
	previous := plush.SetRenderMode(plush.RenderModeVM)
	defer plush.SetRenderMode(previous)

	_, err := plush.Render(`<%= "vm" %>`, plush.NewContext())
	r.ErrorIs(err, plush.ErrVMRendererNotRegistered)
}

func Test_Render_Has_Block(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContext()
	ctx.Set("blockCheck", func(help plush.HelperContext) string {
		if help.HasBlock() {
			s, _ := help.Block()
			return s
		}
		return "no block"
	})
	input := `<%= blockCheck() {return "block"} %>|<%= blockCheck() %>`
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("block|no block", s)
}

func Test_Render_Dash_In_Helper(t *testing.T) {
	r := require.New(t)
	ctx := plush.NewContextWith(map[string]interface{}{
		"my-helper": func() string {
			return "hello"
		},
	})
	s, err := plush.Render(`<%= my-helper() %>`, ctx)
	r.NoError(err)
	r.Equal("hello", s)
}

func Test_Buffalo_Renderer(t *testing.T) {
	r := require.New(t)
	input := `<%= foo() %><%= name %>`
	data := map[string]interface{}{
		"name": "Ringo",
	}
	helpers := map[string]interface{}{
		"foo": func() string {
			return "George"
		},
	}
	s, err := plush.BuffaloRenderer(input, data, helpers)
	r.NoError(err)
	r.Equal("GeorgeRingo", s)
}

func Test_Buffalo_Renderer_Nil_Data(t *testing.T) {
	r := require.New(t)
	input := `<%= foo() %>`
	helpers := map[string]interface{}{
		"foo": func() string {
			return "test"
		},
	}
	s, err := plush.BuffaloRenderer(input, nil, helpers)
	r.NoError(err)
	r.Equal("test", s)
}

func Test_Buffalo_Renderer_Data_Persistence(t *testing.T) {
	r := require.New(t)
	input := `<%= contentFor("name") { %>MD<% }  %>`
	data := map[string]interface{}{}
	s, err := plush.BuffaloRenderer(input, data, map[string]interface{}{})
	r.NoError(err)
	r.Empty(s)
	r.Contains(data, "contentFor:name")
}

func Test_Helper_Nil_Arg(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(nil, "k") %><%= foo(one, "k") %>`
	ctx := plush.NewContextWith(map[string]interface{}{
		"one": map[string]string{
			"k": "test",
		},
		"foo": func(a map[string]string, b string) string {
			if a != nil {
				return a[b]
			}
			return ""
		},
	})
	s, err := plush.Render(input, ctx)
	r.NoError(err)
	r.Equal("test", s)
}

func Test_Undefined_Arg(t *testing.T) {
	r := require.New(t)
	input := `<%= foo(bar) %>`
	ctx := plush.NewContext()
	ctx.Set("foo", func(string) {})

	_, err := plush.Render(input, ctx)
	r.Error(err)
	r.Equal(`line 1: "bar": unknown identifier`, err.Error())
}

func Test_Caching(t *testing.T) {
	r := require.New(t)

	fileCacheName := "testing-123.plush"
	astCacheName := "ast:" + fileCacheName
	template, err := plush.NewTemplate("<%= \"AA\" %>")
	r.NoError(err)

	imC := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(imC)
	template.Input = ""
	template.IsCache = true
	imC.Set(astCacheName, template)

	tc, err := plush.Parse("<%= a %>", fileCacheName)
	r.NoError(err)
	r.Equal(tc, template)

	imC = nil
	tc, err = plush.Parse("<%= a %>")
	r.NoError(err)
	r.NotEqual(tc, template)
}

func Test_Caching_Empty_File_Name(t *testing.T) {
	r := require.New(t)

	fileCacheName := "testing 123"
	template, err := plush.NewTemplate("<%= \"AA\" %>")
	r.NoError(err)

	imC := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(imC)
	imC.Set(fileCacheName, template)

	tc, err := plush.Parse("<%= a %>")
	r.NoError(err)
	r.NotEqual(tc, template)
}
