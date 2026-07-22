package vm

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

func Test_VM_Fast_Static_Name_Cached_Value_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"name": "Mido"})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name"}}, ctx)

	value, ok := fastStaticNameCachedValue(&fastStaticNameOp{lookupIndex: 0, nameIndex: 99}, bindings, []interface{}{"cached"}, []bool{true})
	require.True(t, ok)
	require.Equal(t, "cached", value)

	value, ok = fastStaticNameCachedValue(&fastStaticNameOp{lookupIndex: 2, nameIndex: 0}, bindings, []interface{}{"cached"}, []bool{true})
	require.True(t, ok)
	require.Equal(t, "Mido", value)

	value, ok = fastStaticNameCachedValue(&fastStaticNameOp{lookupIndex: -1, nameIndex: 0}, bindings, nil, nil)
	require.True(t, ok)
	require.Equal(t, "Mido", value)

	value, ok = fastStaticNameCachedValue(nil, bindings, nil, nil)
	require.False(t, ok)
	require.Nil(t, value)
}

func Test_VM_Fast_Render_Mixed_And_Conditional_Remaining_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"items": []string{"x"},
		"name":  "Mido",
		"fail": func() (string, error) {
			return "", errors.New("boom")
		},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"items", "name", "fail"}}, ctx)
	maxInt := int(^uint(0) >> 1)

	grow := fastOutputGrowSize(&fastMixedPlan{
		staticSize: maxInt - 1,
		ops: []fastMixedOp{{
			loop: &compiler.FastLoopPlan{IterableNameIndex: 0, StaticSize: 10},
		}},
	}, bindings)
	require.Equal(t, maxInt-1, grow)

	require.Nil(t, buildFastSimpleConditionalPlan(&compiler.FastConditionalPlan{
		Branches: []compiler.FastConditionalBranch{{
			Condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: false},
		}},
		ElseSegments: []compiler.FastRenderSegment{{
			Kind: compiler.FastRenderSegmentCall,
			Call: &compiler.FastCallPlan{Name: "name", NameIndex: 1},
		}},
	}))

	var out strings.Builder
	wideNames := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	ok, err := renderFastSimpleConditionalPlan(&out, ctx, bindings, &fastSimpleConditionalPlan{
		nameIndexes: wideNames,
		branches: []fastSimpleConditionalBranch{{
			condition: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: false}},
			line:      31,
		}},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, out.String())

	_, err = renderFastSimpleConditionalPlan(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, &fastSimpleConditionalPlan{
		branches: []fastSimpleConditionalBranch{{
			condition: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}},
			line:      32,
		}},
	})
	require.ErrorContains(t, err, "line 32")

	_, err = renderFastSimpleConditionalPlan(&out, ctx, bindings, &fastSimpleConditionalPlan{
		branches: []fastSimpleConditionalBranch{{
			condition: &fastSimpleValuePlan{
				value: &compiler.FastValuePlan{Kind: compiler.FastValueInfix, Operator: "??", Line: 33},
				left:  &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 1}},
				right: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 2}},
			},
			line: 33,
		}},
	})
	require.ErrorContains(t, err, "unknown fast infix operator")

	_, _, err = evalFastSimpleValue(&fastSimpleValuePlan{
		value: &compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: 2,
			Value:     "fail",
			Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepCall, Value: "fail", Line: 34}},
		},
		lookupIndex: -1,
	}, ctx, bindings, nil, nil, nil)
	require.ErrorContains(t, err, "boom")

	value, valueOK, err := evalFastSimpleValue(&fastSimpleValuePlan{
		value: &compiler.FastValuePlan{Kind: compiler.FastValueKind(255)},
	}, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.False(t, valueOK)
	require.Nil(t, value)

	out.Reset()
	ok, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpValue,
		valuePlan: compiler.FastValuePlan{
			Kind:     compiler.FastValueInfix,
			Operator: "==",
			Left:     &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 1},
			Right:    &compiler.FastValuePlan{Kind: compiler.FastValueFloat, FloatValue: 1},
			Line:     35,
		},
		line: 35,
	}}})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "true", out.String())

	out.Reset()
	ok, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpAccessChain,
		valuePlan: compiler.FastValuePlan{
			Kind:     compiler.FastValueInfix,
			Operator: "==",
			Left:     &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "a"},
			Right:    &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "a"},
			Line:     36,
		},
		line: 36,
	}}})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "true", out.String())

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind:      fastMixedOpAccessChain,
		valuePlan: compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: 99, Value: "missing"},
		line:      37,
	}}})
	require.ErrorContains(t, err, "line 37")

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind:      fastMixedOpAccessChain,
		valuePlan: compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"},
		line:      38,
	}}})
	require.ErrorContains(t, err, "line 38")

	_, err = renderFastMixedPlan(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpConditional,
		simpleCond: &fastSimpleConditionalPlan{
			branches: []fastSimpleConditionalBranch{{
				condition: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}},
				line:      39,
			}},
		},
	}}})
	require.ErrorContains(t, err, "line 39")
}

