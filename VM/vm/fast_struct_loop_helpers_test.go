package vm

import (
	"errors"
	"html/template"
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/stretchr/testify/require"
)

type vmStructLoopProduct struct {
	Name string
}

func (p vmStructLoopProduct) Echo() string {
	return p.Name + "!"
}

func (p *vmStructLoopProduct) PointerEcho() string {
	return p.Name + "?"
}

type vmStructLoopStringer struct {
	value string
}

func (s vmStructLoopStringer) String() string {
	return "stringer:" + s.value
}

type vmStructLoopStringerProduct struct {
	Title vmStructLoopStringer
}

func structLoopCallPlan(t *testing.T, args ...compiler.FastValuePlan) *fastStructLoopCallPlan {
	t.Helper()
	plan, ok := buildFastStructLoopCallPlan(&compiler.FastCallPlan{
		Name:      "label",
		NameIndex: 0,
		Args:      args,
		Line:      1,
	}, reflect.TypeOf(vmStructLoopProduct{}))
	require.True(t, ok)
	require.NotNil(t, plan)
	return plan
}

func structLoopNameArg() compiler.FastValuePlan {
	return compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Name", Receiver: "product", Full: "product.Name", Line: 1},
		},
		Line: 1,
	}
}

func Test_VM_Fast_Struct_Loop_Direct_Call_Writers(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"prefix": "<pre>"})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label", "prefix"}}, ctx)
	item := reflect.ValueOf(vmStructLoopProduct{Name: "<bot>"})

	oneArgPlan := structLoopCallPlan(t, structLoopNameArg())
	twoArgPlan := structLoopCallPlan(t, structLoopNameArg(), compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 1, Value: "prefix", Line: 1})

	tests := []struct {
		name     string
		raw      interface{}
		plan     *fastStructLoopCallPlan
		expected string
	}{
		{"string", func(value string) string { return value + "!" }, oneArgPlan, "&lt;bot&gt;!"},
		{"string_string", func(value, prefix string) string { return prefix + value }, twoArgPlan, "&lt;pre&gt;&lt;bot&gt;"},
		{"string_error", func(value string) (string, error) { return value + "?", nil }, oneArgPlan, "&lt;bot&gt;?"},
		{"string_string_error", func(value, prefix string) (string, error) { return prefix + value, nil }, twoArgPlan, "&lt;pre&gt;&lt;bot&gt;"},
		{"html", func(value string) template.HTML { return template.HTML("<b>" + value + "</b>") }, oneArgPlan, "<b><bot></b>"},
		{"html_string", func(value, prefix string) template.HTML { return template.HTML(prefix + value) }, twoArgPlan, "<pre><bot>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := fastStructLoopDirectCallWriterForRaw(tt.raw, tt.plan)
			require.NotNil(t, writer)
			var out strings.Builder
			handled, err := writer(&out, ctx, bindings, tt.plan, 0, item)
			require.NoError(t, err)
			require.True(t, handled)
			require.Equal(t, tt.expected, out.String())
		})
	}

	raw := func(string) (string, error) { return "", errors.New("boom") }
	writer := fastStructLoopDirectCallWriterForRaw(raw, oneArgPlan)
	require.NotNil(t, writer)
	handled, err := writer(&strings.Builder{}, ctx, bindings, oneArgPlan, 0, item)
	require.True(t, handled)
	require.ErrorContains(t, err, "could not call label function")

	missingArgPlan := structLoopCallPlan(t, compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"})
	writer = fastStructLoopDirectCallWriterForRaw(func(value string) string { return value }, missingArgPlan)
	require.NotNil(t, writer)
	handled, err = writer(&strings.Builder{}, ctx, bindings, missingArgPlan, 0, item)
	require.NoError(t, err)
	require.False(t, handled)

	secondMissingArgPlan := structLoopCallPlan(t, structLoopNameArg(), compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"})
	for _, tt := range []struct {
		name string
		raw  interface{}
		plan *fastStructLoopCallPlan
	}{
		{"string_string_second_missing", func(value, prefix string) string { return prefix + value }, secondMissingArgPlan},
		{"string_error_missing", func(value string) (string, error) { return value, nil }, missingArgPlan},
		{"string_string_error_second_missing", func(value, prefix string) (string, error) { return prefix + value, nil }, secondMissingArgPlan},
		{"html_missing", func(value string) template.HTML { return template.HTML(value) }, missingArgPlan},
		{"html_string_second_missing", func(value, prefix string) template.HTML { return template.HTML(prefix + value) }, secondMissingArgPlan},
	} {
		t.Run(tt.name, func(t *testing.T) {
			writer := fastStructLoopDirectCallWriterForRaw(tt.raw, tt.plan)
			require.NotNil(t, writer)
			handled, err := writer(&strings.Builder{}, ctx, bindings, tt.plan, 0, item)
			require.NoError(t, err)
			require.False(t, handled)
		})
	}

	require.Nil(t, fastStructLoopDirectCallWriterForRaw(func(string) string { return "" }, twoArgPlan))
	require.Nil(t, fastStructLoopDirectCallWriterForRaw(func(string, string) string { return "" }, oneArgPlan))
	require.Nil(t, fastStructLoopDirectCallWriterForRaw(func(string) (string, error) { return "", nil }, twoArgPlan))
	require.Nil(t, fastStructLoopDirectCallWriterForRaw(func(string, string) (string, error) { return "", nil }, oneArgPlan))
	require.Nil(t, fastStructLoopDirectCallWriterForRaw(func(string) template.HTML { return "" }, twoArgPlan))
	require.Nil(t, fastStructLoopDirectCallWriterForRaw(func(string, string) template.HTML { return "" }, oneArgPlan))
	require.Nil(t, fastStructLoopDirectCallWriterForRaw("not helper", oneArgPlan))
	require.Nil(t, fastStructLoopDirectCallWriterForRaw(func(string) string { return "" }, nil))
}

func Test_VM_Fast_Struct_Loop_Call_Arg_Values(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"prefix": "pre"})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"prefix"}}, ctx)
	item := reflect.ValueOf(vmStructLoopProduct{Name: "bot"})
	accessPlan := structLoopCallPlan(t, structLoopNameArg()).args[0].accessPlan

	callPlan, ok := buildFastStructLoopCallPlan(nil, reflect.TypeOf(vmStructLoopProduct{}))
	require.False(t, ok)
	require.Nil(t, callPlan)

	nilArgPlan := buildFastStructLoopCallArgPlan(nil, reflect.TypeOf(vmStructLoopProduct{}))
	require.Equal(t, fastStructLoopCallArgNil, nilArgPlan.kind)

	nameNilPlan := buildFastStructLoopCallArgPlan(&compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "nil"}, reflect.TypeOf(vmStructLoopProduct{}))
	require.Equal(t, fastStructLoopCallArgNil, nameNilPlan.kind)

	genericPathPlan := buildFastStructLoopCallArgPlan(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Missing"},
		},
	}, reflect.TypeOf(vmStructLoopProduct{}))
	require.Equal(t, fastStructLoopCallArgGeneric, genericPathPlan.kind)

	tests := []struct {
		name     string
		plan     *fastStructLoopCallArgPlan
		expected interface{}
		ok       bool
	}{
		{"nil plan", nil, nil, true},
		{"key", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgKey}, 7, true},
		{"binding", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBinding, nameIndex: 0}, "pre", true},
		{"nil", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgNil}, nil, true},
		{"string", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgString, stringVal: "x"}, "x", true},
		{"int", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgInt, intVal: 3}, 3, true},
		{"float", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgFloat, floatVal: 1.5}, 1.5, true},
		{"bool", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBool, boolVal: true}, true, true},
		{"access", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: accessPlan}, "bot", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok, err := evalFastStructLoopCallArgValue(tt.plan, ctx, bindings, 7, item)
			require.NoError(t, err)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.expected, value)
		})
	}

	value, ok, err := evalFastStructLoopCallArgValue(&fastStructLoopCallArgPlan{
		kind:  fastStructLoopCallArgGeneric,
		value: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "generic"},
	}, ctx, bindings, 7, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "generic", value)

	_, ok, err = evalFastStructLoopCallArgValue(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBinding, nameIndex: 99}, ctx, bindings, 7, item)
	require.NoError(t, err)
	require.False(t, ok)

	value, ok, err = evalFastStructLoopCallArgValue(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: accessPlan}, ctx, bindings, 7, reflect.Value{})
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, value)

	_, ok, err = evalFastStructLoopCallArgValue(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain}, ctx, bindings, 7, item)
	require.NoError(t, err)
	require.False(t, ok)

	args := &fastCallArgs{}
	require.NoError(t, evalFastStructLoopCallPlanArgs(nil, ctx, bindings, 7, item, args))
	require.NoError(t, evalFastStructLoopCallPlanArgs(&fastStructLoopCallPlan{}, ctx, bindings, 7, item, args))
	err = evalFastStructLoopCallPlanArgs(&fastStructLoopCallPlan{
		args: []fastStructLoopCallArgPlan{{
			kind:      fastStructLoopCallArgBinding,
			nameIndex: 99,
			line:      8,
			value:     compiler.FastValuePlan{Value: "missing"},
		}},
	}, ctx, bindings, 7, item, args)
	require.ErrorContains(t, err, `line 8: "missing": unknown identifier`)
}

