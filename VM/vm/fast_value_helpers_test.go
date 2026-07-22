package vm

import (
	"errors"
	"html/template"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

type vmFastValueInterface struct {
	value interface{}
}

func (v vmFastValueInterface) Interface() interface{} {
	return v.value
}

type vmHTMLValue string

func (v vmHTMLValue) HTML() template.HTML {
	return template.HTML(v)
}

type vmFastOutputStringer string

func (s vmFastOutputStringer) String() string {
	return "stringer:" + string(s)
}

type vmFastValueHiddenChild struct {
	child vmFastPropertyChild
}

func Test_VM_Eval_Fast_Value_Edge_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"say":  func() string { return "hello" },
		"user": vmAccessUser{
			Labels: map[string]string{"status": "<ready>"},
		},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name", "say", "user"}}, ctx)

	value, ok, err := evalFastValue(nil, ctx, bindings, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, value)

	tests := []struct {
		name     string
		plan     compiler.FastValuePlan
		expected interface{}
		ok       bool
	}{
		{"name nil", compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "nil"}, nil, true},
		{"name binding", compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 0, Value: "name"}, "Mido", true},
		{"name missing null", compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", NullOnMissing: true}, nil, true},
		{"name missing hard", compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"}, nil, false},
		{"string", compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "x"}, "x", true},
		{"integer", compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 3}, 3, true},
		{"float", compiler.FastValuePlan{Kind: compiler.FastValueFloat, FloatValue: 1.25}, 1.25, true},
		{"bool", compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}, true, true},
		{"loop key outside loop", compiler.FastValuePlan{Kind: compiler.FastValueLoopKey}, nil, false},
		{"invalid", compiler.FastValuePlan{Kind: compiler.FastValueInvalid}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok, err := evalFastValue(&tt.plan, ctx, bindings, nil)
			require.NoError(t, err)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.expected, value)
		})
	}

	value, ok, err = evalFastValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 1},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueFloat, FloatValue: 1},
	}, ctx, bindings, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, value)

	value, ok, err = evalFastValue(&compiler.FastValuePlan{
		Kind: compiler.FastValueCall,
		Call: &compiler.FastCallPlan{Name: "say", NameIndex: 1},
	}, ctx, bindings, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "hello", value)

	value, ok, err = evalFastValue(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: 99,
		Value:     "missing",
	}, ctx, bindings, nil)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, value)

	value, ok, err = evalFastValue(&compiler.FastValuePlan{
		Kind:          compiler.FastValuePath,
		NameIndex:     99,
		Value:         "missing",
		NullOnMissing: true,
	}, ctx, bindings, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, value)

	value, ok, err = evalFastValue(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: 2,
		Value:     "user",
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Labels", Receiver: "user", Full: "user.Labels", Line: 6},
			{Kind: compiler.FastPathStepIndexString, Value: "status", Line: 6},
		},
	}, ctx, bindings, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "<ready>", value)

	value, ok, err = evalFastLoopValue(nil, ctx, bindings, "key", "value")
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, value)

	value, ok, err = evalFastLoopValue(&compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "loop"}, ctx, bindings, "key", "value")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "loop", value)

	value, ok, err = evalFastInfixValue(nil, ctx, bindings, nil, nil, nil, false)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, value)

	value, ok, err = evalFastInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missingLeft"},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 100, Value: "missingRight"},
	}, ctx, bindings, nil, nil, nil, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, value)

	_, ok, err = evalFastInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left: &compiler.FastValuePlan{
			Kind: compiler.FastValueCall,
			Call: &compiler.FastCallPlan{Name: "say", NameIndex: 1, Line: 7},
		},
		Right: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "hello"},
	}, plush.NewContextWith(map[string]interface{}{"say": func() string { return "hello" }}).WithBudget(plush.NewBudget(0)), bindings, nil, nil, nil, false)
	require.ErrorContains(t, err, "line 7")
	require.True(t, ok)

	_, ok, err = evalFastInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "hello"},
		Right: &compiler.FastValuePlan{
			Kind: compiler.FastValueCall,
			Call: &compiler.FastCallPlan{Name: "say", NameIndex: 1, Line: 8},
		},
	}, plush.NewContextWith(map[string]interface{}{"say": func() string { return "hello" }}).WithBudget(plush.NewBudget(0)), bindings, nil, nil, nil, false)
	require.ErrorContains(t, err, "line 8")
	require.True(t, ok)

	_, ok, err = evalFastInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "??",
		Line:     9,
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 1},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 2},
	}, ctx, bindings, nil, nil, nil, false)
	require.ErrorContains(t, err, "line 9")
	require.True(t, ok)

	_, ok, err = evalFastLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "&&",
		Left: &compiler.FastValuePlan{
			Kind: compiler.FastValueCall,
			Call: &compiler.FastCallPlan{Name: "say", NameIndex: 1, Line: 11},
		},
		Right: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
	}, plush.NewContextWith(map[string]interface{}{"say": func() string { return "hello" }}).WithBudget(plush.NewBudget(0)), bindings, nil, nil, nil, false)
	require.ErrorContains(t, err, "line 11")
	require.True(t, ok)

	_, ok, err = evalFastLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "&&",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
		Right: &compiler.FastValuePlan{
			Kind: compiler.FastValueCall,
			Call: &compiler.FastCallPlan{Name: "say", NameIndex: 1, Line: 12},
		},
	}, plush.NewContextWith(map[string]interface{}{"say": func() string { return "hello" }}).WithBudget(plush.NewBudget(0)), bindings, nil, nil, nil, false)
	require.ErrorContains(t, err, "line 12")
	require.True(t, ok)

	var out strings.Builder
	handled, written, err := writeFastValuePlanOutput(&out, ctx, bindings, &compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: ">",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 3},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 2},
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, written)
	require.Equal(t, "true", out.String())

	out.Reset()
	handled, written, err = writeFastValuePlanOutput(&out, ctx, bindings, &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: 2,
		Value:     "user",
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Labels", Receiver: "user", Full: "user.Labels", Line: 10},
			{Kind: compiler.FastPathStepIndexString, Value: "status", Line: 10},
		},
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, written)
	require.Equal(t, "&lt;ready&gt;", out.String())
}

