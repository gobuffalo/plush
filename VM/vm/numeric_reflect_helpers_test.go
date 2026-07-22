package vm

import (
	"errors"
	"html/template"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

type vmFastArgInt int32
type vmFastArgInt64 int64
type vmFastArgUint uint16
type vmFastArgUint64 uint64
type vmFastArgFloat float32
type vmFastArgFloat64 float64

type vmReflectPrivate struct {
	hidden string
}

func Test_VM_Numeric_Value_From_Reflect_Value_Branches(t *testing.T) {
	_, ok := numericValueFromReflectValue(reflect.Value{})
	require.False(t, ok)

	var nilPtr *int
	_, ok = numericValueFromReflectValue(reflect.ValueOf(nilPtr))
	require.False(t, ok)

	var nilIface interface{}
	_, ok = numericValueFromReflectValue(reflect.ValueOf(&nilIface).Elem())
	require.False(t, ok)

	value, ok := numericValueFromReflectValue(reflect.ValueOf(int16(-7)))
	require.True(t, ok)
	require.Equal(t, numericSigned, value.kind)
	require.Equal(t, int64(-7), value.i)

	value, ok = numericValueFromReflectValue(reflect.ValueOf(uint16(8)))
	require.True(t, ok)
	require.Equal(t, numericUnsigned, value.kind)
	require.Equal(t, uint64(8), value.u)

	value, ok = numericValueFromReflectValue(reflect.ValueOf(float32(1.25)))
	require.True(t, ok)
	require.Equal(t, numericFloat, value.kind)
	require.InDelta(t, 1.25, value.f, 0.0001)

	var iface interface{} = uint32(9)
	value, ok = numericValueFromReflectValue(reflect.ValueOf(&iface).Elem())
	require.True(t, ok)
	require.Equal(t, numericUnsigned, value.kind)
	require.Equal(t, uint64(9), value.u)

	_, ok = numericValueFromReflectValue(reflect.ValueOf("nope"))
	require.False(t, ok)
}

func Test_VM_Fast_Reflect_Interface_And_Write_Value_Edges(t *testing.T) {
	require.Nil(t, fastReflectInterface(reflect.Value{}))

	var nilPtr *int
	require.Nil(t, fastReflectInterface(reflect.ValueOf(nilPtr)))

	var nilIface interface{}
	require.Nil(t, fastReflectInterface(reflect.ValueOf(&nilIface).Elem()))

	var iface interface{} = "value"
	require.Equal(t, "value", fastReflectInterface(reflect.ValueOf(&iface).Elem()))

	privateField := reflect.ValueOf(vmReflectPrivate{hidden: "secret"}).FieldByName("hidden")
	require.Nil(t, fastReflectInterface(privateField))

	ctx := plush.NewContext()
	var out strings.Builder
	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.Value{}))
	require.Empty(t, out.String())

	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf(nilPtr)))
	require.Empty(t, out.String())

	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf(&nilIface).Elem()))
	require.Empty(t, out.String())

	var objectIface interface{} = &object.String{Value: "<obj>"}
	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf(&objectIface).Elem()))
	require.Equal(t, "&lt;obj&gt;", out.String())

	out.Reset()
	require.NoError(t, writeFastReflectValue(&out, ctx, privateField))
	require.Empty(t, out.String())
}

