package vm

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
)

var benchmarkSink string

type benchmarkScenario struct {
	name  string
	input string
	ctx   func() hctx.Context
	reuse bool
}

type benchmarkRobot struct {
	Name    benchmarkName
	Enabled bool
	Stock   uint32
	Friends []benchmarkRobot
}

func (r benchmarkRobot) GetFriends() []benchmarkRobot {
	return r.Friends
}

type benchmarkName string

func (n benchmarkName) Echo() string {
	return string(n)
}

type benchmarkProduct struct {
	Name     string
	Count    int
	Category benchmarkCategory
}

type benchmarkCatalog struct {
	Products []benchmarkStoreProduct
}

type benchmarkStoreProduct struct {
	Name    string
	Stock   uint32
	Friends []benchmarkStoreFriend
}

type benchmarkStoreFriend struct {
	Name string
}

type benchmarkCategory struct {
	Label string
}

func benchmarkScenarios() []benchmarkScenario {
	return []benchmarkScenario{
		{
			name:  "static_html",
			input: `<section><h1>Hello</h1><p>Static content</p></section>`,
			ctx:   benchmarkEmptyContext,
			reuse: true,
		},
		{
			name:  "variable_interpolation",
			input: `<p>Hello <%= name %></p><p><%= title %></p>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{"name": "Mido", "title": "Engineer"})
			},
			reuse: true,
		},
		{
			name:  "helper_heavy",
			input: `<%= greet(name) %><%= greet(name) %><%= greet(name) %><%= greet(name) %><%= greet(name) %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"name": "Mido",
					"greet": func(name string) string {
						return "hi " + name
					},
				})
			},
			reuse: true,
		},
		{
			name:  "name_lookup_heavy",
			input: strings.Repeat(`<%= name %>|`, 40),
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{"name": "Mido"})
			},
			reuse: true,
		},
		{
			name:  "static_output_heavy",
			input: `<section>` + strings.Repeat(`<p>Static VM output paragraph.</p>`, 80) + `</section>`,
			ctx:   benchmarkEmptyContext,
			reuse: true,
		},
		{
			name:  "regex_match",
			input: `<%= name ~= "^Mi.*o$" %><%= name ~= "^Mi.*o$" %><%= name ~= "^Mi.*o$" %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{"name": "Mido"})
			},
			reuse: true,
		},
		{
			name:  "mixed_numeric_operators",
			input: `<%= i32 == 0 %>|<%= u32 == 0 %>|<%= u64one == 1.0 %>|<%= f32i == 3 %>|<%= f64i == i32v %>|<%= robot.Stock == 3.0 %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"i32":    int32(0),
					"i32v":   int32(3),
					"u32":    uint32(0),
					"u64one": uint64(1),
					"f32i":   float32(3),
					"f64i":   float64(3),
					"robot":  benchmarkRobot{Stock: uint32(3)},
				})
			},
			reuse: true,
		},
		{
			name:  "loop_heavy",
			input: `<%= for (i, item) in items { %><%= i %>:<%= item %>;<% } %>`,
			ctx: func() hctx.Context {
				items := make([]string, 100)
				for i := range items {
					items[i] = fmt.Sprintf("item-%d", i)
				}
				return rootplush.NewContextWith(map[string]interface{}{"items": items})
			},
			reuse: true,
		},
		{
			name:  "loop_struct_property",
			input: `<%= for (i, product) in products { %><%= product.Name %>;<% } %>`,
			ctx: func() hctx.Context {
				products := make([]benchmarkRobot, 100)
				for i := range products {
					products[i] = benchmarkRobot{Name: benchmarkName(fmt.Sprintf("product-%d", i))}
				}
				return rootplush.NewContextWith(map[string]interface{}{"products": products})
			},
			reuse: true,
		},
		{
			name:  "loop_concat_output",
			input: `<%= for (i, product) in products { %><%= product.Name + " x " + product.Count %>;<% } %>`,
			ctx: func() hctx.Context {
				products := make([]benchmarkProduct, 100)
				for i := range products {
					products[i] = benchmarkProduct{
						Name:  fmt.Sprintf("product-%d", i),
						Count: i + 1,
					}
				}
				return rootplush.NewContextWith(map[string]interface{}{"products": products})
			},
			reuse: true,
		},
		{
			name:  "nested_flash_loop",
			input: `<%= for (k, messages) in flash { %><%= for (msg) in messages { %><%= if (len(messages) && messages[0] != "skip") { %><%= k %>:<%= msg %>;<% } %><% } %><% } %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"flash": map[string][]string{
						"notice": {"Hello", "Bye", "Ready"},
					},
				})
			},
			reuse: true,
		},
		{
			name:  "nested_struct_property",
			input: `<%= product.Category.Label %>:<%= product.Name %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"product": benchmarkProduct{
						Name:     "Bender",
						Category: benchmarkCategory{Label: "Robot"},
					},
				})
			},
			reuse: true,
		},
		{
			name:  "loop_nested_struct_property",
			input: `<%= for (i, product) in products { %><%= product.Category.Label %>:<%= product.Name %>;<% } %>`,
			ctx: func() hctx.Context {
				products := make([]benchmarkProduct, 100)
				for i := range products {
					products[i] = benchmarkProduct{
						Name:     fmt.Sprintf("product-%d", i),
						Category: benchmarkCategory{Label: "robot"},
					}
				}
				return rootplush.NewContextWith(map[string]interface{}{"products": products})
			},
			reuse: true,
		},
		{
			name:  "loop_helper_call",
			input: `<%= for (i, product) in products { %><%= label(product.Name, prefix) %>;<% } %>`,
			ctx: func() hctx.Context {
				products := make([]benchmarkRobot, 100)
				for i := range products {
					products[i] = benchmarkRobot{Name: benchmarkName(fmt.Sprintf("product-%d", i))}
				}
				return rootplush.NewContextWith(map[string]interface{}{
					"prefix":   "product",
					"products": products,
					"label": func(name benchmarkName, prefix string) string {
						return prefix + ":" + name.Echo()
					},
				})
			},
			reuse: true,
		},
		{
			name:  "loop_helper_call_string",
			input: `<%= for (i, product) in products { %><%= label(product.Name, prefix) %>;<% } %>`,
			ctx: func() hctx.Context {
				products := make([]benchmarkProduct, 100)
				for i := range products {
					products[i] = benchmarkProduct{Name: fmt.Sprintf("product-%d", i)}
				}
				return rootplush.NewContextWith(map[string]interface{}{
					"prefix":   "product",
					"products": products,
					"label": func(name string, prefix string) string {
						return prefix + ":" + name
					},
				})
			},
			reuse: true,
		},
		{
			name:  "loop_helper_call_int",
			input: `<%= for (i, product) in products { %><%= label(i, suffix) %>;<% } %>`,
			ctx: func() hctx.Context {
				products := make([]benchmarkProduct, 100)
				for i := range products {
					products[i] = benchmarkProduct{Name: fmt.Sprintf("product-%d", i)}
				}
				return rootplush.NewContextWith(map[string]interface{}{
					"suffix":   "items",
					"products": products,
					"label": func(index int, suffix string) string {
						return fmt.Sprintf("%d:%s", index, suffix)
					},
				})
			},
			reuse: true,
		},
		{
			name:  "loop_conditional",
			input: `<%= for (i, product) in products { %><%= if product.Enabled { %><%= product.Name %><% } else { %>hidden<% } %>;<% } %>`,
			ctx: func() hctx.Context {
				products := make([]benchmarkRobot, 100)
				for i := range products {
					products[i] = benchmarkRobot{
						Name:    benchmarkName(fmt.Sprintf("product-%d", i)),
						Enabled: i%2 == 0,
					}
				}
				return rootplush.NewContextWith(map[string]interface{}{"products": products})
			},
			reuse: true,
		},
		{
			name:  "loop_infix_conditional",
			input: `<%= for (i, product) in products { %><%= if product.Stock > min && i != 0 { %><%= product.Name %><% } else { %>hidden<% } %>;<% } %>`,
			ctx: func() hctx.Context {
				products := make([]benchmarkRobot, 100)
				for i := range products {
					products[i] = benchmarkRobot{
						Name:  benchmarkName(fmt.Sprintf("product-%d", i)),
						Stock: uint32(i % 5),
					}
				}
				return rootplush.NewContextWith(map[string]interface{}{
					"min":      uint64(2),
					"products": products,
				})
			},
			reuse: true,
		},
		{
			name:  "struct_chain_heavy",
			input: `<%= robot.Name.Echo() %>|<%= robot.GetFriends()[0].Name.Echo() %>|<%= robot.GetFriends()[1].Name.Echo() %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"robot": benchmarkRobot{
						Name: benchmarkName("bender"),
						Friends: []benchmarkRobot{
							{Name: benchmarkName("fry")},
							{Name: benchmarkName("leela")},
						},
					},
				})
			},
			reuse: true,
		},
		{
			name:  "nested_access",
			input: `<%= robots[0].Name %>|<%= robots[1].Name %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"robots": []benchmarkRobot{
						{Name: benchmarkName("bender")},
						{Name: benchmarkName("fry")},
					},
				})
			},
			reuse: true,
		},
		{
			name:  "typed_map_access",
			input: `<%= labels["status"] %>|<%= counts["active"] %>|<%= robots["bender"].Name %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"labels": map[string]string{"status": "ready"},
					"counts": map[string]uint32{"active": 7},
					"robots": map[string]benchmarkRobot{
						"bender": {Name: benchmarkName("Bender")},
					},
				})
			},
			reuse: true,
		},
		{
			name:  "partial_heavy",
			input: `<%= partial("row.plush") %><%= partial("row.plush") %><%= partial("row.plush") %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"name": "Mido",
					"partialFeeder": func(string) (string, error) {
						return `<span><%= name %></span>`, nil
					},
				})
			},
			reuse: true,
		},
		{
			name:  "partial_data_map",
			input: `<%= partial("row.plush", {name: first, title: robot.Name}) %><%= partial("row.plush", {name: second, title: "literal"}) %><%= partial("row.plush", {name: third, title: robot.Name}) %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"first":  "Mido",
					"second": "Fry",
					"third":  "Leela",
					"robot":  benchmarkRobot{Name: benchmarkName("Bender")},
					"partialFeeder": func(string) (string, error) {
						return `<span><%= name %>:<%= title %></span>`, nil
					},
				})
			},
			reuse: true,
		},
		{
			name:  "partial_data_helper_map",
			input: `<%= partial("row.plush", {label: label(product.Name, prefix)}) %><%= partial("row.plush", {label: label(product.Name, prefix)}) %><%= partial("row.plush", {label: label(product.Name, prefix)}) %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"product": benchmarkProduct{Name: "Bender"},
					"prefix":  "robot",
					"label": func(name string, prefix string) string {
						return prefix + ":" + name
					},
					"partialFeeder": func(string) (string, error) {
						return `<span><%= label %></span>`, nil
					},
				})
			},
			reuse: true,
		},
		{
			name:  "partial_simple_access",
			input: `<%= partial("row.plush") %><%= partial("row.plush") %><%= partial("row.plush") %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"robot":  benchmarkRobot{Name: benchmarkName("Bender")},
					"labels": map[string]string{"status": "ready"},
					"count":  uint32(1),
					"partialFeeder": func(string) (string, error) {
						return `<span><%= robot.Name %>:<%= labels["status"] %>:<%= count == 1 %></span>`, nil
					},
				})
			},
			reuse: true,
		},
		{
			name:  "punch_hole",
			input: `<%= "a" %><%H "hole" %><%= "b" %><%H "hole2" %>`,
			ctx:   benchmarkEmptyContext,
		},
		{
			name:  "whitespace_trim",
			input: "a \n <%- \"b\" %> \n c",
			ctx:   benchmarkEmptyContext,
			reuse: true,
		},
		{
			name:  "optional_parentheses",
			input: `<%= if enabled { %>yes<% } else if fallback { %>fallback<% } else { %>no<% } %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{"enabled": false, "fallback": true})
			},
			reuse: true,
		},
		{
			name:  "conditional_simple_access",
			input: `<%= if robot.Stock > min { %><%= robot.Name %><% } else if fallback { %><%= labels["status"] %>:<%= count == 1 %><% } else { %>no<% } %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"robot":    benchmarkRobot{Name: benchmarkName("Bender"), Stock: 0},
					"min":      uint32(0),
					"fallback": true,
					"labels":   map[string]string{"status": "ready"},
					"count":    uint32(1),
				})
			},
			reuse: true,
		},
		{
			name: "realistic_mixed",
			input: `<article>
<h1><%= title %></h1>
<%= if user.Name { %><p><%= greet(user.Name) %></p><% } %>
<ul><%= for (i, item) in items { %><li><%= i %>: <%= item.Name.Echo() %></li><% } %></ul>
</article>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"title": "Robots",
					"user":  benchmarkRobot{Name: benchmarkName("Mido")},
					"items": []benchmarkRobot{
						{Name: benchmarkName("bender")},
						{Name: benchmarkName("fry")},
						{Name: benchmarkName("leela")},
					},
					"greet": func(name benchmarkName) template.HTML {
						return template.HTML("hello " + name.Echo())
					},
				})
			},
			reuse: true,
		},
		{
			name: "generic_storefront_shape",
			input: `<% let title = "Products" %>
<%= wrap({class: "grid"}) { %>
<h2><%= title %></h2>
<%= if (!hidden) { %>
<%= for (_, product) in catalog.Products { %>
<article>
<h3><%= product.Name %></h3>
<%= if (product.Stock > 0) { %><span><%= product.Stock %></span><% } else { %><span>sold out</span><% } %>
<ul><%= for (_, friend) in product.Friends { %><%= if (friend.Name == "stop") { %><%= break %><% } %><li><%= friend.Name %></li><% } %></ul>
</article>
<% } %>
<% } %>
<% } %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"hidden": false,
					"catalog": benchmarkCatalog{Products: []benchmarkStoreProduct{
						{Name: "Bender", Stock: 3, Friends: []benchmarkStoreFriend{{Name: "Fry"}, {Name: "Leela"}}},
						{Name: "Zoidberg", Stock: 0, Friends: []benchmarkStoreFriend{{Name: "Hermes"}, {Name: "stop"}, {Name: "Amy"}}},
						{Name: "Nibbler", Stock: 1, Friends: []benchmarkStoreFriend{{Name: "Mido"}}},
					}},
					"wrap": func(data map[string]interface{}, help rootplush.HelperContext) (template.HTML, error) {
						body, err := help.Block()
						if err != nil {
							return "", err
						}
						return template.HTML(`<section class="` + fmt.Sprint(data["class"]) + `">` + body + `</section>`), nil
					},
				})
			},
			reuse: true,
		},
	}
}

func benchmarkEmptyContext() hctx.Context {
	return rootplush.NewContext()
}

func BenchmarkRenderOneShot(b *testing.B) {
	for _, scenario := range benchmarkScenarios() {
		b.Run(scenario.name+"/interpreter", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				out, err := rootplush.Render(scenario.input, scenario.ctx())
				if err != nil {
					b.Skipf("interpreter does not support this benchmark scenario: %v", err)
				}
				benchmarkSink = out
			}
		})
		b.Run(scenario.name+"/vm", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				out, err := Render(scenario.input, scenario.ctx())
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSink = out
			}
		})
	}
}

func BenchmarkRenderCompiledReuse(b *testing.B) {
	for _, scenario := range benchmarkScenarios() {
		if !scenario.reuse {
			continue
		}
		b.Run(scenario.name+"/vm_reused_bytecode", func(b *testing.B) {
			tmpl, err := Compile(scenario.input)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := tmpl.Render(scenario.ctx())
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSink = out
			}
		})
	}
}

func BenchmarkRenderReusedContextMatrix(b *testing.B) {
	for _, scenario := range benchmarkScenarios() {
		if !scenario.reuse {
			continue
		}

		b.Run(scenario.name+"/interpreter_parsed_reused_context", func(b *testing.B) {
			tmpl, err := rootplush.NewTemplate(scenario.input)
			if err != nil {
				b.Skipf("interpreter does not support this benchmark scenario: %v", err)
			}
			ctx := scenario.ctx()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, _, err := tmpl.Exec(ctx)
				if err != nil {
					b.Skipf("interpreter does not support this benchmark scenario: %v", err)
				}
				benchmarkSink = out
			}
		})

		b.Run(scenario.name+"/vm_compiled_reused_context", func(b *testing.B) {
			tmpl, err := Compile(scenario.input)
			if err != nil {
				b.Fatal(err)
			}
			ctx := scenario.ctx()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := tmpl.Render(ctx)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSink = out
			}
		})
	}
}

func BenchmarkRenderCachedFilename(b *testing.B) {
	for _, scenario := range benchmarkScenarios() {
		if !scenario.reuse {
			continue
		}
		b.Run(scenario.name+"/vm_cached_render", func(b *testing.B) {
			cache := inmemory.NewMemoryCache()
			rootplush.PlushCacheSetup(cache)
			filename := "bench-" + scenario.name + ".plush"
			warmCtx := scenario.ctx()
			warmCtx.Set(meta.TemplateFileKey, filename)
			if _, err := Render(scenario.input, warmCtx); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := scenario.ctx()
				ctx.Set(meta.TemplateFileKey, filename)
				out, err := Render(scenario.input, ctx)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSink = out
			}
		})
	}
}

func BenchmarkRenderSourceBytecodeCache(b *testing.B) {
	scenarios := []benchmarkScenario{
		{
			name:  "variable_snippet",
			input: `<script>window.store = "<%= storeURL %>"; window.page = "<%= page %>";</script>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"storeURL": "devstorev1.com",
					"page":     "product",
				})
			},
			reuse: true,
		},
		{
			name:  "helper_snippet",
			input: `<%= assetTag("app.js") %><%= assetTag("app.css") %><%= assetTag(name + ".js") %>`,
			ctx: func() hctx.Context {
				return rootplush.NewContextWith(map[string]interface{}{
					"name": "product",
					"assetTag": func(name string) template.HTML {
						return template.HTML(`<script src="/assets/` + name + `"></script>`)
					},
				})
			},
			reuse: true,
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name+"/interpreter_no_filename", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				out, err := rootplush.Render(scenario.input, scenario.ctx())
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSink = out
			}
		})

		b.Run(scenario.name+"/vm_no_filename_uncached", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				clearSourceBytecodeCacheForTest()
				out, err := Render(scenario.input, scenario.ctx())
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSink = out
			}
		})
		clearSourceBytecodeCacheForTest()

		b.Run(scenario.name+"/vm_no_filename_source_cache", func(b *testing.B) {
			warmCtx := scenario.ctx()
			if _, err := Render(scenario.input, warmCtx); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := Render(scenario.input, scenario.ctx())
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSink = out
			}
		})
		clearSourceBytecodeCacheForTest()
	}
}

