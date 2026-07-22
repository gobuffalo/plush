package vm

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

func vmPartialDataPlan(name string, key string) (*compiler.FastPartialPlan, *fastPartialDataBindingPlan) {
	partial := &compiler.FastPartialPlan{
		Name: name,
		Data: []compiler.FastPartialDataPair{{
			Key:   key,
			Value: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "Mido"},
			Line:  3,
		}},
		Line: 3,
	}
	return partial, buildFastPartialDataBindingPlan(partial)
}

func Test_VM_Fast_Data_Partial_Direct_Edge_Branches(t *testing.T) {
	partial, dataPlan := vmPartialDataPlan("edge_data_direct_static.plush", "name")
	ctx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span>static</span>`, nil
		},
	})
	var out strings.Builder

	handled, err := renderFastDataPartialInto(nil, partial, ctx, fastRenderBindings{}, dataPlan)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastDataPartialInto(&out, nil, ctx, fastRenderBindings{}, dataPlan)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastDataPartialDirectInto(nil, partial, ctx, fastRenderBindings{}, dataPlan)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastDataPartialDirectInto(&out, nil, ctx, fastRenderBindings{}, dataPlan)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastDataPartialDirectInto(&out, partial, ctx, fastRenderBindings{}, nil)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastDataPartialDirectInto(&out, partial, ctx, fastRenderBindings{}, &fastPartialDataBindingPlan{})
	require.NoError(t, err)
	require.False(t, handled)

	layoutPartial, layoutPlan := vmPartialDataPlan("edge_data_direct_layout.plush", "layout")
	handled, err = renderFastDataPartialDirectInto(&out, layoutPartial, ctx, fastRenderBindings{}, layoutPlan)
	require.NoError(t, err)
	require.False(t, handled)

	jsCtx := plush.NewContextWith(map[string]interface{}{"contentType": "application/javascript"})
	handled, err = renderFastDataPartialDirectInto(&out, &compiler.FastPartialPlan{Name: "edge.html", Line: 4}, jsCtx, fastRenderBindings{}, dataPlan)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastDataPartialDirectInto(&out, partial, plush.NewContext(), fastRenderBindings{}, dataPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 3")
	require.ErrorContains(t, err, "could not find partial feeder")

	errorCtx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return "", fmt.Errorf("missing partial")
		},
	})
	handled, err = renderFastDataPartialDirectInto(&out, partial, errorCtx, fastRenderBindings{}, dataPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 3: missing partial")

	parseCtx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<%=`, nil
		},
	})
	handled, err = renderFastDataPartialDirectInto(&out, partial, parseCtx, fastRenderBindings{}, dataPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 3")

	filenameCtx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span>static</span>`, nil
		},
	})
	filenameCtx.Set(meta.TemplateBaseFileNameKey, "index")
	filenameCtx.Set(meta.TemplateExtensionKey, "plush")
	filenameCtx.Set(meta.TemplateFileKey, 12)
	handled, err = renderFastDataPartialDirectInto(&out, partial, filenameCtx, fastRenderBindings{}, dataPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 3")
	require.ErrorContains(t, err, "expected fileKey to be a string")

	out.Reset()
	handled, err = renderFastDataPartialDirectInto(&out, partial, ctx, fastRenderBindings{}, dataPlan)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `<span>static</span>`, out.String())
}

func Test_VM_Fast_Data_Partial_Direct_Cached_Bytecode_Fallback_Branches(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	ctx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span>ignored</span>`, nil
		},
	})
	var out strings.Builder

	partial, dataPlan := vmPartialDataPlan("edge_data_direct_holes.plush", "name")
	plush.CacheVMBytecodeForCleanFilename(partial.Name, nil, &compiler.Bytecode{HasHoles: true})
	handled, err := renderFastDataPartialDirectInto(&out, partial, ctx, fastRenderBindings{}, dataPlan)
	require.NoError(t, err)
	require.False(t, handled)

	partial, dataPlan = vmPartialDataPlan("edge_data_direct_no_fast_plan.plush", "name")
	plush.CacheVMBytecodeForCleanFilename(partial.Name, nil, &compiler.Bytecode{})
	handled, err = renderFastDataPartialDirectInto(&out, partial, ctx, fastRenderBindings{}, dataPlan)
	require.NoError(t, err)
	require.False(t, handled)

	partial, dataPlan = vmPartialDataPlan("edge_data_direct_special_binding.plush", "name")
	plush.CacheVMBytecodeForCleanFilename(partial.Name, nil, &compiler.Bytecode{
		FastRenderPlan: &compiler.FastRenderPlan{Bindings: []string{"contentType"}},
	})
	handled, err = renderFastDataPartialDirectInto(&out, partial, ctx, fastRenderBindings{}, dataPlan)
	require.NoError(t, err)
	require.False(t, handled)

	partial, dataPlan = vmPartialDataPlan("edge_data_direct_empty_fast_plan.plush", "name")
	plush.CacheVMBytecodeForCleanFilename(partial.Name, nil, &compiler.Bytecode{
		FastRenderPlan: &compiler.FastRenderPlan{Bindings: []string{"name"}},
	})
	handled, err = renderFastDataPartialDirectInto(&out, partial, ctx, fastRenderBindings{}, dataPlan)
	require.NoError(t, err)
	require.False(t, handled)

	errorPartial := &compiler.FastPartialPlan{
		Name: "edge_data_direct_static_error.plush",
		Data: []compiler.FastPartialDataPair{{
			Key: "name",
			Value: compiler.FastValuePlan{
				Kind: compiler.FastValueCall,
				Call: &compiler.FastCallPlan{
					Name:      "echo",
					NameIndex: 0,
					Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueString, Value: "go"}},
					Line:      33,
				},
			},
			Line: 33,
		}},
		Line: 33,
	}
	errorPlan := buildFastPartialDataBindingPlan(errorPartial)
	errorCtx := plush.NewContextWith(map[string]interface{}{
		"echo": func(string) string { return "go" },
		"partialFeeder": func(string) (string, error) {
			return `<span>ignored</span>`, nil
		},
	}).WithBudget(plush.NewBudget(0))
	plush.CacheVMBytecodeForCleanFilename(errorPartial.Name, nil, &compiler.Bytecode{Static: true, StaticOutput: "never"})
	handled, err = renderFastDataPartialDirectInto(&out, errorPartial, errorCtx, fastRenderBindings{}, errorPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 33")
}