func Test_VM_Fast_Struct_Loop_Value_And_Condition_Edges(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"flag": true, "prefix": "pre"})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"flag", "prefix"}}, ctx)
	item := reflect.ValueOf(vmStructLoopProduct{Name: "bot"})
	namePlan := structLoopNameArg()
	nameArgPlan := structLoopCallPlan(t, namePlan).args[0]

	value, ok, err := evalFastStructLoopValue(nil, ctx, bindings, "key", item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, value)

	value, ok, err = evalFastStructLoopValue(&compiler.FastValuePlan{Kind: compiler.FastValueLoopKey}, ctx, bindings, "key", item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "key", value)

	value, ok, err = evalFastStructLoopValue(&compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: 1, Value: "prefix"}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "pre", value)

	value, ok, err = evalFastStructLoopValue(&compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: -1}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, vmStructLoopProduct{Name: "bot"}, value)

	value, ok, err = evalFastStructLoopPathValue(&namePlan, ctx, bindings, reflect.Value{})
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, value)

	truthy, ok, err := isTruthyFastStructLoopValue(&namePlan, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, truthy)

	truthy, ok, err = isTruthyFastStructLoopValue(&compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: -1}, ctx, bindings, nil, reflect.Value{})
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, truthy)

	truthy, ok, err = isTruthyFastStructLoopValue(nil, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, truthy)

	truthy, ok, err = isTruthyFastStructLoopValue(&compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: -1}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, truthy)

	infix := &compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     &namePlan,
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "bot"},
		Line:     11,
	}
	value, ok, err = evalFastStructLoopInfixValue(infix, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, value)

	value, ok, err = evalFastStructLoopInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "&&",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: false},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, false, value)

	value, ok, err = evalFastStructLoopInfixValue(nil, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, value)

	value, ok, err = evalFastStructLoopInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missingLeft"},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 100, Value: "missingRight"},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, value)

	_, ok, err = evalFastStructLoopInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     &namePlan,
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "bot"},
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, item)
	require.ErrorContains(t, err, "line 1")
	require.True(t, ok)

	_, ok, err = evalFastStructLoopInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "bot"},
		Right:    &namePlan,
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, item)
	require.ErrorContains(t, err, "line 1")
	require.True(t, ok)

	_, ok, err = evalFastStructLoopInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "??",
		Left:     &namePlan,
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "bot"},
		Line:     12,
	}, ctx, bindings, nil, item)
	require.True(t, ok)
	require.ErrorContains(t, err, "line 12")

	value, ok, err = evalFastStructLoopLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "??",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, value)

	value, ok, err = evalFastStructLoopLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "&&",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: false},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, false, value)

	value, ok, err = evalFastStructLoopLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "||",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: false},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, value)

	value, ok, err = evalFastStructLoopLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "&&",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: false},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, false, value)

	value, ok, err = evalFastStructLoopLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "||",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, value)

	_, ok, err = evalFastStructLoopLogicalInfixValue(&compiler.FastValuePlan{
		Operator: "&&",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
		Right:    &namePlan,
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, item)
	require.ErrorContains(t, err, "line 1")
	require.True(t, ok)

	truthy, ok, err = isTruthyFastStructLoopCondition(nil, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, truthy)

	truthy, ok, err = isTruthyFastStructLoopCondition(&fastStructLoopConditionalWriterBranch{
		condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, truthy)

	truthy, ok, err = isTruthyFastStructLoopCondition(&fastStructLoopConditionalWriterBranch{
		conditionPlan: &fastStructLoopConditionPlan{
			kind:  fastStructLoopConditionTruthy,
			value: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBool, boolVal: false},
		},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, truthy)

	truthy, ok, err = evalFastStructLoopConditionPlan(nil, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, truthy)

	truthy, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:  fastStructLoopConditionTruthy,
		value: nameArgPlan,
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, truthy)

	truthy, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:     fastStructLoopConditionLogical,
		operator: "&&",
		left: &fastStructLoopConditionPlan{
			kind:  fastStructLoopConditionTruthy,
			value: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBool, boolVal: false},
		},
		right: &fastStructLoopConditionPlan{
			kind:  fastStructLoopConditionTruthy,
			value: nameArgPlan,
		},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, truthy)

	truthy, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:     fastStructLoopConditionLogical,
		operator: "||",
		left: &fastStructLoopConditionPlan{
			kind:  fastStructLoopConditionTruthy,
			value: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBool, boolVal: true},
		},
		right: &fastStructLoopConditionPlan{
			kind:  fastStructLoopConditionTruthy,
			value: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBinding, nameIndex: 99},
		},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, truthy)

	truthy, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:     fastStructLoopConditionLogical,
		operator: "??",
		left: &fastStructLoopConditionPlan{
			kind:  fastStructLoopConditionTruthy,
			value: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBool, boolVal: true},
		},
		right: &fastStructLoopConditionPlan{
			kind:  fastStructLoopConditionTruthy,
			value: nameArgPlan,
		},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, truthy)

	truthy, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:       fastStructLoopConditionInfix,
		operator:   "==",
		leftValue:  nameArgPlan,
		rightValue: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgString, stringVal: "bot"},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, truthy)

	truthy, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:       fastStructLoopConditionInfix,
		operator:   "==",
		leftValue:  fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBinding, nameIndex: 99},
		rightValue: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBinding, nameIndex: 100},
	}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, truthy)

	_, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:       fastStructLoopConditionInfix,
		operator:   "??",
		leftValue:  nameArgPlan,
		rightValue: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgString, stringVal: "bot"},
		line:       13,
	}, ctx, bindings, nil, item)
	require.True(t, ok)
	require.ErrorContains(t, err, "line 13")

	truthy, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{kind: fastStructLoopConditionKind(99)}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, truthy)

	operand, ok, err := evalFastStructLoopConditionOperand(nil, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, operand.isNil())

	operand, ok, err = evalFastStructLoopConditionOperand(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.False(t, ok)
	require.True(t, operand.isNil())

	operand, ok, err = evalFastStructLoopConditionOperand(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: nameArgPlan.accessPlan}, ctx, bindings, nil, reflect.Value{})
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, operand.isNil())

	_, ok, err = evalFastStructLoopConditionOperand(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: nameArgPlan.accessPlan}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, item)
	require.ErrorContains(t, err, "line 1")
	require.True(t, ok)

	_, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:  fastStructLoopConditionTruthy,
		value: nameArgPlan,
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, item)
	require.ErrorContains(t, err, "line 1")
	require.True(t, ok)

	_, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:     fastStructLoopConditionLogical,
		operator: "&&",
		left:     &fastStructLoopConditionPlan{kind: fastStructLoopConditionTruthy, value: nameArgPlan},
		right:    &fastStructLoopConditionPlan{kind: fastStructLoopConditionTruthy, value: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBool, boolVal: true}},
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, item)
	require.ErrorContains(t, err, "line 1")
	require.True(t, ok)

	_, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:     fastStructLoopConditionLogical,
		operator: "&&",
		left:     &fastStructLoopConditionPlan{kind: fastStructLoopConditionTruthy, value: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBool, boolVal: true}},
		right:    &fastStructLoopConditionPlan{kind: fastStructLoopConditionTruthy, value: nameArgPlan},
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, item)
	require.ErrorContains(t, err, "line 1")
	require.True(t, ok)

	_, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:       fastStructLoopConditionInfix,
		operator:   "==",
		leftValue:  nameArgPlan,
		rightValue: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgString, stringVal: "bot"},
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, item)
	require.ErrorContains(t, err, "line 1")
	require.True(t, ok)

	_, ok, err = evalFastStructLoopConditionPlan(&fastStructLoopConditionPlan{
		kind:       fastStructLoopConditionInfix,
		operator:   "==",
		leftValue:  fastStructLoopCallArgPlan{kind: fastStructLoopCallArgString, stringVal: "bot"},
		rightValue: nameArgPlan,
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, item)
	require.ErrorContains(t, err, "line 1")
	require.True(t, ok)

	arg, ok, err := evalFastStructLoopCallArgString(nil, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, arg)

	arg, ok, err = evalFastStructLoopCallArgString(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain}, ctx, bindings, nil, reflect.Value{})
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, arg)

	arg, ok, err = evalFastStructLoopCallArgString(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, arg)

	arg, ok, err = evalFastStructLoopCallArgString(&nameArgPlan, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "bot", arg)

	_, ok, err = evalFastStructLoopCallArgString(&nameArgPlan, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, item)
	require.ErrorContains(t, err, "line 1")
	require.True(t, ok)

	childValuePlan := compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Child", Receiver: "product", Full: "product.Child", Line: 1},
		},
		Line: 1,
	}
	childArgPlan := buildFastStructLoopCallArgPlan(&childValuePlan, reflect.TypeOf(vmFastPropertyUser{}))
	arg, ok, err = evalFastStructLoopCallArgString(&childArgPlan, ctx, bindings, nil, reflect.ValueOf(vmFastPropertyUser{}))
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, arg)

	stringerValuePlan := compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Title", Receiver: "product", Full: "product.Title", Line: 1},
		},
		Line: 1,
	}
	stringerArgPlan := buildFastStructLoopCallArgPlan(&stringerValuePlan, reflect.TypeOf(vmStructLoopStringerProduct{}))
	arg, ok, err = evalFastStructLoopCallArgString(&stringerArgPlan, ctx, bindings, nil, reflect.ValueOf(vmStructLoopStringerProduct{Title: vmStructLoopStringer{value: "title"}}))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "stringer:title", arg)

	arg, ok, err = evalFastStructLoopCallArgString(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgInt, intVal: 7}, ctx, bindings, nil, item)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, arg)

	var stringIface interface{} = "iface"
	reflectedString, ok := (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(stringIface)}).stringValue()
	require.True(t, ok)
	require.Equal(t, "iface", reflectedString)

	var nilBoolIface interface{}
	_, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(&nilBoolIface).Elem()}).boolValue()
	require.False(t, ok)

	var boolIface interface{} = true
	reflectedBool, ok := (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(boolIface)}).boolValue()
	require.True(t, ok)
	require.True(t, reflectedBool)

	var nilStringReflectIface interface{}
	_, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(&nilStringReflectIface).Elem()}).stringValue()
	require.False(t, ok)

	var nilStringIface interface{} = (*string)(nil)
	_, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(nilStringIface)}).stringValue()
	require.False(t, ok)
}