func Test_VM_Fast_Value_Missing_Name_Branches(t *testing.T) {
	require.Empty(t, fastSimpleValueMissingName(nil))
	require.Empty(t, fastValueMissingName(nil))

	require.Equal(t, "helper", fastValueMissingName(&compiler.FastValuePlan{
		Kind: compiler.FastValueCall,
		Call: &compiler.FastCallPlan{Name: "helper"},
	}))
	require.Equal(t, "direct", fastValueMissingName(&compiler.FastValuePlan{Value: "direct"}))
	require.Equal(t, "left", fastValueMissingName(&compiler.FastValuePlan{
		Left:  &compiler.FastValuePlan{Value: "left"},
		Right: &compiler.FastValuePlan{Value: "right"},
	}))
	require.Equal(t, "right", fastValueMissingName(&compiler.FastValuePlan{
		Right: &compiler.FastValuePlan{Value: "right"},
	}))
	require.Empty(t, fastValueMissingName(&compiler.FastValuePlan{}))
}

func Test_VM_Fast_Render_Inline_And_Simple_Value_Edges(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"bad":  7,
		"echo": func(value string) string { return value + "!" },
		"zero": func() string { return "zero!" },
	})
	plan := &compiler.FastRenderPlan{Bindings: []string{"name", "bad", "echo", "zero"}}
	bindings := newFastRenderBindings(plan, ctx)
	var out strings.Builder

	rendered, handled, err := renderFastPlanWithBindingPlan(nil, ctx, nil)
	require.NoError(t, err)
	require.False(t, handled)
	require.Empty(t, rendered)

	ok, err := renderFastPlanInlineSafe(nil, plan, ctx, nil)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = renderFastPlanInlineSafe(&out, nil, ctx, nil)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = renderFastPlanInlineWithBindings(nil, plan, ctx, bindings)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = renderFastPlanInlineWithBindings(&out, nil, ctx, bindings)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = renderFastPlanInlineWithBindings(&out, &compiler.FastRenderPlan{}, ctx, bindings)
	require.NoError(t, err)
	require.False(t, ok)
	nilPreparedPlan := &compiler.FastRenderPlan{}
	nilPreparedPlan.Prepared.Store((*fastMixedPlan)(nil))
	ok, err = renderFastPlanInlineWithBindings(&out, nilPreparedPlan, ctx, bindings)
	require.NoError(t, err)
	require.False(t, ok)

	require.Nil(t, topLevelFastBindingPlan(nil, ctx))
	require.Nil(t, topLevelFastBindingPlan(&compiler.FastRenderPlan{}, ctx))
	require.Nil(t, topLevelFastBindingPlan(plan, newLookupTestContext(map[string]interface{}{"name": "Mido"})))
	require.False(t, canCacheFastRenderBindingPlan(nil))
	require.False(t, canCacheFastRenderBindingPlan(newLookupTestContext(map[string]interface{}{"name": "Mido"})))
	prepared := topLevelFastBindingPlan(plan, ctx)
	require.NotNil(t, prepared)
	require.Same(t, prepared, topLevelFastBindingPlan(plan, ctx))
	mismatched := &fastRenderBindingPlan{names: []string{"other"}}
	otherPlan := &compiler.FastRenderPlan{Bindings: []string{"name"}}
	otherPlan.BindingPrepared.Store(mismatched)
	require.NotSame(t, mismatched, topLevelFastBindingPlan(otherPlan, ctx))

	require.Zero(t, fastOutputGrowSize(nil, bindings))
	require.Zero(t, fastLoopGrowSize(nil))
	length, lengthOK := fastIterableLen(nil)
	require.True(t, lengthOK)
	require.Zero(t, length)
	length, lengthOK = fastIterableLen(3)
	require.False(t, lengthOK)
	require.Zero(t, length)
	var nilArrayPointer *[2]string
	length, lengthOK = fastIterableLen(nilArrayPointer)
	require.True(t, lengthOK)
	require.Zero(t, length)
	require.Equal(t, 0, fastOutputGrowSize(&fastMixedPlan{ops: []fastMixedOp{{loop: &compiler.FastLoopPlan{IterableName: "missing", IterableNameIndex: 99}}}}, bindings))
	require.Equal(t, 0, fastOutputGrowSize(&fastMixedPlan{ops: []fastMixedOp{{loop: &compiler.FastLoopPlan{IterableName: "name", IterableNameIndex: 0}}}}, bindings))

	require.Nil(t, buildFastStaticNamePlan(nil))
	require.Nil(t, buildFastStaticNamePlan(&fastMixedPlan{}))
	require.Nil(t, buildFastStaticNamePlan(&fastMixedPlan{ops: []fastMixedOp{{kind: fastMixedOpStatic}}}))
	require.Equal(t, fastMixedOpStatic, buildFastMixedPlan(&compiler.FastRenderPlan{Segments: []compiler.FastRenderSegment{{Kind: compiler.FastRenderSegmentKind(255)}}}).ops[0].kind)
	require.Equal(t, -1, (&fastStaticNamePlan{}).bindNameIndex(-1))
	require.Equal(t, -1, (*fastSimplePlan)(nil).bindNameIndex(0))
	require.Equal(t, -1, (&fastSimplePlan{}).bindNameIndex(-1))
	require.Nil(t, buildFastPartialDataBindingPlan(&compiler.FastPartialPlan{Data: []compiler.FastPartialDataPair{{Key: "bad", Value: compiler.FastValuePlan{Kind: compiler.FastValueCall}}}}))

	value, valueOK, err := evalFastSimpleValue(nil, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, valueOK)
	require.Nil(t, value)

	tests := []struct {
		name     string
		plan     *fastSimpleValuePlan
		expected interface{}
		ok       bool
	}{
		{"name nil", &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "nil"}}, nil, true},
		{"name cached", &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "name"}, lookupIndex: 0}, "cached", true},
		{"name missing null", &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", NullOnMissing: true}, lookupIndex: -1}, nil, true},
		{"name missing hard", &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"}, lookupIndex: -1}, nil, false},
		{"string", &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "x"}}, "x", true},
		{"integer", &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 3}}, 3, true},
		{"float", &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueFloat, FloatValue: 1.25}}, 1.25, true},
		{"bool", &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}}, true, true},
		{"path missing null", &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: 99, Value: "missing", NullOnMissing: true}, lookupIndex: -1}, nil, true},
		{"path missing hard", &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: 99, Value: "missing"}, lookupIndex: -1}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, valueOK, err := evalFastSimpleValue(tt.plan, ctx, bindings, []interface{}{"cached"}, []bool{true}, nil)
			require.NoError(t, err)
			require.Equal(t, tt.ok, valueOK)
			require.Equal(t, tt.expected, value)
		})
	}

	pathCallValue, pathCallOK, err := evalFastSimpleValue(&fastSimpleValuePlan{
		value: &compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: 3,
			Value:     "zero",
			Path: []compiler.FastPathStep{
				{Kind: compiler.FastPathStepCall, Value: "zero", Line: 1},
			},
		},
		lookupIndex: -1,
	}, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, pathCallOK)
	require.Equal(t, "zero!", pathCallValue)

	callValue, callOK, err := evalFastSimpleCallValue(nil, ctx, bindings, nil, nil)
	require.NoError(t, err)
	require.False(t, callOK)
	require.Nil(t, callValue)

	callValue, callOK, err = evalFastCallValuePlan(nil, nil, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, callOK)
	require.Nil(t, callValue)

	callValue, callOK, err = evalFastCallValuePlan(&compiler.FastCallPlan{Name: "missing", NameIndex: 99}, nil, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.False(t, callOK)
	require.Nil(t, callValue)

	_, callOK, err = evalFastCallValuePlan(&compiler.FastCallPlan{Name: "echo", NameIndex: 2, Args: []compiler.FastValuePlan{{Kind: compiler.FastValueString, Value: "go"}}, Line: 6}, []*fastSimpleValuePlan{{value: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "go"}}}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, nil, nil)
	require.ErrorContains(t, err, "line 6")
	require.True(t, callOK)

	callValue, callOK, err = evalFastCallValuePlan(&compiler.FastCallPlan{Name: "echo", NameIndex: 2, Args: []compiler.FastValuePlan{{Kind: compiler.FastValueString, Value: "go"}}, Line: 7}, []*fastSimpleValuePlan{{value: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "go"}}}, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, callOK)
	require.Equal(t, "go!", callValue)

	callValue, callOK, err = evalFastCallValuePlan(&compiler.FastCallPlan{Name: "echo", NameIndex: 2, Args: []compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 8}}, Line: 8}, nil, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.False(t, callOK)
	require.Nil(t, callValue)

	_, callOK, err = evalFastCallValuePlan(&compiler.FastCallPlan{Name: "bad", NameIndex: 1, Line: 9}, nil, ctx, bindings, nil, nil, nil)
	require.ErrorContains(t, err, "line 9")
	require.True(t, callOK)

	infixValue, infixOK, err := evalFastSimpleInfixValue(&fastSimpleValuePlan{
		value: &compiler.FastValuePlan{Kind: compiler.FastValueInfix, Operator: "==", Line: 10},
		left:  &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missingLeft"}},
		right: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 100, Value: "missingRight"}},
	}, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, infixOK)
	require.Equal(t, true, infixValue)

	infixValue, infixOK, err = evalFastSimpleInfixValue(&fastSimpleValuePlan{
		value: &compiler.FastValuePlan{Kind: compiler.FastValueInfix, Operator: "&&", Line: 11},
		left:  &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: false}},
		right: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"}},
	}, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.True(t, infixOK)
	require.Equal(t, false, infixValue)

	infixValue, infixOK, err = evalFastSimpleInfixValue(nil, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.False(t, infixOK)
	require.Nil(t, infixValue)

	errorCall := &fastSimpleValuePlan{
		value: &compiler.FastValuePlan{
			Kind: compiler.FastValueCall,
			Call: &compiler.FastCallPlan{
				Name:      "echo",
				NameIndex: 2,
				Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueString, Value: "go"}},
				Line:      13,
			},
		},
		args:        []*fastSimpleValuePlan{{value: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "go"}}},
		lookupIndex: -1,
	}
	_, infixOK, err = evalFastSimpleInfixValue(&fastSimpleValuePlan{
		value: &compiler.FastValuePlan{Kind: compiler.FastValueInfix, Operator: "==", Line: 13},
		left:  errorCall,
		right: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "go"}},
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, nil, nil)
	require.ErrorContains(t, err, "line 13")
	require.True(t, infixOK)

	_, infixOK, err = evalFastSimpleInfixValue(&fastSimpleValuePlan{
		value: &compiler.FastValuePlan{Kind: compiler.FastValueInfix, Operator: "==", Line: 14},
		left:  &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "go"}},
		right: errorCall,
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, nil, nil)
	require.ErrorContains(t, err, "line 13")
	require.True(t, infixOK)

	infixValue, infixOK, err = evalFastSimpleLogicalInfixValue(&fastSimpleValuePlan{
		value: &compiler.FastValuePlan{Kind: compiler.FastValueInfix, Operator: "??", Line: 12},
		left:  &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}},
		right: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}},
	}, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.False(t, infixOK)
	require.Nil(t, infixValue)

	infixValue, infixOK, err = evalFastSimpleLogicalInfixValue(nil, ctx, bindings, nil, nil, nil)
	require.NoError(t, err)
	require.False(t, infixOK)
	require.Nil(t, infixValue)

	_, infixOK, err = evalFastSimpleLogicalInfixValue(&fastSimpleValuePlan{
		value: &compiler.FastValuePlan{Kind: compiler.FastValueInfix, Operator: "&&", Line: 15},
		left:  errorCall,
		right: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}},
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, nil, nil)
	require.ErrorContains(t, err, "line 13")
	require.True(t, infixOK)

	_, infixOK, err = evalFastSimpleLogicalInfixValue(&fastSimpleValuePlan{
		value: &compiler.FastValuePlan{Kind: compiler.FastValueInfix, Operator: "&&", Line: 16},
		left:  &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}},
		right: errorCall,
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, nil, nil, nil)
	require.ErrorContains(t, err, "line 13")
	require.True(t, infixOK)

	var nilConditionalPlan *fastSimpleConditionalPlan
	require.Equal(t, -1, nilConditionalPlan.bindNameIndex(0))
	conditionalPlan := &fastSimpleConditionalPlan{}
	require.Equal(t, -1, conditionalPlan.bindNameIndex(-1))
	require.Equal(t, 0, conditionalPlan.bindNameIndex(3))
	require.Equal(t, 0, conditionalPlan.bindNameIndex(3))
	require.Equal(t, []int{3}, conditionalPlan.nameIndexes)

	var nilPartialDataPlan *fastPartialDataBindingPlan
	require.Equal(t, -1, nilPartialDataPlan.bindNameIndex(0))
	partialDataPlan := &fastPartialDataBindingPlan{}
	require.Equal(t, -1, partialDataPlan.bindNameIndex(-1))
	require.Equal(t, 0, partialDataPlan.bindNameIndex(4))
	require.Equal(t, 0, partialDataPlan.bindNameIndex(4))
	require.Equal(t, []int{4}, partialDataPlan.nameIndexes)
}