func Test_VM_Partial_Bytecode_Link_For_Input_Uses_Cached_VM_Bytecode(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	ctx := plush.NewContext()
	filename := "edge_cached_link.plush"
	bytecode := &compiler.Bytecode{Static: true, StaticOutput: "cached"}
	plush.CacheVMBytecodeForCleanFilename(filename, nil, bytecode)

	link, err := partialBytecodeLinkForInput("ignored", filename, ctx)
	require.NoError(t, err)
	require.NotNil(t, link)
	require.Same(t, bytecode, link.bytecode)
}

func Test_VM_Fast_Data_Partial_Fallback_Error_Branches(t *testing.T) {
	partial, dataPlan := vmPartialDataPlan("edge_data_fallback.html", "name")
	var out strings.Builder

	budgetCtx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span>never</span>`, nil
		},
	}).WithBudget(plush.NewBudget(0))
	handled, err := renderFastDataPartialInto(&out, partial, budgetCtx, fastRenderBindings{}, dataPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 3")

	missingPartial := &compiler.FastPartialPlan{
		Name: "edge_data_missing.html",
		Data: []compiler.FastPartialDataPair{{
			Key:   "name",
			Value: compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "missing", NameIndex: 99},
			Line:  15,
		}},
		Line: 15,
	}
	missingDataPlan := buildFastPartialDataBindingPlan(missingPartial)
	jsCtx := plush.NewContextWith(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})
	handled, err = renderFastDataPartialInto(&out, missingPartial, jsCtx, fastRenderBindings{}, missingDataPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 15")

	out.Reset()
	handled, err = renderFastDataPartialInto(&out, partial, jsCtx, fastRenderBindings{}, &fastPartialDataBindingPlan{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, template.JSEscapeString("<span>Mido</span>"), out.String())

	handled, err = renderFastDataPartialInto(&out, missingPartial, jsCtx, fastRenderBindings{}, nil)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 15")

	filenameCtx := plush.NewContextWith(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})
	filenameCtx.Set(meta.TemplateBaseFileNameKey, "index")
	filenameCtx.Set(meta.TemplateExtensionKey, "plush")
	filenameCtx.Set(meta.TemplateFileKey, 12)
	handled, err = renderFastDataPartialInto(&out, partial, filenameCtx, fastRenderBindings{}, dataPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 3")
	require.ErrorContains(t, err, "expected fileKey to be a string")

	parseCtx := plush.NewContextWith(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(string) (string, error) {
			return `<%=`, nil
		},
	})
	handled, err = renderFastDataPartialInto(&out, partial, parseCtx, fastRenderBindings{}, dataPlan)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 3")
}

func Test_VM_Fast_Data_Partial_Non_ID_Fallback_Error_Branches(t *testing.T) {
	var out strings.Builder

	missingPartial := &compiler.FastPartialPlan{
		Name: "edge_data_slow_apply_missing.plush",
		Data: []compiler.FastPartialDataPair{{
			Key:   "name",
			Value: compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "missing", NameIndex: 99},
			Line:  51,
		}},
		Line: 51,
	}
	ctx := newVMFallbackContext(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span>never</span>`, nil
		},
	})
	handled, err := renderFastDataPartialInto(&out, missingPartial, ctx, fastRenderBindings{}, &fastPartialDataBindingPlan{})
	require.True(t, handled)
	require.ErrorContains(t, err, "line 51")
	require.ErrorContains(t, err, `"missing": unknown identifier`)

	inlineErrPartial := &compiler.FastPartialPlan{
		Name: "edge_data_slow_inline_parse.plush",
		Data: []compiler.FastPartialDataPair{{
			Key:   "layout",
			Value: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: ""},
			Line:  52,
		}},
		Line: 52,
	}
	inlineErrCtx := newVMFallbackContext(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<%=`, nil
		},
	})
	handled, err = renderFastDataPartialInto(&out, inlineErrPartial, inlineErrCtx, fastRenderBindings{}, buildFastPartialDataBindingPlan(inlineErrPartial))
	require.True(t, handled)
	require.ErrorContains(t, err, "line 52")

	jsErrPartial := &compiler.FastPartialPlan{
		Name: "edge_data_slow_js_parse.plush",
		Data: []compiler.FastPartialDataPair{{
			Key:   "layout",
			Value: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: ""},
			Line:  53,
		}},
		Line: 53,
	}
	jsErrCtx := newVMFallbackContext(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(string) (string, error) {
			return `<%=`, nil
		},
	})
	handled, err = renderFastDataPartialInto(&out, jsErrPartial, jsErrCtx, fastRenderBindings{}, buildFastPartialDataBindingPlan(jsErrPartial))
	require.True(t, handled)
	require.ErrorContains(t, err, "line 53")
}

func Test_VM_Fast_No_Data_Partial_Direct_Edge_Branches(t *testing.T) {
	var out strings.Builder
	ctx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span>static no data</span>`, nil
		},
	})

	handled, err := renderFastNoDataPartialDirectInto(nil, "edge_no_data_static.plush", ctx, 5)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastNoDataPartialDirectInto(&out, "edge_no_data_static.plush", nil, 5)
	require.NoError(t, err)
	require.False(t, handled)

	jsCtx := plush.NewContextWith(map[string]interface{}{"contentType": "application/javascript"})
	handled, err = renderFastNoDataPartialDirectInto(&out, "edge_no_data.html", jsCtx, 6)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastNoDataPartialDirectInto(&out, "edge_no_data_missing.plush", plush.NewContext(), 7)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 7")
	require.ErrorContains(t, err, "could not find partial feeder")

	errorCtx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return "", fmt.Errorf("missing partial")
		},
	})
	handled, err = renderFastNoDataPartialDirectInto(&out, "edge_no_data_error.plush", errorCtx, 8)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 8: missing partial")

	filenameCtx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span>static no data</span>`, nil
		},
	})
	filenameCtx.Set(meta.TemplateBaseFileNameKey, "index")
	filenameCtx.Set(meta.TemplateExtensionKey, "plush")
	filenameCtx.Set(meta.TemplateFileKey, 12)
	handled, err = renderFastNoDataPartialDirectInto(&out, "edge_no_data_filename.plush", filenameCtx, 9)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 9")
	require.ErrorContains(t, err, "expected fileKey to be a string")

	parseCtx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<%=`, nil
		},
	})
	handled, err = renderFastNoDataPartialDirectInto(&out, "edge_no_data_parse.plush", parseCtx, 10)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 10")

	out.Reset()
	handled, err = renderFastNoDataPartialDirectInto(&out, "edge_no_data_static.plush", ctx, 5)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `<span>static no data</span>`, out.String())
}