func Test_VM_Fast_Struct_Loop_Writer_Op_Edges(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"label": func(value string) string { return value + "!" },
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
	item := reflect.ValueOf(vmStructLoopProduct{Name: "<bot>"})
	nameLookup := cachedPropertyLookup(reflect.TypeOf(vmStructLoopProduct{}), "Name")
	callPlan := structLoopCallPlan(t, structLoopNameArg())
	nameAccessPlan := callPlan.args[0].accessPlan

	ops := []fastStructLoopWriterOp{
		{kind: fastStructLoopWriterStatic, value: "key="},
		{kind: fastStructLoopWriterKey},
		{kind: fastStructLoopWriterStatic, value: " name="},
		{
			kind:       fastStructLoopWriterField,
			name:       "Name",
			receiver:   "product",
			full:       "product.Name",
			line:       2,
			fieldIndex: nameLookup.fieldIndex,
			fieldType:  stringType,
		},
		{kind: fastStructLoopWriterStatic, value: " chain="},
		{kind: fastStructLoopWriterAccessChain, accessPlan: nameAccessPlan},
		{kind: fastStructLoopWriterStatic, value: " call="},
		{kind: fastStructLoopWriterCall, call: callPlan},
		{kind: fastStructLoopWriterStatic, value: " method="},
		{
			kind: fastStructLoopWriterMethodCall,
			methodPlan: &fastLoopMethodCallPlan{
				method: compiler.FastPathStep{Value: "Echo", Receiver: "product", Full: "product.Echo", Line: 3},
				call:   compiler.FastPathStep{Value: "Echo", Line: 3},
				lookup: cachedPropertyLookup(reflect.TypeOf(vmStructLoopProduct{}), "Echo"),
			},
		},
	}

	var out strings.Builder
	err := renderFastStructLoopWriterOps(&out, ctx, bindings, &fastStructLoopRenderState{}, nil, ops, 4, item)
	require.NoError(t, err)
	require.Equal(t, "key=4 name=&lt;bot&gt; chain=&lt;bot&gt; call=&lt;bot&gt;! method=&lt;bot&gt;!", out.String())

	out.Reset()
	err = renderFastStructLoopWriterOps(&out, ctx, bindings, &fastStructLoopRenderState{}, nil, []fastStructLoopWriterOp{ops[3]}, 0, reflect.Value{})
	require.NoError(t, err)
	require.Empty(t, out.String())

	err = renderFastStructLoopWriterOps(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, &fastStructLoopRenderState{}, nil, []fastStructLoopWriterOp{ops[3]}, 0, item)
	require.ErrorContains(t, err, "line 2")

	profileLookup := cachedPropertyLookup(reflect.TypeOf(vmAccessWrapper{}), "Profile")
	out.Reset()
	err = renderFastStructLoopWriterOps(&out, ctx, bindings, &fastStructLoopRenderState{}, nil, []fastStructLoopWriterOp{{
		kind:       fastStructLoopWriterField,
		name:       "Profile",
		receiver:   "wrapper",
		full:       "wrapper.Profile",
		line:       6,
		fieldIndex: profileLookup.fieldIndex,
		fieldType:  reflect.TypeOf(vmAccessProfile{}),
	}}, 0, reflect.ValueOf(vmAccessWrapper{Profile: vmAccessProfile{Name: "profile"}}))
	require.NoError(t, err)
	require.Empty(t, out.String())

	hiddenLookup := cachedPropertyLookup(reflect.TypeOf(vmFastPropertyUser{}), "hidden")
	err = renderFastStructLoopWriterOps(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, nil, []fastStructLoopWriterOp{{
		kind:       fastStructLoopWriterField,
		name:       "hidden",
		receiver:   "user",
		full:       "user.hidden",
		line:       7,
		fieldIndex: hiddenLookup.fieldIndex,
		fieldType:  stringType,
	}}, 0, reflect.ValueOf(vmFastPropertyUser{hidden: "secret"}))
	require.ErrorContains(t, err, "line 7")

	err = renderFastStructLoopWriterOps(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, nil, []fastStructLoopWriterOp{{
		kind: fastStructLoopWriterAccessChain,
		accessPlan: &fastAccessChainPlan{steps: []fastAccessChainStep{{
			kind:      fastAccessStepField,
			name:      "hidden",
			receiver:  "user",
			full:      "user.hidden",
			line:      7,
			fieldType: stringType,
			lookup:    hiddenLookup,
		}}},
	}}, 0, reflect.ValueOf(vmFastPropertyUser{hidden: "secret"}))
	require.ErrorContains(t, err, "line 7")

	out.Reset()
	err = renderFastStructLoopWriterOps(&out, ctx, bindings, &fastStructLoopRenderState{}, nil, []fastStructLoopWriterOp{{
		kind:       fastStructLoopWriterAccessChain,
		accessPlan: &fastAccessChainPlan{steps: []fastAccessChainStep{{kind: fastAccessStepKind(99)}}},
	}}, 0, item)
	require.NoError(t, err)
	require.Empty(t, out.String())

	err = renderFastStructLoopWriterOps(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, nil, []fastStructLoopWriterOp{{
		kind: fastStructLoopWriterMethodCall,
		methodPlan: &fastLoopMethodCallPlan{
			method: compiler.FastPathStep{Value: "Missing", Receiver: "product", Full: "product.Missing", Line: 8},
			call:   compiler.FastPathStep{Value: "Missing", Line: 8},
			lookup: propertyLookup{kind: propertyLookupMissing},
		},
	}}, 0, item)
	require.ErrorContains(t, err, "line 8")

	err = renderFastStructLoopWriterOps(&strings.Builder{}, plush.NewContext(), fastRenderBindings{}, &fastStructLoopRenderState{}, nil, []fastStructLoopWriterOp{{
		kind: fastStructLoopWriterCall,
		call: structLoopCallPlan(t, structLoopNameArg()),
	}}, 0, item)
	require.ErrorContains(t, err, `"label": unknown identifier`)

	out.Reset()
	conditional := &fastStructLoopConditionalWriterPlan{
		branches: []fastStructLoopConditionalWriterBranch{{
			conditionPlan: &fastStructLoopConditionPlan{
				kind:  fastStructLoopConditionTruthy,
				value: fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBool, boolVal: false},
			},
			ops:  []fastStructLoopWriterOp{{kind: fastStructLoopWriterStatic, value: "branch"}},
			line: 8,
		}},
		elseOps: []fastStructLoopWriterOp{{kind: fastStructLoopWriterStatic, value: "else"}},
	}
	err = renderFastStructLoopWriterOps(&out, ctx, bindings, &fastStructLoopRenderState{}, &compiler.FastLoopPlan{Line: 8}, []fastStructLoopWriterOp{{
		kind:        fastStructLoopWriterConditional,
		conditional: conditional,
	}}, 0, item)
	require.NoError(t, err)
	require.Equal(t, "else", out.String())

	require.NoError(t, renderFastStructLoopConditional(&out, ctx, bindings, &fastStructLoopRenderState{}, nil, nil, 0, item))
	require.NoError(t, renderFastStructLoopConditional(&out, ctx, bindings, &fastStructLoopRenderState{}, nil, &fastStructLoopConditionalWriterPlan{}, 0, item))

	err = renderFastStructLoopConditional(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(1)), bindings, &fastStructLoopRenderState{}, nil, &fastStructLoopConditionalWriterPlan{
		branches: []fastStructLoopConditionalWriterBranch{{
			condition: structLoopNameArg(),
			line:      9,
		}},
	}, 0, item)
	require.ErrorContains(t, err, "line 1")

	err = renderFastStructLoopConditional(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, &fastStructLoopRenderState{}, nil, conditional, 0, item)
	require.ErrorContains(t, err, "line 8")

	err = renderFastStructLoopWriterOps(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, &fastStructLoopRenderState{}, nil, []fastStructLoopWriterOp{{
		kind:        fastStructLoopWriterConditional,
		conditional: conditional,
	}}, 0, item)
	require.ErrorContains(t, err, "line 8")
}