func Test_VM_Fast_Arg_Numeric_Conversion_Branches(t *testing.T) {
	valueInt, ok := fastArgInt64(&object.Integer{Value: 7})
	require.True(t, ok)
	require.Equal(t, int64(7), valueInt)

	intCases := []struct {
		name     string
		value    interface{}
		expected int64
	}{
		{"int", int(-1), -1},
		{"int8", int8(-2), -2},
		{"int16", int16(-3), -3},
		{"int32", int32(-4), -4},
		{"int64", int64(-5), -5},
		{"uint", uint(1), 1},
		{"uint8", uint8(2), 2},
		{"uint16", uint16(3), 3},
		{"uint32", uint32(4), 4},
		{"uint64", uint64(5), 5},
		{"uintptr", uintptr(6), 6},
		{"float32", float32(7.5), 7},
		{"float64", float64(8.5), 8},
		{"native-int16", &object.Native{Value: int16(9)}, 9},
	}
	for _, tt := range intCases {
		t.Run("int64_"+tt.name, func(t *testing.T) {
			valueInt, ok := fastArgInt64(tt.value)
			require.True(t, ok)
			require.Equal(t, tt.expected, valueInt)
		})
	}

	valueInt, ok = fastArgInt64(vmFastArgInt(-3))
	require.True(t, ok)
	require.Equal(t, int64(-3), valueInt)

	valueInt, ok = fastArgInt64(vmFastArgUint64(12))
	require.True(t, ok)
	require.Equal(t, int64(12), valueInt)

	valueInt, ok = fastArgInt64(vmFastArgFloat(2.75))
	require.True(t, ok)
	require.Equal(t, int64(2), valueInt)

	if uint64(^uint(0)) > uint64(math.MaxInt64) {
		_, ok = fastArgInt64((^uint(0) >> 1) + 1)
		require.False(t, ok)
	}

	if uint64(^uintptr(0)) > uint64(math.MaxInt64) {
		_, ok = fastArgInt64((^uintptr(0) >> 1) + 1)
		require.False(t, ok)
	}

	_, ok = fastArgInt64(uint64(math.MaxInt64) + 1)
	require.False(t, ok)

	_, ok = fastArgInt64(vmFastArgUint64(math.MaxUint64))
	require.False(t, ok)

	_, ok = fastArgInt64(struct{}{})
	require.False(t, ok)

	_, ok = fastArgInt64(nil)
	require.False(t, ok)

	valueUint, ok := fastArgUint64(uint8(4))
	require.True(t, ok)
	require.Equal(t, uint64(4), valueUint)

	uintCases := []struct {
		name     string
		value    interface{}
		expected uint64
	}{
		{"int", int(1), 1},
		{"int8", int8(2), 2},
		{"int16", int16(3), 3},
		{"int32", int32(4), 4},
		{"int64", int64(5), 5},
		{"uint", uint(6), 6},
		{"uint8", uint8(7), 7},
		{"uint16", uint16(8), 8},
		{"uint32", uint32(9), 9},
		{"uint64", uint64(10), 10},
		{"uintptr", uintptr(11), 11},
		{"float32", float32(12.75), 12},
		{"float64", float64(13.75), 13},
		{"native-uint16", &object.Native{Value: uint16(14)}, 14},
	}
	for _, tt := range uintCases {
		t.Run("uint64_"+tt.name, func(t *testing.T) {
			valueUint, ok := fastArgUint64(tt.value)
			require.True(t, ok)
			require.Equal(t, tt.expected, valueUint)
		})
	}

	valueUint, ok = fastArgUint64(vmFastArgUint(5))
	require.True(t, ok)
	require.Equal(t, uint64(5), valueUint)

	valueUint, ok = fastArgUint64(vmFastArgInt64(15))
	require.True(t, ok)
	require.Equal(t, uint64(15), valueUint)

	valueUint, ok = fastArgUint64(vmFastArgFloat(6.25))
	require.True(t, ok)
	require.Equal(t, uint64(6), valueUint)

	_, ok = fastArgUint64(int32(-1))
	require.False(t, ok)

	_, ok = fastArgUint64(int8(-1))
	require.False(t, ok)

	_, ok = fastArgUint64(int16(-1))
	require.False(t, ok)

	_, ok = fastArgUint64(int64(-1))
	require.False(t, ok)

	_, ok = fastArgUint64(vmFastArgInt(-1))
	require.False(t, ok)

	_, ok = fastArgUint64(vmFastArgFloat64(-2.25))
	require.False(t, ok)

	_, ok = fastArgUint64(float32(-1))
	require.False(t, ok)

	_, ok = fastArgUint64(float64(-1))
	require.False(t, ok)

	_, ok = fastArgUint64(struct{}{})
	require.False(t, ok)

	_, ok = fastArgUint64(nil)
	require.False(t, ok)

	valueFloat, ok := fastArgFloat64(uint32(9))
	require.True(t, ok)
	require.Equal(t, float64(9), valueFloat)

	floatCases := []struct {
		name     string
		value    interface{}
		expected float64
	}{
		{"int", int(-1), -1},
		{"int8", int8(-2), -2},
		{"int16", int16(-3), -3},
		{"int32", int32(-4), -4},
		{"int64", int64(-5), -5},
		{"uint", uint(1), 1},
		{"uint8", uint8(2), 2},
		{"uint16", uint16(3), 3},
		{"uint32", uint32(4), 4},
		{"uint64", uint64(5), 5},
		{"uintptr", uintptr(6), 6},
		{"float32", float32(7.25), 7.25},
		{"float64", float64(8.5), 8.5},
		{"native-float32", &object.Native{Value: float32(9.5)}, 9.5},
	}
	for _, tt := range floatCases {
		t.Run("float64_"+tt.name, func(t *testing.T) {
			valueFloat, ok := fastArgFloat64(tt.value)
			require.True(t, ok)
			require.InDelta(t, tt.expected, valueFloat, 0.0001)
		})
	}

	valueFloat, ok = fastArgFloat64(vmFastArgInt(-10))
	require.True(t, ok)
	require.Equal(t, float64(-10), valueFloat)

	valueFloat, ok = fastArgFloat64(vmFastArgUint(11))
	require.True(t, ok)
	require.Equal(t, float64(11), valueFloat)

	valueFloat, ok = fastArgFloat64(vmFastArgFloat(1.5))
	require.True(t, ok)
	require.InDelta(t, 1.5, valueFloat, 0.0001)

	_, ok = fastArgFloat64(struct{}{})
	require.False(t, ok)

	_, ok = fastArgFloat64(nil)
	require.False(t, ok)

	_, ok = fastArgInt64(object.NullObject)
	require.False(t, ok)
}