func Test_VM_Fast_No_Data_Partial_Direct_Cached_Bytecode_Fallback_Branches(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	ctx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span>ignored</span>`, nil
		},
	})
	var out strings.Builder

	plush.CacheVMBytecodeForCleanFilename("edge_no_data_direct_holes.plush", nil, &compiler.Bytecode{HasHoles: true})
	handled, err := renderFastNoDataPartialDirectInto(&out, "edge_no_data_direct_holes.plush", ctx, 41)
	require.NoError(t, err)
	require.False(t, handled)

	plush.CacheVMBytecodeForCleanFilename("edge_no_data_direct_special_binding.plush", nil, &compiler.Bytecode{
		FastRenderPlan: &compiler.FastRenderPlan{Bindings: []string{"contentType"}},
	})
	handled, err = renderFastNoDataPartialDirectInto(&out, "edge_no_data_direct_special_binding.plush", ctx, 42)
	require.NoError(t, err)
	require.False(t, handled)

	plush.CacheVMBytecodeForCleanFilename("edge_no_data_direct_empty_fast_plan.plush", nil, &compiler.Bytecode{
		FastRenderPlan: &compiler.FastRenderPlan{Bindings: []string{"name"}},
	})
	handled, err = renderFastNoDataPartialDirectInto(&out, "edge_no_data_direct_empty_fast_plan.plush", ctx, 43)
	require.NoError(t, err)
	require.False(t, handled)

	blockTmpl, err := Compile(`<%= wrap({}) { %>body<% } %>`)
	require.NoError(t, err)
	blockFilename := "edge_no_data_cached_block_call.plush"
	plush.CacheVMBytecodeForCleanFilename(blockFilename, nil, blockTmpl.bytecode)
	ctx.Set(meta.TemplateFileKey, blockFilename)
	previousFallback := plush.SetVMGenericFallback(true)
	handled, err = renderCachedPartialBytecodeInto(&out, ctx, blockFilename, false)
	plush.SetVMGenericFallback(previousFallback)
	require.NoError(t, err)
	require.False(t, handled)
}

func Test_VM_Direct_Partial_Uses_Cached_Bytecode_Before_Feeder(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	filename := "edge_direct_cached_before_feeder.plush"
	cachedBytecode := &compiler.Bytecode{Static: true, StaticOutput: "cached"}
	plush.CacheVMBytecodeForCleanFilename(filename, nil, cachedBytecode)

	feederCalls := 0
	ctx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			feederCalls++
			return `<span>from feeder</span>`, nil
		},
	})

	link, handled, err := directPartialBytecodeLinkForName(filename, filename, ctx)
	require.NoError(t, err)
	require.True(t, handled)
	require.NotNil(t, link)
	require.Same(t, cachedBytecode, link.bytecode)
	require.Equal(t, 0, feederCalls)
}

func Test_VM_Fast_No_Data_Partial_Direct_Uses_Current_Feeder_Source_When_Source_Is_Unknown(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	firstCtx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})

	var out strings.Builder
	handled, err := renderFastNoDataPartialDirectInto(&out, "edge_no_data_cached_before_feeder.html", firstCtx, 44)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `<span>Mido</span>`, out.String())

	feederCalls := 0
	secondCtx := plush.NewContextWith(map[string]interface{}{
		"name": "Fry",
		"partialFeeder": func(string) (string, error) {
			feederCalls++
			return `<span><%= name %></span>`, nil
		},
	})

	out.Reset()
	handled, err = renderFastNoDataPartialDirectInto(&out, "edge_no_data_cached_before_feeder.html", secondCtx, 45)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `<span>Fry</span>`, out.String())
	require.Equal(t, 1, feederCalls)
}

func Test_VM_Fast_No_Data_Partial_Direct_Skips_Contextual_Helper_From_Current_Feeder_Source(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	bytecode := requireCompiledBytecode(t, `<%= filename() %>`)
	plush.CacheVMBytecodeForCleanFilename("edge_no_data_contextual_helper.plush", nil, bytecode)

	ctx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<%= filename() %>`, nil
		},
	})
	var out strings.Builder

	handled, err := renderFastNoDataPartialDirectInto(&out, "edge_no_data_contextual_helper.plush", ctx, 46)
	require.NoError(t, err)
	require.False(t, handled)
	require.Empty(t, out.String())
}