func Test_VM_Fast_Struct_Loop_Method_Call_And_Reflect_Edges(t *testing.T) {
	ctx := plush.NewContext()
	item := reflect.ValueOf(vmStructLoopProduct{Name: "<bot>"})

	require.NoError(t, writeFastLoopMethodCall(&strings.Builder{}, ctx, nil, item))
	require.NoError(t, writeFastLoopMethodCall(&strings.Builder{}, ctx, &fastLoopMethodCallPlan{
		method: compiler.FastPathStep{Value: "Echo", Receiver: "product", Full: "product.Echo", Line: 3},
		call:   compiler.FastPathStep{Value: "Echo", Line: 3},
		lookup: cachedPropertyLookup(reflect.TypeOf(vmStructLoopProduct{}), "Echo"),
	}, reflect.Value{}))

	var out strings.Builder
	valueMethod := &fastLoopMethodCallPlan{
		method: compiler.FastPathStep{Value: "Echo", Receiver: "product", Full: "product.Echo", Line: 4},
		call:   compiler.FastPathStep{Value: "Echo", Line: 4},
		lookup: cachedPropertyLookup(reflect.TypeOf(vmStructLoopProduct{}), "Echo"),
	}
	require.NoError(t, writeFastLoopMethodCall(&out, ctx, valueMethod, item))
	require.Equal(t, "&lt;bot&gt;!", out.String())

	out.Reset()
	pointerMethod := &fastLoopMethodCallPlan{
		method: compiler.FastPathStep{Value: "PointerEcho", Receiver: "product", Full: "product.PointerEcho", Line: 5},
		call:   compiler.FastPathStep{Value: "PointerEcho", Line: 5},
		lookup: cachedPropertyLookup(reflect.TypeOf(vmStructLoopProduct{}), "PointerEcho"),
	}
	require.NoError(t, writeFastLoopMethodCall(&out, ctx, pointerMethod, item))
	require.Equal(t, "&lt;bot&gt;?", out.String())

	addressable := reflect.ValueOf(&vmStructLoopProduct{Name: "addr"}).Elem()
	method, ok := fastBoundMethodValue(addressable, cachedPropertyLookup(reflect.TypeOf(vmStructLoopProduct{}), "PointerEcho"))
	require.True(t, ok)
	results := method.Call(nil)
	require.Len(t, results, 1)
	require.Equal(t, "addr?", results[0].Interface())

	_, ok = fastBoundMethodValue(reflect.Value{}, cachedPropertyLookup(reflect.TypeOf(vmStructLoopProduct{}), "Echo"))
	require.False(t, ok)
	_, ok = fastBoundMethodValue(item, propertyLookup{kind: propertyLookupMissing})
	require.False(t, ok)

	missing := &fastLoopMethodCallPlan{
		method: compiler.FastPathStep{Value: "Missing", Receiver: "product", Full: "product.Missing", Line: 6},
		call:   compiler.FastPathStep{Value: "Missing", Line: 6},
		lookup: propertyLookup{kind: propertyLookupMissing},
	}
	err := writeFastLoopMethodCall(&strings.Builder{}, ctx, missing, item)
	require.ErrorContains(t, err, "line 6")

	err = writeFastLoopMethodCall(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), valueMethod, item)
	require.ErrorContains(t, err, "line 4")

	err = writeFastLoopMethodCall(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(1)), valueMethod, item)
	require.ErrorContains(t, err, "line 4")

	out.Reset()
	require.NoError(t, writeFastReflectCallResults(&out, ctx, "struct", []reflect.Value{reflect.ValueOf(vmSegmentUser{Name: "ignored"})}))
	require.Empty(t, out.String())

	receiver, ok, err := fastLoopMethodReceiver(nil, reflect.Value{}, ctx)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, receiver.IsValid())

	defaultChain := &fastAccessChainPlan{steps: []fastAccessChainStep{{kind: fastAccessStepKind(99)}}}
	receiver, ok, err = fastLoopMethodReceiver(defaultChain, item, ctx)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, receiver.IsValid())

	fieldChain, ok := fastAccessChainPlanFor(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Child", Receiver: "user", Full: "user.Child", Line: 9},
		},
	}, reflect.TypeOf(vmFastPropertyUser{}))
	require.True(t, ok)
	value, ok, err := evalFastAccessChainReflectValue(fieldChain, reflect.ValueOf(vmFastPropertyUser{}), ctx)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, value.IsValid())

	_, ok, err = evalFastAccessChainReflectValue(fieldChain, reflect.ValueOf(vmFastPropertyUser{Child: &vmFastPropertyChild{Name: "kid"}}), plush.NewContext().WithBudget(plush.NewBudget(0)))
	require.ErrorContains(t, err, "line 9")
	require.True(t, ok)
}

