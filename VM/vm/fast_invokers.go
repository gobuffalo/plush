package vm

import (
	"fmt"
	"html/template"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

func writeFastBuilderInvokerForRaw(raw interface{}) writeFastBuilderInvoker {
	switch raw.(type) {
	case func():
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			raw.(func())()
			return nil
		}
	case func() string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			out.WriteString(template.HTMLEscapeString(raw.(func() string)()))
			return nil
		}
	case func(string) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			out.WriteString(template.HTMLEscapeString(raw.(func(string) string)(arg)))
			return nil
		}
	case func(string, string) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return errFastWriteUnsupported
			}
			out.WriteString(template.HTMLEscapeString(raw.(func(string, string) string)(first, second)))
			return nil
		}
	case func(int) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawIntArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(int) string)(arg))
			return nil
		}
	case func(int64) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawInt64Arg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(int64) string)(arg))
			return nil
		}
	case func(uint) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawUintArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(uint) string)(arg))
			return nil
		}
	case func(uint32) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawUint32Arg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(uint32) string)(arg))
			return nil
		}
	case func(uint64) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawUint64Arg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(uint64) string)(arg))
			return nil
		}
	case func(bool) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawBoolArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(bool) string)(arg))
			return nil
		}
	case func(float64) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawFloat64Arg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(float64) string)(arg))
			return nil
		}
	case func(int, string) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawIntArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(int, string) string)(first, second))
			return nil
		}
	case func(string, int) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawIntArg(args.Raw(1))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(string, int) string)(first, second))
			return nil
		}
	case func(uint32, string) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawUint32Arg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(uint32, string) string)(first, second))
			return nil
		}
	case func(string, uint32) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawUint32Arg(args.Raw(1))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(string, uint32) string)(first, second))
			return nil
		}
	case func(bool, string) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawBoolArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(bool, string) string)(first, second))
			return nil
		}
	case func(string, bool) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawBoolArg(args.Raw(1))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(string, bool) string)(first, second))
			return nil
		}
	case func(float64, string) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawFloat64Arg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(float64, string) string)(first, second))
			return nil
		}
	case func(string, float64) string:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawFloat64Arg(args.Raw(1))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastEscapedString(out, raw.(func(string, float64) string)(first, second))
			return nil
		}
	case func() (string, error):
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			value, err := raw.(func() (string, error))()
			if err != nil {
				return fmt.Errorf("could not call %s function: %w", name, err)
			}
			out.WriteString(template.HTMLEscapeString(value))
			return nil
		}
	case func(string) (string, error):
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			value, err := raw.(func(string) (string, error))(arg)
			if err != nil {
				return fmt.Errorf("could not call %s function: %w", name, err)
			}
			out.WriteString(template.HTMLEscapeString(value))
			return nil
		}
	case func() template.HTML:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			out.WriteString(string(raw.(func() template.HTML)()))
			return nil
		}
	case func(string) template.HTML:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			out.WriteString(string(raw.(func(string) template.HTML)(arg)))
			return nil
		}
	case func() (template.HTML, error):
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			value, err := raw.(func() (template.HTML, error))()
			if err != nil {
				return fmt.Errorf("could not call %s function: %w", name, err)
			}
			out.WriteString(string(value))
			return nil
		}
	case func(string) (template.HTML, error):
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			value, err := raw.(func(string) (template.HTML, error))(arg)
			if err != nil {
				return fmt.Errorf("could not call %s function: %w", name, err)
			}
			out.WriteString(string(value))
			return nil
		}
	case func() bool:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			out.WriteString(strconv.FormatBool(raw.(func() bool)()))
			return nil
		}
	case func() int:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			out.WriteString(strconv.FormatInt(int64(raw.(func() int)()), 10))
			return nil
		}
	case func() int64:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			out.WriteString(strconv.FormatInt(raw.(func() int64)(), 10))
			return nil
		}
	case func() uint:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			out.WriteString(strconv.FormatUint(uint64(raw.(func() uint)()), 10))
			return nil
		}
	case func() uint64:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			out.WriteString(strconv.FormatUint(raw.(func() uint64)(), 10))
			return nil
		}
	case func() float64:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			out.WriteString(strconv.FormatFloat(raw.(func() float64)(), 'g', -1, 64))
			return nil
		}
	case func() object.Object:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 0 {
				return errFastWriteUnsupported
			}
			writeFastObject(out, ctx, raw.(func() object.Object)())
			return nil
		}
	case func(string) object.Object:
		return func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs) error {
			if args.Len() != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastObject(out, ctx, raw.(func(string) object.Object)(arg))
			return nil
		}
	}
	return nil
}