func Test_VM_Fast_Data_Partial_Direct_Uses_Current_Feeder_Source_When_Source_Is_Unknown(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	tmpl, err := Compile(`<%= partial("edge_data_cached_before_feeder.html", {name: first}) %>`)
	require.NoError(t, err)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 1)
	require.NotNil(t, mixed.ops[0].partial)
	require.NotNil(t, mixed.ops[0].partialData)

	firstCtx := plush.NewContextWith(map[string]interface{}{
		"first": "Mido",
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})

	var out strings.Builder
	handled, err := renderFastDataPartialDirectInto(&out, mixed.ops[0].partial, firstCtx, newFastRenderBindings(tmpl.bytecode.FastRenderPlan, firstCtx), mixed.ops[0].partialData)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `<span>Mido</span>`, out.String())

	feederCalls := 0
	secondCtx := plush.NewContextWith(map[string]interface{}{
		"first": "Fry",
		"partialFeeder": func(string) (string, error) {
			feederCalls++
			return `<span><%= name %></span>`, nil
		},
	})

	out.Reset()
	handled, err = renderFastDataPartialDirectInto(&out, mixed.ops[0].partial, secondCtx, newFastRenderBindings(tmpl.bytecode.FastRenderPlan, secondCtx), mixed.ops[0].partialData)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `<span>Fry</span>`, out.String())
	require.Equal(t, 1, feederCalls)
}

