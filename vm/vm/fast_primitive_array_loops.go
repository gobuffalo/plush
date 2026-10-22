package vm

import (
	"reflect"
	"strings"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/vm/compiler"
)

// Array lengths are part of their Go types. Reflective scalar access handles
// every length without copying the array or boxing each element as interface{}.
// The caller has already dereferenced array pointers.
func renderFastPrimitiveArrayLoop(out *strings.Builder, ctx hctx.Context, loop *compiler.FastLoopPlan, values reflect.Value) (bool, error) {
	if values.Kind() != reflect.Array {
		return false, nil
	}
	// Defined element types can implement custom formatting; keep those on
	// the general path instead of formatting their underlying scalar kind.
	if values.Type().Elem().PkgPath() != "" {
		return false, nil
	}
	kind := values.Type().Elem().Kind()
	switch kind {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
	default:
		return false, nil
	}
	prefix, suffix, ok := fastPrimitiveValueLoopParts(loop)
	if !ok {
		return false, nil
	}
	// A predictable switch on the fixed kind benchmarks faster than an
	// indirect per-kind writer call.
	for i := 0; i < values.Len(); i++ {
		if err := spendFastLoop(ctx, loop.Line); err != nil {
			return true, err
		}
		out.WriteString(prefix)
		value := values.Index(i)
		switch kind {
		case reflect.String:
			writeFastEscapedString(out, value.String())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			writeBuilderFastInt(out, value.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			writeBuilderFastUint(out, value.Uint())
		case reflect.Float32:
			writeBuilderFastFloat(out, value.Float(), 32)
		case reflect.Float64:
			writeBuilderFastFloat(out, value.Float(), 64)
		case reflect.Bool:
			if value.Bool() {
				out.WriteString("true")
			} else {
				out.WriteString("false")
			}
		}
		out.WriteString(suffix)
	}
	return true, nil
}
