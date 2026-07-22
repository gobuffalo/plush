package vm

import (
	"errors"
	"html/template"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

type vmRawStringAlias string
type vmRawBoolAlias bool

func fastArgs(values ...interface{}) *fastCallArgs {
	args := &fastCallArgs{}
	for _, value := range values {
		args.Append(value)
	}
	return args
}

func requireFastBuilderUnsupported(t *testing.T, raw interface{}, args *fastCallArgs) {
	t.Helper()
	invoker := writeFastBuilderInvokerForRaw(raw)
	require.NotNil(t, invoker)
	require.ErrorIs(t, invoker(&strings.Builder{}, plush.NewContext(), "helper", raw, args), errFastWriteUnsupported)
}

func requireFastValueUnsupported(t *testing.T, raw interface{}, args *fastCallArgs) {
	t.Helper()
	invoker := valueFastInvokerForRaw(raw)
	require.NotNil(t, invoker)
	_, err := invoker("helper", raw, args)
	require.ErrorIs(t, err, errFastWriteUnsupported)
}

func requireFastWriteUnsupported(t *testing.T, machine *VM, frame *Frame, raw interface{}, args []object.Object) {
	t.Helper()
	invoker := writeFastInvokerForRaw(raw)
	require.NotNil(t, invoker)
	require.ErrorIs(t, invoker(machine, frame, "helper", raw, args), errFastWriteUnsupported)
}

func Test_VM_Fast_Write_Raw_Arg_Helpers(t *testing.T) {
	value, ok := fastWriteRawStringArg("raw")
	require.True(t, ok)
	require.Equal(t, "raw", value)

	value, ok = fastWriteRawStringArg(&object.String{Value: "object"})
	require.True(t, ok)
	require.Equal(t, "object", value)

	value, ok = fastWriteRawStringArg(&object.Native{Value: "native"})
	require.True(t, ok)
	require.Equal(t, "native", value)

	value, ok = fastWriteRawStringArg(vmFastOutputStringer("value"))
	require.True(t, ok)
	require.Equal(t, "stringer:value", value)

	value, ok = fastWriteRawStringArg(vmRawStringAlias("alias"))
	require.True(t, ok)
	require.Equal(t, "alias", value)

	_, ok = fastWriteRawStringArg(&object.Builtin{})
	require.False(t, ok)
	_, ok = fastWriteRawStringArg(12)
	require.False(t, ok)

	boolValue, ok := fastWriteRawBoolArg(true)
	require.True(t, ok)
	require.True(t, boolValue)
	boolValue, ok = fastWriteRawBoolArg(vmRawBoolAlias(true))
	require.True(t, ok)
	require.True(t, boolValue)
	_, ok = fastWriteRawBoolArg("true")
	require.False(t, ok)

	intValue, ok := fastWriteRawIntArg(int64(3))
	require.True(t, ok)
	require.Equal(t, 3, intValue)
	_, ok = fastWriteRawIntArg(uint64(^uint(0)))
	require.False(t, ok)

	uintValue, ok := fastWriteRawUintArg(uint64(4))
	require.True(t, ok)
	require.Equal(t, uint(4), uintValue)

	uint32Value, ok := fastWriteRawUint32Arg(uint64(5))
	require.True(t, ok)
	require.Equal(t, uint32(5), uint32Value)
	_, ok = fastWriteRawUint32Arg(uint64(^uint32(0)) + 1)
	require.False(t, ok)

	uint64Value, ok := fastWriteRawUint64Arg(uint32(6))
	require.True(t, ok)
	require.Equal(t, uint64(6), uint64Value)

	floatValue, ok := fastWriteRawFloat64Arg(float32(1.5))
	require.True(t, ok)
	require.InDelta(t, 1.5, floatValue, 0.0001)

	require.True(t, fastInt64FitsIntSize(0, 32))
	require.True(t, fastInt64FitsIntSize(int64(^uint(0)>>1), 64))
	require.False(t, fastInt64FitsIntSize(int64(1)<<40, 32))
	require.True(t, fastUint64FitsUintSize(0, 32))
	require.True(t, fastUint64FitsUintSize(uint64(^uint(0)), 64))
	require.False(t, fastUint64FitsUintSize(uint64(1)<<40, 32))
}

func Test_VM_Fast_Builder_Invokers_Unsupported_Return_Signatures(t *testing.T) {
	requireFastBuilderUnsupported(t, func() string { return "" }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func() (string, error) { return "", nil }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func(value string) (string, error) { return value, nil }, fastArgs())
	requireFastBuilderUnsupported(t, func(value string) (string, error) { return value, nil }, fastArgs(1))
	requireFastBuilderUnsupported(t, func() template.HTML { return "" }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func(value string) template.HTML { return template.HTML(value) }, fastArgs())
	requireFastBuilderUnsupported(t, func(value string) template.HTML { return template.HTML(value) }, fastArgs(1))
	requireFastBuilderUnsupported(t, func() (template.HTML, error) { return "", nil }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func(value string) (template.HTML, error) { return template.HTML(value), nil }, fastArgs())
	requireFastBuilderUnsupported(t, func(value string) (template.HTML, error) { return template.HTML(value), nil }, fastArgs(1))
	requireFastBuilderUnsupported(t, func() bool { return true }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func() int { return 1 }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func() int64 { return 1 }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func() uint { return 1 }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func() uint64 { return 1 }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func() float64 { return 1 }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func() object.Object { return Null }, fastArgs("extra"))
	requireFastBuilderUnsupported(t, func(value string) object.Object { return &object.String{Value: value} }, fastArgs())
	requireFastBuilderUnsupported(t, func(value string) object.Object { return &object.String{Value: value} }, fastArgs(1))

	stringErr := func(value string) (string, error) { return "", errors.New("boom") }
	invoker := writeFastBuilderInvokerForRaw(stringErr)
	require.NotNil(t, invoker)
	require.ErrorContains(t, invoker(&strings.Builder{}, plush.NewContext(), "bad", stringErr, fastArgs("x")), "could not call bad function")

	htmlArgErr := func(value string) (template.HTML, error) { return "", errors.New("boom") }
	invoker = writeFastBuilderInvokerForRaw(htmlArgErr)
	require.NotNil(t, invoker)
	require.ErrorContains(t, invoker(&strings.Builder{}, plush.NewContext(), "bad", htmlArgErr, fastArgs("x")), "could not call bad function")

	htmlNoArgErr := func() (template.HTML, error) { return "", errors.New("boom") }
	invoker = writeFastBuilderInvokerForRaw(htmlNoArgErr)
	require.NotNil(t, invoker)
	require.ErrorContains(t, invoker(&strings.Builder{}, plush.NewContext(), "bad", htmlNoArgErr, fastArgs()), "could not call bad function")
}

func Test_VM_Fast_Builder_Invokers_For_Common_Signatures(t *testing.T) {
	tests := []struct {
		name     string
		raw      interface{}
		args     *fastCallArgs
		expected string
	}{
		{"no_args_string", func() string { return "<ok>" }, fastArgs(), "&lt;ok&gt;"},
		{"string", func(value string) string { return value + "!" }, fastArgs("<name>"), "&lt;name&gt;!"},
		{"string_string", func(first, second string) string { return first + ":" + second }, fastArgs("a", "b"), "a:b"},
		{"int", func(value int) string { return strings.Repeat("x", value) }, fastArgs(3), "xxx"},
		{"int64", func(value int64) string { return strings.Repeat("y", int(value)) }, fastArgs(int64(2)), "yy"},
		{"uint", func(value uint) string { return strings.Repeat("u", int(value)) }, fastArgs(uint(2)), "uu"},
		{"uint32", func(value uint32) string { return strings.Repeat("v", int(value)) }, fastArgs(uint32(2)), "vv"},
		{"uint64", func(value uint64) string { return strings.Repeat("w", int(value)) }, fastArgs(uint64(2)), "ww"},
		{"bool", func(value bool) string { return map[bool]string{true: "yes", false: "no"}[value] }, fastArgs(true), "yes"},
		{"float64", func(value float64) string { return strings.Repeat("f", int(value)) }, fastArgs(float64(2.9)), "ff"},
		{"int_string", func(count int, value string) string { return strings.Repeat(value, count) }, fastArgs(2, "a"), "aa"},
		{"string_int", func(value string, count int) string { return strings.Repeat(value, count) }, fastArgs("b", 2), "bb"},
		{"uint32_string", func(count uint32, value string) string { return strings.Repeat(value, int(count)) }, fastArgs(uint32(2), "c"), "cc"},
		{"string_uint32", func(value string, count uint32) string { return strings.Repeat(value, int(count)) }, fastArgs("d", uint32(2)), "dd"},
		{"bool_string", func(ok bool, value string) string {
			if ok {
				return value
			}
			return ""
		}, fastArgs(true, "e"), "e"},
		{"string_bool", func(value string, ok bool) string {
			if ok {
				return value
			}
			return ""
		}, fastArgs("g", true), "g"},
		{"float64_string", func(count float64, value string) string { return strings.Repeat(value, int(count)) }, fastArgs(float64(2), "h"), "hh"},
		{"string_float64", func(value string, count float64) string { return strings.Repeat(value, int(count)) }, fastArgs("i", float64(2)), "ii"},
		{"string_error", func() (string, error) { return "<safe?>", nil }, fastArgs(), "&lt;safe?&gt;"},
		{"string_arg_error", func(value string) (string, error) { return value + "?", nil }, fastArgs("<arg>"), "&lt;arg&gt;?"},
		{"html", func() template.HTML { return template.HTML("<b>") }, fastArgs(), "<b>"},
		{"html_arg", func(value string) template.HTML { return template.HTML("<" + value + ">") }, fastArgs("i"), "<i>"},
		{"html_error", func() (template.HTML, error) { return template.HTML("<em>"), nil }, fastArgs(), "<em>"},
		{"html_arg_error", func(value string) (template.HTML, error) { return template.HTML("<" + value + ">"), nil }, fastArgs("strong"), "<strong>"},
		{"bool_return", func() bool { return true }, fastArgs(), "true"},
		{"int_return", func() int { return 12 }, fastArgs(), "12"},
		{"int64_return", func() int64 { return 13 }, fastArgs(), "13"},
		{"uint_return", func() uint { return 14 }, fastArgs(), "14"},
		{"uint64_return", func() uint64 { return 15 }, fastArgs(), "15"},
		{"float_return", func() float64 { return 1.25 }, fastArgs(), "1.25"},
		{"object_return", func() object.Object { return &object.String{Value: "<obj>"} }, fastArgs(), "&lt;obj&gt;"},
		{"object_arg_return", func(value string) object.Object { return &object.String{Value: value} }, fastArgs("<argobj>"), "&lt;argobj&gt;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := writeFastBuilderInvokerForRaw(tt.raw)
			require.NotNil(t, invoker)
			var out strings.Builder
			require.NoError(t, invoker(&out, plush.NewContext(), "helper", tt.raw, tt.args))
			require.Equal(t, tt.expected, out.String())
		})
	}

	raw := func() (string, error) { return "", errors.New("boom") }
	invoker := writeFastBuilderInvokerForRaw(raw)
	require.NotNil(t, invoker)
	require.ErrorContains(t, invoker(&strings.Builder{}, plush.NewContext(), "bad", raw, fastArgs()), "could not call bad function")

	called := false
	voidRaw := func() { called = true }
	voidInvoker := writeFastBuilderInvokerForRaw(voidRaw)
	require.NotNil(t, voidInvoker)
	require.NoError(t, voidInvoker(&strings.Builder{}, plush.NewContext(), "void", voidRaw, fastArgs()))
	require.True(t, called)
	require.ErrorIs(t, voidInvoker(&strings.Builder{}, plush.NewContext(), "void", voidRaw, fastArgs("extra")), errFastWriteUnsupported)

	require.Nil(t, writeFastBuilderInvokerForRaw("not a helper"))
	require.ErrorIs(t, writeFastBuilderInvokerForRaw(func(value string) string { return value })(&strings.Builder{}, plush.NewContext(), "helper", func(value string) string { return value }, fastArgs()), errFastWriteUnsupported)
	require.ErrorIs(t, writeFastBuilderInvokerForRaw(func(value string) string { return value })(&strings.Builder{}, plush.NewContext(), "helper", func(value string) string { return value }, fastArgs(12)), errFastWriteUnsupported)
	require.ErrorIs(t, writeFastBuilderInvokerForRaw(func(first, second string) string { return first + second })(&strings.Builder{}, plush.NewContext(), "helper", func(first, second string) string { return first + second }, fastArgs("a")), errFastWriteUnsupported)
	require.ErrorIs(t, writeFastBuilderInvokerForRaw(func(first, second string) string { return first + second })(&strings.Builder{}, plush.NewContext(), "helper", func(first, second string) string { return first + second }, fastArgs(1, "b")), errFastWriteUnsupported)
	require.ErrorIs(t, writeFastBuilderInvokerForRaw(func(first, second string) string { return first + second })(&strings.Builder{}, plush.NewContext(), "helper", func(first, second string) string { return first + second }, fastArgs("a", 2)), errFastWriteUnsupported)
	require.ErrorIs(t, writeFastBuilderInvokerForRaw(func(value int) string { return "" })(&strings.Builder{}, plush.NewContext(), "helper", func(value int) string { return "" }, fastArgs()), errFastWriteUnsupported)
	require.ErrorIs(t, writeFastBuilderInvokerForRaw(func(value int) string { return "" })(&strings.Builder{}, plush.NewContext(), "helper", func(value int) string { return "" }, fastArgs("bad")), errFastWriteUnsupported)
	require.ErrorIs(t, writeFastBuilderInvokerForRaw(func(value int64) string { return "" })(&strings.Builder{}, plush.NewContext(), "helper", func(value int64) string { return "" }, fastArgs()), errFastWriteUnsupported)
	require.ErrorIs(t, writeFastBuilderInvokerForRaw(func(value int64) string { return "" })(&strings.Builder{}, plush.NewContext(), "helper", func(value int64) string { return "" }, fastArgs("bad")), errFastWriteUnsupported)
	require.ErrorIs(t, writeFastBuilderInvokerForRaw(func(value uint) string { return "" })(&strings.Builder{}, plush.NewContext(), "helper", func(value uint) string { return "" }, fastArgs()), errFastWriteUnsupported)
	requireFastBuilderUnsupported(t, func(value uint) string { return "" }, fastArgs("bad"))
	requireFastBuilderUnsupported(t, func(value uint32) string { return "" }, fastArgs())
	requireFastBuilderUnsupported(t, func(value uint32) string { return "" }, fastArgs("bad"))
	requireFastBuilderUnsupported(t, func(value uint64) string { return "" }, fastArgs())
	requireFastBuilderUnsupported(t, func(value uint64) string { return "" }, fastArgs("bad"))
	requireFastBuilderUnsupported(t, func(value bool) string { return "" }, fastArgs())
	requireFastBuilderUnsupported(t, func(value bool) string { return "" }, fastArgs("bad"))
	requireFastBuilderUnsupported(t, func(value float64) string { return "" }, fastArgs())
	requireFastBuilderUnsupported(t, func(value float64) string { return "" }, fastArgs("bad"))
	requireFastBuilderUnsupported(t, func(count int, value string) string { return "" }, fastArgs(1))
	requireFastBuilderUnsupported(t, func(count int, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastBuilderUnsupported(t, func(count int, value string) string { return "" }, fastArgs(1, 2))
	requireFastBuilderUnsupported(t, func(value string, count int) string { return "" }, fastArgs("x"))
	requireFastBuilderUnsupported(t, func(value string, count int) string { return "" }, fastArgs(1, 2))
	requireFastBuilderUnsupported(t, func(value string, count int) string { return "" }, fastArgs("x", "bad"))
	requireFastBuilderUnsupported(t, func(count uint32, value string) string { return "" }, fastArgs(uint32(1)))
	requireFastBuilderUnsupported(t, func(count uint32, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastBuilderUnsupported(t, func(count uint32, value string) string { return "" }, fastArgs(uint32(1), 2))
	requireFastBuilderUnsupported(t, func(value string, count uint32) string { return "" }, fastArgs("x"))
	requireFastBuilderUnsupported(t, func(value string, count uint32) string { return "" }, fastArgs(1, uint32(2)))
	requireFastBuilderUnsupported(t, func(value string, count uint32) string { return "" }, fastArgs("x", "bad"))
	requireFastBuilderUnsupported(t, func(ok bool, value string) string { return "" }, fastArgs(true))
	requireFastBuilderUnsupported(t, func(ok bool, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastBuilderUnsupported(t, func(ok bool, value string) string { return "" }, fastArgs(true, 2))
	requireFastBuilderUnsupported(t, func(value string, ok bool) string { return "" }, fastArgs("x"))
	requireFastBuilderUnsupported(t, func(value string, ok bool) string { return "" }, fastArgs(1, true))
	requireFastBuilderUnsupported(t, func(value string, ok bool) string { return "" }, fastArgs("x", "bad"))
	requireFastBuilderUnsupported(t, func(count float64, value string) string { return "" }, fastArgs(float64(1)))
	requireFastBuilderUnsupported(t, func(count float64, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastBuilderUnsupported(t, func(count float64, value string) string { return "" }, fastArgs(float64(1), 2))
	requireFastBuilderUnsupported(t, func(value string, count float64) string { return "" }, fastArgs("x"))
	requireFastBuilderUnsupported(t, func(value string, count float64) string { return "" }, fastArgs(1, float64(2)))
	requireFastBuilderUnsupported(t, func(value string, count float64) string { return "" }, fastArgs("x", "bad"))
}

func Test_VM_Value_Fast_Invokers_For_Common_Signatures(t *testing.T) {
	tests := []struct {
		name     string
		raw      interface{}
		args     *fastCallArgs
		expected interface{}
	}{
		{"no_args_string", func() string { return "ok" }, fastArgs(), "ok"},
		{"string", func(value string) string { return value + "!" }, fastArgs("name"), "name!"},
		{"int", func(value int) string { return strings.Repeat("x", value) }, fastArgs(2), "xx"},
		{"int64", func(value int64) string { return strings.Repeat("i", int(value)) }, fastArgs(int64(2)), "ii"},
		{"uint", func(value uint) string { return strings.Repeat("u", int(value)) }, fastArgs(uint(2)), "uu"},
		{"uint32", func(value uint32) string { return strings.Repeat("v", int(value)) }, fastArgs(uint32(2)), "vv"},
		{"uint64", func(value uint64) string { return strings.Repeat("w", int(value)) }, fastArgs(uint64(2)), "ww"},
		{"bool", func(value bool) string {
			if value {
				return "yes"
			}
			return "no"
		}, fastArgs(true), "yes"},
		{"float64", func(value float64) string { return strings.Repeat("f", int(value)) }, fastArgs(float64(2.1)), "ff"},
		{"string_string", func(first, second string) string { return first + second }, fastArgs("a", "b"), "ab"},
		{"int_string", func(count int, value string) string { return strings.Repeat(value, count) }, fastArgs(2, "a"), "aa"},
		{"string_int", func(value string, count int) string { return strings.Repeat(value, count) }, fastArgs("b", 2), "bb"},
		{"int64_string", func(count int64, value string) string { return strings.Repeat(value, int(count)) }, fastArgs(int64(2), "c"), "cc"},
		{"string_int64", func(value string, count int64) string { return strings.Repeat(value, int(count)) }, fastArgs("d", int64(2)), "dd"},
		{"uint_string", func(count uint, value string) string { return strings.Repeat(value, int(count)) }, fastArgs(uint(2), "e"), "ee"},
		{"string_uint", func(value string, count uint) string { return strings.Repeat(value, int(count)) }, fastArgs("f", uint(2)), "ff"},
		{"uint32_string", func(count uint32, value string) string { return strings.Repeat(value, int(count)) }, fastArgs(uint32(2), "g"), "gg"},
		{"string_uint32", func(value string, count uint32) string { return strings.Repeat(value, int(count)) }, fastArgs("h", uint32(2)), "hh"},
		{"uint64_string", func(count uint64, value string) string { return strings.Repeat(value, int(count)) }, fastArgs(uint64(2), "i"), "ii"},
		{"string_uint64", func(value string, count uint64) string { return strings.Repeat(value, int(count)) }, fastArgs("j", uint64(2)), "jj"},
		{"bool_string", func(ok bool, value string) string {
			if ok {
				return value
			}
			return ""
		}, fastArgs(true, "k"), "k"},
		{"string_bool", func(value string, ok bool) string {
			if ok {
				return value
			}
			return ""
		}, fastArgs("l", true), "l"},
		{"float_string", func(count float64, value string) string { return strings.Repeat(value, int(count)) }, fastArgs(float64(2), "m"), "mm"},
		{"string_float", func(value string, count float64) string { return strings.Repeat(value, int(count)) }, fastArgs("n", float64(2)), "nn"},
		{"html", func() template.HTML { return template.HTML("<b>") }, fastArgs(), template.HTML("<b>")},
		{"bool_return", func() bool { return true }, fastArgs(), true},
		{"int_return", func() int { return 7 }, fastArgs(), 7},
		{"int64_return", func() int64 { return 8 }, fastArgs(), int64(8)},
		{"uint_return", func() uint { return 9 }, fastArgs(), uint(9)},
		{"uint64_return", func() uint64 { return 10 }, fastArgs(), uint64(10)},
		{"float_return", func() float64 { return 2.5 }, fastArgs(), 2.5},
		{"object_return", func() object.Object { return &object.String{Value: "obj"} }, fastArgs(), &object.String{Value: "obj"}},
		{"string_error", func() (string, error) { return "ok", nil }, fastArgs(), "ok"},
		{"html_error", func() (template.HTML, error) { return template.HTML("<ok>"), nil }, fastArgs(), template.HTML("<ok>")},
		{"string_arg_error", func(value string) (string, error) { return value + "!", nil }, fastArgs("arg"), "arg!"},
		{"string_string_error", func(first, second string) (string, error) { return first + second, nil }, fastArgs("a", "b"), "ab"},
		{"html_arg", func(value string) template.HTML { return template.HTML("<" + value + ">") }, fastArgs("i"), template.HTML("<i>")},
		{"html_string_string", func(first, second string) template.HTML { return template.HTML("<" + first + second + ">") }, fastArgs("e", "m"), template.HTML("<em>")},
		{"html_arg_error", func(value string) (template.HTML, error) { return template.HTML("<" + value + ">"), nil }, fastArgs("strong"), template.HTML("<strong>")},
		{"html_string_string_error", func(first, second string) (template.HTML, error) {
			return template.HTML("<" + first + second + ">"), nil
		}, fastArgs("s", "pan"), template.HTML("<span>")},
		{"object_arg", func(value string) object.Object { return &object.String{Value: value} }, fastArgs("obj"), &object.String{Value: "obj"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := valueFastInvokerForRaw(tt.raw)
			require.NotNil(t, invoker)
			value, err := invoker("helper", tt.raw, tt.args)
			require.NoError(t, err)
			require.Equal(t, tt.expected, value)
		})
	}

	raw := func() (template.HTML, error) { return "", errors.New("boom") }
	invoker := valueFastInvokerForRaw(raw)
	require.NotNil(t, invoker)
	_, err := invoker("bad", raw, fastArgs())
	require.ErrorContains(t, err, "could not call bad function")

	called := false
	voidRaw := func() { called = true }
	voidInvoker := valueFastInvokerForRaw(voidRaw)
	require.NotNil(t, voidInvoker)
	value, err := voidInvoker("void", voidRaw, fastArgs())
	require.NoError(t, err)
	require.Nil(t, value)
	require.True(t, called)
	_, err = voidInvoker("void", voidRaw, fastArgs("extra"))
	require.ErrorIs(t, err, errFastWriteUnsupported)

	require.Nil(t, valueFastInvokerForRaw(struct{}{}))
	_, err = valueFastInvokerForRaw(func() string { return "" })("helper", func() string { return "" }, fastArgs("extra"))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value string) string { return value })("helper", func(value string) string { return value }, fastArgs())
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value string) string { return value })("helper", func(value string) string { return value }, fastArgs(12))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value int) string { return "" })("helper", func(value int) string { return "" }, fastArgs())
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value int) string { return "" })("helper", func(value int) string { return "" }, fastArgs("bad"))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value int64) string { return "" })("helper", func(value int64) string { return "" }, fastArgs())
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value int64) string { return "" })("helper", func(value int64) string { return "" }, fastArgs("bad"))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value uint) string { return "" })("helper", func(value uint) string { return "" }, fastArgs())
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value uint) string { return "" })("helper", func(value uint) string { return "" }, fastArgs("bad"))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value uint32) string { return "" })("helper", func(value uint32) string { return "" }, fastArgs())
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value uint32) string { return "" })("helper", func(value uint32) string { return "" }, fastArgs("bad"))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value uint64) string { return "" })("helper", func(value uint64) string { return "" }, fastArgs())
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value uint64) string { return "" })("helper", func(value uint64) string { return "" }, fastArgs("bad"))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value bool) string { return "" })("helper", func(value bool) string { return "" }, fastArgs())
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value bool) string { return "" })("helper", func(value bool) string { return "" }, fastArgs("bad"))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value float64) string { return "" })("helper", func(value float64) string { return "" }, fastArgs())
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(value float64) string { return "" })("helper", func(value float64) string { return "" }, fastArgs("bad"))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(first, second string) string { return first + second })("helper", func(first, second string) string { return first + second }, fastArgs("a"))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(first, second string) string { return first + second })("helper", func(first, second string) string { return first + second }, fastArgs(1, "b"))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	_, err = valueFastInvokerForRaw(func(first, second string) string { return first + second })("helper", func(first, second string) string { return first + second }, fastArgs("a", 2))
	require.ErrorIs(t, err, errFastWriteUnsupported)
	requireFastValueUnsupported(t, func(count int, value string) string { return "" }, fastArgs(1))
	requireFastValueUnsupported(t, func(count int, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastValueUnsupported(t, func(count int, value string) string { return "" }, fastArgs(1, 2))
	requireFastValueUnsupported(t, func(value string, count int) string { return "" }, fastArgs("x"))
	requireFastValueUnsupported(t, func(value string, count int) string { return "" }, fastArgs(1, 2))
	requireFastValueUnsupported(t, func(value string, count int) string { return "" }, fastArgs("x", "bad"))
	requireFastValueUnsupported(t, func(count int64, value string) string { return "" }, fastArgs(int64(1)))
	requireFastValueUnsupported(t, func(count int64, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastValueUnsupported(t, func(count int64, value string) string { return "" }, fastArgs(int64(1), 2))
	requireFastValueUnsupported(t, func(value string, count int64) string { return "" }, fastArgs("x"))
	requireFastValueUnsupported(t, func(value string, count int64) string { return "" }, fastArgs(1, int64(2)))
	requireFastValueUnsupported(t, func(value string, count int64) string { return "" }, fastArgs("x", "bad"))
	requireFastValueUnsupported(t, func(count uint, value string) string { return "" }, fastArgs(uint(1)))
	requireFastValueUnsupported(t, func(count uint, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastValueUnsupported(t, func(count uint, value string) string { return "" }, fastArgs(uint(1), 2))
	requireFastValueUnsupported(t, func(value string, count uint) string { return "" }, fastArgs("x"))
	requireFastValueUnsupported(t, func(value string, count uint) string { return "" }, fastArgs(1, uint(2)))
	requireFastValueUnsupported(t, func(value string, count uint) string { return "" }, fastArgs("x", "bad"))
	requireFastValueUnsupported(t, func(count uint32, value string) string { return "" }, fastArgs(uint32(1)))
	requireFastValueUnsupported(t, func(count uint32, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastValueUnsupported(t, func(count uint32, value string) string { return "" }, fastArgs(uint32(1), 2))
	requireFastValueUnsupported(t, func(value string, count uint32) string { return "" }, fastArgs("x"))
	requireFastValueUnsupported(t, func(value string, count uint32) string { return "" }, fastArgs(1, uint32(2)))
	requireFastValueUnsupported(t, func(value string, count uint32) string { return "" }, fastArgs("x", "bad"))
	requireFastValueUnsupported(t, func(count uint64, value string) string { return "" }, fastArgs(uint64(1)))
	requireFastValueUnsupported(t, func(count uint64, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastValueUnsupported(t, func(count uint64, value string) string { return "" }, fastArgs(uint64(1), 2))
	requireFastValueUnsupported(t, func(value string, count uint64) string { return "" }, fastArgs("x"))
	requireFastValueUnsupported(t, func(value string, count uint64) string { return "" }, fastArgs(1, uint64(2)))
	requireFastValueUnsupported(t, func(value string, count uint64) string { return "" }, fastArgs("x", "bad"))
	requireFastValueUnsupported(t, func(ok bool, value string) string { return "" }, fastArgs(true))
	requireFastValueUnsupported(t, func(ok bool, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastValueUnsupported(t, func(ok bool, value string) string { return "" }, fastArgs(true, 2))
	requireFastValueUnsupported(t, func(value string, ok bool) string { return "" }, fastArgs("x"))
	requireFastValueUnsupported(t, func(value string, ok bool) string { return "" }, fastArgs(1, true))
	requireFastValueUnsupported(t, func(value string, ok bool) string { return "" }, fastArgs("x", "bad"))
	requireFastValueUnsupported(t, func(count float64, value string) string { return "" }, fastArgs(float64(1)))
	requireFastValueUnsupported(t, func(count float64, value string) string { return "" }, fastArgs("bad", "x"))
	requireFastValueUnsupported(t, func(count float64, value string) string { return "" }, fastArgs(float64(1), 2))
	requireFastValueUnsupported(t, func(value string, count float64) string { return "" }, fastArgs("x"))
	requireFastValueUnsupported(t, func(value string, count float64) string { return "" }, fastArgs(1, float64(2)))
	requireFastValueUnsupported(t, func(value string, count float64) string { return "" }, fastArgs("x", "bad"))
}

func Test_VM_Value_Fast_Invokers_Unsupported_Return_Signatures(t *testing.T) {
	requireFastValueUnsupported(t, func() template.HTML { return "" }, fastArgs("extra"))
	requireFastValueUnsupported(t, func() bool { return true }, fastArgs("extra"))
	requireFastValueUnsupported(t, func() int { return 1 }, fastArgs("extra"))
	requireFastValueUnsupported(t, func() int64 { return 1 }, fastArgs("extra"))
	requireFastValueUnsupported(t, func() uint { return 1 }, fastArgs("extra"))
	requireFastValueUnsupported(t, func() uint64 { return 1 }, fastArgs("extra"))
	requireFastValueUnsupported(t, func() float64 { return 1 }, fastArgs("extra"))
	requireFastValueUnsupported(t, func() object.Object { return Null }, fastArgs("extra"))
	requireFastValueUnsupported(t, func() (string, error) { return "", nil }, fastArgs("extra"))
	requireFastValueUnsupported(t, func() (template.HTML, error) { return "", nil }, fastArgs("extra"))
	requireFastValueUnsupported(t, func(value string) (string, error) { return value, nil }, fastArgs())
	requireFastValueUnsupported(t, func(value string) (string, error) { return value, nil }, fastArgs(1))
	requireFastValueUnsupported(t, func(first, second string) (string, error) { return first + second, nil }, fastArgs("a"))
	requireFastValueUnsupported(t, func(first, second string) (string, error) { return first + second, nil }, fastArgs(1, "b"))
	requireFastValueUnsupported(t, func(first, second string) (string, error) { return first + second, nil }, fastArgs("a", 2))
	requireFastValueUnsupported(t, func(value string) template.HTML { return template.HTML(value) }, fastArgs())
	requireFastValueUnsupported(t, func(value string) template.HTML { return template.HTML(value) }, fastArgs(1))
	requireFastValueUnsupported(t, func(first, second string) template.HTML { return template.HTML(first + second) }, fastArgs("a"))
	requireFastValueUnsupported(t, func(first, second string) template.HTML { return template.HTML(first + second) }, fastArgs(1, "b"))
	requireFastValueUnsupported(t, func(first, second string) template.HTML { return template.HTML(first + second) }, fastArgs("a", 2))
	requireFastValueUnsupported(t, func(value string) (template.HTML, error) { return template.HTML(value), nil }, fastArgs())
	requireFastValueUnsupported(t, func(value string) (template.HTML, error) { return template.HTML(value), nil }, fastArgs(1))
	requireFastValueUnsupported(t, func(first, second string) (template.HTML, error) { return template.HTML(first + second), nil }, fastArgs("a"))
	requireFastValueUnsupported(t, func(first, second string) (template.HTML, error) { return template.HTML(first + second), nil }, fastArgs(1, "b"))
	requireFastValueUnsupported(t, func(first, second string) (template.HTML, error) { return template.HTML(first + second), nil }, fastArgs("a", 2))
	requireFastValueUnsupported(t, func(value string) object.Object { return &object.String{Value: value} }, fastArgs())
	requireFastValueUnsupported(t, func(value string) object.Object { return &object.String{Value: value} }, fastArgs(1))

	stringErr := func() (string, error) { return "", errors.New("boom") }
	invoker := valueFastInvokerForRaw(stringErr)
	require.NotNil(t, invoker)
	_, err := invoker("bad", stringErr, fastArgs())
	require.ErrorContains(t, err, "could not call bad function")

	stringArgErr := func(value string) (string, error) { return "", errors.New("boom") }
	invoker = valueFastInvokerForRaw(stringArgErr)
	require.NotNil(t, invoker)
	_, err = invoker("bad", stringArgErr, fastArgs("x"))
	require.ErrorContains(t, err, "could not call bad function")

	stringStringErr := func(first, second string) (string, error) { return "", errors.New("boom") }
	invoker = valueFastInvokerForRaw(stringStringErr)
	require.NotNil(t, invoker)
	_, err = invoker("bad", stringStringErr, fastArgs("a", "b"))
	require.ErrorContains(t, err, "could not call bad function")

	htmlArgErr := func(value string) (template.HTML, error) { return "", errors.New("boom") }
	invoker = valueFastInvokerForRaw(htmlArgErr)
	require.NotNil(t, invoker)
	_, err = invoker("bad", htmlArgErr, fastArgs("x"))
	require.ErrorContains(t, err, "could not call bad function")

	htmlStringStringErr := func(first, second string) (template.HTML, error) { return "", errors.New("boom") }
	invoker = valueFastInvokerForRaw(htmlStringStringErr)
	require.NotNil(t, invoker)
	_, err = invoker("bad", htmlStringStringErr, fastArgs("a", "b"))
	require.ErrorContains(t, err, "could not call bad function")
}

func Test_VM_Write_Fast_Invokers_For_Frame_Output(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())
	frame := machine.currentFrame()

	tests := []struct {
		name     string
		raw      interface{}
		args     []object.Object
		expected string
	}{
		{"no_args_void", func() {}, nil, ""},
		{"no_args_string", func() string { return "<ok>" }, nil, "&lt;ok&gt;"},
		{"string", func(value string) string { return value + "!" }, []object.Object{&object.String{Value: "<name>"}}, "&lt;name&gt;!"},
		{"string_string", func(first, second string) string { return first + second }, []object.Object{&object.String{Value: "a"}, &object.String{Value: "b"}}, "ab"},
		{"int", func(value int) string { return strings.Repeat("x", value) }, []object.Object{&object.Integer{Value: 2}}, "xx"},
		{"int64", func(value int64) string { return strings.Repeat("i", int(value)) }, []object.Object{&object.Integer{Value: 2}}, "ii"},
		{"uint", func(value uint) string { return strings.Repeat("u", int(value)) }, []object.Object{&object.Integer{Value: 2}}, "uu"},
		{"uint32", func(value uint32) string { return strings.Repeat("u", int(value)) }, []object.Object{&object.Integer{Value: 2}}, "uu"},
		{"uint64", func(value uint64) string { return strings.Repeat("w", int(value)) }, []object.Object{&object.Integer{Value: 2}}, "ww"},
		{"bool", func(value bool) string {
			if value {
				return "yes"
			}
			return "no"
		}, []object.Object{True}, "yes"},
		{"float64", func(value float64) string { return strings.Repeat("f", int(value)) }, []object.Object{&object.Float{Value: 2}}, "ff"},
		{"int_string", func(count int, value string) string { return strings.Repeat(value, count) }, []object.Object{&object.Integer{Value: 2}, &object.String{Value: "a"}}, "aa"},
		{"string_int", func(value string, count int) string { return strings.Repeat(value, count) }, []object.Object{&object.String{Value: "b"}, &object.Integer{Value: 2}}, "bb"},
		{"uint32_string", func(count uint32, value string) string { return strings.Repeat(value, int(count)) }, []object.Object{&object.Integer{Value: 2}, &object.String{Value: "c"}}, "cc"},
		{"string_uint32", func(value string, count uint32) string { return strings.Repeat(value, int(count)) }, []object.Object{&object.String{Value: "d"}, &object.Integer{Value: 2}}, "dd"},
		{"bool_string", func(ok bool, value string) string {
			if ok {
				return value
			}
			return "no"
		}, []object.Object{True, &object.String{Value: "e"}}, "e"},
		{"string_bool", func(value string, ok bool) string {
			if ok {
				return value
			}
			return "no"
		}, []object.Object{&object.String{Value: "f"}, True}, "f"},
		{"float_string", func(count float64, value string) string { return strings.Repeat(value, int(count)) }, []object.Object{&object.Float{Value: 2}, &object.String{Value: "g"}}, "gg"},
		{"string_float", func(value string, count float64) string { return strings.Repeat(value, int(count)) }, []object.Object{&object.String{Value: "h"}, &object.Float{Value: 2}}, "hh"},
		{"html", func() template.HTML { return template.HTML("<b>") }, nil, "<b>"},
		{"html_arg", func(value string) template.HTML { return template.HTML("<" + value + ">") }, []object.Object{&object.String{Value: "i"}}, "<i>"},
		{"bool_return", func() bool { return true }, nil, "true"},
		{"int_return", func() int { return 12 }, nil, "12"},
		{"int64_return", func() int64 { return 13 }, nil, "13"},
		{"uint_return", func() uint { return 13 }, nil, "13"},
		{"uint64_return", func() uint64 { return 14 }, nil, "14"},
		{"float_return", func() float64 { return 1.5 }, nil, "1.5"},
		{"object_return", func() object.Object { return &object.String{Value: "<obj>"} }, nil, "&lt;obj&gt;"},
		{"object_arg_return", func(value string) object.Object { return &object.String{Value: value} }, []object.Object{&object.String{Value: "<argobj>"}}, "&lt;argobj&gt;"},
		{"string_error", func() (string, error) { return "<ok>", nil }, nil, "&lt;ok&gt;"},
		{"string_arg_error", func(value string) (string, error) { return value + "!", nil }, []object.Object{&object.String{Value: "<errarg>"}}, "&lt;errarg&gt;!"},
		{"html_error", func() (template.HTML, error) { return template.HTML("<em>"), nil }, nil, "<em>"},
		{"html_arg_error", func(value string) (template.HTML, error) { return template.HTML("<" + value + ">"), nil }, []object.Object{&object.String{Value: "strong"}}, "<strong>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame.output.Reset()
			frame.hasOutput = false
			invoker := writeFastInvokerForRaw(tt.raw)
			require.NotNil(t, invoker)
			require.NoError(t, invoker(machine, frame, "helper", tt.raw, tt.args))
			require.True(t, frame.hasOutput)
			require.Equal(t, tt.expected, frame.output.String())
		})
	}

	raw := func() (string, error) { return "", errors.New("boom") }
	invoker := writeFastInvokerForRaw(raw)
	require.NotNil(t, invoker)
	require.ErrorContains(t, invoker(machine, frame, "bad", raw, nil), "could not call bad function")

	stringArgRaw := func(string) (string, error) { return "", errors.New("boom") }
	stringArgInvoker := writeFastInvokerForRaw(stringArgRaw)
	require.NotNil(t, stringArgInvoker)
	require.ErrorContains(t, stringArgInvoker(machine, frame, "bad", stringArgRaw, []object.Object{&object.String{Value: "x"}}), "could not call bad function")

	htmlRaw := func() (template.HTML, error) { return "", errors.New("boom") }
	htmlInvoker := writeFastInvokerForRaw(htmlRaw)
	require.NotNil(t, htmlInvoker)
	require.ErrorContains(t, htmlInvoker(machine, frame, "bad", htmlRaw, nil), "could not call bad function")

	htmlArgRaw := func(string) (template.HTML, error) { return "", errors.New("boom") }
	htmlArgInvoker := writeFastInvokerForRaw(htmlArgRaw)
	require.NotNil(t, htmlArgInvoker)
	require.ErrorContains(t, htmlArgInvoker(machine, frame, "bad", htmlArgRaw, []object.Object{&object.String{Value: "x"}}), "could not call bad function")

	require.Nil(t, writeFastInvokerForRaw(struct{}{}))
	require.ErrorIs(t, writeFastInvokerForRaw(func(value string) string { return value })(machine, frame, "helper", func(value string) string { return value }, nil), errFastWriteUnsupported)
	requireFastWriteUnsupported(t, machine, frame, func(value string) string { return value }, []object.Object{&object.Integer{Value: 1}})
	require.ErrorIs(t, writeFastInvokerForRaw(func(value string) template.HTML { return template.HTML(value) })(machine, frame, "helper", func(value string) template.HTML { return template.HTML(value) }, nil), errFastWriteUnsupported)
	requireFastWriteUnsupported(t, machine, frame, func(value int) string { return "" }, nil)
	requireFastWriteUnsupported(t, machine, frame, func(value int) string { return "" }, []object.Object{&object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(value int64) string { return "" }, nil)
	requireFastWriteUnsupported(t, machine, frame, func(value int64) string { return "" }, []object.Object{&object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(value uint) string { return "" }, nil)
	requireFastWriteUnsupported(t, machine, frame, func(value uint) string { return "" }, []object.Object{&object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(value uint32) string { return "" }, nil)
	requireFastWriteUnsupported(t, machine, frame, func(value uint32) string { return "" }, []object.Object{&object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(value uint64) string { return "" }, nil)
	requireFastWriteUnsupported(t, machine, frame, func(value uint64) string { return "" }, []object.Object{&object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(value bool) string { return "" }, nil)
	requireFastWriteUnsupported(t, machine, frame, func(value bool) string { return "" }, []object.Object{&object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(value float64) string { return "" }, nil)
	requireFastWriteUnsupported(t, machine, frame, func(value float64) string { return "" }, []object.Object{&object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(count int, value string) string { return "" }, []object.Object{&object.Integer{Value: 1}})
	requireFastWriteUnsupported(t, machine, frame, func(count int, value string) string { return "" }, []object.Object{&object.String{Value: "bad"}, &object.String{Value: "x"}})
	requireFastWriteUnsupported(t, machine, frame, func(count int, value string) string { return "" }, []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, count int) string { return "" }, []object.Object{&object.String{Value: "x"}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, count int) string { return "" }, []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, count int) string { return "" }, []object.Object{&object.String{Value: "x"}, &object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(count uint32, value string) string { return "" }, []object.Object{&object.Integer{Value: 1}})
	requireFastWriteUnsupported(t, machine, frame, func(count uint32, value string) string { return "" }, []object.Object{&object.String{Value: "bad"}, &object.String{Value: "x"}})
	requireFastWriteUnsupported(t, machine, frame, func(count uint32, value string) string { return "" }, []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, count uint32) string { return "" }, []object.Object{&object.String{Value: "x"}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, count uint32) string { return "" }, []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, count uint32) string { return "" }, []object.Object{&object.String{Value: "x"}, &object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(ok bool, value string) string { return "" }, []object.Object{True})
	requireFastWriteUnsupported(t, machine, frame, func(ok bool, value string) string { return "" }, []object.Object{&object.String{Value: "bad"}, &object.String{Value: "x"}})
	requireFastWriteUnsupported(t, machine, frame, func(ok bool, value string) string { return "" }, []object.Object{True, &object.Integer{Value: 2}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, ok bool) string { return "" }, []object.Object{&object.String{Value: "x"}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, ok bool) string { return "" }, []object.Object{&object.Integer{Value: 1}, True})
	requireFastWriteUnsupported(t, machine, frame, func(value string, ok bool) string { return "" }, []object.Object{&object.String{Value: "x"}, &object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(count float64, value string) string { return "" }, []object.Object{&object.Float{Value: 1}})
	requireFastWriteUnsupported(t, machine, frame, func(count float64, value string) string { return "" }, []object.Object{&object.String{Value: "bad"}, &object.String{Value: "x"}})
	requireFastWriteUnsupported(t, machine, frame, func(count float64, value string) string { return "" }, []object.Object{&object.Float{Value: 1}, &object.Integer{Value: 2}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, count float64) string { return "" }, []object.Object{&object.String{Value: "x"}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, count float64) string { return "" }, []object.Object{&object.Integer{Value: 1}, &object.Float{Value: 2}})
	requireFastWriteUnsupported(t, machine, frame, func(value string, count float64) string { return "" }, []object.Object{&object.String{Value: "x"}, &object.String{Value: "bad"}})
	requireFastWriteUnsupported(t, machine, frame, func(value string) (string, error) { return value, nil }, nil)
	requireFastWriteUnsupported(t, machine, frame, func(value string) (string, error) { return value, nil }, []object.Object{&object.Integer{Value: 1}})
	requireFastWriteUnsupported(t, machine, frame, func(value string) template.HTML { return template.HTML(value) }, []object.Object{&object.Integer{Value: 1}})
	requireFastWriteUnsupported(t, machine, frame, func(value string) (template.HTML, error) { return template.HTML(value), nil }, nil)
	requireFastWriteUnsupported(t, machine, frame, func(value string) (template.HTML, error) { return template.HTML(value), nil }, []object.Object{&object.Integer{Value: 1}})
}

func Test_VM_Write_Fast_Invokers_Unsupported_Return_Signatures(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())
	frame := machine.currentFrame()
	extra := []object.Object{&object.String{Value: "extra"}}

	requireFastWriteUnsupported(t, machine, frame, func() {}, extra)
	requireFastWriteUnsupported(t, machine, frame, func() string { return "" }, extra)
	requireFastWriteUnsupported(t, machine, frame, func(first, second string) string { return first + second }, []object.Object{&object.String{Value: "a"}})
	requireFastWriteUnsupported(t, machine, frame, func(first, second string) string { return first + second }, []object.Object{&object.Integer{Value: 1}, &object.String{Value: "b"}})
	requireFastWriteUnsupported(t, machine, frame, func(first, second string) string { return first + second }, []object.Object{&object.String{Value: "a"}, &object.Integer{Value: 2}})
	requireFastWriteUnsupported(t, machine, frame, func() (string, error) { return "", nil }, extra)
	requireFastWriteUnsupported(t, machine, frame, func() template.HTML { return "" }, extra)
	requireFastWriteUnsupported(t, machine, frame, func() (template.HTML, error) { return "", nil }, extra)
	requireFastWriteUnsupported(t, machine, frame, func() bool { return true }, extra)
	requireFastWriteUnsupported(t, machine, frame, func() int { return 1 }, extra)
	requireFastWriteUnsupported(t, machine, frame, func() int64 { return 1 }, extra)
	requireFastWriteUnsupported(t, machine, frame, func() uint { return 1 }, extra)
	requireFastWriteUnsupported(t, machine, frame, func() uint64 { return 1 }, extra)
	requireFastWriteUnsupported(t, machine, frame, func() float64 { return 1 }, extra)
	requireFastWriteUnsupported(t, machine, frame, func() object.Object { return Null }, extra)
	requireFastWriteUnsupported(t, machine, frame, func(value string) object.Object { return &object.String{Value: value} }, nil)
	requireFastWriteUnsupported(t, machine, frame, func(value string) object.Object { return &object.String{Value: value} }, []object.Object{&object.Integer{Value: 1}})
}