func valueFastInvokerForRaw(raw interface{}) valueFastInvoker {
	switch raw.(type) {
	case func():
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			raw.(func())()
			return nil, nil
		}
	case func() string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			return raw.(func() string)(), nil
		}
	case func(string) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string) string)(arg), nil
		}
	case func(int) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawIntArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(int) string)(arg), nil
		}
	case func(int64) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawInt64Arg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(int64) string)(arg), nil
		}
	case func(uint) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawUintArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(uint) string)(arg), nil
		}
	case func(uint32) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawUint32Arg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(uint32) string)(arg), nil
		}
	case func(uint64) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawUint64Arg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(uint64) string)(arg), nil
		}
	case func(bool) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawBoolArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(bool) string)(arg), nil
		}
	case func(float64) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawFloat64Arg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(float64) string)(arg), nil
		}
	case func(string, string) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, string) string)(first, second), nil
		}
	case func(int, string) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawIntArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(int, string) string)(first, second), nil
		}
	case func(string, int) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawIntArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, int) string)(first, second), nil
		}
	case func(int64, string) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawInt64Arg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(int64, string) string)(first, second), nil
		}
	case func(string, int64) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawInt64Arg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, int64) string)(first, second), nil
		}
	case func(uint, string) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawUintArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(uint, string) string)(first, second), nil
		}
	case func(string, uint) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawUintArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, uint) string)(first, second), nil
		}
	case func(uint32, string) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawUint32Arg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(uint32, string) string)(first, second), nil
		}
	case func(string, uint32) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawUint32Arg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, uint32) string)(first, second), nil
		}
	case func(uint64, string) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawUint64Arg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(uint64, string) string)(first, second), nil
		}
	case func(string, uint64) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawUint64Arg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, uint64) string)(first, second), nil
		}
	case func(bool, string) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawBoolArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(bool, string) string)(first, second), nil
		}
	case func(string, bool) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawBoolArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, bool) string)(first, second), nil
		}
	case func(float64, string) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawFloat64Arg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(float64, string) string)(first, second), nil
		}
	case func(string, float64) string:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawFloat64Arg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, float64) string)(first, second), nil
		}
	case func() template.HTML:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			return raw.(func() template.HTML)(), nil
		}
	case func() bool:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			return raw.(func() bool)(), nil
		}
	case func() int:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			return raw.(func() int)(), nil
		}
	case func() int64:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			return raw.(func() int64)(), nil
		}
	case func() uint:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			return raw.(func() uint)(), nil
		}
	case func() uint64:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			return raw.(func() uint64)(), nil
		}
	case func() float64:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			return raw.(func() float64)(), nil
		}
	case func() object.Object:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			return raw.(func() object.Object)(), nil
		}
	case func() (string, error):
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			value, err := raw.(func() (string, error))()
			if err != nil {
				return nil, fmt.Errorf("could not call %s function: %w", name, err)
			}
			return value, nil
		}
	case func() (template.HTML, error):
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 0 {
				return nil, errFastWriteUnsupported
			}
			value, err := raw.(func() (template.HTML, error))()
			if err != nil {
				return nil, fmt.Errorf("could not call %s function: %w", name, err)
			}
			return value, nil
		}
	case func(string) (string, error):
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			value, err := raw.(func(string) (string, error))(arg)
			if err != nil {
				return nil, fmt.Errorf("could not call %s function: %w", name, err)
			}
			return value, nil
		}
	case func(string, string) (string, error):
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			value, err := raw.(func(string, string) (string, error))(first, second)
			if err != nil {
				return nil, fmt.Errorf("could not call %s function: %w", name, err)
			}
			return value, nil
		}
	case func(string) template.HTML:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string) template.HTML)(arg), nil
		}
	case func(string, string) template.HTML:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, string) template.HTML)(first, second), nil
		}
	case func(string) (template.HTML, error):
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			value, err := raw.(func(string) (template.HTML, error))(arg)
			if err != nil {
				return nil, fmt.Errorf("could not call %s function: %w", name, err)
			}
			return value, nil
		}
	case func(string, string) (template.HTML, error):
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 2 {
				return nil, errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args.Raw(1))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			value, err := raw.(func(string, string) (template.HTML, error))(first, second)
			if err != nil {
				return nil, fmt.Errorf("could not call %s function: %w", name, err)
			}
			return value, nil
		}
	case func(string) object.Object:
		return func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string) object.Object)(arg), nil
		}
	}
	return nil
}