func Test_VM_Fast_Value_Field_And_Access_Edge_Branches(t *testing.T) {
	ctx := plush.NewContext()
	nilChildPlan := &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Child", Receiver: "user", Full: "user.Child", Line: 21},
			{Kind: compiler.FastPathStepProperty, Value: "Name", Receiver: "user", Full: "user.Child.Name", Line: 21},
		},
	}

	value, handled, err := evalFastFieldChainValue(nilChildPlan, vmFastPropertyUser{}, ctx)
	require.NoError(t, err)
	require.True(t, handled)
	require.Nil(t, value)

	hiddenPlan := &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "child", Receiver: "user", Full: "user.child", Line: 22},
			{Kind: compiler.FastPathStepProperty, Value: "Name", Receiver: "user", Full: "user.child.Name", Line: 22},
		},
	}
	_, handled, err = evalFastFieldChainValue(hiddenPlan, vmFastValueHiddenChild{child: vmFastPropertyChild{Name: "hidden"}}, ctx)
	require.ErrorContains(t, err, "line 22")
	require.True(t, handled)

	chain, ok := fastFieldChainPlanFor(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Name"},
			{Kind: compiler.FastPathStepProperty, Value: "Missing"},
		},
	}, reflect.TypeOf(vmStructLoopProduct{}))
	require.False(t, ok)
	require.Nil(t, chain)

	value, handled, err = evalFastAccessChainValue(&compiler.FastValuePlan{Kind: compiler.FastValuePath}, nil, ctx)
	require.NoError(t, err)
	require.True(t, handled)
	require.Nil(t, value)

	chainPlan, next, ok := buildFastAccessChainPlanForSteps([]compiler.FastPathStep{{Kind: compiler.FastPathStepKind(255)}}, reflect.TypeOf(vmStructLoopProduct{}))
	require.False(t, ok)
	require.Nil(t, chainPlan)
	require.Nil(t, next)

	indexStep, next, ok := buildFastIndexAccessStep(reflect.TypeOf([2]string{}), &compiler.FastPathStep{Kind: compiler.FastPathStepIndexInteger, Index: 1, Line: 23})
	require.True(t, ok)
	require.Equal(t, fastAccessStepIndex, indexStep.kind)
	require.Equal(t, stringType, next)

	key, keyOK := fastAccessMapKeyValue(reflect.TypeOf(""), &compiler.FastPathStep{Kind: compiler.FastPathStepKind(255)})
	require.False(t, keyOK)
	require.False(t, key.IsValid())

	value, ok, err = evalFastAccessChainPlanValue(&fastAccessChainPlan{steps: []fastAccessChainStep{{
		kind:      fastAccessStepField,
		name:      "Child",
		receiver:  "user",
		full:      "user.Child",
		line:      24,
		fieldType: reflect.TypeOf((*vmFastPropertyChild)(nil)),
		lookup:    cachedPropertyLookup(reflect.TypeOf(vmFastPropertyUser{}), "Child"),
	}, {
		kind:      fastAccessStepField,
		name:      "Name",
		receiver:  "user",
		full:      "user.Child.Name",
		line:      24,
		fieldType: stringType,
		lookup:    cachedPropertyLookup(reflect.TypeOf(vmFastPropertyChild{}), "Name"),
	}}}, reflect.ValueOf(vmFastPropertyUser{}), ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, value)

	var out strings.Builder
	handledOut, err := writeFastAccessChainPlanOutput(&out, ctx, &fastAccessChainPlan{steps: []fastAccessChainStep{{
		kind:      fastAccessStepField,
		name:      "Child",
		receiver:  "user",
		full:      "user.Child",
		line:      25,
		fieldType: reflect.TypeOf((*vmFastPropertyChild)(nil)),
		lookup:    cachedPropertyLookup(reflect.TypeOf(vmFastPropertyUser{}), "Child"),
	}, {
		kind:      fastAccessStepField,
		name:      "Name",
		receiver:  "user",
		full:      "user.Child.Name",
		line:      25,
		fieldType: stringType,
		lookup:    cachedPropertyLookup(reflect.TypeOf(vmFastPropertyChild{}), "Name"),
	}}}, reflect.ValueOf(vmFastPropertyUser{}))
	require.NoError(t, err)
	require.True(t, handledOut)
	require.Empty(t, out.String())

	directValue, found, directHandled := fastAccessDirectMapIndex(reflect.ValueOf(map[string]string{"key": "value"}), &fastAccessChainStep{mapString: "key", mapDirect: fastMapDirectKind(255)})
	require.False(t, directHandled)
	require.False(t, found)
	require.False(t, directValue.IsValid())

	handledOut, err = writeFastDirectMapIndexOutput(&out, ctx, reflect.ValueOf(map[string]string{"key": "value"}), &fastAccessChainStep{mapString: "key", mapDirect: fastMapDirectKind(255)})
	require.NoError(t, err)
	require.False(t, handledOut)
}