func BenchmarkRenderModeMatrix(b *testing.B) {
	for _, scenario := range benchmarkScenarios() {
		if !scenario.reuse {
			continue
		}

		b.Run(scenario.name+"/interpreter_uncached", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				out, err := rootplush.Render(scenario.input, scenario.ctx())
				if err != nil {
					b.Skipf("interpreter does not support this benchmark scenario: %v", err)
				}
				benchmarkSink = out
			}
		})

		b.Run(scenario.name+"/interpreter_ast_cache", func(b *testing.B) {
			cache := inmemory.NewMemoryCache()
			rootplush.PlushCacheSetup(cache)
			filename := "bench-interpreter-" + scenario.name + ".plush"
			warmCtx := scenario.ctx()
			warmCtx.Set(meta.TemplateFileKey, filename)
			out, err := rootplush.Render(scenario.input, warmCtx)
			if err != nil {
				b.Skipf("interpreter does not support this benchmark scenario: %v", err)
			}
			benchmarkSink = out

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := scenario.ctx()
				ctx.Set(meta.TemplateFileKey, filename)
				out, err := rootplush.Render(scenario.input, ctx)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSink = out
			}
		})

		b.Run(scenario.name+"/vm_uncached", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				out, err := Render(scenario.input, scenario.ctx())
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSink = out
			}
		})

		b.Run(scenario.name+"/vm_bytecode_cache", func(b *testing.B) {
			cache := inmemory.NewMemoryCache()
			rootplush.PlushCacheSetup(cache)
			filename := "bench-vm-" + scenario.name + ".plush"
			warmCtx := scenario.ctx()
			warmCtx.Set(meta.TemplateFileKey, filename)
			if _, err := Render(scenario.input, warmCtx); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := scenario.ctx()
				ctx.Set(meta.TemplateFileKey, filename)
				out, err := Render(scenario.input, ctx)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSink = out
			}
		})
	}
}