func contextualValueFastInvokerForRaw(raw interface{}) contextualValueFastInvoker {
	switch raw.(type) {
	case func(string, plush.HelperContext) string:
		return func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, plush.HelperContext) string)(arg, plush.NewHelperContext(ctx, nil)), nil
		}
	case func(string, hctx.HelperContext) string:
		return func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, hctx.HelperContext) string)(arg, plush.NewHelperContext(ctx, nil)), nil
		}
	case func(string, plush.HelperContext) (string, error):
		return func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			value, err := raw.(func(string, plush.HelperContext) (string, error))(arg, plush.NewHelperContext(ctx, nil))
			if err != nil {
				return nil, fmt.Errorf("could not call %s function: %w", name, err)
			}
			return value, nil
		}
	case func(string, hctx.HelperContext) (string, error):
		return func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			value, err := raw.(func(string, hctx.HelperContext) (string, error))(arg, plush.NewHelperContext(ctx, nil))
			if err != nil {
				return nil, fmt.Errorf("could not call %s function: %w", name, err)
			}
			return value, nil
		}
	case func(string, plush.HelperContext) template.HTML:
		return func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, plush.HelperContext) template.HTML)(arg, plush.NewHelperContext(ctx, nil)), nil
		}
	case func(string, hctx.HelperContext) template.HTML:
		return func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			return raw.(func(string, hctx.HelperContext) template.HTML)(arg, plush.NewHelperContext(ctx, nil)), nil
		}
	case func(string, plush.HelperContext) (template.HTML, error):
		return func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			value, err := raw.(func(string, plush.HelperContext) (template.HTML, error))(arg, plush.NewHelperContext(ctx, nil))
			if err != nil {
				return nil, fmt.Errorf("could not call %s function: %w", name, err)
			}
			return value, nil
		}
	case func(string, hctx.HelperContext) (template.HTML, error):
		return func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error) {
			if args.Len() != 1 {
				return nil, errFastWriteUnsupported
			}
			arg, ok := fastWriteRawStringArg(args.Raw(0))
			if !ok {
				return nil, errFastWriteUnsupported
			}
			value, err := raw.(func(string, hctx.HelperContext) (template.HTML, error))(arg, plush.NewHelperContext(ctx, nil))
			if err != nil {
				return nil, fmt.Errorf("could not call %s function: %w", name, err)
			}
			return value, nil
		}
	}

	rt := reflect.TypeOf(raw)
	if rt == nil || rt.Kind() != reflect.Func || rt.IsVariadic() || rt.NumIn() < 2 {
		return nil
	}
	helperIndex := rt.NumIn() - 1
	if optionalArgKindFor(rt.In(helperIndex)) != optionalArgHelperContext {
		return nil
	}
	requiredArgs := helperIndex
	return func(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, error) {
		if args.Len() != requiredArgs {
			return nil, errFastWriteUnsupported
		}
		var scratch [4]reflect.Value
		reflectArgs := scratch[:0]
		if cap(reflectArgs) < rt.NumIn() {
			reflectArgs = make([]reflect.Value, 0, rt.NumIn())
		}
		for pos := 0; pos < requiredArgs; pos++ {
			arg, err := fastReflectArgForCall(name, pos, args.Raw(pos), rt.In(pos))
			if err != nil {
				return nil, errFastWriteUnsupported
			}
			reflectArgs = append(reflectArgs, arg)
		}
		helperCtx, ok := fastOptionalArg(optionalArgHelperContext, rt.In(helperIndex), ctx)
		if !ok {
			return nil, errFastWriteUnsupported
		}
		reflectArgs = append(reflectArgs, helperCtx)
		res := reflect.ValueOf(raw).Call(reflectArgs)
		if len(res) == 0 {
			return nil, nil
		}
		if err := lastReturnError(res); err != nil {
			return nil, fmt.Errorf("could not call %s function: %w", name, err)
		}
		if isNilReflectValue(res[0]) {
			return nil, nil
		}
		return res[0].Interface(), nil
	}
}