func Test_VM_Fast_Index_Value_Branches(t *testing.T) {
	arrayValue, err := fastIndexValue(&object.Array{Elements: []object.Object{&object.String{Value: "zero"}}}, 0)
	require.NoError(t, err)
	require.Equal(t, &object.String{Value: "zero"}, arrayValue)

	missingArrayValue, err := fastIndexValue(&object.Array{}, 0)
	require.NoError(t, err)
	require.Nil(t, missingArrayValue)

	hashKey := (&object.Integer{Value: 3}).HashKey()
	hashValue, err := fastIndexValue(&object.Hash{Pairs: map[object.HashKey]object.HashPair{
		hashKey: {Key: &object.Integer{Value: 3}, Value: &object.String{Value: "three"}},
	}}, 3)
	require.NoError(t, err)
	require.Equal(t, &object.String{Value: "three"}, hashValue)

	missingHashValue, err := fastIndexValue(&object.Hash{Pairs: map[object.HashKey]object.HashPair{}}, 3)
	require.NoError(t, err)
	require.Nil(t, missingHashValue)

	nilValue, err := fastIndexValue(nil, 0)
	require.NoError(t, err)
	require.Nil(t, nilValue)

	var nilSlice *[]string
	nilValue, err = fastIndexValue(nilSlice, 0)
	require.NoError(t, err)
	require.Nil(t, nilValue)

	sliceValue, err := fastIndexValue([]string{"a", "b"}, 1)
	require.NoError(t, err)
	require.Equal(t, "b", sliceValue)

	rawSlice := []string{"ptr"}
	sliceValue, err = fastIndexValue(&rawSlice, 0)
	require.NoError(t, err)
	require.Equal(t, "ptr", sliceValue)

	_, err = fastIndexValue([]string{"a"}, 2)
	require.ErrorContains(t, err, "out of bounds")

	mapValue, err := fastIndexValue(map[int]string{7: "seven"}, 7)
	require.NoError(t, err)
	require.Equal(t, "seven", mapValue)

	mapInterfaceValue, err := fastIndexValue(map[interface{}]string{8: "eight"}, 8)
	require.NoError(t, err)
	require.Equal(t, "eight", mapInterfaceValue)

	missingMapValue, err := fastIndexValue(map[int]string{}, 7)
	require.NoError(t, err)
	require.Nil(t, missingMapValue)

	_, err = fastIndexValue(map[bool]string{}, 7)
	require.ErrorContains(t, err, "cannot use")

	_, err = fastIndexValue(struct{}{}, 1)
	require.ErrorContains(t, err, "could not index")

	_, err = fastIndexValue(&object.String{Value: "not-indexable"}, 1)
	require.ErrorContains(t, err, "could not index")
}

