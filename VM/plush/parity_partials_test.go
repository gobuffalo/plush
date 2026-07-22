package plush_test

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/VM/plush"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

func Test_Parity_Partials_Simple(t *testing.T) {
	compareRender(t, `<%= partial("hello.plush") %>`, contextWith(map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			return `<p>Hello</p>`, nil
		},
	}))
}

func Test_Parity_Phase_12_Nested_Partial_Path_Metadata(t *testing.T) {
	gg := meta.TemplateFileKey
	partials := map[string]string{
		"partials/code-1.plush.html": `<%= partial("testing/code-2.plush.html") %>`,
		"testing/code-2.plush.html":  `<%= partial("testing/code-3.plush.html") %>`,
		"testing/code-3.plush.html":  `CODE3 PRINT <%= ` + gg + ` %>`,
	}

	compareRender(t, `<%= partial("partials/code-1.plush.html") %>`, func() hctx.Context {
		ctx := rootplush.NewContext()
		ctx.Set("partialFeeder", func(name string) (string, error) {
			return partials[name], nil
		})
		ctx.Set(meta.TemplateBaseFileNameKey, "path/product_listing")
		ctx.Set(meta.TemplateFileKey, "/fake/templates/path/product_listing.html")
		ctx.Set(meta.TemplateExtensionKey, "html")
		return ctx
	})
}

func Test_Parity_Phase_12_Partials_Data_Recursion_Layout_And_Yield(t *testing.T) {
	compareRender(t, `<%= partial("card", {name: "Mido", layout: "shell"}) %>`, contextWith(map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			if name == "shell" {
				return `<main><%= yield %></main>`, nil
			}
			return `<p>Hello <%= name %></p>`, nil
		},
	}))

	compareRender(t, `<%= partial("count") %><%= number %>`, contextWith(map[string]interface{}{
		"number": 3,
		"partialFeeder": func(string) (string, error) {
			return `<%= if (number > 0) { %><% let number = number - 1 %><%= partial("count") %><%= number %>, <% } %>`, nil
		},
	}))
}

func Test_Parity_Partials_Data_Map_Values(t *testing.T) {
	compareRender(t, `<%= partial("row", {name: first, title: robot.Name}) %>|<%= partial("row", {name: second, title: "literal"}) %>|<%= name %>`, contextWith(map[string]interface{}{
		"name":   "Outer",
		"first":  "Mido",
		"second": "Fry",
		"robot": struct {
			Name string
		}{Name: "Bender"},
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %>:<%= title %></span>`, nil
		},
	}))
}

func Test_Parity_Partials_Data_Map_Helper_Call_Value(t *testing.T) {
	compareRender(t, `<%= partial("row", {label: label(product.Name, prefix)}) %>|<%= product.Name %>`, contextWith(map[string]interface{}{
		"product": struct {
			Name string
		}{Name: "<Bender>"},
		"prefix": "robot",
		"label": func(name string, prefix string) string {
			return prefix + ":" + name
		},
		"partialFeeder": func(string) (string, error) {
			return `<span><%= label %></span>`, nil
		},
	}))
}

func Test_Parity_Phase_12_Partial_Java_Script_Escaping(t *testing.T) {
	compareRender(t, `<%= partial("index.html") %>|<%= partial("index.js") %>|<%= partial("index") %>`, contextWith(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(string) (string, error) {
			return `alert('\'Hello\'');`, nil
		},
	}))

	compareRender(t, `<%= partial("js_having_html_partial.js") %>|<%= partial("js_having_js_partial.js") %>`, contextWith(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(name string) (string, error) {
			switch name {
			case "js_having_html_partial.js":
				return `alert('<%= partial("t1.html") %>');`, nil
			case "js_having_js_partial.js":
				return `alert('<%= partial("t1.js") %>');`, nil
			case "t1.html":
				return `<div><%= partial("p1.html") %></div>`, nil
			case "t1.js":
				return `<div><%= partial("p1.js") %></div>`, nil
			case "p1.html", "p1.js":
				return `<span>FORM</span>`, nil
			default:
				return "", fmt.Errorf("unknown partial %s", name)
			}
		},
	}))
}

func Test_Parity_Phase_12_Partial_No_Default_Helper_Override(t *testing.T) {
	compareRender(t, `<%= partial("index") %>`, contextWith(map[string]interface{}{
		"truncate": func(s string, opts hctx.Map) string {
			return s
		},
		"partialFeeder": func(string) (string, error) {
			return `<%= truncate("xxxxxxxxxxxaaaaaaaaaa", {size: 10}) %>`, nil
		},
	}))
}

func Test_Parity_Phase_12_Partial_Feeder_Errors(t *testing.T) {
	compareBothRenderError(t, `<%= partial("missing") %>`, contextWith(map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			return "", fmt.Errorf("missing %s", name)
		},
	}))

	compareBothRenderError(t, `<%= partial("bad") %>`, contextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<%= missing </div>`, nil
		},
	}))
}