func writeFastInvokerForRaw(raw interface{}) writeFastInvoker {
	switch raw.(type) {
	case func():
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			raw.(func())()
			markFastOutput(frame)
			return nil
		}
	case func() string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func() string)())
			return nil
		}
	case func(string) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteStringArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(string) string)(arg))
			return nil
		}
	case func(string, string) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteStringArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteStringArg(args[1])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(string, string) string)(first, second))
			return nil
		}
	case func(int) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawIntArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(int) string)(arg))
			return nil
		}
	case func(int64) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawInt64Arg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(int64) string)(arg))
			return nil
		}
	case func(uint) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawUintArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(uint) string)(arg))
			return nil
		}
	case func(uint32) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawUint32Arg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(uint32) string)(arg))
			return nil
		}
	case func(uint64) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawUint64Arg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(uint64) string)(arg))
			return nil
		}
	case func(bool) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawBoolArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(bool) string)(arg))
			return nil
		}
	case func(float64) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteRawFloat64Arg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(float64) string)(arg))
			return nil
		}
	case func(int, string) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawIntArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args[1])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(int, string) string)(first, second))
			return nil
		}
	case func(string, int) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawIntArg(args[1])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(string, int) string)(first, second))
			return nil
		}
	case func(uint32, string) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawUint32Arg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args[1])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(uint32, string) string)(first, second))
			return nil
		}
	case func(string, uint32) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawUint32Arg(args[1])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(string, uint32) string)(first, second))
			return nil
		}
	case func(bool, string) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawBoolArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args[1])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(bool, string) string)(first, second))
			return nil
		}
	case func(string, bool) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawBoolArg(args[1])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(string, bool) string)(first, second))
			return nil
		}
	case func(float64, string) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawFloat64Arg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawStringArg(args[1])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(float64, string) string)(first, second))
			return nil
		}
	case func(string, float64) string:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 2 {
				return errFastWriteUnsupported
			}
			first, ok := fastWriteRawStringArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			second, ok := fastWriteRawFloat64Arg(args[1])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastString(frame, raw.(func(string, float64) string)(first, second))
			return nil
		}
	case func() (string, error):
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			value, err := raw.(func() (string, error))()
			if err != nil {
				return fmt.Errorf("could not call %s function: %w", name, err)
			}
			writeFastString(frame, value)
			return nil
		}
	case func(string) (string, error):
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteStringArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			value, err := raw.(func(string) (string, error))(arg)
			if err != nil {
				return fmt.Errorf("could not call %s function: %w", name, err)
			}
			writeFastString(frame, value)
			return nil
		}
	case func() template.HTML:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			writeFastHTML(frame, raw.(func() template.HTML)())
			return nil
		}
	case func(string) template.HTML:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteStringArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			writeFastHTML(frame, raw.(func(string) template.HTML)(arg))
			return nil
		}
	case func() (template.HTML, error):
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			value, err := raw.(func() (template.HTML, error))()
			if err != nil {
				return fmt.Errorf("could not call %s function: %w", name, err)
			}
			writeFastHTML(frame, value)
			return nil
		}
	case func(string) (template.HTML, error):
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteStringArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			value, err := raw.(func(string) (template.HTML, error))(arg)
			if err != nil {
				return fmt.Errorf("could not call %s function: %w", name, err)
			}
			writeFastHTML(frame, value)
			return nil
		}
	case func() bool:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			writeFastBool(frame, raw.(func() bool)())
			return nil
		}
	case func() int:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			writeFastInt(frame, int64(raw.(func() int)()))
			return nil
		}
	case func() int64:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			writeFastInt(frame, raw.(func() int64)())
			return nil
		}
	case func() uint:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			writeFastUint(frame, uint64(raw.(func() uint)()))
			return nil
		}
	case func() uint64:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			writeFastUint(frame, raw.(func() uint64)())
			return nil
		}
	case func() float64:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			writeFastFloat(frame, raw.(func() float64)(), 64)
			return nil
		}
	case func() object.Object:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 0 {
				return errFastWriteUnsupported
			}
			vm.writeFrameOutput(frame, raw.(func() object.Object)())
			return nil
		}
	case func(string) object.Object:
		return func(vm *VM, frame *Frame, name string, raw interface{}, args []object.Object) error {
			if len(args) != 1 {
				return errFastWriteUnsupported
			}
			arg, ok := fastWriteStringArg(args[0])
			if !ok {
				return errFastWriteUnsupported
			}
			vm.writeFrameOutput(frame, raw.(func(string) object.Object)(arg))
			return nil
		}
	}
	return nil
}