func Test_VM_Fast_String_Index_Value_Branches(t *testing.T) {
	hashKey := (&object.String{Value: "name"}).HashKey()
	hashValue, err := fastStringIndexValue(&object.Hash{Pairs: map[object.HashKey]object.HashPair{
		hashKey: {Key: &object.String{Value: "name"}, Value: &object.String{Value: "Mido"}},
	}}, "name")
	require.NoError(t, err)
	require.Equal(t, &object.String{Value: "Mido"}, hashValue)

	missingHashValue, err := fastStringIndexValue(&object.Hash{Pairs: map[object.HashKey]object.HashPair{}}, "name")
	require.NoError(t, err)
	require.Nil(t, missingHashValue)

	nilValue, err := fastStringIndexValue(nil, "name")
	require.NoError(t, err)
	require.Nil(t, nilValue)

	var nilMap *map[string]int
	nilValue, err = fastStringIndexValue(nilMap, "name")
	require.NoError(t, err)
	require.Nil(t, nilValue)

	mapValue, err := fastStringIndexValue(map[string]int{"count": 4}, "count")
	require.NoError(t, err)
	require.Equal(t, 4, mapValue)

	rawMap := map[string]int{"count": 5}
	mapValue, err = fastStringIndexValue(&rawMap, "count")
	require.NoError(t, err)
	require.Equal(t, 5, mapValue)

	mapInterfaceValue, err := fastStringIndexValue(map[interface{}]string{"name": "Mido"}, "name")
	require.NoError(t, err)
	require.Equal(t, "Mido", mapInterfaceValue)

	missingMapValue, err := fastStringIndexValue(map[string]int{}, "count")
	require.NoError(t, err)
	require.Nil(t, missingMapValue)

	_, err = fastStringIndexValue(map[int]string{}, "name")
	require.ErrorContains(t, err, "cannot use")

	_, err = fastStringIndexValue([]string{"a"}, "name")
	require.ErrorContains(t, err, "non int")

	_, err = fastStringIndexValue(struct{}{}, "name")
	require.ErrorContains(t, err, "could not index")

	_, err = fastStringIndexValue(&object.String{Value: "not-indexable"}, "name")
	require.ErrorContains(t, err, "could not index")
}