func Test_VM_Fast_Reflect_Arg_Value_For_Call_Branches(t *testing.T) {
	value, err := fastReflectArgValueForCall("helper", 0, reflect.Value{}, stringType)
	require.NoError(t, err)
	require.Equal(t, "", value.Interface())

	var nilPtr *vmFastPropertyUser
	value, err = fastReflectArgValueForCall("helper", 0, reflect.ValueOf(nilPtr), reflect.TypeOf(&vmFastPropertyUser{}))
	require.NoError(t, err)
	require.True(t, value.IsNil())

	value, err = fastReflectArgValueForCall("helper", 0, reflect.ValueOf("name"), stringType)
	require.NoError(t, err)
	require.Equal(t, "name", value.Interface())

	value, err = fastReflectArgValueForCall("helper", 1, reflect.ValueOf(int32(7)), reflect.TypeOf(int64(0)))
	require.NoError(t, err)
	require.Equal(t, int64(7), value.Interface())

	var iface interface{} = int32(8)
	value, err = fastReflectArgValueForCall("helper", 1, reflect.ValueOf(&iface).Elem(), reflect.TypeOf(int64(0)))
	require.NoError(t, err)
	require.Equal(t, int64(8), value.Interface())

	_, err = fastReflectArgValueForCall("helper", 2, reflect.ValueOf(struct{}{}), stringType)
	require.ErrorContains(t, err, "invalid argument")

	privateField := reflect.ValueOf(vmReflectPrivate{hidden: "secret"}).FieldByName("hidden")
	_, err = fastReflectArgValueForCall("helper", 3, privateField, reflect.TypeOf(0))
	require.ErrorContains(t, err, "invalid argument")
}

func Test_VM_Write_Fast_Reflect_Call_Results_Branches(t *testing.T) {
	ctx := plush.NewContext()
	var out strings.Builder

	require.NoError(t, writeFastReflectCallResults(&out, ctx, "helper", nil))
	require.Empty(t, out.String())

	require.ErrorContains(t, writeFastReflectCallResults(&out, ctx, "helper", []reflect.Value{
		reflect.ValueOf("ignored"),
		reflect.ValueOf(errors.New("boom")),
	}), "could not call helper function")

	require.NoError(t, writeFastReflectCallResults(&out, ctx, "helper", []reflect.Value{reflect.Value{}}))
	require.Empty(t, out.String())

	var nilString *string
	require.NoError(t, writeFastReflectCallResults(&out, ctx, "helper", []reflect.Value{reflect.ValueOf(nilString)}))
	require.Empty(t, out.String())

	tests := []struct {
		name     string
		value    reflect.Value
		expected string
	}{
		{"html", reflect.ValueOf(template.HTML("<b>")), "<b>"},
		{"string", reflect.ValueOf("<x>"), "&lt;x&gt;"},
		{"object", reflect.ValueOf(&object.String{Value: "<obj>"}), "&lt;obj&gt;"},
		{"bool", reflect.ValueOf(true), "true"},
		{"int", reflect.ValueOf(int32(-3)), "-3"},
		{"uint", reflect.ValueOf(uint32(4)), "4"},
		{"float", reflect.ValueOf(float32(1.5)), "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out.Reset()
			require.NoError(t, writeFastReflectCallResults(&out, ctx, "helper", []reflect.Value{tt.value}))
			require.Equal(t, tt.expected, out.String())
		})
	}
}
