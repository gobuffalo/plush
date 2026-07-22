package vm

import (
	"html/template"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

type vmFieldStringer string

func (s vmFieldStringer) String() string {
	return "stringer:" + string(s)
}

type vmFieldHTMLer struct{}

func (vmFieldHTMLer) HTML() template.HTML {
	return template.HTML("<b>html</b>")
}

type vmFieldKinds struct {
	HTML     template.HTML
	Stringer vmFieldStringer
	String   string
	Bool     bool
	Int      int32
	Uint     uint32
	Float    float32
	Struct   struct{}
	hidden   string
}

func Test_VM_Numeric_Comparison_Helper_Edges(t *testing.T) {
	_, ok := numericValueFromGo(nil)
	require.False(t, ok)
	var nilIface interface{}
	_, ok = numericValueFromGo(&nilIface)
	require.False(t, ok)
	var iface interface{} = int16(-9)
	value, ok := numericValueFromGo(iface)
	require.True(t, ok)
	require.Equal(t, numericSigned, value.kind)
	require.Equal(t, int64(-9), value.i)
	value, ok = numericValueFromGo(int32(-3))
	require.True(t, ok)
	require.Equal(t, numericSigned, value.kind)
	value, ok = numericValueFromGo(uint32(4))
	require.True(t, ok)
	require.Equal(t, numericUnsigned, value.kind)
	value, ok = numericValueFromGo(float32(1.5))
	require.True(t, ok)
	require.Equal(t, numericFloat, value.kind)
	_, ok = numericValueFromGo("nope")
	require.False(t, ok)

	require.True(t, compareNumericEquality(numericValue{kind: numericFloat, f: 1}, numericValue{kind: numericSigned, i: 1}))
	require.True(t, compareNumericEquality(numericValue{kind: numericUnsigned, u: 9}, numericValue{kind: numericSigned, i: 9}))
	require.False(t, compareNumericEquality(numericValue{kind: numericSigned, i: -1}, numericValue{kind: numericUnsigned, u: 1}))
	require.False(t, compareNumericEquality(numericValue{kind: numericSigned, i: -1}, numericValue{kind: numericUnsigned, u: uint64(math.MaxInt64) + 1}))
	require.False(t, compareNumericEquality(numericValue{kind: numericUnsigned, u: 1}, numericValue{kind: numericSigned, i: -1}))
	require.False(t, compareNumericEquality(numericValue{kind: numericUnsigned, u: uint64(math.MaxInt64) + 1}, numericValue{kind: numericSigned, i: -1}))
	require.False(t, compareNumericEquality(numericValue{kind: numericUnsigned, u: uint64(math.MaxInt64) + 1}, numericValue{kind: numericSigned, i: math.MaxInt64}))

	result, err := compareNumericOrdered(code.OpGreaterThan, numericValue{kind: numericFloat, f: 2.5}, numericValue{kind: numericSigned, i: 2})
	require.NoError(t, err)
	require.True(t, result)

	result, err = compareNumericOrdered(code.OpGreaterEqual, numericValue{kind: numericSigned, i: 2}, numericValue{kind: numericSigned, i: 2})
	require.NoError(t, err)
	require.True(t, result)

	result, err = compareNumericOrdered(code.OpGreaterThan, numericValue{kind: numericUnsigned, u: 4}, numericValue{kind: numericUnsigned, u: 9})
	require.NoError(t, err)
	require.False(t, result)

	result, err = compareNumericOrdered(code.OpGreaterThan, numericValue{kind: numericSigned, i: -1}, numericValue{kind: numericUnsigned, u: 0})
	require.NoError(t, err)
	require.False(t, result)

	result, err = compareNumericOrdered(code.OpGreaterEqual, numericValue{kind: numericSigned, i: -1}, numericValue{kind: numericUnsigned, u: 0})
	require.NoError(t, err)
	require.False(t, result)

	result, err = compareNumericOrdered(code.OpGreaterEqual, numericValue{kind: numericSigned, i: 7}, numericValue{kind: numericUnsigned, u: 7})
	require.NoError(t, err)
	require.True(t, result)

	result, err = compareNumericOrdered(code.OpGreaterThan, numericValue{kind: numericUnsigned, u: 0}, numericValue{kind: numericSigned, i: -1})
	require.NoError(t, err)
	require.True(t, result)

	result, err = compareNumericOrdered(code.OpGreaterEqual, numericValue{kind: numericUnsigned, u: 0}, numericValue{kind: numericSigned, i: -1})
	require.NoError(t, err)
	require.True(t, result)

	result, err = compareNumericOrdered(code.OpGreaterThan, numericValue{kind: numericUnsigned, u: 5}, numericValue{kind: numericSigned, i: 7})
	require.NoError(t, err)
	require.False(t, result)

	_, err = compareNumericOrdered(code.OpAdd, numericValue{kind: numericSigned, i: 1}, numericValue{kind: numericUnsigned, u: 1})
	require.ErrorContains(t, err, "unknown ordered comparison")

	_, err = compareFloatOrdered(code.OpAdd, 1, 2)
	require.ErrorContains(t, err, "unknown ordered comparison")
	_, err = compareIntOrdered(code.OpAdd, 1, 2)
	require.ErrorContains(t, err, "unknown ordered comparison")
	_, err = compareUintOrdered(code.OpAdd, 1, 2)
	require.ErrorContains(t, err, "unknown ordered comparison")
}

func Test_VM_Numeric_Operation_Object_Branches(t *testing.T) {
	obj, err := numericOperationObject(code.OpAdd, numericValue{kind: numericFloat, f: 1.5}, numericValue{kind: numericSigned, i: 2})
	require.NoError(t, err)
	require.Equal(t, &object.Float{Value: 3.5}, obj)
	obj, err = numericOperationObject(code.OpSub, numericValue{kind: numericFloat, f: 3}, numericValue{kind: numericFloat, f: 1.25})
	require.NoError(t, err)
	require.Equal(t, &object.Float{Value: 1.75}, obj)
	obj, err = numericOperationObject(code.OpMul, numericValue{kind: numericFloat, f: 2}, numericValue{kind: numericFloat, f: 4})
	require.NoError(t, err)
	require.Equal(t, &object.Float{Value: 8}, obj)
	obj, err = numericOperationObject(code.OpDiv, numericValue{kind: numericFloat, f: 5}, numericValue{kind: numericFloat, f: 2})
	require.NoError(t, err)
	require.Equal(t, &object.Float{Value: 2.5}, obj)
	_, err = numericOperationObject(code.OpDiv, numericValue{kind: numericFloat, f: 1}, numericValue{kind: numericFloat, f: 0})
	require.ErrorContains(t, err, "division by zero")
	_, err = numericOperationObject(code.OpEqual, numericValue{kind: numericFloat, f: 1}, numericValue{kind: numericFloat, f: 1})
	require.ErrorContains(t, err, "unknown numeric operator")

	obj, err = numericOperationObject(code.OpAdd, numericValue{kind: numericSigned, i: 2}, numericValue{kind: numericSigned, i: 3})
	require.NoError(t, err)
	require.Equal(t, &object.Integer{Value: 5}, obj)
	obj, err = numericOperationObject(code.OpSub, numericValue{kind: numericSigned, i: 2}, numericValue{kind: numericSigned, i: 3})
	require.NoError(t, err)
	require.Equal(t, &object.Integer{Value: -1}, obj)
	obj, err = numericOperationObject(code.OpMul, numericValue{kind: numericSigned, i: 2}, numericValue{kind: numericSigned, i: 3})
	require.NoError(t, err)
	require.Equal(t, &object.Integer{Value: 6}, obj)
	obj, err = numericOperationObject(code.OpDiv, numericValue{kind: numericSigned, i: 7}, numericValue{kind: numericSigned, i: 2})
	require.NoError(t, err)
	require.Equal(t, &object.Integer{Value: 3}, obj)
	_, err = numericOperationObject(code.OpDiv, numericValue{kind: numericSigned, i: 1}, numericValue{kind: numericSigned, i: 0})
	require.ErrorContains(t, err, "division by zero")
	_, err = numericOperationObject(code.OpEqual, numericValue{kind: numericSigned, i: 1}, numericValue{kind: numericSigned, i: 1})
	require.ErrorContains(t, err, "unknown numeric operator")

	bigUint := uint64(math.MaxInt64) + 2
	obj, err = numericOperationObject(code.OpAdd, numericValue{kind: numericUnsigned, u: bigUint}, numericValue{kind: numericUnsigned, u: 1})
	require.NoError(t, err)
	require.Equal(t, &object.Native{Value: bigUint + 1}, obj)
	obj, err = numericOperationObject(code.OpSub, numericValue{kind: numericUnsigned, u: bigUint}, numericValue{kind: numericUnsigned, u: 1})
	require.NoError(t, err)
	require.Equal(t, &object.Native{Value: bigUint - 1}, obj)
	obj, err = numericOperationObject(code.OpMul, numericValue{kind: numericUnsigned, u: bigUint}, numericValue{kind: numericUnsigned, u: 1})
	require.NoError(t, err)
	require.Equal(t, &object.Native{Value: bigUint}, obj)
	obj, err = numericOperationObject(code.OpDiv, numericValue{kind: numericUnsigned, u: bigUint}, numericValue{kind: numericUnsigned, u: 2})
	require.NoError(t, err)
	require.Equal(t, &object.Integer{Value: int64(bigUint / 2)}, obj)
	_, err = numericOperationObject(code.OpAdd, numericValue{kind: numericUnsigned, u: math.MaxUint64}, numericValue{kind: numericUnsigned, u: 1})
	require.ErrorContains(t, err, "integer overflow")
	_, err = numericOperationObject(code.OpSub, numericValue{kind: numericUnsigned, u: bigUint}, numericValue{kind: numericUnsigned, u: bigUint + 1})
	require.ErrorContains(t, err, "integer underflow")
	_, err = numericOperationObject(code.OpMul, numericValue{kind: numericUnsigned, u: math.MaxUint64}, numericValue{kind: numericUnsigned, u: 2})
	require.ErrorContains(t, err, "integer overflow")
	_, err = numericOperationObject(code.OpDiv, numericValue{kind: numericUnsigned, u: 1}, numericValue{kind: numericUnsigned, u: 0})
	require.ErrorContains(t, err, "division by zero")
	_, err = numericOperationObject(code.OpEqual, numericValue{kind: numericUnsigned, u: 1}, numericValue{kind: numericUnsigned, u: 1})
	require.ErrorContains(t, err, "unknown numeric operator")
	_, err = numericOperationObject(code.OpDiv, numericValue{kind: numericUnsigned, u: bigUint}, numericValue{kind: numericUnsigned, u: 0})
	require.ErrorContains(t, err, "division by zero")
	_, err = numericOperationObject(code.OpEqual, numericValue{kind: numericUnsigned, u: bigUint}, numericValue{kind: numericUnsigned, u: 1})
	require.ErrorContains(t, err, "unknown numeric operator")
	_, err = numericOperationObject(code.OpAdd, numericValue{kind: numericUnsigned, u: math.MaxUint64}, numericValue{kind: numericSigned, i: -1})
	require.ErrorContains(t, err, "unsupported mixed signed/unsigned")
}

func Test_VM_Truthy_Fast_Reflect_Value_Branches(t *testing.T) {
	require.False(t, isTruthyFastReflectValue(reflect.Value{}))

	var nilIface interface{}
	require.False(t, isTruthyFastReflectValue(reflect.ValueOf(&nilIface).Elem()))

	var objIface interface{} = object.FalseObject
	require.False(t, isTruthyFastReflectValue(reflect.ValueOf(&objIface).Elem()))

	require.True(t, isTruthyFastReflectValue(reflect.ValueOf(true)))
	require.False(t, isTruthyFastReflectValue(reflect.ValueOf(false)))
	require.True(t, isTruthyFastReflectValue(reflect.ValueOf("x")))
	require.False(t, isTruthyFastReflectValue(reflect.ValueOf("")))

	var nilUser *vmFastPropertyUser
	require.False(t, isTruthyFastReflectValue(reflect.ValueOf(nilUser)))
	require.True(t, isTruthyFastReflectValue(reflect.ValueOf(&vmFastPropertyUser{})))
	require.True(t, isTruthyFastReflectValue(reflect.ValueOf(0)))
}

func Test_VM_Write_Fast_Field_Branches(t *testing.T) {
	ctx := plush.NewContext()
	fields := vmFieldKinds{
		HTML:     template.HTML("<b>"),
		Stringer: vmFieldStringer("value"),
		String:   "<x>",
		Bool:     true,
		Int:      -3,
		Uint:     4,
		Float:    1.5,
	}
	rv := reflect.ValueOf(fields)

	tests := []struct {
		name     string
		field    string
		expected string
		handled  bool
	}{
		{"html", "HTML", "<b>", true},
		{"stringer", "Stringer", "stringer:value", true},
		{"string", "String", "&lt;x&gt;", true},
		{"bool", "Bool", "true", true},
		{"int", "Int", "-3", true},
		{"uint", "Uint", "4", true},
		{"float", "Float", "1.5", true},
		{"struct", "Struct", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			handled, err := writeFastField(&out, ctx, rv.FieldByName(tt.field), object.PropertyAccess{}, tt.field, rv.FieldByName(tt.field).Type())
			require.NoError(t, err)
			require.Equal(t, tt.handled, handled)
			require.Equal(t, tt.expected, out.String())
		})
	}

	var nilString *string
	handled, err := writeFastField(&strings.Builder{}, ctx, reflect.ValueOf(&nilString).Elem(), object.PropertyAccess{}, "Nil", stringType)
	require.NoError(t, err)
	require.True(t, handled)

	value := "ptr"
	fieldValue, err := fastFieldValue(reflect.ValueOf(&value), object.PropertyAccess{}, "Value")
	require.NoError(t, err)
	require.Equal(t, "ptr", fieldValue)

	fieldValue, err = fastFieldValue(rv.FieldByName("String"), object.PropertyAccess{}, "String")
	require.NoError(t, err)
	require.Equal(t, "<x>", fieldValue)

	_, err = writeFastField(&strings.Builder{}, ctx, rv.FieldByName("hidden"), object.PropertyAccess{}, "hidden", stringType)
	require.ErrorContains(t, err, "unexported field")

	require.True(t, canWriteFastGoValue(vmFieldHTMLer{}))
}

func Test_VM_Eval_Fast_Logical_Infix_Value_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"left": true, "right": false})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"left", "right"}}, ctx)

	result, ok, err := evalFastLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "&&",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: false},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "missing", NameIndex: 99},
	}, ctx, bindings, nil, nil, nil, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, false, result)

	result, ok, err = evalFastLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "||",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "missing", NameIndex: 99},
	}, ctx, bindings, nil, nil, nil, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, result)

	result, ok, err = evalFastLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "&&",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "left", NameIndex: 0},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "right", NameIndex: 1},
	}, ctx, bindings, nil, nil, nil, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, false, result)

	result, ok, err = evalFastLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "??",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "left", NameIndex: 0},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "right", NameIndex: 1},
	}, ctx, bindings, nil, nil, nil, false)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, result)

	result, ok, err = evalFastLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "&&",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueLoopKey},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: -1},
	}, ctx, bindings, "key", "value", nil, true)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, result)
}
