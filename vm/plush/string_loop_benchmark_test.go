package plush_test

import (
	"html"
	"strings"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/vm/plush"
)

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
