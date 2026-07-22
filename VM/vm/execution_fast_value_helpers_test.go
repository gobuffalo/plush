package vm

import (
	"math"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

func Test_VM_Execute_Comparison_Or_Logical_Branches(t *testing.T) {
	tests := []struct {
		name     string
		op       code.Opcode
		left     object.Object
		right    object.Object
		expected bool
	}{
		{"equal", code.OpEqual, &object.Integer{Value: 1}, &object.Native{Value: uint32(1)}, true},
		{"not equal", code.OpNotEqual, &object.String{Value: "a"}, &object.String{Value: "b"}, true},
		{"greater", code.OpGreaterThan, &object.Integer{Value: 2}, &object.Integer{Value: 1}, true},
		{"greater equal", code.OpGreaterEqual, &object.Native{Value: int32(2)}, &object.Integer{Value: 2}, true},
		{"matches", code.OpMatches, &object.String{Value: "abc"}, &object.String{Value: "^a"}, true},
		{"and", code.OpAnd, object.TrueObject, &object.String{Value: "x"}, true},
		{"or", code.OpOr, object.FalseObject, &object.String{Value: "x"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := newRuntimeHelperTestVM(plush.NewContext())
			require.NoError(t, machine.push(tt.left))
			require.NoError(t, machine.push(tt.right))
			require.NoError(t, machine.executeComparisonOrLogical(tt.op))
			result := machine.pop()
			require.Equal(t, nativeBoolToBooleanObject(tt.expected), result)
		})
	}

	machine := newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.push(&object.String{Value: "abc"}))
	require.NoError(t, machine.push(&object.String{Value: "("}))
	require.ErrorContains(t, machine.executeComparisonOrLogical(code.OpMatches), "couldn't compile regex")

	machine = newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.push(&object.String{Value: "a"}))
	require.NoError(t, machine.push(&object.String{Value: "b"}))
	require.ErrorContains(t, machine.executeComparisonOrLogical(code.OpAdd), "unknown comparison operator")

	machine = newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.push(&object.Integer{Value: 1}))
	require.NoError(t, machine.push(&object.String{Value: "one"}))
	require.ErrorContains(t, machine.executeComparisonOrLogical(code.OpEqual), "unable to operate")

	machine = newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.push(&object.Integer{Value: 1}))
	require.NoError(t, machine.push(&object.String{Value: "one"}))
	require.ErrorContains(t, machine.executeComparisonOrLogical(code.OpNotEqual), "unable to operate")

	machine = newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.push(&object.Integer{Value: 1}))
	require.NoError(t, machine.push(&object.String{Value: "one"}))
	require.ErrorContains(t, machine.executeComparisonOrLogical(code.OpGreaterThan), "unable to operate")

	_, err := compareOrdered(code.OpEqual, &object.String{Value: "a"}, &object.String{Value: "b"})
	require.ErrorContains(t, err, "unknown ordered comparison")
}

func Test_VM_Execute_Arithmetic_And_Native_Slice_Edge_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.push(&object.Integer{Value: 1}))
	require.NoError(t, machine.push(&object.Integer{Value: 2}))
	require.ErrorContains(t, machine.executeBinaryOperation(code.OpEqual), "unknown binary operator")

	handled, err := machine.executeNativeSliceAppend(object.NullObject, &object.String{Value: "x"})
	require.NoError(t, err)
	require.False(t, handled)

	var nilStrings *[]string
	handled, err = machine.executeNativeSliceAppend(&object.Native{Value: nilStrings}, &object.String{Value: "x"})
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = machine.executeNativeSliceAppend(&object.Native{Value: 7}, &object.String{Value: "x"})
	require.NoError(t, err)
	require.False(t, handled)

	machine = newRuntimeHelperTestVM(plush.NewContext())
	handled, err = machine.executeNativeSliceAppend(&object.Native{Value: []string{"a"}}, object.NullObject)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, []string{"a", ""}, object.ToGo(machine.pop()))

	machine = newRuntimeHelperTestVM(plush.NewContext())
	handled, err = machine.executeNativeSliceAppend(&object.Native{Value: []interface{}{"a"}}, object.NullObject)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, []interface{}{"a", nil}, object.ToGo(machine.pop()))

	machine = newRuntimeHelperTestVM(plush.NewContext())
	handled, err = machine.executeNativeSliceAppend(&object.Native{Value: []string{}}, &object.Integer{Value: 1})
	require.True(t, handled)
	require.ErrorContains(t, err, "cannot append")

	machine = newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.executeAdd(object.TrueObject, object.TrueObject))
	require.Same(t, object.TrueObject, machine.pop())

	machine = newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.executeAdd(&object.Native{Value: []string{"a"}}, &object.String{Value: "b"}))
	require.Equal(t, []string{"a", "b"}, object.ToGo(machine.pop()))

	machine = newRuntimeHelperTestVM(plush.NewContext())
	require.ErrorContains(t, machine.executeNumericOperation(code.OpAdd, &object.String{Value: "a"}, &object.Integer{Value: 1}), "unsupported types")
	require.ErrorContains(t, machine.executeNumericOperation(code.OpEqual, &object.Integer{Value: 1}, &object.Integer{Value: 1}), "unknown numeric operator")
	require.ErrorContains(t, machine.executeNumericOperation(code.OpAdd, &object.Native{Value: uint64(math.MaxUint64)}, &object.Integer{Value: -1}), "unsupported mixed signed/unsigned")

	require.NoError(t, machine.push(&object.Float{Value: 1.5}))
	require.NoError(t, machine.executeMinusOperator())
	require.Equal(t, &object.Float{Value: -1.5}, machine.pop())

	require.NoError(t, machine.push(&object.Integer{Value: 3}))
	require.NoError(t, machine.executeMinusOperator())
	require.Equal(t, &object.Integer{Value: -3}, machine.pop())

	require.NoError(t, machine.push(&object.String{Value: "bad"}))
	require.ErrorContains(t, machine.executeMinusOperator(), "unsupported type for negation")
}

func Test_VM_Write_Fast_Value_Plan_Output_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"user": vmFastPropertyUser{Name: "<mido>"},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"user"}}, ctx)
	var out strings.Builder

	handled, ok, err := writeFastValuePlanOutput(&out, ctx, bindings, nil)
	require.NoError(t, err)
	require.False(t, handled)
	require.False(t, ok)

	handled, ok, err = writeFastValuePlanOutput(&out, ctx, bindings, &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "plain"})
	require.NoError(t, err)
	require.False(t, handled)
	require.False(t, ok)

	handled, ok, err = writeFastValuePlanOutput(&out, ctx, bindings, &compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 1},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueFloat, FloatValue: 1},
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, ok)
	require.Equal(t, "true", out.String())

	out.Reset()
	handled, ok, err = writeFastValuePlanOutput(&out, ctx, bindings, &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: 0,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Name", Receiver: "user", Full: "user.Name", Line: 1},
		},
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, ok)
	require.Equal(t, "&lt;mido&gt;", out.String())

	out.Reset()
	handled, ok, err = writeFastValuePlanOutput(&out, ctx, bindings, &compiler.FastValuePlan{
		Kind:          compiler.FastValuePath,
		NameIndex:     99,
		Value:         "missing",
		NullOnMissing: true,
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, ok)
	require.Empty(t, out.String())

	handled, ok, err = writeFastValuePlanOutput(&out, ctx, bindings, &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: 99,
		Value:     "missing",
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.False(t, ok)
}
