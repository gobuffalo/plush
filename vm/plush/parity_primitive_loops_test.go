package plush_test

import (
	"math"
	"reflect"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

type primitiveLoopCustomValue int

func (primitiveLoopCustomValue) String() string { return "custom" }

func Test_Parity_Primitive_Loops_Types(t *testing.T) {
	for _, tt := range []struct {
		name     string
		items    interface{}
		expected string
	}{
		{"int", []int{-1000, 0, 1000}, "-1000;0;1000;"},
		{"int8 limits", []int8{math.MinInt8, 0, math.MaxInt8}, "-128;0;127;"},
		{"int16 limits", []int16{math.MinInt16, 0, math.MaxInt16}, "-32768;0;32767;"},
		{"int32 limits", []int32{math.MinInt32, 0, math.MaxInt32}, "-2147483648;0;2147483647;"},
		{"int64 limits", []int64{math.MinInt64, math.MaxInt64}, "-9223372036854775808;9223372036854775807;"},
		{"uint", []uint{0, 1000}, "0;1000;"},
		{"uint8 limits", []uint8{0, math.MaxUint8}, "0;255;"},
		{"uint16 limits", []uint16{0, math.MaxUint16}, "0;65535;"},
		{"uint32 limits", []uint32{0, math.MaxUint32}, "0;4294967295;"},
		{"uint64 limits", []uint64{0, math.MaxUint64}, "0;18446744073709551615;"},
		{"float32", []float32{-1.25, 0, 1.25}, "-1.25;0;1.25;"},
		{"float64", []float64{-1.25, 0, 1.25}, "-1.25;0;1.25;"},
		{"special floats", []float64{math.Inf(-1), math.Inf(1), math.NaN(), math.Copysign(0, -1)}, "-Inf;+Inf;NaN;-0;"},
		{"bool", []bool{true, false}, "true;false;"},
		{"empty int", []int{}, ""},
		{"nil int64", []int64(nil), ""},
		{"nil uint64", []uint64(nil), ""},
		{"nil float32", []float32(nil), ""},
		{"nil float64", []float64(nil), ""},
		{"nil bool", []bool(nil), ""},
		{"custom type fallback", []primitiveLoopCustomValue{1, 2}, "custom;custom;"},
		{"fixed array", [2]int{1, 2}, "1;2;"},
		{"mixed fallback", []interface{}{"<a>", 1, false}, "&lt;a&gt;;1;false;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requireBothRender(t, `<%= for (item) in items { %><%= item %>;<% } %>after`, tt.expected+"after", contextWith(map[string]interface{}{"items": tt.items}))
			values := reflect.ValueOf(tt.items)
			if values.Kind() == reflect.Slice {
				array := reflect.New(reflect.ArrayOf(values.Len(), values.Type().Elem())).Elem()
				for i := 0; i < values.Len(); i++ {
					array.Index(i).Set(values.Index(i))
				}
				for _, items := range []interface{}{array.Interface(), array.Addr().Interface()} {
					requireBothRender(t, `<%= for (item) in items { %><%= item %>;<% } %>after`, tt.expected+"after", contextWith(map[string]interface{}{"items": items}))
				}
			}
		})
	}
}

func Test_Parity_Primitive_Loops_Budgets(t *testing.T) {
	costs := rootplush.ZeroCosts()
	costs.LoopIteration = 1
	for _, items := range []interface{}{
		[]int{1, 2, 3}, []int8{1, 2, 3}, []int16{1, 2, 3}, []int32{1, 2, 3}, []int64{1, 2, 3},
		[]uint{1, 2, 3}, []uint8{1, 2, 3}, []uint16{1, 2, 3}, []uint32{1, 2, 3}, []uint64{1, 2, 3},
		[]float32{1, 2, 3}, []float64{1, 2, 3}, []bool{true, false, true},
		[3]int{1, 2, 3}, &[3]int{1, 2, 3}, [3]string{"a", "b", "c"},
	} {
		result := compareBudgetRender(t, `<%= for (item) in items { %>[<%= item %>]<% } %>`, 2, costs, func() map[string]interface{} {
			return map[string]interface{}{"items": items}
		})
		require.ErrorIs(t, result.vmErr, rootplush.ErrBudgetExceeded)
	}
}

func Test_Parity_Primitive_Loops_String_Bodies(t *testing.T) {
	for _, tt := range []struct {
		name, input, expected string
		items                 []string
	}{
		{"value", `<%= for (item) in items { %><%= item %><% } %>`, "ab", []string{"a", "b"}},
		{"suffix", `<%= for (item) in items { %><%= item %>;<% } %>after`, "a;b;after", []string{"a", "b"}},
		{"prefix", `<%= for (item) in items { %>:<%= item %><% } %>`, ":a:b", []string{"a", "b"}},
		{"escaping", `<%= for (item) in items { %><li><%= item %></li><% } %>`, `<li>&lt;&amp;&#34;&#39;&gt;</li>`, []string{`<&"'>`}},
		{"empty", `<%= for (item) in items { %><li><%= item %></li><% } %>after`, "after", []string{}},
		{"nil", `<%= for (item) in items { %><li><%= item %></li><% } %>after`, "after", nil},
		{"empty strings", `<%= for (item) in items { %>[<%= item %>]<% } %>`, "[][a][]", []string{"", "a", ""}},
		{"silent", `<% for (item) in items { %><%= item %>;<% } %>after`, "after", []string{"a", "b"}},
		{"continue fallback", `<%= for (item) in items { if (item == "a") { continue } %><%= item %>;<% } %>`, "b;", []string{"a", "b"}},
		{"break fallback", `<%= for (item) in items { %><%= item %>;<% break } %>`, "a;", []string{"a", "b"}},
		{"key fallback", `<%= for (i, item) in items { %><%= i %>:<%= item %>;<% } %>`, "0:a;1:b;", []string{"a", "b"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requireBothRender(t, tt.input, tt.expected, contextWith(map[string]interface{}{"items": tt.items}))
		})
	}
}

func Test_Parity_Primitive_Loops_String_Budget(t *testing.T) {
	costs := rootplush.ZeroCosts()
	costs.LoopIteration = 1
	for _, limit := range []int64{2, 3} {
		result := compareBudgetRender(t, `<%= for (item) in items { %><%= item %>;<% } %>`, limit, costs, func() map[string]interface{} {
			return map[string]interface{}{"items": []string{"a", "b", "c"}}
		})
		if limit == 3 {
			require.Equal(t, "a;b;c;", result.vmOut)
		} else {
			require.ErrorIs(t, result.vmErr, rootplush.ErrBudgetExceeded)
		}
	}
}
