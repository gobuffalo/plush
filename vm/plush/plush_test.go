package plush_test

import (
	"fmt"
	"io"
	"os"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	vmplush "github.com/gobuffalo/plush/v5/vm/plush"
	"github.com/stretchr/testify/require"
)

func Test_Render_Uses_Compiled_VM_Path(t *testing.T) {
	ctx := rootplush.NewContextWith(map[string]interface{}{
		"name": "mark",
	})

	out, err := vmplush.Render(`<p><%= name %></p>`, ctx)
	if err != nil {
		t.Fatalf("Render returned error: %s", err)
	}
	if out != "<p>mark</p>" {
		t.Fatalf("wrong output: %q", out)
	}
}

func Test_Root_Render_And_VM_Render_Coexist(t *testing.T) {
	input := `<%= name %>`

	rootOut, rootErr := rootplush.Render(input, rootplush.NewContextWith(map[string]interface{}{
		"name": "root",
	}))
	if rootErr != nil {
		t.Fatalf("root Render returned error: %s", rootErr)
	}

	vmOut, vmErr := vmplush.Render(input, rootplush.NewContextWith(map[string]interface{}{
		"name": "vm",
	}))
	if vmErr != nil {
		t.Fatalf("VM Render returned error: %s", vmErr)
	}

	if rootOut != "root" {
		t.Fatalf("root renderer output changed: %q", rootOut)
	}
	if vmOut != "vm" {
		t.Fatalf("VM renderer output wrong: %q", vmOut)
	}
}

func Test_Set_Fast_Helper_Custom_Fast_Render_And_Fallback(t *testing.T) {
	fallbackCalls := 0
	ctx := rootplush.NewContextWith(map[string]interface{}{
		"amount": 12.5,
		"money": func(value interface{}) string {
			fallbackCalls++
			return fmt.Sprintf("fallback:%v", value)
		},
	})
	vmplush.SetFastHelper(ctx, "money", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
		amount, ok := args.Float64(0)
		if !ok {
			return vmplush.ErrFastUnsupported
		}
		w.WriteEscapedString(fmt.Sprintf("$%.2f", amount))
		return nil
	})

	out, err := vmplush.Render(`<%= money(amount) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "$12.50", out)
	require.Zero(t, fallbackCalls)

	ctx.Set("amount", "n/a")
	out, err = vmplush.Render(`<%= money(amount) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "fallback:n/a", out)
	require.Equal(t, 1, fallbackCalls)
}

func Test_Set_Fast_Helper_Custom_Bytecode_Path(t *testing.T) {
	fallbackCalls := 0
	ctx := rootplush.NewContextWith(map[string]interface{}{
		"amount": int32(7),
		"money": func(value interface{}) string {
			fallbackCalls++
			return fmt.Sprintf("fallback:%v", value)
		},
	})
	vmplush.SetFastHelper(ctx, "money", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
		amount, ok := args.Int64(0)
		if !ok {
			return vmplush.ErrFastUnsupported
		}
		w.WriteEscapedString(fmt.Sprintf("fast:%d", amount))
		return nil
	})

	out, err := vmplush.Render(`<% let local = amount %><%= money(local) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "fast:7", out)
	require.Zero(t, fallbackCalls)
}

func Test_Set_Fast_No_Context_Value_Helper(t *testing.T) {
	ctx := rootplush.NewContextWith(map[string]interface{}{
		"decorate": func(value string) string { return "fallback:" + value },
	})
	vmplush.SetFastNoContextValueHelper(ctx, "decorate", func(args vmplush.FastArgs) (interface{}, error) {
		value, ok := args.String(0)
		if !ok || args.Len() != 1 {
			return nil, vmplush.ErrFastUnsupported
		}
		return "fast:" + value, nil
	})

	out, err := vmplush.Render(`<% let result = decorate("value") %><%= result %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "fast:value", out)
}