func Test_VM_Fast_Direct_Map_Index_Helpers(t *testing.T) {
	tests := []struct {
		name     string
		raw      interface{}
		kind     fastMapDirectKind
		expected string
	}{
		{"string", map[string]string{"key": "<value>"}, fastMapDirectStringString, "&lt;value&gt;"},
		{"int", map[string]int{"key": 12}, fastMapDirectStringInt, "12"},
		{"uint32", map[string]uint32{"key": 13}, fastMapDirectStringUint32, "13"},
		{"interface", map[string]interface{}{"key": template.HTML("<b>")}, fastMapDirectStringInterface, "<b>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &fastAccessChainStep{mapString: "key", mapDirect: tt.kind}
			value, found, handled := fastAccessDirectMapIndex(reflect.ValueOf(tt.raw), step)
			require.True(t, handled)
			require.True(t, found)
			require.True(t, value.IsValid())

			var out strings.Builder
			handled, err := writeFastDirectMapIndexOutput(&out, plush.NewContext(), reflect.ValueOf(tt.raw), step)
			require.NoError(t, err)
			require.True(t, handled)
			require.Equal(t, tt.expected, out.String())
		})
	}

	step := &fastAccessChainStep{mapString: "missing", mapDirect: fastMapDirectStringString}
	value, found, handled := fastAccessDirectMapIndex(reflect.ValueOf(map[string]string{}), step)
	require.True(t, handled)
	require.False(t, found)
	require.False(t, value.IsValid())

	value, found, handled = fastAccessDirectMapIndex(reflect.ValueOf(map[string]string{"key": "value"}), &fastAccessChainStep{mapString: "key", mapDirect: fastMapDirectStringInt})
	require.False(t, handled)
	require.False(t, found)
	require.False(t, value.IsValid())

	value, found, handled = fastAccessDirectMapIndex(reflect.ValueOf(map[string]int{}), &fastAccessChainStep{mapString: "missing", mapDirect: fastMapDirectStringInt})
	require.True(t, handled)
	require.False(t, found)
	require.False(t, value.IsValid())

	value, found, handled = fastAccessDirectMapIndex(reflect.ValueOf(map[string]string{"key": "value"}), &fastAccessChainStep{mapString: "key", mapDirect: fastMapDirectStringUint32})
	require.False(t, handled)
	require.False(t, found)
	require.False(t, value.IsValid())

	value, found, handled = fastAccessDirectMapIndex(reflect.ValueOf(map[string]uint32{}), &fastAccessChainStep{mapString: "missing", mapDirect: fastMapDirectStringUint32})
	require.True(t, handled)
	require.False(t, found)
	require.False(t, value.IsValid())

	value, found, handled = fastAccessDirectMapIndex(reflect.ValueOf(map[string]string{"key": "value"}), &fastAccessChainStep{mapString: "key", mapDirect: fastMapDirectStringInterface})
	require.False(t, handled)
	require.False(t, found)
	require.False(t, value.IsValid())

	value, found, handled = fastAccessDirectMapIndex(reflect.ValueOf(map[string]interface{}{}), &fastAccessChainStep{mapString: "missing", mapDirect: fastMapDirectStringInterface})
	require.True(t, handled)
	require.False(t, found)
	require.False(t, value.IsValid())

	value, found, handled = fastAccessDirectMapIndex(reflect.ValueOf(map[string]interface{}{"nil": nil}), &fastAccessChainStep{mapString: "nil", mapDirect: fastMapDirectStringInterface})
	require.True(t, handled)
	require.True(t, found)
	require.True(t, value.IsValid())
	require.Nil(t, value.Interface())

	key, keyOK := fastAccessMapKeyValue(nil, &compiler.FastPathStep{Kind: compiler.FastPathStepIndexString, Value: "x"})
	require.False(t, keyOK)
	require.False(t, key.IsValid())

	key, keyOK = fastAccessMapKeyValue(reflect.TypeOf(true), &compiler.FastPathStep{Kind: compiler.FastPathStepIndexInteger, Index: 1})
	require.False(t, keyOK)
	require.False(t, key.IsValid())

	key, keyOK = fastRuntimeMapKeyValue(nil, &fastAccessChainStep{index: 1})
	require.False(t, keyOK)
	require.False(t, key.IsValid())

	key, keyOK = fastRuntimeMapKeyValue(reflect.TypeOf(true), &fastAccessChainStep{index: 1})
	require.False(t, keyOK)
	require.False(t, key.IsValid())

	handled, err := writeFastDirectMapIndexOutput(&strings.Builder{}, nil, reflect.ValueOf(map[int]string{}), step)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = writeFastDirectMapIndexOutput(&strings.Builder{}, nil, reflect.ValueOf(map[string]string{"key": "value"}), &fastAccessChainStep{mapString: "key", mapDirect: fastMapDirectStringInt})
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = writeFastDirectMapIndexOutput(&strings.Builder{}, nil, reflect.ValueOf(map[string]string{"key": "value"}), &fastAccessChainStep{mapString: "key", mapDirect: fastMapDirectStringUint32})
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = writeFastDirectMapIndexOutput(&strings.Builder{}, nil, reflect.ValueOf(map[string]string{"key": "value"}), &fastAccessChainStep{mapString: "key", mapDirect: fastMapDirectStringInterface})
	require.NoError(t, err)
	require.False(t, handled)

	var missingOut strings.Builder
	handled, err = writeFastDirectMapIndexOutput(&missingOut, nil, reflect.ValueOf(map[string]int{}), &fastAccessChainStep{mapString: "missing", mapDirect: fastMapDirectStringInt})
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, missingOut.String())

	handled, err = writeFastDirectMapIndexOutput(&missingOut, nil, reflect.ValueOf(map[string]uint32{}), &fastAccessChainStep{mapString: "missing", mapDirect: fastMapDirectStringUint32})
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, missingOut.String())

	handled, err = writeFastDirectMapIndexOutput(&missingOut, nil, reflect.ValueOf(map[string]interface{}{}), &fastAccessChainStep{mapString: "missing", mapDirect: fastMapDirectStringInterface})
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, missingOut.String())
}

