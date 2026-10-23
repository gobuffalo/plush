package plush_test

import (
	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/vm/plush"
	"strings"
	"testing"
)

func primitiveBenchmarkSlice[T any](value T) []T {
	items := make([]T, 100)
	for i := range items {
		items[i] = value
	}
	return items
}

func BenchmarkPrimitiveValueLoop(b *testing.B) {
	for _, scenario := range []struct {
		name     string
		items    interface{}
		expected string
	}{
		{"string_100", primitiveBenchmarkSlice("item"), "item;"},
		{"int_100", primitiveBenchmarkSlice(int(1000)), "1000;"},
		{"int8_100", primitiveBenchmarkSlice(int8(100)), "100;"},
		{"int16_100", primitiveBenchmarkSlice(int16(1000)), "1000;"},
		{"int32_100", primitiveBenchmarkSlice(int32(1000)), "1000;"},
		{"int64_100", primitiveBenchmarkSlice(int64(-1000)), "-1000;"},
		{"uint_100", primitiveBenchmarkSlice(uint(1000)), "1000;"},
		{"uint8_100", primitiveBenchmarkSlice(uint8(200)), "200;"},
		{"uint16_100", primitiveBenchmarkSlice(uint16(1000)), "1000;"},
		{"uint32_100", primitiveBenchmarkSlice(uint32(1000)), "1000;"},
		{"uint64_100", primitiveBenchmarkSlice(uint64(1000)), "1000;"},
		{"float32_100", primitiveBenchmarkSlice(float32(1.25)), "1.25;"},
		{"float64_100", primitiveBenchmarkSlice(float64(1.25)), "1.25;"},
		{"bool_100", primitiveBenchmarkSlice(true), "true;"},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			tmpl, err := vmplush.Compile(`<%= for (item) in items { %><%= item %>;<% } %>`)
			if err != nil {
				b.Fatal(err)
			}
			ctx := rootplush.NewContextWith(map[string]interface{}{"items": scenario.items})
			out, err := tmpl.Render(ctx)
			if err != nil || out != strings.Repeat(scenario.expected, 100) {
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
