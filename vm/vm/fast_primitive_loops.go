package vm

import (
	"strings"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/vm/compiler"
)

// Match a single value output with optional static text on either side.
// More complex bodies keep the general loop path, including control flow.
func fastPrimitiveValueLoopParts(loop *compiler.FastLoopPlan) (string, string, bool) {
	if loop == nil || len(loop.Parts) == 0 || len(loop.Parts) > 3 {
		return "", "", false
	}
	parts := loop.Parts
	prefix, suffix := "", ""
	if parts[0].Kind == compiler.FastLoopPartStatic {
		prefix = parts[0].Value
		parts = parts[1:]
	}
	if len(parts) == 0 || parts[0].Kind != compiler.FastLoopPartValue {
		return "", "", false
	}
	parts = parts[1:]
	if len(parts) > 0 {
		if len(parts) != 1 || parts[0].Kind != compiler.FastLoopPartStatic {
			return "", "", false
		}
		suffix = parts[0].Value
	}
	return prefix, suffix, true
}

func renderFastPrimitiveValueLoop(out *strings.Builder, ctx hctx.Context, loop *compiler.FastLoopPlan, iter interface{}) (bool, error) {
	prefix, suffix, ok := fastPrimitiveValueLoopParts(loop)
	if !ok {
		return false, nil
	}
	switch values := iter.(type) {
	case []string:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeFastEscapedString)
	case []int:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeFastPrimitiveSigned[int])
	case []int8:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeFastPrimitiveSigned[int8])
	case []int16:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeFastPrimitiveSigned[int16])
	case []int32:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeFastPrimitiveSigned[int32])
	case []int64:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeBuilderFastInt)
	case []uint:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeFastPrimitiveUnsigned[uint])
	case []uint8:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeFastPrimitiveUnsigned[uint8])
	case []uint16:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeFastPrimitiveUnsigned[uint16])
	case []uint32:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeFastPrimitiveUnsigned[uint32])
	case []uint64:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, writeBuilderFastUint)
	case []float32:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, func(out *strings.Builder, value float32) { writeBuilderFastFloat(out, float64(value), 32) })
	case []float64:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, func(out *strings.Builder, value float64) { writeBuilderFastFloat(out, value, 64) })
	case []bool:
		return true, renderFastPrimitiveValues(out, ctx, loop.Line, values, prefix, suffix, func(out *strings.Builder, value bool) {
			if value {
				out.WriteString("true")
			} else {
				out.WriteString("false")
			}
		})
	default:
		return false, nil
	}
}

func writeFastPrimitiveSigned[T int | int8 | int16 | int32](out *strings.Builder, value T) {
	writeBuilderFastInt(out, int64(value))
}

func writeFastPrimitiveUnsigned[T uint | uint8 | uint16 | uint32](out *strings.Builder, value T) {
	writeBuilderFastUint(out, uint64(value))
}

func renderFastPrimitiveValues[T any](out *strings.Builder, ctx hctx.Context, line int, iter []T, prefix, suffix string, write func(*strings.Builder, T)) error {
	for _, value := range iter {
		if err := spendFastLoop(ctx, line); err != nil {
			return err
		}
		out.WriteString(prefix)
		write(out, value)
		out.WriteString(suffix)
	}
	return nil
}