func markFastOutput(frame *Frame) {
	if frame != nil {
		frame.hasOutput = true
	}
}

func writeFastString(frame *Frame, value string) {
	if frame == nil {
		return
	}
	frame.hasOutput = true
	writeFastEscapedString(&frame.output, value)
}

func writeFastHTML(frame *Frame, value template.HTML) {
	if frame == nil {
		return
	}
	frame.hasOutput = true
	frame.output.WriteString(string(value))
}

func writeFastBool(frame *Frame, value bool) {
	if frame == nil {
		return
	}
	frame.hasOutput = true
	frame.output.WriteString(strconv.FormatBool(value))
}

func writeFastInt(frame *Frame, value int64) {
	if frame == nil {
		return
	}
	frame.hasOutput = true
	var buf [20]byte
	frame.output.Write(strconv.AppendInt(buf[:0], value, 10))
}

func writeFastUint(frame *Frame, value uint64) {
	if frame == nil {
		return
	}
	frame.hasOutput = true
	var buf [20]byte
	frame.output.Write(strconv.AppendUint(buf[:0], value, 10))
}

func writeFastFloat(frame *Frame, value float64, bits int) {
	if frame == nil {
		return
	}
	frame.hasOutput = true
	var buf [32]byte
	frame.output.Write(strconv.AppendFloat(buf[:0], value, 'g', -1, bits))
}

func fastWriteStringArg(obj object.Object) (string, bool) {
	switch obj := obj.(type) {
	case *object.String:
		return obj.Value, true
	case *object.Native:
		value, ok := obj.Value.(string)
		return value, ok
	}
	return "", false
}

func fastWriteRawStringArg(value interface{}) (string, bool) {
	value = fastArgGoValue(value)
	switch value := value.(type) {
	case string:
		return value, true
	case object.Object:
		raw, ok := object.ToGo(value).(string)
		return raw, ok
	default:
		raw, ok := value.(fmt.Stringer)
		if ok {
			return raw.String(), true
		}
		rv := reflect.ValueOf(value)
		if rv.IsValid() && rv.Kind() == reflect.String {
			return rv.String(), true
		}
		return "", false
	}
}

func fastWriteRawBoolArg(value interface{}) (bool, bool) {
	value = fastArgGoValue(value)
	if raw, ok := value.(bool); ok {
		return raw, true
	}
	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.Bool {
		return rv.Bool(), true
	}
	return false, false
}

func fastWriteRawIntArg(value interface{}) (int, bool) {
	raw, ok := fastArgInt64(value)
	if !ok || !fastInt64FitsInt(raw) {
		return 0, false
	}
	return int(raw), true
}

func fastWriteRawInt64Arg(value interface{}) (int64, bool) {
	return fastArgInt64(value)
}

func fastWriteRawUintArg(value interface{}) (uint, bool) {
	raw, ok := fastArgUint64(value)
	if !ok || !fastUint64FitsUint(raw) {
		return 0, false
	}
	return uint(raw), true
}

func fastWriteRawUint32Arg(value interface{}) (uint32, bool) {
	raw, ok := fastArgUint64(value)
	if !ok || raw > uint64(^uint32(0)) {
		return 0, false
	}
	return uint32(raw), true
}

func fastWriteRawUint64Arg(value interface{}) (uint64, bool) {
	return fastArgUint64(value)
}

func fastWriteRawFloat64Arg(value interface{}) (float64, bool) {
	return fastArgFloat64(value)
}

func fastInt64FitsInt(value int64) bool {
	return fastInt64FitsIntSize(value, strconv.IntSize)
}

func fastInt64FitsIntSize(value int64, intSize int) bool {
	if intSize == 32 {
		return value >= math.MinInt32 && value <= math.MaxInt32
	}
	return true
}

func fastUint64FitsUint(value uint64) bool {
	return fastUint64FitsUintSize(value, strconv.IntSize)
}

func fastUint64FitsUintSize(value uint64, intSize int) bool {
	if intSize == 32 {
		return value <= math.MaxUint32
	}
	return true
}