func Test_VM_Write_Fast_Reflect_And_Go_Values(t *testing.T) {
	ctx := plush.NewContext()
	machine := newRuntimeHelperTestVM(ctx)
	var out strings.Builder

	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf(template.HTML("<b>"))))
	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf("<str>")))
	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf(true)))
	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf(int32(3))))
	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf(uint32(4))))
	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf(float32(1.5))))
	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf(vmFastOutputStringer("value"))))
	require.Equal(t, "<b>&lt;str&gt;true341.5stringer:value", out.String())

	var nilInterface interface{}
	require.Nil(t, fastReflectInterface(reflect.ValueOf(&nilInterface).Elem()))
	require.NoError(t, writeFastReflectValue(&out, ctx, reflect.ValueOf(&nilInterface).Elem()))

	out.Reset()
	machine.writeGoValue(&out, vmHole{input: `"hole"`})
	machine.writeGoValue(&out, vmFastValueInterface{value: "<iface>"})
	machine.writeGoValue(&out, vmHTMLValue("<safe>"))
	machine.writeGoValue(&out, []interface{}{"<x>", template.HTML("<y>")})
	machine.writeGoValue(&out, [2]int{1, 2})
	require.Contains(t, out.String(), plush.PunchHoleMarkerName(0))
	require.Contains(t, out.String(), "&lt;iface&gt;")
	require.Contains(t, out.String(), "<safe>")
	require.Contains(t, out.String(), "&lt;x&gt;<y>12")
	require.Len(t, *machine.holes, 1)
}

func Test_VM_Can_Write_Fast_Go_Value_Branches(t *testing.T) {
	require.True(t, canWriteFastGoValue(nil))
	require.True(t, canWriteFastGoValue(template.HTML("<b>")))
	require.True(t, canWriteFastGoValue(vmFastValueInterface{value: "<iface>"}))
	require.True(t, canWriteFastGoValue([]interface{}{"x", 1, template.HTML("<b>")}))
	require.True(t, canWriteFastGoValue([]string{"a", "b"}))
	require.True(t, canWriteFastGoValue(&object.String{Value: "ok"}))
	require.False(t, canWriteFastGoValue(&object.Builtin{}))
	require.False(t, canWriteFastGoValue([]interface{}{&object.Builtin{}}))
}

func Test_VM_Write_Fast_Go_Value_Output_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"TIME_FORMAT": "2006"})
	when := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	var out strings.Builder

	require.True(t, writeFastGoValue(&out, ctx, nil))
	require.True(t, writeFastGoValue(&out, ctx, when))
	require.True(t, writeFastGoValue(&out, ctx, &when))

	var nilTime *time.Time
	require.True(t, writeFastGoValue(&out, ctx, nilTime))

	require.True(t, writeFastGoValue(&out, ctx, vmFastValueInterface{value: "<iface>"}))
	require.True(t, writeFastGoValue(&out, ctx, template.HTML("<html>")))
	require.True(t, writeFastGoValue(&out, ctx, vmHTMLValue("<safe>")))
	require.True(t, writeFastGoValue(&out, ctx, "<str>"))
	require.True(t, writeFastGoValue(&out, ctx, true))
	require.True(t, writeFastGoValue(&out, ctx, int(-1)))
	require.True(t, writeFastGoValue(&out, ctx, int8(-2)))
	require.True(t, writeFastGoValue(&out, ctx, int16(-3)))
	require.True(t, writeFastGoValue(&out, ctx, int32(-4)))
	require.True(t, writeFastGoValue(&out, ctx, int64(-5)))
	require.True(t, writeFastGoValue(&out, ctx, uint(1)))
	require.True(t, writeFastGoValue(&out, ctx, uint8(2)))
	require.True(t, writeFastGoValue(&out, ctx, uint16(3)))
	require.True(t, writeFastGoValue(&out, ctx, uint32(4)))
	require.True(t, writeFastGoValue(&out, ctx, uint64(5)))
	require.True(t, writeFastGoValue(&out, ctx, uintptr(6)))
	require.True(t, writeFastGoValue(&out, ctx, float32(1.25)))
	require.True(t, writeFastGoValue(&out, ctx, float64(2.5)))
	require.True(t, writeFastGoValue(&out, ctx, vmFastOutputStringer("value")))
	require.True(t, writeFastGoValue(&out, ctx, []string{"<a>", "b"}))
	require.True(t, writeFastGoValue(&out, ctx, [2]string{"x", "<y>"}))
	require.True(t, writeFastGoValue(&out, ctx, struct{ Name string }{Name: "ignored"}))

	rendered := out.String()
	require.Contains(t, rendered, "20242024")
	require.Contains(t, rendered, "&lt;iface&gt;<html><safe>&lt;str&gt;true")
	require.Contains(t, rendered, "-1-2-3-4-51234561.252.5")
	require.Contains(t, rendered, "stringer:value&lt;a&gt;bx&lt;y&gt;")

	out.Reset()
	require.False(t, writeFastGoValue(&out, ctx, []interface{}{"ok", &object.Builtin{}}))
	require.Equal(t, "ok", out.String())
}