func Test_Set_Fast_Read_Only_Value_Helper(t *testing.T) {
	ctx := rootplush.NewContextWith(map[string]interface{}{
		"prefix":   "read-only",
		"decorate": func(value string) string { return "fallback:" + value },
	})
	vmplush.SetFastReadOnlyValueHelper(ctx, "decorate", func(callCtx vmplush.FastReadOnlyContext, args vmplush.FastArgs) (interface{}, error) {
		_, writable := interface{}(callCtx).(hctx.Context)
		require.False(t, writable)
		value, ok := args.String(0)
		if !ok || args.Len() != 1 {
			return nil, vmplush.ErrFastUnsupported
		}
		prefix, _ := callCtx.Value("prefix").(string)
		return prefix + ":" + value, nil
	})

	out, err := vmplush.Render(`<% let prefix = "local" %><% let result = decorate("value") %><%= result %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "local:value", out)
}

func Test_Buffalo_Renderer_VM_Partial_Sees_Top_Level_Let_From_Layout(t *testing.T) {
	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)

	type settingValue struct {
		Text string
	}
	type setting struct {
		Value settingValue
	}
	type record struct {
		Attributes map[string]interface{}
	}

	data := map[string]interface{}{
		"viewData": map[string]interface{}{
			"Current": record{
				Attributes: map[string]interface{}{
					"primary": setting{Value: settingValue{Text: "#123456"}},
				},
			},
		},
	}
	helpers := map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			require.Equal(t, "partials/color-token.plush.html", name)
			return `<span class="token"><%= activeRecord.Attributes["primary"].Value.Text %></span>`, nil
		},
	}

	out, err := rootplush.BuffaloRendererWithContext(`<% let activeRecord = viewData.Current %><%= partial("partials/color-token.plush.html") %>`, data, helpers, nil)
	require.NoError(t, err)
	require.Contains(t, out, "<span class=\"token\">#123456</span>")
}

func Test_Buffalo_Renderer_VM_Cached_Layout_Partial_Sees_Top_Level_Let(t *testing.T) {
	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)

	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	type settingValue struct {
		Text string
	}
	type setting struct {
		Value settingValue
	}
	type record struct {
		Attributes map[string]interface{}
	}

	input := `<% let activeRecord = viewData.Current %><%= partial("partials/color-token.plush.html") %>`
	helpers := map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			require.Equal(t, "partials/color-token.plush.html", name)
			return `color: <%= activeRecord.Attributes["primary"].Value.Text %>;`, nil
		},
	}

	for _, color := range []string{"#123456", "#abcdef"} {
		data := map[string]interface{}{
			"viewData": map[string]interface{}{
				"Current": record{
					Attributes: map[string]interface{}{
						"primary": setting{Value: settingValue{Text: color}},
					},
				},
			},
		}
		out, err := rootplush.BuffaloRendererWithContext(input, data, helpers, func(ctx *rootplush.Context) {
			ctx.Set(meta.TemplateFileKey, "templates/layout.plush.html")
		})
		require.NoError(t, err)
		require.Contains(t, out, "color: "+color+";")
	}
}

func Test_Buffalo_Renderer_VM_Cached_Layout_Partial_Sees_Top_Level_Let_Alias_From_Config(t *testing.T) {
	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)

	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	type settingValue struct {
		Text string
	}
	type setting struct {
		Value settingValue
	}
	type record struct {
		Attributes map[string]interface{}
	}
	type pageConfig struct {
		Current *record
	}

	input := `<% let layoutState = pageConfig %><% let activeRecord = layoutState.Current %><%= partial("partials/color-token.plush.html") %>`
	helpers := map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			require.Equal(t, "partials/color-token.plush.html", name)
			return `color: <%= activeRecord.Attributes["primary"].Value.Text %>;`, nil
		},
	}

	for _, color := range []string{"#123456", "#abcdef"} {
		data := map[string]interface{}{
			"pageConfig": &pageConfig{
				Current: &record{
					Attributes: map[string]interface{}{
						"primary": setting{Value: settingValue{Text: color}},
					},
				},
			},
		}
		out, err := rootplush.BuffaloRendererWithContext(input, data, helpers, func(ctx *rootplush.Context) {
			ctx.Set(meta.TemplateFileKey, "templates/layout.plush.html")
		})
		require.NoError(t, err)
		require.Contains(t, out, "color: "+color+";")
	}
}

func Test_Buffalo_Renderer_VM_Cached_Branch_Partial_Assigns_Parent_Binding(t *testing.T) {
	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)
	previousFallback := rootplush.SetVMGenericFallback(true)
	defer rootplush.SetVMGenericFallback(previousFallback)

	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	input := `<%= if (currentRoute.PathName == "target") { %><% let registry = {} %><%= partial("partials/registry-file.html") %><%= registry["primary"] %><% } %>`
	helpers := map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			require.Equal(t, "partials/registry-file.html", name)
			return `<% registry = {"primary": entryID} %>`, nil
		},
	}

	for _, id := range []string{"first-id", "second-id"} {
		data := map[string]interface{}{
			"currentRoute": struct {
				PathName string
			}{PathName: "target"},
			"entryID": id,
		}
		out, err := rootplush.BuffaloRendererWithContext(input, data, helpers, func(ctx *rootplush.Context) {
			ctx.Set(meta.TemplateFileKey, "templates/layout.plush.html")
		})
		require.NoError(t, err)
		require.Equal(t, id, out)
	}
}

func Test_Buffalo_Renderer_VM_Generic_Partial_Sees_Branch_Let(t *testing.T) {
	previous := rootplush.SetRenderMode(rootplush.RenderModeVM)
	defer rootplush.SetRenderMode(previous)
	previousFallback := rootplush.SetVMGenericFallback(true)
	defer rootplush.SetVMGenericFallback(previousFallback)

	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	input := `<% let done = false %><% done = {"value": true} %><%= if (currentRoute.PathName == "target") { %><% let registry = {} %><%= partial("partials/registry-file.html") %><%= registry["primary"] %><% } %>`
	helpers := map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			require.Equal(t, "partials/registry-file.html", name)
			return `<% registry = {"primary": entryID} %>`, nil
		},
	}

	data := map[string]interface{}{
		"currentRoute": struct {
			PathName string
		}{PathName: "target"},
		"entryID": "current-id",
	}
	out, err := rootplush.BuffaloRendererWithContext(input, data, helpers, func(ctx *rootplush.Context) {
		ctx.Set(meta.TemplateFileKey, "templates/layout.plush.html")
	})
	require.NoError(t, err)
	require.Equal(t, "current-id", out)
	diagnostics, ok := rootplush.RenderDiagnosticsFromData(data)
	require.True(t, ok)
	require.Equal(t, rootplush.RenderFastPathGeneric, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
}

func Test_Clear_Fast_Helper_Removes_Custom_Fast_Render(t *testing.T) {
	fastCalls := 0
	fallbackCalls := 0
	ctx := rootplush.NewContextWith(map[string]interface{}{
		"name": "plush",
		"label": func(value interface{}) string {
			fallbackCalls++
			return fmt.Sprintf("fallback:%v", value)
		},
	})
	vmplush.SetFastHelper(ctx, "label", func(w vmplush.FastWriter, args vmplush.FastArgs) error {
		fastCalls++
		value, ok := args.String(0)
		if !ok {
			return vmplush.ErrFastUnsupported
		}
		w.WriteEscapedString("fast:" + value)
		return nil
	})

	out, err := vmplush.Render(`<%= label(name) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "fast:plush", out)
	require.Equal(t, 1, fastCalls)
	require.Zero(t, fallbackCalls)

	vmplush.ClearFastHelper(ctx, "label")
	out, err = vmplush.Render(`<%= label(name) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "fallback:plush", out)
	require.Equal(t, 1, fastCalls)
	require.Equal(t, 1, fallbackCalls)
}

func Test_Run_Script_Installs_Print_Helpers(t *testing.T) {
	ctx := rootplush.NewContext()
	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	err = vmplush.RunScript(`print("hello"); println(" world")`, ctx)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(output))
}