func Test_VM_Fast_Simple_Plan_Builder_Edge_Branches(t *testing.T) {
	require.Nil(t, buildFastSimplePlan(nil))
	require.Nil(t, buildFastSimplePlan(&fastMixedPlan{}))
	require.Nil(t, buildFastSimplePlan(&fastMixedPlan{ops: []fastMixedOp{{kind: fastMixedOpStatic}}}))
	require.Nil(t, buildFastSimplePlan(&fastMixedPlan{ops: []fastMixedOp{{
		kind:      fastMixedOpAccessChain,
		valuePlan: compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: -1},
	}}}))
	require.Nil(t, buildFastSimplePlan(&fastMixedPlan{ops: []fastMixedOp{{
		kind:      fastMixedOpValue,
		valuePlan: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "x"},
	}}}))
	require.Nil(t, buildFastSimplePlan(&fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpValue,
		valuePlan: compiler.FastValuePlan{
			Kind:     compiler.FastValueInfix,
			Operator: "==",
			Left:     &compiler.FastValuePlan{Kind: compiler.FastValueLoopKey},
			Right:    &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "x"},
		},
	}}}))

	require.Nil(t, buildFastSimpleConditionalPlan(nil))
	require.Nil(t, buildFastSimpleConditionalPlan(&compiler.FastConditionalPlan{}))
	require.Nil(t, buildFastSimpleConditionalPlan(&compiler.FastConditionalPlan{
		Branches: []compiler.FastConditionalBranch{{
			Condition: compiler.FastValuePlan{Kind: compiler.FastValueLoopKey},
		}},
	}))
}