func Test_VM_Fast_Struct_Loop_Builder_Edge_Branches(t *testing.T) {
	elemType := reflect.TypeOf(vmStructLoopProduct{})

	writerPlan, ok := buildFastStructLoopWriterPlan(nil, elemType)
	require.False(t, ok)
	require.Nil(t, writerPlan)

	ops, ok := buildFastStructLoopWriterOps([]compiler.FastLoopPart{{Kind: compiler.FastLoopPartKind(99)}}, elemType)
	require.False(t, ok)
	require.Nil(t, ops)

	ops, ok = buildFastStructLoopWriterOps([]compiler.FastLoopPart{{Kind: compiler.FastLoopPartValueProperty}}, elemType)
	require.False(t, ok)
	require.Nil(t, ops)

	ops, ok = buildFastStructLoopWriterOps([]compiler.FastLoopPart{{Kind: compiler.FastLoopPartValueProperty, Value: "Missing"}}, elemType)
	require.False(t, ok)
	require.Nil(t, ops)

	ops, ok = buildFastStructLoopWriterOps([]compiler.FastLoopPart{{
		Kind:      compiler.FastLoopPartValuePath,
		ValuePlan: compiler.FastValuePlan{Kind: compiler.FastValuePath},
	}}, elemType)
	require.False(t, ok)
	require.Nil(t, ops)

	ops, ok = buildFastStructLoopWriterOps([]compiler.FastLoopPart{{Kind: compiler.FastLoopPartCall}}, elemType)
	require.False(t, ok)
	require.Nil(t, ops)

	ops, ok = buildFastStructLoopWriterOps([]compiler.FastLoopPart{{Kind: compiler.FastLoopPartConditional}}, elemType)
	require.False(t, ok)
	require.Nil(t, ops)

	conditional, ok := buildFastStructLoopConditionalWriterPlan(nil, elemType)
	require.False(t, ok)
	require.Nil(t, conditional)

	conditional, ok = buildFastStructLoopConditionalWriterPlan(&compiler.FastLoopConditionalPlan{
		Branches: []compiler.FastLoopConditionalBranch{{
			Condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
			Parts:     []compiler.FastLoopPart{{Kind: compiler.FastLoopPartValueProperty, Value: "Missing"}},
			Line:      2,
		}},
	}, elemType)
	require.False(t, ok)
	require.Nil(t, conditional)

	conditional, ok = buildFastStructLoopConditionalWriterPlan(&compiler.FastLoopConditionalPlan{
		ElseParts: []compiler.FastLoopPart{{Kind: compiler.FastLoopPartValueProperty, Value: "Missing"}},
	}, elemType)
	require.False(t, ok)
	require.Nil(t, conditional)

	require.Nil(t, buildFastStructLoopConditionPlan(nil, elemType))
	require.Nil(t, buildFastStructLoopConditionPlan(&compiler.FastValuePlan{Kind: compiler.FastValueInfix, Operator: "==", Left: nil, Right: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}}, elemType))
	require.Nil(t, buildFastStructLoopConditionPlan(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "&&",
		Left: &compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: -1,
			Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Missing"}},
		},
		Right: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
	}, elemType))
	require.Nil(t, buildFastStructLoopConditionPlan(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left: &compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: -1,
			Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Missing"}},
		},
		Right: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
	}, elemType))
	require.Nil(t, buildFastStructLoopConditionPlan(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
		Right: &compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: -1,
			Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Missing"}},
		},
	}, elemType))
	require.Nil(t, buildFastStructLoopConditionPlan(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Missing"}},
	}, elemType))

	methodPlan, ok := buildFastLoopMethodCallPlan(nil, elemType)
	require.False(t, ok)
	require.Nil(t, methodPlan)

	methodPlan, ok = buildFastLoopMethodCallPlan(&compiler.FastValuePlan{Kind: compiler.FastValuePath}, elemType)
	require.False(t, ok)
	require.Nil(t, methodPlan)

	methodPlan, ok = buildFastLoopMethodCallPlan(&compiler.FastValuePlan{
		Kind: compiler.FastValuePath,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Echo"},
			{Kind: compiler.FastPathStepProperty, Value: "Echo"},
		},
	}, elemType)
	require.False(t, ok)
	require.Nil(t, methodPlan)

	methodPlan, ok = buildFastLoopMethodCallPlan(&compiler.FastValuePlan{
		Kind: compiler.FastValuePath,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Echo", Method: false},
			{Kind: compiler.FastPathStepCall, Value: "Echo"},
		},
	}, elemType)
	require.False(t, ok)
	require.Nil(t, methodPlan)

	methodPlan, ok = buildFastLoopMethodCallPlan(&compiler.FastValuePlan{
		Kind: compiler.FastValuePath,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Missing", Method: true},
			{Kind: compiler.FastPathStepCall, Value: "Missing"},
		},
	}, elemType)
	require.False(t, ok)
	require.Nil(t, methodPlan)

	methodPlan, ok = buildFastLoopMethodCallPlan(&compiler.FastValuePlan{
		Kind: compiler.FastValuePath,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Missing"},
			{Kind: compiler.FastPathStepProperty, Value: "Echo", Method: true},
			{Kind: compiler.FastPathStepCall, Value: "Echo"},
		},
	}, elemType)
	require.False(t, ok)
	require.Nil(t, methodPlan)

	partWithBadCachedField := compiler.FastLoopPart{Kind: compiler.FastLoopPartValueProperty, Value: "Name"}
	partWithBadCachedField.PropertyCache.Store(&propertyInlineCacheEntry{
		typ:    elemType,
		lookup: propertyLookup{kind: propertyLookupField, fieldIndex: []int{99}},
	})
	ops, ok = buildFastStructLoopWriterOps([]compiler.FastLoopPart{partWithBadCachedField}, elemType)
	require.False(t, ok)
	require.Nil(t, ops)
}