func Test_VM_Write_Fast_Object_Output_Branches(t *testing.T) {
	ctx := plush.NewContext()
	var out strings.Builder
	obj := &object.Array{Elements: []object.Object{
		&object.String{Value: "<a>"},
		&object.Integer{Value: 2},
		&object.Float{Value: 1.25},
		&object.Boolean{Value: true},
		&object.Native{Value: template.HTML("<b>")},
		object.NullObject,
	}}

	require.True(t, writeFastObject(&out, ctx, nil))
	require.True(t, writeFastObject(&out, ctx, object.NullObject))
	require.True(t, writeFastObject(&out, ctx, obj))
	require.Equal(t, "&lt;a&gt;21.25true<b>", out.String())

	require.True(t, canWriteFastObject(nil))
	require.True(t, canWriteFastObject(object.NullObject))
	require.True(t, canWriteFastObject(obj))
	require.False(t, writeFastObject(&out, ctx, &object.Builtin{}))
	require.False(t, writeFastObject(&out, ctx, &object.Array{Elements: []object.Object{&object.Builtin{}}}))
	require.False(t, canWriteFastObject(&object.Builtin{}))
	require.False(t, canWriteFastObject(&object.Array{Elements: []object.Object{&object.Builtin{}}}))
}

func Test_VM_Write_Go_Value_Output_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"TIME_FORMAT": "2006-01-02"})
	machine := newRuntimeHelperTestVM(ctx)
	when := time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)
	var out strings.Builder

	machine.writeGoValue(&out, nil)
	machine.writeGoValue(&out, &object.Array{Elements: []object.Object{
		&object.String{Value: "<a>"},
		&object.Integer{Value: 7},
		&object.Float{Value: 1.5},
		&object.Boolean{Value: false},
		object.NullObject,
	}})
	machine.writeGoValue(&out, when)
	machine.writeGoValue(&out, &when)

	var nilTime *time.Time
	machine.writeGoValue(&out, nilTime)

	machine.writeGoValue(&out, vmFastValueInterface{value: "<iface>"})
	machine.writeGoValue(&out, template.HTML("<html>"))
	machine.writeGoValue(&out, vmHTMLValue("<safe>"))
	machine.writeGoValue(&out, "<str>")
	machine.writeGoValue(&out, true)
	machine.writeGoValue(&out, uint32(4))
	machine.writeGoValue(&out, int32(-5))
	machine.writeGoValue(&out, float32(1.25))
	machine.writeGoValue(&out, vmFastOutputStringer("value"))
	machine.writeGoValue(&out, []string{"<x>", "y"})
	machine.writeGoValue(&out, []interface{}{template.HTML("<z>"), "<w>"})
	machine.writeGoValue(&out, [2]uint8{1, 2})
	machine.writeGoValue(&out, struct{ Name string }{Name: "ignored"})

	rendered := out.String()
	require.Contains(t, rendered, "&lt;a&gt;71.5false")
	require.Contains(t, rendered, "2025-02-032025-02-03")
	require.Contains(t, rendered, "&lt;iface&gt;<html><safe>&lt;str&gt;true4-51.25")
	require.Contains(t, rendered, "stringer:value&lt;x&gt;y<z>&lt;w&gt;12")

	out.Reset()
	machine.writeObject(&out, &object.Native{Value: "<native>"})
	require.Equal(t, "&lt;native&gt;", out.String())

	out.Reset()
	machine.ctx = nil
	machine.writeGoValue(&out, when)
	require.Equal(t, when.Format(plush.DefaultTimeFormat), out.String())

	require.True(t, isTruthy(&object.Native{Value: 1}))
	require.True(t, isTruthyFastValue(1))
}

