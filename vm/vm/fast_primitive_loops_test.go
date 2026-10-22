package vm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/vm/compiler"
	"github.com/stretchr/testify/require"
)

func Test_Fast_Primitive_Loop_Types(t *testing.T) {
	loop := &compiler.FastLoopPlan{Line: 5, Parts: []compiler.FastLoopPart{
		{Kind: compiler.FastLoopPartStatic, Value: "["},
		{Kind: compiler.FastLoopPartValue},
		{Kind: compiler.FastLoopPartStatic, Value: "]"},
	}}
	for _, tt := range []struct {
		items interface{}
		want  string
	}{
		{[]string{"<a>", ""}, "[&lt;a&gt;][]"},
		{[]int{-1, 2}, "[-1][2]"},
		{[]int8{-1, 2}, "[-1][2]"},
		{[]int16{-1, 2}, "[-1][2]"},
		{[]int32{-1, 2}, "[-1][2]"},
		{[]int64{-1, 2}, "[-1][2]"},
		{[]uint{1, 2}, "[1][2]"},
		{[]uint8{1, 2}, "[1][2]"},
		{[]uint16{1, 2}, "[1][2]"},
		{[]uint32{1, 2}, "[1][2]"},
		{[]uint64{1, 2}, "[1][2]"},
		{[]float32{-1.25, 2.5}, "[-1.25][2.5]"},
		{[]float64{-1.25, 2.5}, "[-1.25][2.5]"},
		{[]bool{true, false}, "[true][false]"},
	} {
		t.Run(fmt.Sprintf("%T", tt.items), func(t *testing.T) {
			values := reflect.ValueOf(tt.items)
			array := reflect.New(reflect.ArrayOf(values.Len(), values.Type().Elem())).Elem()
			for i := 0; i < values.Len(); i++ {
				array.Index(i).Set(values.Index(i))
			}
			for _, items := range []interface{}{tt.items, array.Interface(), array.Addr().Interface()} {
				var out strings.Builder
				values := reflect.ValueOf(items)
				if values.Kind() == reflect.Ptr {
					values = values.Elem()
				}
				var handled bool
				var err error
				if values.Kind() == reflect.Array {
					handled, err = renderFastPrimitiveArrayLoop(&out, plush.NewContext(), loop, values)
				} else {
					handled, err = renderFastPrimitiveValueLoop(&out, plush.NewContext(), loop, items)
				}
				require.NoError(t, err)
				require.True(t, handled)
				require.Equal(t, tt.want, out.String())
			}
		})
	}
}

func Test_Fast_Primitive_Loop_Unsupported_Body_Does_Not_Write(t *testing.T) {
	for _, parts := range [][]compiler.FastLoopPart{
		nil,
		{{Kind: compiler.FastLoopPartStatic, Value: "prefix"}},
		{{Kind: compiler.FastLoopPartValue}, {Kind: compiler.FastLoopPartValue}},
		{{Kind: compiler.FastLoopPartValue}, {Kind: compiler.FastLoopPartBreak}},
		{{Kind: compiler.FastLoopPartStatic}, {Kind: compiler.FastLoopPartValue}, {Kind: compiler.FastLoopPartValue}},
	} {
		var out strings.Builder
		out.WriteString("before")
		handled, err := renderFastPrimitiveValueLoop(&out, plush.NewContext(), &compiler.FastLoopPlan{Parts: parts}, []string{"item"})
		require.NoError(t, err)
		require.False(t, handled)
		require.Equal(t, "before", out.String())
	}
}

type fastPrimitiveNamedElement int

func (fastPrimitiveNamedElement) String() string { return "custom" }

func Test_Fast_Primitive_Array_Unsupported_Types_Keep_Fallback(t *testing.T) {
	loop := &compiler.FastLoopPlan{Parts: []compiler.FastLoopPart{{Kind: compiler.FastLoopPartValue}}}
	for _, items := range []interface{}{
		[2]fastPrimitiveNamedElement{1, 2}, [2]interface{}{1, "a"},
		[2]complex64{1, 2}, [2]uintptr{1, 2}, (*[2]int)(nil),
	} {
		t.Run(fmt.Sprintf("%T", items), func(t *testing.T) {
			var out strings.Builder
			out.WriteString("before")
			handled, err := renderFastPrimitiveArrayLoop(&out, plush.NewContext(), loop, reflect.ValueOf(items))
			require.NoError(t, err)
			require.False(t, handled)
			require.Equal(t, "before", out.String())
		})
	}
}