func Test_VM_Fast_No_Data_Partial_Fallback_Edge_Branches(t *testing.T) {
	var out strings.Builder

	errCtx := plush.NewContextWith(map[string]interface{}{
		"contentType": "application/javascript",
	})
	handled, err := renderFastNoDataPartialInto(&out, "edge_missing_feeder.html", errCtx, 11)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 11")
	require.ErrorContains(t, err, "could not find partial feeder")

	feederErrCtx := plush.NewContextWith(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(string) (string, error) {
			return "", fmt.Errorf("fallback missing")
		},
	})
	handled, err = renderFastNoDataPartialInto(&out, "edge_fallback_error.html", feederErrCtx, 12)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 12: fallback missing")

	metadataCtx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<%= ` + meta.TemplateFileKey + ` %>`, nil
		},
	})
	out.Reset()
	handled, err = renderFastNoDataPartialInto(&out, "edge_fallback_metadata.plush", metadataCtx, 13)
	require.NoError(t, err)
	require.True(t, handled)
	require.Contains(t, out.String(), "edge_fallback_metadata.plush")

	budgetCtx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span>never</span>`, nil
		},
	}).WithBudget(plush.NewBudget(0))
	handled, err = renderFastNoDataPartialInto(&out, "edge_budget.plush", budgetCtx, 14)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 14")
}

func Test_VM_Fast_No_Data_Partial_Fallback_Metadata_And_Parse_Errors(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	var out strings.Builder

	fastMetaErrCtx := plush.NewContextWith(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(string) (string, error) {
			return `<span>never</span>`, nil
		},
	})
	fastMetaErrCtx.Set(meta.TemplateBaseFileNameKey, "index")
	fastMetaErrCtx.Set(meta.TemplateExtensionKey, "plush")
	fastMetaErrCtx.Set(meta.TemplateFileKey, 12)
	handled, err := renderFastNoDataPartialInto(&out, "edge_no_data_fast_meta_error.plush", fastMetaErrCtx, 61)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 61")
	require.ErrorContains(t, err, "expected fileKey to be a string")

	slowMetaErrCtx := newVMFallbackContext(map[string]interface{}{
		"contentType":                "application/javascript",
		"partialFeeder":              func(string) (string, error) { return `<span>never</span>`, nil },
		meta.TemplateBaseFileNameKey: "index",
		meta.TemplateExtensionKey:    "plush",
		meta.TemplateFileKey:         12,
	})
	handled, err = renderFastNoDataPartialInto(&out, "edge_no_data_slow_meta_error.plush", slowMetaErrCtx, 62)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 62")
	require.ErrorContains(t, err, "expected fileKey to be a string")

	jsErrCtx := newVMFallbackContext(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(string) (string, error) {
			return `<%=`, nil
		},
	})
	handled, err = renderFastNoDataPartialInto(&out, "edge_no_data_slow_js_parse.plush", jsErrCtx, 64)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 64")
}

