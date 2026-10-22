package plush_test

import (
	"strings"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/vm/plush"
)

func BenchmarkChainedLoop(b *testing.B) {
	items := make([]string, 100)
	for i := range items {
		items[i] = "item"
	}
	list := &chainedLoopItems{items: items}
	container := &chainedLoopContainer{items: list}
	fields := &chainedLoopFields{Containers: &chainedLoopFieldItems{Containers: items}}
	expected := strings.Repeat("item;", len(items))

	for _, scenario := range []struct {
		name  string
		input string
	}{
		{"direct", `<%= for (item) in items { %><%= item %>;<% } %>`},
		{"single_call", `<%= for (item) in list.GetItems() { %><%= item %>;<% } %>`},
		{"chained_calls", `<%= for (item) in container.GetItems().GetItems() { %><%= item %>;<% } %>`},
		{"nested_fields", `<%= for (item) in b.Containers.Containers { %><%= item %>;<% } %>`},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			b.Run("compile", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := vmplush.Compile(scenario.input); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("render", func(b *testing.B) {
				tmpl, err := vmplush.Compile(scenario.input)
				if err != nil {
					b.Fatal(err)
				}
				ctx := rootplush.NewContextWith(map[string]interface{}{
					"items": items, "list": list, "container": container, "b": fields,
				})
				out, err := tmpl.Render(ctx)
				if err != nil || out != expected {
					b.Fatalf("unexpected render: output %q, error %v", out, err)
				}
				diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
				if !ok || diagnostics.FastPath != rootplush.RenderFastPathFast {
					b.Fatalf("expected fast rendering, got %+v", diagnostics)
				}
				b.Logf("render path: %s", diagnostics.FastPath)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := tmpl.Render(ctx); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