func Test_VM_Numeric_Operation_Branches(t *testing.T) {
	result, err := numericOperationObject(code.OpAdd, numericValue{kind: numericFloat, f: 1.5}, numericValue{kind: numericSigned, i: 2})
	require.NoError(t, err)
	require.Equal(t, &object.Float{Value: 3.5}, result)

	result, err = numericOperationObject(code.OpSub, numericValue{kind: numericSigned, i: 7}, numericValue{kind: numericSigned, i: 2})
	require.NoError(t, err)
	require.Equal(t, &object.Integer{Value: 5}, result)

	result, err = numericOperationObject(code.OpMul, numericValue{kind: numericSigned, i: 3}, numericValue{kind: numericSigned, i: 4})
	require.NoError(t, err)
	require.Equal(t, &object.Integer{Value: 12}, result)

	result, err = numericOperationObject(code.OpDiv, numericValue{kind: numericUnsigned, u: 9}, numericValue{kind: numericUnsigned, u: 3})
	require.NoError(t, err)
	require.Equal(t, &object.Integer{Value: 3}, result)

	_, err = numericOperationObject(code.OpDiv, numericValue{kind: numericFloat, f: 1}, numericValue{kind: numericFloat, f: 0})
	require.ErrorContains(t, err, "division by zero")

	_, err = numericOperationObject(code.OpDiv, numericValue{kind: numericSigned, i: 1}, numericValue{kind: numericSigned, i: 0})
	require.ErrorContains(t, err, "division by zero")

	_, err = numericOperationObject(code.OpAdd, numericValue{kind: numericUnsigned, u: math.MaxUint64}, numericValue{kind: numericUnsigned, u: 1})
	require.ErrorContains(t, err, "integer overflow")

	_, err = numericOperationObject(code.OpSub, numericValue{kind: numericUnsigned, u: uint64(math.MaxInt64) + 1}, numericValue{kind: numericUnsigned, u: uint64(math.MaxInt64) + 2})
	require.ErrorContains(t, err, "integer underflow")

	_, err = numericOperationObject(code.OpMul, numericValue{kind: numericUnsigned, u: math.MaxUint64}, numericValue{kind: numericUnsigned, u: 2})
	require.ErrorContains(t, err, "integer overflow")

	_, err = numericOperationObject(code.OpDiv, numericValue{kind: numericUnsigned, u: 1}, numericValue{kind: numericUnsigned, u: 0})
	require.ErrorContains(t, err, "division by zero")

	_, err = numericOperationObject(code.OpPop, numericValue{kind: numericSigned, i: -1}, numericValue{kind: numericUnsigned, u: uint64(math.MaxInt64) + 1})
	require.ErrorContains(t, err, "unsupported mixed signed/unsigned operation")

	require.Equal(t, ">", orderedOperatorString(code.OpGreaterThan))
	require.Equal(t, ">=", orderedOperatorString(code.OpGreaterEqual))
	require.Equal(t, "0", orderedOperatorString(code.OpConstant))
}

func Test_VM_Fast_Runtime_Map_Key_Value_Branches(t *testing.T) {
	step := &fastAccessChainStep{mapKey: reflect.ValueOf("name")}
	key, ok := fastRuntimeMapKeyValue(stringType, step)
	require.True(t, ok)
	require.Equal(t, "name", key.Interface())

	step = &fastAccessChainStep{index: 3}
	key, ok = fastRuntimeMapKeyValue(reflect.TypeOf(int64(0)), step)
	require.True(t, ok)
	require.Equal(t, int64(3), key.Interface())

	_, ok = fastRuntimeMapKeyValue(boolType, &fastAccessChainStep{index: 3})
	require.False(t, ok)

	_, ok = fastRuntimeMapKeyValue(nil, nil)
	require.False(t, ok)
}

func Test_VM_Fast_Path_Step_Call_And_Default(t *testing.T) {
	ctx := plush.NewContext()
	value, err := evalFastPathStep(func() string { return "<called>" }, &compiler.FastPathStep{Kind: compiler.FastPathStepCall, Value: "call", Line: 1}, ctx)
	require.NoError(t, err)
	require.Equal(t, "<called>", value)

	_, err = evalFastPathStep(vmSegmentUser{Name: "Mido"}, &compiler.FastPathStep{Kind: compiler.FastPathStepProperty, Value: "Name", Line: 2}, plush.NewContext().WithBudget(plush.NewBudget(0)))
	require.ErrorContains(t, err, "line 2")

	value, err = evalFastPathStep(map[string]string{"name": "Mido"}, &compiler.FastPathStep{Kind: compiler.FastPathStepIndexString, Value: "name", Line: 3}, ctx)
	require.NoError(t, err)
	require.Equal(t, "Mido", value)

	_, err = evalFastPathStep(func() (string, error) { return "", errors.New("boom") }, &compiler.FastPathStep{Kind: compiler.FastPathStepCall, Value: "call", Line: 5}, ctx)
	require.ErrorContains(t, err, "line 5")
	require.ErrorContains(t, err, "boom")

	_, err = evalFastPathStep(func() string { return "blocked" }, &compiler.FastPathStep{Kind: compiler.FastPathStepCall, Value: "call", Line: 4}, plush.NewContext().WithBudget(plush.NewBudget(0)))
	require.ErrorContains(t, err, "line 4")

	value, err = evalFastPathStep("base", &compiler.FastPathStep{Kind: 255}, ctx)
	require.NoError(t, err)
	require.Nil(t, value)
}