func Test_VM_Fast_Simple_Plan_Edge_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name":        "Mido",
		"unsupported": &object.Builtin{},
		"user":        vmSegmentUser{Name: "Bender"},
		"scalar":      3,
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name", "unsupported", "user", "scalar"}}, ctx)
	var out strings.Builder

	ok, err := renderFastSimplePlan(nil, ctx, bindings, &fastSimplePlan{})
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderFastSimplePlan(&out, ctx, bindings, nil)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{ops: []fastSimpleOp{{}}})
	require.NoError(t, err)
	require.False(t, ok)

	value, cachedOK := fastSimpleCachedOpValue(nil, bindings, nil, nil)
	require.False(t, cachedOK)
	require.Nil(t, value)

	value, cachedOK = fastSimpleCachedOpValue(&fastSimpleOp{op: &fastMixedOp{nameIndex: 0}, lookupIndex: -1}, bindings, nil, nil)
	require.True(t, cachedOK)
	require.Equal(t, "Mido", value)

	require.Zero(t, fastSimplePlanGrowSize(nil))
	require.Equal(t, 0, fastSimplePlanGrowSize(&fastSimplePlan{ops: []fastSimpleOp{{}}}))
	require.Equal(t, 19, fastSimplePlanGrowSize(&fastSimplePlan{ops: []fastSimpleOp{{op: &fastMixedOp{prefix: "pre", kind: fastMixedOpName}}}}))

	out.Reset()
	ok, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		nameIndexes: []int{99, 0},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{kind: fastMixedOpName, prefix: "pre", nameIndex: 99, value: "missing", nullOnMissing: true, line: 1}, lookupIndex: 0},
			{op: &fastMixedOp{kind: fastMixedOpName, nameIndex: 0, value: "name", line: 1}, lookupIndex: 1},
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "preMido", out.String())

	ok, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		nameIndexes: []int{1},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{kind: fastMixedOpName, nameIndex: 1, value: "unsupported", line: 2}, lookupIndex: 0},
		},
	})
	require.NoError(t, err)
	require.False(t, ok)

	_, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		nameIndexes: []int{99},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{kind: fastMixedOpName, nameIndex: 99, value: "missing", line: 3}, lookupIndex: 0},
		},
	})
	require.ErrorContains(t, err, "line 3")

	_, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		nameIndexes: []int{99},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{kind: fastMixedOpProperty, nameIndex: 99, value: "missing", property: "Name", line: 4}, lookupIndex: 0},
		},
	})
	require.ErrorContains(t, err, "line 4")

	_, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		nameIndexes: []int{2},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{kind: fastMixedOpProperty, nameIndex: 2, value: "user", property: "Missing", receiver: "user", full: "user.Missing", line: 5}, lookupIndex: 0},
		},
	})
	require.ErrorContains(t, err, "line 5")

	_, err = renderFastSimplePlan(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, &fastSimplePlan{
		nameIndexes: []int{2},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{kind: fastMixedOpProperty, nameIndex: 2, value: "user", property: "Name", receiver: "user", full: "user.Name", line: 51}, lookupIndex: 0},
		},
	})
	require.ErrorContains(t, err, "line 51")

	ok, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		nameIndexes: []int{3},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{
				kind:      fastMixedOpAccessChain,
				nameIndex: 3,
				valuePlan: compiler.FastValuePlan{
					Kind:      compiler.FastValuePath,
					NameIndex: 3,
					Value:     "scalar",
					Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Name"}},
					Line:      6,
				},
				line: 6,
			}, lookupIndex: 0},
		},
	})
	require.NoError(t, err)
	require.False(t, ok)

	out.Reset()
	ok, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		nameIndexes: []int{99},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{
				kind: fastMixedOpAccessChain,
				valuePlan: compiler.FastValuePlan{
					Kind:          compiler.FastValuePath,
					NameIndex:     99,
					Value:         "missing",
					NullOnMissing: true,
					Path:          vmAccessProfileNamePlan(99).Path,
					Line:          61,
				},
				line: 61,
			}, lookupIndex: 0},
			{op: &fastMixedOp{kind: fastMixedOpStatic, prefix: "after"}},
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "after", out.String())

	_, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		nameIndexes: []int{99},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{
				kind: fastMixedOpAccessChain,
				valuePlan: compiler.FastValuePlan{
					Kind:      compiler.FastValuePath,
					NameIndex: 99,
					Value:     "missing",
					Path:      vmAccessProfileNamePlan(99).Path,
					Line:      62,
				},
				line: 62,
			}, lookupIndex: 0},
		},
	})
	require.ErrorContains(t, err, "line 62")

	accessCtx := plush.NewContextWith(map[string]interface{}{"user": vmAccessUser{Profile: &vmAccessProfile{Name: "Bender"}}})
	accessBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"user"}}, accessCtx)
	_, err = renderFastSimplePlan(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), accessBindings, &fastSimplePlan{
		nameIndexes: []int{0},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{
				kind:      fastMixedOpAccessChain,
				valuePlan: *vmAccessProfileNamePlan(0),
				line:      63,
			}, lookupIndex: 0},
		},
	})
	require.ErrorContains(t, err, "line 1")

	ok, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		ops: []fastSimpleOp{{op: &fastMixedOp{kind: fastMixedOpValue, valuePlan: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "x"}}}},
	})
	require.NoError(t, err)
	require.False(t, ok)

	infixPlan := compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "??",
		Value:    "bad",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "Mido"},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "Mido"},
	}
	_, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		ops: []fastSimpleOp{{op: &fastMixedOp{kind: fastMixedOpValue, valuePlan: infixPlan, line: 7}, value: &fastSimpleValuePlan{
			value: &infixPlan,
			left:  &fastSimpleValuePlan{value: infixPlan.Left},
			right: &fastSimpleValuePlan{value: infixPlan.Right},
		}}},
	})
	require.ErrorContains(t, err, "unknown fast infix operator")

	_, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		ops: []fastSimpleOp{{op: &fastMixedOp{kind: fastMixedOpValue, valuePlan: compiler.FastValuePlan{Kind: compiler.FastValueInfix, Value: "missing"}, line: 71}, value: &fastSimpleValuePlan{
			value: &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"},
		}}},
	})
	require.ErrorContains(t, err, "line 71")

	out.Reset()
	ok, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		ops: []fastSimpleOp{{op: &fastMixedOp{kind: fastMixedOpValue, valuePlan: compiler.FastValuePlan{Kind: compiler.FastValueInfix}, line: 72}, value: &fastSimpleValuePlan{
			value: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "<raw>"},
		}}},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "&lt;raw&gt;", out.String())

	ok, err = renderFastSimplePlan(&out, ctx, bindings, &fastSimplePlan{
		ops: []fastSimpleOp{{op: &fastMixedOp{kind: fastMixedOpKind(255)}}},
	})
	require.NoError(t, err)
	require.False(t, ok)

	wideCtx := plush.NewContextWith(map[string]interface{}{
		"n0": "zero", "n1": "one", "n2": "two", "n3": "three", "n4": "four",
		"n5": "five", "n6": "six", "n7": "seven", "n8": "eight",
	})
	wideBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"n0", "n1", "n2", "n3", "n4", "n5", "n6", "n7", "n8"}}, wideCtx)
	out.Reset()
	ok, err = renderFastSimplePlan(&out, wideCtx, wideBindings, &fastSimplePlan{
		nameIndexes: []int{0, 1, 2, 3, 4, 5, 6, 7, 8},
		ops: []fastSimpleOp{
			{op: &fastMixedOp{kind: fastMixedOpName, nameIndex: 8, value: "n8", line: 73}, lookupIndex: 8},
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "eight", out.String())

	ok, err = renderFastSimpleConditionalPlan(nil, ctx, bindings, &fastSimpleConditionalPlan{})
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderFastSimpleConditionalPlan(&out, ctx, bindings, nil)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderFastSimpleConditionalPlan(&out, ctx, bindings, &fastSimpleConditionalPlan{
		branches: []fastSimpleConditionalBranch{{condition: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}}, line: 8}},
	})
	require.NoError(t, err)
	require.True(t, ok)

	out.Reset()
	ok, err = renderFastSimpleConditionalPlan(&out, ctx, bindings, &fastSimpleConditionalPlan{
		branches:     []fastSimpleConditionalBranch{{condition: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: false}}, line: 9}},
		elseSegments: &fastSimplePlan{ops: []fastSimpleOp{{op: &fastMixedOp{kind: fastMixedOpStatic, prefix: "else"}}}},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "else", out.String())
}