func Test_VM_Partial_Setup_Template_File_Edges(t *testing.T) {
	ctx := plush.NewContext()
	ctx.Set(meta.TemplateBaseFileNameKey, "index")
	ctx.Set(meta.TemplateExtensionKey, "plush")
	ctx.Set(meta.TemplateFileKey, 12)
	require.ErrorContains(t, setupPartialTemplateFile(ctx, "row.plush"), "expected fileKey to be a string")

	ctx = plush.NewContext()
	ctx.Set(meta.TemplateBaseFileNameKey, "index")
	ctx.Set(meta.TemplateExtensionKey, "plush")
	ctx.Set(meta.TemplateFileKey, "templates/index.plush")
	ctx.Set(vmAlreadyInPartial, "parent.plush")
	require.NoError(t, setupPartialTemplateFile(ctx, "row.plush"))
	require.Equal(t, "templates/row.plush", ctx.Value(meta.TemplateFileKey))

	ctx = plush.NewContext()
	require.NoError(t, setupPartialTemplateFile(ctx, "row.plush"))
	require.Equal(t, "row.plush", ctx.Value(meta.TemplateFileKey))

	setupPartialNesting(ctx, "row.plush")
	require.Equal(t, "row.plush", ctx.Value(vmAlreadyInPartial))
}

func Test_VM_Apply_Fast_Partial_Data_Slow_Error_Edges(t *testing.T) {
	parent := plush.NewContext()
	partialCtx := borrowPartialOverlayContext(parent)
	defer releasePartialOverlayContext(partialCtx)

	require.NoError(t, applyFastPartialDataSlow(nil, nil, parent, fastRenderBindings{}))
	require.NoError(t, applyFastPartialDataSlow(partialCtx, nil, parent, fastRenderBindings{}))

	partial := &compiler.FastPartialPlan{
		Data: []compiler.FastPartialDataPair{{
			Key:   "name",
			Value: compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "missing", NameIndex: 99},
			Line:  12,
		}},
	}
	err := applyFastPartialDataSlow(partialCtx, partial, parent, fastRenderBindings{})
	require.ErrorContains(t, err, "line 12")
	require.ErrorContains(t, err, `"missing": unknown identifier`)
}

func Test_VM_Fast_Partial_Data_Binding_Helper_Error_Edges(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"echo": func(string) string { return "go" },
	}).WithBudget(plush.NewBudget(0))
	parentBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"echo"}}, ctx)
	plan := &fastPartialDataBindingPlan{
		keys: []string{"name"},
		pairs: []fastPartialDataBindingPair{{
			key:  "name",
			line: 44,
			value: &fastSimpleValuePlan{
				value: &compiler.FastValuePlan{
					Kind: compiler.FastValueCall,
					Call: &compiler.FastCallPlan{
						Name:      "echo",
						NameIndex: 0,
						Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueString, Value: "go"}},
						Line:      44,
					},
				},
				args: []*fastSimpleValuePlan{{value: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "go"}}},
			},
		}},
	}

	_, err := evalFastPartialDataPairValue(&plan.pairs[0], ctx, parentBindings)
	require.ErrorContains(t, err, "line 44")

	bindings := fastRenderBindings{names: []string{"name"}}
	err = attachFastPartialDataLocalsFromPlan(&bindings, plan, ctx, parentBindings, &fastPartialLocalStorage{})
	require.ErrorContains(t, err, "line 44")

	partialCtx := borrowPartialOverlayContext(plush.NewContext())
	defer releasePartialOverlayContext(partialCtx)
	handled, err := applyFastPartialDataBindingPlan(partialCtx, plan, ctx, parentBindings)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 44")

	widePlan := &fastPartialDataBindingPlan{
		keys: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
		pairs: []fastPartialDataBindingPair{
			{key: "a", value: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "wide"}}},
		},
	}
	wideCtx := borrowPartialOverlayContext(plush.NewContext())
	defer releasePartialOverlayContext(wideCtx)
	handled, err = applyFastPartialDataBindingPlan(wideCtx, widePlan, plush.NewContext(), fastRenderBindings{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "wide", wideCtx.Value("a"))
}

