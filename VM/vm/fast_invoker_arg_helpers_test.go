package vm

import (
	"fmt"
	"math"
	"testing"

	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

type vmRawString string
type vmRawBool bool

func (s vmRawString) String() string {
	return "stringer:" + string(s)
}

func Test_VM_Fast_Invoker_String_And_Bool_Arg_Helpers(t *testing.T) {
	value, ok := fastWriteStringArg(&object.String{Value: "object"})
	require.True(t, ok)
	require.Equal(t, "object", value)

	value, ok = fastWriteStringArg(&object.Native{Value: "native"})
	require.True(t, ok)
	require.Equal(t, "native", value)

	_, ok = fastWriteStringArg(&object.Integer{Value: 1})
	require.False(t, ok)

	value, ok = fastWriteRawStringArg("raw")
	require.True(t, ok)
	require.Equal(t, "raw", value)

	value, ok = fastWriteRawStringArg(&object.String{Value: "wrapped"})
	require.True(t, ok)
	require.Equal(t, "wrapped", value)

	value, ok = fastWriteRawStringArg(&object.Native{Value: "native"})
	require.True(t, ok)
	require.Equal(t, "native", value)

	value, ok = fastWriteRawStringArg(vmRawString("value"))
	require.True(t, ok)
	require.Equal(t, "stringer:value", value)

	type plainString string
	value, ok = fastWriteRawStringArg(plainString("plain"))
	require.True(t, ok)
	require.Equal(t, "plain", value)

	_, ok = fastWriteRawStringArg(7)
	require.False(t, ok)

	boolValue, ok := fastWriteRawBoolArg(true)
	require.True(t, ok)
	require.True(t, boolValue)

	boolValue, ok = fastWriteRawBoolArg(vmRawBool(true))
	require.True(t, ok)
	require.True(t, boolValue)

	_, ok = fastWriteRawBoolArg("true")
	require.False(t, ok)
}

func Test_VM_Fast_Invoker_Numeric_Arg_Helpers(t *testing.T) {
	value, ok := fastWriteRawIntArg(int32(7))
	require.True(t, ok)
	require.Equal(t, 7, value)

	_, ok = fastWriteRawIntArg(uint64(math.MaxUint64))
	require.False(t, ok)

	int64Value, ok := fastWriteRawInt64Arg(uint32(9))
	require.True(t, ok)
	require.Equal(t, int64(9), int64Value)

	uintValue, ok := fastWriteRawUintArg(uint32(10))
	require.True(t, ok)
	require.Equal(t, uint(10), uintValue)

	_, ok = fastWriteRawUintArg(-1)
	require.False(t, ok)

	uint64Value, ok := fastWriteRawUint64Arg(uint32(11))
	require.True(t, ok)
	require.Equal(t, uint64(11), uint64Value)

	floatValue, ok := fastWriteRawFloat64Arg(fmt.Stringer(nil))
	require.False(t, ok)
	require.Zero(t, floatValue)

	floatValue, ok = fastWriteRawFloat64Arg(float32(1.5))
	require.True(t, ok)
	require.InDelta(t, 1.5, floatValue, 0.001)
}
