package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

func Test_VM_Render_Fast_Conditional_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"flag": false})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"flag"}}, ctx)
	var out strings.Builder

	handled, err := renderFastConditional(&out, ctx, bindings, nil)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastConditional(&out, ctx, bindings, &compiler.FastConditionalPlan{
		Branches: []compiler.FastConditionalBranch{{
			Condition: compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", NullOnMissing: true, Line: 1},
			Segments:  []compiler.FastRenderSegment{{Kind: compiler.FastRenderSegmentStatic, Value: "then"}},
			Line:      1,
		}},
		ElseSegments: []compiler.FastRenderSegment{{Kind: compiler.FastRenderSegmentStatic, Value: "else"}},
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "else", out.String())

	out.Reset()
	handled, err = renderFastConditional(&out, ctx, bindings, &compiler.FastConditionalPlan{
		Branches: []compiler.FastConditionalBranch{{
			Condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true, Line: 1},
			Segments:  []compiler.FastRenderSegment{{Kind: compiler.FastRenderSegmentStatic, Value: "then"}},
			Line:      1,
		}},
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "then", out.String())
}

func Test_VM_Render_Fast_Conditional_Error_And_Empty_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"user": struct{}{}})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"user"}}, ctx)
	conditional := &compiler.FastConditionalPlan{
		Branches: []compiler.FastConditionalBranch{{
			Condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true, Line: 9},
			Segments:  []compiler.FastRenderSegment{{Kind: compiler.FastRenderSegmentStatic, Value: "blocked"}},
			Line:      9,
		}},
	}

	handled, err := renderFastConditional(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, conditional)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 9")

	conditional.Branches[0].Condition = compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: 0,
		Value:     "user",
		Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Missing", Receiver: "user", Full: "user.Missing", Line: 10}},
		Line:      10,
	}
	handled, err = renderFastConditional(&strings.Builder{}, ctx, bindings, conditional)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 10")

	conditional.Branches[0].Condition = compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 11}
	conditional.Branches[0].Segments = nil
	handled, err = renderFastConditional(&strings.Builder{}, ctx, bindings, conditional)
	require.NoError(t, err)
	require.True(t, handled)
}

func Test_VM_Render_Fast_Loop_Error_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"items": []interface{}{"a"},
		"objs":  &object.Array{Elements: []object.Object{&object.String{Value: "<obj>"}}},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"items", "objs"}}, ctx)
	var out strings.Builder

	require.NoError(t, renderFastPartialSegmentWithDataPlan(nil, ctx, bindings, &compiler.FastPartialPlan{Name: "row", Line: 1}, nil))

	handled, err := renderFastLoop(&out, ctx, bindings, &compiler.FastLoopPlan{
		IterableName:      "objs",
		IterableNameIndex: 1,
		Line:              1,
		Parts:             []compiler.FastLoopPart{{Kind: compiler.FastLoopPartValue}},
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "&lt;obj&gt;", out.String())

	err = renderFastLoopParts(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, &compiler.FastLoopPlan{Line: 12}, []compiler.FastLoopPart{{
		Kind:     compiler.FastLoopPartValueProperty,
		Value:    "Name",
		Receiver: "item",
		Full:     "item.Name",
		Line:     12,
	}}, 0, struct{}{})
	require.ErrorContains(t, err, "line 12")

	err = renderFastLoopParts(&out, ctx, bindings, &compiler.FastLoopPlan{Line: 13}, []compiler.FastLoopPart{{
		Kind: compiler.FastLoopPartValuePath,
		ValuePlan: compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: -1,
			Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Missing", Receiver: "item", Full: "item.Missing", Line: 13}},
			Line:      13,
		},
	}}, 0, "value")
	require.ErrorContains(t, err, "line 13")

	err = renderFastLoopParts(&out, ctx, bindings, &compiler.FastLoopPlan{Line: 14}, []compiler.FastLoopPart{{
		Kind: compiler.FastLoopPartConditional,
		Conditional: &compiler.FastLoopConditionalPlan{Branches: []compiler.FastLoopConditionalBranch{{
			Condition: compiler.FastValuePlan{
				Kind:      compiler.FastValuePath,
				NameIndex: -1,
				Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Missing", Receiver: "item", Full: "item.Missing", Line: 14}},
				Line:      14,
			},
			Line: 14,
		}}},
	}}, 0, "value")
	require.ErrorContains(t, err, "line 14")
}