func Test_VM_Render_Linked_Partial_Cache_And_Inline_Edges(t *testing.T) {
	var out strings.Builder

	ok, err := renderLinkedPartialInline(nil, `<span>ignored</span>`, nil)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderLinkedPartialInline(&out, `<span>inline static</span>`, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, `<span>inline static</span>`, out.String())

	ctx := plush.NewContextWith(map[string]interface{}{"name": "Mido"})
	rendered, err := renderLinkedPartial(`<span><%= name %></span>`, ctx)
	require.NoError(t, err)
	require.Equal(t, `<span>Mido</span>`, rendered)
	links := partialBytecodeLinks(ctx)
	require.Equal(t, 1, links.Len())

	ctx.Set("name", "Leela")
	rendered, err = renderLinkedPartial(`<span><%= name %></span>`, ctx)
	require.NoError(t, err)
	require.Equal(t, `<span>Leela</span>`, rendered)
	require.Equal(t, 1, links.Len())

	blockCtx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"wrap": func(_ map[string]interface{}, help plush.HelperContext) (template.HTML, error) {
			block, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML("<b>" + block + "</b>"), nil
		},
	})
	blockPartial := `<%= wrap({}) { %><%= name %><% } %>`
	blockTmpl, err := Compile(blockPartial)
	require.NoError(t, err)
	previousFallback := plush.SetVMGenericFallback(true)
	defer plush.SetVMGenericFallback(previousFallback)
	require.True(t, shouldFallbackPartialBytecode(blockTmpl.bytecode))
	rendered, err = renderLinkedPartial(blockPartial, blockCtx)
	require.NoError(t, err)
	require.Equal(t, `<b>Mido</b>`, rendered)

	static, err := renderLinkedPartialBytecode(&partialBytecodeLink{bytecode: &compiler.Bytecode{
		Static:       true,
		StaticOutput: "static bytecode",
	}}, ctx, "", false)
	require.NoError(t, err)
	require.Equal(t, "static bytecode", static)

	dynamic, err := Compile(`<%= name %>`)
	require.NoError(t, err)
	dynamic.bytecode.FastRenderPlan = nil
	rendered, err = renderLinkedPartialBytecode(&partialBytecodeLink{bytecode: dynamic.bytecode}, ctx, "", false)
	require.NoError(t, err)
	require.Equal(t, "Leela", rendered)

	rendered, err = renderLinkedPartial(`<span>nil ctx</span>`, nil)
	require.NoError(t, err)
	require.Equal(t, `<span>nil ctx</span>`, rendered)

	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	cachedBytecode := &compiler.Bytecode{Static: true, StaticOutput: "cached linked"}
	plush.CacheVMBytecodeForCleanFilename("edge_linked_cached.plush", nil, cachedBytecode)
	cachedCtx := plush.NewContext()
	cachedCtx.Set(meta.TemplateFileKey, "edge_linked_cached.plush")

	rendered, err = renderLinkedPartial(`<span>different source</span>`, cachedCtx)
	require.NoError(t, err)
	require.Equal(t, "cached linked", rendered)

	out.Reset()
	ok, err = renderLinkedPartialInline(&out, `<span>different inline source</span>`, cachedCtx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "cached linked", out.String())

	out.Reset()
	ok, err = renderLinkedPartialInline(&out, `<span><%= name %></span>`, ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, `<span>Leela</span>`, out.String())

	out.Reset()
	ok, err = renderLinkedPartialInline(&out, `<span><%= name %></span>`, ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, `<span>Leela</span>`, out.String())

	holeCtx := plush.NewContext()
	holeCtx.Set(meta.TemplateFileKey, "edge_linked_hole.plush")
	holeCtx.Set("name", "Amy")
	holes := []plush.HoleMarker{
		plush.NewHoleMarker(plush.PunchHoleMarkerName(0), `<%= name %>`, 1, 15),
	}
	plush.CachePunchHoleSkeleton("edge_linked_hole.plush", holeCtx, "A<PLUSH_HOLE_0>B", holes, true)
	rendered, err = renderLinkedPartial(`<span>ignored</span>`, holeCtx)
	require.NoError(t, err)
	require.Equal(t, "AAmyB", rendered)

	out.Reset()
	ok, err = renderLinkedPartialInline(&out, `<span>ignored</span>`, holeCtx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "AAmyB", out.String())

	rendered, err = renderLinkedPartialBytecode(nil, ctx, "", false)
	require.NoError(t, err)
	require.Empty(t, rendered)

	_, err = renderLinkedPartial(`<%=`, plush.NewContext())
	require.Error(t, err)

	out.Reset()
	ok, err = renderLinkedPartialInline(&out, `<%=`, plush.NewContext())
	require.Error(t, err)
	require.False(t, ok)
}