func Test_VM_Fast_Static_Name_Inline_Safe_Edge_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"n0":          "zero",
		"n1":          "one",
		"n2":          "two",
		"n3":          "three",
		"n4":          "four",
		"n5":          "five",
		"n6":          "six",
		"n7":          "seven",
		"n8":          "eight",
		"unsupported": &object.Builtin{},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{
		"n0", "n1", "n2", "n3", "n4", "n5", "n6", "n7", "n8", "unsupported",
	}}, ctx)

	ok, err := renderFastStaticNamePlanInlineSafe(nil, ctx, bindings, &fastStaticNamePlan{})
	require.NoError(t, err)
	require.False(t, ok)

	var out strings.Builder
	ok, err = renderFastStaticNamePlanInlineSafe(&out, ctx, bindings, nil)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderFastStaticNamePlanInlineSafe(&out, ctx, bindings, &fastStaticNamePlan{
		nameIndexes: []int{0, 1, 2, 3, 4, 5, 6, 7, 8},
		ops: []fastStaticNameOp{
			{prefix: "pre:", nameIndex: -1},
			{nameIndex: 8, lookupIndex: 8, value: "n8", line: 1},
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "pre:eight", out.String())

	out.Reset()
	ok, err = renderFastStaticNamePlanInlineSafe(&out, ctx, bindings, &fastStaticNamePlan{
		nameIndexes: []int{99},
		ops: []fastStaticNameOp{
			{nameIndex: 99, lookupIndex: 0, value: "missing", nullOnMissing: true, line: 2},
			{prefix: "after", nameIndex: -1},
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "after", out.String())

	_, err = renderFastStaticNamePlanInlineSafe(&out, ctx, bindings, &fastStaticNamePlan{
		nameIndexes: []int{99},
		ops: []fastStaticNameOp{
			{nameIndex: 99, lookupIndex: 0, value: "missing", line: 3},
		},
	})
	require.ErrorContains(t, err, "line 3")

	ok, err = renderFastStaticNamePlanInlineSafe(&out, ctx, bindings, &fastStaticNamePlan{
		nameIndexes: []int{9},
		ops: []fastStaticNameOp{
			{nameIndex: 9, lookupIndex: 0, value: "unsupported", line: 4},
		},
	})
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderFastStaticNamePlan(&out, ctx, bindings, nil)
	require.NoError(t, err)
	require.False(t, ok)

	out.Reset()
	ok, err = renderFastStaticNamePlan(&out, ctx, bindings, &fastStaticNamePlan{
		ops: []fastStaticNameOp{
			{prefix: "pre:", nameIndex: -1},
			{nameIndex: 0, lookupIndex: -1, value: "n0", line: 5},
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "pre:zero", out.String())

	out.Reset()
	ok, err = renderFastStaticNamePlan(&out, ctx, bindings, &fastStaticNamePlan{
		ops: []fastStaticNameOp{
			{nameIndex: 99, lookupIndex: -1, value: "missing", nullOnMissing: true, line: 6},
			{prefix: "after", nameIndex: -1},
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "after", out.String())

	ok, err = renderFastStaticNamePlan(&out, ctx, bindings, &fastStaticNamePlan{
		ops: []fastStaticNameOp{
			{nameIndex: 9, lookupIndex: -1, value: "unsupported", line: 7},
		},
	})
	require.NoError(t, err)
	require.False(t, ok)
}

func Test_VM_Fast_Property_Reflect_Output_Edges(t *testing.T) {
	ctx := plush.NewContext()
	var out strings.Builder
	access := object.PropertyAccess{Receiver: "user", Full: "user.Name"}
	user := vmFastPropertyUser{Name: "<mido>", Count: 7}
	var nameSlot object.InlineCacheSlot

	require.NoError(t, writeFastPropertyReflectOutput(&out, ctx, reflect.Value{}, "Name", access, &nameSlot))
	require.Empty(t, out.String())

	var nilUser *vmFastPropertyUser
	require.NoError(t, writeFastPropertyReflectOutput(&out, ctx, reflect.ValueOf(nilUser), "Name", access, &nameSlot))
	require.Empty(t, out.String())

	require.NoError(t, writeFastPropertyReflectOutput(&out, ctx, reflect.ValueOf(user), "Name", access, &nameSlot))
	require.Equal(t, "&lt;mido&gt;", out.String())

	out.Reset()
	require.NoError(t, writeFastPropertyReflectOutput(&out, ctx, reflect.ValueOf(&user), "Name", access, &nameSlot))
	require.Equal(t, "&lt;mido&gt;", out.String())

	out.Reset()
	err := writeFastPropertyReflectOutput(&out, ctx, reflect.ValueOf(user), "Name", object.PropertyAccess{Receiver: "user", Full: "user.Name()", Method: true}, &nameSlot)
	require.ErrorContains(t, err, "does not have a method")
	require.Empty(t, out.String())

	out.Reset()
	var echoSlot object.InlineCacheSlot
	require.NoError(t, writeFastPropertyReflectOutput(&out, ctx, reflect.ValueOf(user), "Echo", object.PropertyAccess{Receiver: "user", Full: "user.Echo()", Method: true}, &echoSlot))
	require.Empty(t, out.String())
}

func Test_VM_Fast_Property_Value_From_Reflect_Manual_Entries(t *testing.T) {
	user := vmFastPropertyUser{Name: "Mido", Count: 7}
	rv := reflect.ValueOf(user)

	valueLookup := cachedPropertyLookup(rv.Type(), "Echo")
	value, err := fastPropertyValueFromReflect(rv, user, "Echo", object.PropertyAccess{Receiver: "user", Full: "user.Echo()", Method: true}, &propertyInlineCacheEntry{lookup: valueLookup})
	require.NoError(t, err)
	require.IsType(t, func() string { return "" }, value)

	pointerLookup := cachedPropertyLookup(rv.Type(), "PointerEcho")
	value, err = fastPropertyValueFromReflect(rv, user, "PointerEcho", object.PropertyAccess{Receiver: "user", Full: "user.PointerEcho()", Method: true}, &propertyInlineCacheEntry{lookup: pointerLookup})
	require.NoError(t, err)
	require.IsType(t, func() string { return "" }, value)

	fieldLookup := cachedPropertyLookup(rv.Type(), "Count")
	value, err = fastPropertyValueFromReflect(rv, user, "Count", object.PropertyAccess{Receiver: "user", Full: "user.Count"}, &propertyInlineCacheEntry{lookup: fieldLookup})
	require.NoError(t, err)
	require.Equal(t, int32(7), value)

	_, err = fastPropertyValueFromReflect(rv, user, "Count", object.PropertyAccess{Receiver: "user", Full: "user.Count()", Method: true}, &propertyInlineCacheEntry{lookup: fieldLookup})
	require.ErrorContains(t, err, "does not have a method")

	_, err = fastPropertyValueFromReflect(reflect.ValueOf(3), 3, "Missing", object.PropertyAccess{Receiver: "value", Full: "value.Missing"}, &propertyInlineCacheEntry{lookup: propertyLookup{kind: propertyLookupMissing}})
	require.ErrorContains(t, err, "does not have a field or method")

	nilString := (*string)(nil)
	value, err = fastFieldValue(reflect.ValueOf(&nilString).Elem(), object.PropertyAccess{}, "Name")
	require.NoError(t, err)
	require.Nil(t, value)

	privateField := reflect.ValueOf(vmFastPropertyUser{hidden: "secret"}).FieldByName("hidden")
	_, err = fastFieldValue(privateField, object.PropertyAccess{}, "hidden")
	require.ErrorContains(t, err, "cannot return value obtained from unexported field or method")
}

func Test_VM_Partial_Overlay_Update_And_Name_For_ID_Edges(t *testing.T) {
	parent := &lookupTestContext{values: map[string]interface{}{"parent": "old"}}
	ctx := borrowPartialOverlayContext(parent)
	defer releasePartialOverlayContext(ctx)

	require.False(t, (*partialOverlayContext)(nil).Update("x", "y"))
	require.False(t, (*partialOverlayContext)(nil).Update("x", nil))
	_, ok := (*partialOverlayContext)(nil).nameForID(0)
	require.False(t, ok)

	id := ctx.InternID("parent")
	name, ok := ctx.nameForID(id)
	require.True(t, ok)
	require.Equal(t, "parent", name)
	require.True(t, ctx.UpdateID(id, "new"))
	require.Equal(t, "new", parent.values["parent"])

	ctx.Set("local", "first")
	require.True(t, ctx.Update("local", "second"))
	value, ok := ctx.Lookup("local")
	require.True(t, ok)
	require.Equal(t, "second", value)
	require.False(t, ctx.Update("missing", "value"))

	ctx.rememberIDName(-1, "bad")
	ctx.rememberIDName(99, "")
	_, ok = ctx.nameForID(-1)
	require.False(t, ok)
}

func Test_VM_Loop_Predicate_Unknown_Opcode_Branches(t *testing.T) {
	block := executeForClosure(code.Instructions{byte(255)}, 0)
	require.False(t, loopCanUseRawValues(block))
	require.False(t, loopNeedsContextWrites(block))
}