func Test_VM_Fast_Struct_Field_Loop_Render_And_Cache_Branches(t *testing.T) {
	ctx := plush.NewContext()
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{}, ctx)
	iter := reflect.ValueOf([]vmStructLoopProduct{{Name: "<bot>"}, {Name: "fry"}})
	elemType := reflect.TypeOf(vmStructLoopProduct{})
	var out strings.Builder

	handled, err := renderFastStructFieldLoop(&out, ctx, bindings, nil, iter)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastStructFieldLoop(&out, ctx, bindings, &compiler.FastLoopPlan{}, reflect.ValueOf([]string{"not-struct"}))
	require.NoError(t, err)
	require.False(t, handled)

	invalidLoop := &compiler.FastLoopPlan{Parts: []compiler.FastLoopPart{{
		Kind:  compiler.FastLoopPartValueProperty,
		Value: "Missing",
		Line:  2,
	}}}
	handled, err = renderFastStructFieldLoop(&out, ctx, bindings, invalidLoop, iter)
	require.NoError(t, err)
	require.False(t, handled)
	_, ok := fastStructLoopWriterPlanFor(invalidLoop, elemType)
	require.False(t, ok)
	_, ok = fastStructLoopWriterPlanFor(invalidLoop, elemType)
	require.False(t, ok)

	validLoop := &compiler.FastLoopPlan{
		Line: 3,
		Parts: []compiler.FastLoopPart{
			{Kind: compiler.FastLoopPartKey},
			{Kind: compiler.FastLoopPartStatic, Value: ":"},
			{Kind: compiler.FastLoopPartValueProperty, Value: "Name", Receiver: "product", Full: "product.Name", Line: 3},
			{Kind: compiler.FastLoopPartStatic, Value: ";"},
		},
	}
	plan, ok := fastStructLoopWriterPlanFor(validLoop, elemType)
	require.True(t, ok)
	require.NotNil(t, plan)
	cached, ok := fastStructLoopWriterPlanFor(validLoop, elemType)
	require.True(t, ok)
	require.Same(t, plan, cached)

	out.Reset()
	handled, err = renderFastStructFieldLoop(&out, ctx, bindings, validLoop, iter)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "0:&lt;bot&gt;;1:fry;", out.String())

	handled, err = renderFastStructFieldLoop(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, validLoop, iter)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 3")

	hiddenLoop := &compiler.FastLoopPlan{Line: 4, Parts: []compiler.FastLoopPart{{
		Kind:     compiler.FastLoopPartValueProperty,
		Value:    "hidden",
		Receiver: "product",
		Full:     "product.hidden",
		Line:     4,
	}}}
	handled, err = renderFastStructFieldLoop(&strings.Builder{}, ctx, bindings, hiddenLoop, reflect.ValueOf([]vmFastPropertyUser{{hidden: "secret"}}))
	require.True(t, handled)
	require.ErrorContains(t, err, "line 4")
}

