// Package arraybench benchmarks the public renderer without linking the
// renderer packages' correctness tests into the benchmark binary.
package arraybench

import (
	"html"
	"reflect"
	"strings"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/vm/plush"
)

type chainedLoopContainer struct{ items *chainedLoopItems }

func (c *chainedLoopContainer) GetItems() *chainedLoopItems { return c.items }

type chainedLoopItems struct{ items []string }

func (c *chainedLoopItems) GetItems() []string { return c.items }

func BenchmarkFixedArrayLoop(b *testing.B) {
	for _, scenario := range []struct {
		name    string
		count   int
		value   interface{}
		text    string
		pointer bool
	}{
		{"string_1", 1, "item", "item", false},
		{"string_100", 100, "item", "item", false},
		{"string_1000", 1000, "item", "item", false},
		{"int_100", 100, int(1000), "1000", false},
		{"uint64_100", 100, uint64(1000), "1000", false},
		{"float64_100", 100, float64(1.25), "1.25", false},
		{"bool_100", 100, true, "true", false},
		{"pointer_string_100", 100, "item", "item", true},
		{"pointer_int_100", 100, int(1000), "1000", true},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			array := reflect.New(reflect.ArrayOf(scenario.count, reflect.TypeOf(scenario.value))).Elem()
			for i := 0; i < array.Len(); i++ {
				array.Index(i).Set(reflect.ValueOf(scenario.value))
			}
			items := array.Interface()
			if scenario.pointer {
				items = array.Addr().Interface()
			}
			tmpl, err := vmplush.Compile(`<%= for (item) in items { %><%= item %>;<% } %>`)
			if err != nil {
				b.Fatal(err)
			}
			ctx := rootplush.NewContextWith(map[string]interface{}{"items": items})
			out, err := tmpl.Render(ctx)
			if err != nil || out != strings.Repeat(scenario.text+";", scenario.count) {
				b.Fatalf("unexpected output %q: %v", out, err)
			}
			diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
			if !ok || diagnostics.FastPath != rootplush.RenderFastPathFast {
				b.Fatalf("unexpected path: %+v", diagnostics)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := tmpl.Render(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkStringValueLoop(b *testing.B) {
	for _, scenario := range []struct {
		name  string
		count int
		value string
		input string
	}{
		{"plain_1", 1, "item", `<%= for (item) in items { %><%= item %>;<% } %>`},
		{"plain_100", 100, "item", `<%= for (item) in items { %><%= item %>;<% } %>`},
		{"plain_1000", 1000, "item", `<%= for (item) in items { %><%= item %>;<% } %>`},
		{"escaped_100", 100, `<b>&"'</b>`, `<%= for (item) in items { %><%= item %>;<% } %>`},
		{"conditional_control_100", 100, "item", `<%= for (item) in items { %><%= if (item != "") { %><%= item %>;<% } %><% } %>`},
		{"chained_getters_100", 100, "item", `<%= for (item) in container.GetItems().GetItems() { %><%= item %>;<% } %>`},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			items := make([]string, scenario.count)
			for i := range items {
				items[i] = scenario.value
			}
			expected := strings.Repeat(html.EscapeString(scenario.value)+";", scenario.count)
			ctx := rootplush.NewContextWith(map[string]interface{}{
				"items":     items,
				"container": &chainedLoopContainer{items: &chainedLoopItems{items: items}},
			})
			tmpl, err := vmplush.Compile(scenario.input)
			if err != nil {
				b.Fatal(err)
			}
			out, err := tmpl.Render(ctx)
			if err != nil || out != expected {
				b.Fatalf("unexpected render %q, %v", out, err)
			}
			diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
			if !ok || diagnostics.FastPath != rootplush.RenderFastPathFast {
				b.Fatalf("unexpected render path: %+v", diagnostics)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := tmpl.Render(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