func Test_Phase_12_VM_Plush_Cache_Hole_Skeleton_For_Plush_Filenames(t *testing.T) {
	for _, filename := range []string{"phase12.plush", "phase12.plush.html"} {
		t.Run(filename, func(t *testing.T) {
			cache := inmemory.NewMemoryCache()
			rootplush.PlushCacheSetup(cache)
			defer rootplush.ClearTemplateCache()

			ctx := rootplush.NewContextWith(map[string]interface{}{
				"items": []string{"a", "b"},
			})
			ctx.Set(meta.TemplateFileKey, filename)

			input := `<% let suffix = "1" %><%= items[0] %><%H suffix %><%= items[1] %>`
			out, err := vmplush.Render(input, ctx)
			require.NoError(t, err)
			require.Equal(t, "a1b", out)
			requireCacheKey(t, cache, rootplush.GenerateASTKey(filename))
			requireCacheKey(t, cache, "full:"+filename)

			out, err = vmplush.Render(`<% let suffix = "2" %><%= items[0] %><%H suffix %><%= items[1] %>`, ctx)
			require.NoError(t, err)
			require.Equal(t, "a2b", out)
		})
	}
}

func Test_Phase_12_VM_Plush_Non_Plush_Filename_Does_Not_Use_Hole_Cache(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, "phase12.txt")

	out, err := vmplush.Render(`<%= "a" %><%H "hole" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "a<PLUSH_HOLE_0>", out)

	out, err = vmplush.Render(`<%= "b" %><%H "hole" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "b<PLUSH_HOLE_0>", out)
}

func Test_VM_Bytecode_Caches_HTML_Template_Filenames(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	filename := "bytecode-cache.html"
	require.False(t, rootplush.IsPlushTemplateFile(filename))
	require.True(t, rootplush.IsVMBytecodeCacheableTemplateFile(filename))

	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := vmplush.Render(`<%= "old" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "old", out)
	requireCacheKey(t, cache, rootplush.GenerateASTKey(filename))

	out, err = vmplush.Render(`<%= "new" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "new", out)
}

func Test_Phase_12_VM_Plush_Cache_Uses_Current_URL_In_Full_Key(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, "phase12-url.plush")
	ctx.Set(meta.TemplateCurrentUrlKey, "/products/123?ignored=true")

	out, err := vmplush.Render(`<%= "x" %><%H "hole" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "xhole", out)
	requireCacheKey(t, cache, "full:phase12-url.plush|url:products_123")
}

func Test_Phase_12_VM_Plush_Caches_Bytecode_By_AST_Key(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	filename := "phase12-bytecode.plush"
	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := vmplush.Render(`<%= "old" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "old", out)
	requireCacheKey(t, cache, rootplush.GenerateASTKey(filename))

	out, err = vmplush.Render(`<%= "new" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "new", out)

	cache.Delete(rootplush.GenerateASTKey(filename))
	out, err = vmplush.Render(`<%= "new" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "new", out)
}

func Test_Phase_12_VM_Plush_Caches_Bytecode_Without_AST_Or_Source(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	filename := "phase12-bytecode-only.plush"
	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := vmplush.Render(`<%= "old" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "old", out)

	key := rootplush.GenerateASTKey(filename)
	entry, ok := cache.Get(key)
	require.True(t, ok)
	require.NotNil(t, entry.VMBytecode)
	require.Nil(t, entry.Program)
	require.Empty(t, entry.Input)

	out, err = vmplush.Render(`<%= "new" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "new", out)
	entry, ok = cache.Get(key)
	require.True(t, ok)
	require.Nil(t, entry.Program)

	out, err = rootplush.Render(`<%= "new" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "new", out)
	entry, ok = cache.Get(key)
	require.True(t, ok)
	require.NotNil(t, entry.Program)
	require.NotNil(t, entry.VMBytecode)
	require.Empty(t, entry.Input)
}

func Test_Phase_12_VM_Plush_Uses_Existing_Interpreter_AST_Cache(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	rootplush.PlushCacheSetup(cache)
	defer rootplush.ClearTemplateCache()

	filename := "phase12-interpreter-ast.plush"
	ctx := rootplush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := rootplush.Render(`<%= "ast" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "ast", out)

	key := rootplush.GenerateASTKey(filename)
	entry, ok := cache.Get(key)
	require.True(t, ok)
	require.NotNil(t, entry.Program)
	require.Nil(t, entry.VMBytecode)
	require.Empty(t, entry.Input)

	out, err = vmplush.Render(`<%= "new" %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "new", out)

	entry, ok = cache.Get(key)
	require.True(t, ok)
	require.Nil(t, entry.Program)
	require.NotNil(t, entry.VMBytecode)
	require.Empty(t, entry.Input)
}

func requireCacheKey(t *testing.T, cache *inmemory.MemoryCache, key string) {
	t.Helper()
	_, ok := cache.Get(key)
	require.True(t, ok, "expected cache key %q", key)
}

func Test_Parity_Phase_12_Metadata_Normalization(t *testing.T) {
	compareRender(t, `<%= partial("child.plush.html") %>`, contextWith(map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			require.True(t, strings.HasSuffix(name, ".plush.html"))
			return `<%= filename() %>`, nil
		},
	}))
}

func Test_Parity_Phase_12_Partial_Safe_Yield_HTML(t *testing.T) {
	compareRender(t, `<%= partial("body", {layout: "layout"}) %>`, contextWith(map[string]interface{}{
		"partialFeeder": func(name string) (string, error) {
			if name == "layout" {
				return `<section><%= yield %></section>`, nil
			}
			return string(template.HTML(`<strong>safe</strong>`)), nil
		},
	}))
}