func Test_VM_Render_Fast_Loop_Conditional_Error_And_Empty_Branches(t *testing.T) {
	ctx := plush.NewContext()
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{}, ctx)
	loop := &compiler.FastLoopPlan{Line: 1}
	conditional := &compiler.FastLoopConditionalPlan{
		Branches: []compiler.FastLoopConditionalBranch{{
			Condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true, Line: 15},
			Parts:     []compiler.FastLoopPart{{Kind: compiler.FastLoopPartStatic, Value: "blocked"}},
			Line:      15,
		}},
	}

	err := renderFastLoopConditional(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, loop, conditional, 0, "value")
	require.ErrorContains(t, err, "line 15")

	conditional.Branches[0].Condition = compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Missing", Receiver: "item", Full: "item.Missing", Line: 16}},
		Line:      16,
	}
	err = renderFastLoopConditional(&strings.Builder{}, ctx, bindings, loop, conditional, 0, "value")
	require.ErrorContains(t, err, "line 16")

	conditional.Branches[0].Condition = compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 17}
	conditional.Branches[0].Parts = nil
	err = renderFastLoopConditional(&strings.Builder{}, ctx, bindings, loop, conditional, 0, "value")
	require.NoError(t, err)
}

func Test_VM_Eval_Fast_Loop_Call_Args_Empty_And_Missing(t *testing.T) {
	ctx := plush.NewContext()
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{}, ctx)

	args, err := evalFastLoopCallArgsInto(nil, ctx, bindings, 0, "value", nil)
	require.NoError(t, err)
	require.Nil(t, args)

	args, err = evalFastLoopCallArgsInto([]compiler.FastValuePlan{{Kind: compiler.FastValuePath, NameIndex: -1}}, ctx, bindings, 0, "value", nil)
	require.NoError(t, err)
	require.NotNil(t, args)
	require.Equal(t, "value", args.Raw(0))

	args, err = evalFastLoopCallArgsInto([]compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 18}}, ctx, bindings, 0, "value", nil)
	require.Nil(t, args)
	require.ErrorContains(t, err, "line 18")

	args, err = evalFastLoopCallArgsInto([]compiler.FastValuePlan{{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Missing", Receiver: "item", Full: "item.Missing", Line: 19}},
		Line:      19,
	}}, ctx, bindings, 0, struct{}{}, nil)
	require.Nil(t, args)
	require.ErrorContains(t, err, "line 19")
}

func Test_VM_Write_Fast_Loop_Call_Part_Error_And_Fast_Helper_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"label": func(value string) string { return "slow:" + value },
	})
	SetFastHelper(ctx, "label", func(w FastWriter, args FastArgs) error {
		value, ok := args.String(0)
		require.True(t, ok)
		w.WriteEscapedString("fast:" + value)
		return nil
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
	call := &compiler.FastCallPlan{
		Name:      "label",
		NameIndex: 0,
		Args:      []compiler.FastValuePlan{{Kind: compiler.FastValuePath, NameIndex: -1, Line: 1}},
		Line:      1,
	}
	var out strings.Builder
	require.NoError(t, writeFastLoopCallPart(&out, ctx, bindings, nil, 0, "item"))
	require.Empty(t, out.String())

	require.NoError(t, writeFastLoopCallPart(&out, ctx, bindings, call, 0, "<item>"))
	require.Equal(t, "fast:&lt;item&gt;", out.String())

	missingCall := &compiler.FastCallPlan{Name: "missing", NameIndex: 99, Line: 7}
	err := writeFastLoopCallPart(&strings.Builder{}, ctx, bindings, missingCall, 0, "item")
	require.ErrorContains(t, err, `line 7`)

	err = writeFastLoopCallPart(&strings.Builder{}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, call, 0, "item")
	require.ErrorContains(t, err, `line 1`)

	badArg := &compiler.FastCallPlan{
		Name:      "label",
		NameIndex: 0,
		Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 8}},
		Line:      8,
	}
	err = writeFastLoopCallPart(&strings.Builder{}, ctx, bindings, badArg, 0, "item")
	require.ErrorContains(t, err, `line 8`)

	slowCtx := plush.NewContextWith(map[string]interface{}{
		"label": func(value string) string { return value },
	})
	slowBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, slowCtx)
	err = writeFastLoopCallPart(&strings.Builder{}, slowCtx, slowBindings, badArg, 0, "item")
	require.ErrorContains(t, err, `line 8`)

	SetFastHelper(ctx, "label", func(FastWriter, FastArgs) error {
		return errors.New("boom")
	})
	err = writeFastLoopCallPart(&strings.Builder{}, ctx, bindings, call, 0, "item")
	require.ErrorContains(t, err, `line 1`)
}
