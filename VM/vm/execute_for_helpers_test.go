package vm

import (
	"html/template"
	"sort"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

type vmForProduct struct {
	Name string
}

type vmForIterator struct {
	values []interface{}
	index  int
}

func (i *vmForIterator) Next() interface{} {
	if i.index >= len(i.values) {
		return nil
	}
	value := i.values[i.index]
	i.index++
	return value
}

func newExecuteForTestVM(constants ...object.Object) *VM {
	bytecode := &compiler.Bytecode{
		Constants:    constants,
		Instructions: code.Make(code.OpNull),
	}
	return NewWithContext(bytecode, plush.NewContext())
}

func executeForClosure(instructions code.Instructions, numLocals int) *object.Closure {
	return &object.Closure{Fn: &object.CompiledFunction{
		Instructions: instructions,
		NumLocals:    numLocals,
	}}
}

func executeForInstructions(parts ...code.Instructions) code.Instructions {
	out := code.Instructions{}
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

func executeForReturnValueClosure() *object.Closure {
	return executeForClosure(executeForInstructions(
		code.Make(code.OpGetLocal, 1),
		code.Make(code.OpReturnValue),
	), 2)
}

func executeForWriteValueClosure() *object.Closure {
	return executeForClosure(executeForInstructions(
		code.Make(code.OpWriteLocal, 1),
		code.Make(code.OpReturn),
	), 2)
}

func Test_VM_Execute_For_Iterable_Branches(t *testing.T) {
	block := executeForReturnValueClosure()

	tests := []struct {
		name     string
		iterable object.Object
		expected []interface{}
	}{
		{
			name:     "nil iterable",
			iterable: object.NullObject,
			expected: nil,
		},
		{
			name:     "string slice",
			iterable: &object.Native{Value: []string{"a", "b"}},
			expected: []interface{}{"a", "b"},
		},
		{
			name:     "interface slice",
			iterable: &object.Native{Value: []interface{}{"x", 2}},
			expected: []interface{}{"x", 2},
		},
		{
			name:     "object slice",
			iterable: &object.Native{Value: []object.Object{&object.String{Value: "obj"}, &object.Integer{Value: 4}}},
			expected: []interface{}{"obj", 4},
		},
		{
			name:     "reflect slice",
			iterable: &object.Native{Value: []int{5, 6}},
			expected: []interface{}{5, 6},
		},
		{
			name:     "pointer to slice",
			iterable: &object.Native{Value: &[]string{"ptr"}},
			expected: []interface{}{"ptr"},
		},
		{
			name:     "array",
			iterable: &object.Native{Value: [2]string{"aa", "bb"}},
			expected: []interface{}{"aa", "bb"},
		},
		{
			name:     "iterator",
			iterable: &object.Native{Value: &vmForIterator{values: []interface{}{"next", 7}}},
			expected: []interface{}{"next", 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := newExecuteForTestVM().executeFor(tt.iterable, block, "i", "item")
			require.NoError(t, err)
			array, ok := result.(*object.Array)
			require.True(t, ok)
			require.Len(t, array.Elements, len(tt.expected))
			for i, expected := range tt.expected {
				require.Equal(t, expected, object.ToGo(array.Elements[i]))
			}
		})
	}

	var nilSlice *[]string
	result, err := newExecuteForTestVM().executeFor(&object.Native{Value: nilSlice}, block, "i", "item")
	require.NoError(t, err)
	require.Empty(t, result.(*object.Array).Elements)

	_, err = newExecuteForTestVM().executeFor(&object.Native{Value: 7}, block, "i", "item")
	require.ErrorContains(t, err, "could not iterate")
}

func Test_VM_Execute_For_Map_Break_Continue_And_Output_Branches(t *testing.T) {
	mapResult, err := newExecuteForTestVM().executeFor(
		&object.Native{Value: map[string]int{"b": 2, "a": 1}},
		executeForReturnValueClosure(),
		"k",
		"v",
	)
	require.NoError(t, err)
	mapArray := mapResult.(*object.Array)
	got := []string{}
	for _, element := range mapArray.Elements {
		got = append(got, element.Inspect())
	}
	sort.Strings(got)
	require.Equal(t, []string{"1", "2"}, got)

	continueResult, err := newExecuteForTestVM().executeFor(
		&object.Native{Value: []string{"a", "b"}},
		executeForClosure(executeForInstructions(
			code.Make(code.OpWriteLocal, 1),
			code.Make(code.OpContinue),
		), 2),
		"i",
		"item",
	)
	require.NoError(t, err)
	continueArray := continueResult.(*object.Array)
	require.Len(t, continueArray.Elements, 2)
	require.Equal(t, template.HTML("a"), object.ToGo(continueArray.Elements[0]))
	require.Equal(t, template.HTML("b"), object.ToGo(continueArray.Elements[1]))

	breakResult, err := newExecuteForTestVM().executeFor(
		&object.Native{Value: []string{"a", "b", "c"}},
		executeForClosure(executeForInstructions(
			code.Make(code.OpWriteLocal, 1),
			code.Make(code.OpBreak),
		), 2),
		"i",
		"item",
	)
	require.NoError(t, err)
	breakArray := breakResult.(*object.Array)
	require.Len(t, breakArray.Elements, 1)
	require.Equal(t, template.HTML("a"), object.ToGo(breakArray.Elements[0]))

	writeResult, err := newExecuteForTestVM().executeFor(
		&object.Native{Value: []string{"x"}},
		executeForWriteValueClosure(),
		"i",
		"item",
	)
	require.NoError(t, err)
	writeArray := writeResult.(*object.Array)
	require.Len(t, writeArray.Elements, 1)
	require.Equal(t, template.HTML("x"), object.ToGo(writeArray.Elements[0]))
}

func Test_VM_Execute_For_Context_Write_And_Budget_Branches(t *testing.T) {
	nameBlock := executeForClosure(executeForInstructions(
		code.Make(code.OpGetName, 0),
		code.Make(code.OpReturnValue),
	), 1)

	machine := newExecuteForTestVM(&object.String{Value: "item"})
	result, err := machine.executeFor(
		&object.Native{Value: []string{"ctx-value"}},
		nameBlock,
		"i",
		"item",
	)
	require.NoError(t, err)
	array := result.(*object.Array)
	require.Len(t, array.Elements, 1)
	require.Equal(t, "ctx-value", object.ToGo(array.Elements[0]))

	budgetMachine := newExecuteForTestVM()
	budgetMachine.ctx = plush.NewContext().WithBudget(plush.NewBudget(0))
	result, err = budgetMachine.executeFor(
		&object.Native{Value: []string{"blocked"}},
		executeForReturnValueClosure(),
		"i",
		"item",
	)
	require.Error(t, err)
	require.Empty(t, result.(*object.Array).Elements)
}

func Test_VM_Execute_For_Raw_Local_Property_Loop(t *testing.T) {
	machine := newExecuteForTestVM(&object.String{Value: "Name"})
	block := executeForClosure(executeForInstructions(
		code.Make(code.OpWriteLocalProperty, 1, 0),
		code.Make(code.OpReturn),
	), 2)

	require.True(t, loopCanUseRawValues(block))
	require.False(t, loopNeedsContextWrites(block))

	result, err := machine.executeFor(&object.Native{Value: []vmForProduct{{Name: "<bender>"}}}, block, "i", "product")
	require.NoError(t, err)
	array := result.(*object.Array)
	require.Len(t, array.Elements, 1)
	require.Equal(t, template.HTML("&lt;bender&gt;"), object.ToGo(array.Elements[0]))
}

func Test_VM_Execute_For_Raw_Local_Property_Error_And_Iterator_Branches(t *testing.T) {
	rawNameBlock := executeForClosure(executeForInstructions(
		code.Make(code.OpWriteLocalProperty, 1, 0),
		code.Make(code.OpReturn),
	), 2)

	machine := newExecuteForTestVM(&object.String{Value: "Name"})
	result, err := machine.executeFor(&object.Native{Value: []interface{}{vmForProduct{Name: "<fry>"}}}, rawNameBlock, "i", "product")
	require.NoError(t, err)
	array := result.(*object.Array)
	require.Len(t, array.Elements, 1)
	require.Equal(t, template.HTML("&lt;fry&gt;"), object.ToGo(array.Elements[0]))

	result, err = machine.executeFor(&object.Native{Value: &vmForIterator{values: []interface{}{vmForProduct{Name: "<zoidberg>"}}}}, rawNameBlock, "i", "product")
	require.NoError(t, err)
	array = result.(*object.Array)
	require.Len(t, array.Elements, 1)
	require.Equal(t, template.HTML("&lt;zoidberg&gt;"), object.ToGo(array.Elements[0]))

	missingBlock := executeForClosure(executeForInstructions(
		code.Make(code.OpWriteLocalProperty, 1, 0),
		code.Make(code.OpReturn),
	), 2)
	missingMachine := newExecuteForTestVM(&object.String{Value: "Missing"})
	result, err = missingMachine.executeFor(&object.Native{Value: []string{"bad"}}, missingBlock, "i", "product")
	require.ErrorContains(t, err, "does not have a field or method")
	require.Empty(t, result.(*object.Array).Elements)

	result, err = missingMachine.executeFor(&object.Native{Value: map[string]vmForProduct{"bad": {Name: "Bender"}}}, missingBlock, "i", "product")
	require.ErrorContains(t, err, "does not have a field or method")
	require.Empty(t, result.(*object.Array).Elements)
}

func Test_VM_Loop_Helper_Predicates(t *testing.T) {
	require.False(t, loopCanUseRawValues(nil))
	require.True(t, loopNeedsContextWrites(nil))

	nameBlock := executeForClosure(executeForInstructions(
		code.Make(code.OpGetName, 0),
		code.Make(code.OpReturnValue),
	), 1)
	require.False(t, loopCanUseRawValues(nameBlock))
	require.True(t, loopNeedsContextWrites(nameBlock))
	require.False(t, loopCanUseRawValues(executeForClosure(code.Instructions{byte(255)}, 0)))

	rawKey, rawValue := loopObjects("key", nil, true)
	require.Equal(t, &object.Native{Value: "key"}, rawKey)
	require.Same(t, Null, rawValue)

	wrappedKey, wrappedValue := loopObjects("key", 3, false)
	require.Equal(t, &object.String{Value: "key"}, wrappedKey)
	require.Equal(t, &object.Integer{Value: 3}, wrappedValue)
}