func Test_VM_Fast_String_Key_Value_Loop_Edges(t *testing.T) {
	var out strings.Builder

	separator, suffix, ok := fastStringKeyValueLoopParts(nil)
	require.False(t, ok)
	require.Empty(t, separator)
	require.Empty(t, suffix)

	separator, suffix, ok = fastStringKeyValueLoopParts(&compiler.FastLoopPlan{Parts: []compiler.FastLoopPart{
		{Kind: compiler.FastLoopPartValue},
		{Kind: compiler.FastLoopPartStatic, Value: ":"},
		{Kind: compiler.FastLoopPartValue},
		{Kind: compiler.FastLoopPartStatic, Value: ";"},
	}})
	require.False(t, ok)
	require.Empty(t, separator)
	require.Empty(t, suffix)

	handled, err := renderFastStringKeyValueLoop(&out, plush.NewContext(), &compiler.FastLoopPlan{}, []string{"nope"})
	require.NoError(t, err)
	require.False(t, handled)
	require.Empty(t, out.String())

	loop := &compiler.FastLoopPlan{
		Line: 5,
		Parts: []compiler.FastLoopPart{
			{Kind: compiler.FastLoopPartKey},
			{Kind: compiler.FastLoopPartStatic, Value: ":"},
			{Kind: compiler.FastLoopPartValue},
			{Kind: compiler.FastLoopPartStatic, Value: ";"},
		},
	}

	handled, err = renderFastStringKeyValueLoop(&out, plush.NewContext(), loop, []string{"<a>", "b"})
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "0:&lt;a&gt;;1:b;", out.String())

	handled, err = renderFastStringKeyValueLoop(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), loop, []string{"blocked"})
	require.True(t, handled)
	require.ErrorContains(t, err, "line 5")
}
