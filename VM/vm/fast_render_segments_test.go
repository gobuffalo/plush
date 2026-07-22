package vm

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

type vmSegmentUser struct {
	Name string
}

func Test_VM_Render_Fast_Segments_Mixed_Kinds(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"user": vmSegmentUser{Name: "<Bender>"},
		"upper": func(value string) string {
			return strings.ToUpper(value)
		},
		"items": []string{"a", "b"},
		"partialFeeder": func(string) (string, error) {
			return `<em><%= name %></em>`, nil
		},
	})
	plan := &compiler.FastRenderPlan{Bindings: []string{"name", "user", "upper", "items"}}
	bindings := newFastRenderBindings(plan, ctx)
	segments := []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentStatic, Value: "S:"},
		{Kind: compiler.FastRenderSegmentName, NameIndex: 0, Value: "name", Line: 1},
		{Kind: compiler.FastRenderSegmentStatic, Value: "|P:"},
		{Kind: compiler.FastRenderSegmentProperty, NameIndex: 1, Value: "user", Property: "Name", Receiver: "user", Full: "user.Name", Line: 1},
		{Kind: compiler.FastRenderSegmentStatic, Value: "|V:"},
		{Kind: compiler.FastRenderSegmentValue, ValuePlan: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "<value>"}, Line: 1},
		{Kind: compiler.FastRenderSegmentStatic, Value: "|C:"},
		{Kind: compiler.FastRenderSegmentCall, Call: &compiler.FastCallPlan{
			Name:      "upper",
			NameIndex: 2,
			Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueString, Value: "go"}},
			Line:      1,
		}},
		{Kind: compiler.FastRenderSegmentStatic, Value: "|IF:"},
		{Kind: compiler.FastRenderSegmentConditional, Conditional: &compiler.FastConditionalPlan{
			Branches: []compiler.FastConditionalBranch{
				{
					Condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
					Segments:  []compiler.FastRenderSegment{{Kind: compiler.FastRenderSegmentStatic, Value: "yes"}},
					Line:      1,
				},
			},
		}},
		{Kind: compiler.FastRenderSegmentStatic, Value: "|L:"},
		{Kind: compiler.FastRenderSegmentLoop, Loop: &compiler.FastLoopPlan{
			IterableName:      "items",
			IterableNameIndex: 3,
			Line:              1,
			Parts: []compiler.FastLoopPart{
				{Kind: compiler.FastLoopPartKey},
				{Kind: compiler.FastLoopPartStatic, Value: "="},
				{Kind: compiler.FastLoopPartValue},
				{Kind: compiler.FastLoopPartStatic, Value: ";"},
			},
		}},
		{Kind: compiler.FastRenderSegmentStatic, Value: "|PART:"},
		{Kind: compiler.FastRenderSegmentPartial, Partial: &compiler.FastPartialPlan{Name: "row.plush", Line: 1}},
	}

	var out strings.Builder
	ok, err := renderFastSegments(&out, ctx, bindings, segments)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "S:Mido|P:&lt;Bender&gt;|V:&lt;value&gt;|C:GO|IF:yes|L:0=a;1=b;|PART:<em>Mido</em>", out.String())
}

func Test_VM_Render_Fast_Segments_Missing_And_Default_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"name": "Mido"})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name"}}, ctx)

	var out strings.Builder
	ok, err := renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentName, NameIndex: 99, Value: "missing", NullOnMissing: true, Line: 1},
		{Kind: compiler.FastRenderSegmentStatic, Value: "after"},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "after", out.String())

	_, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentName, NameIndex: 99, Value: "missing", Line: 1},
	})
	require.ErrorContains(t, err, `"missing": unknown identifier`)

	ok, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{{Kind: 255}})
	require.NoError(t, err)
	require.False(t, ok)
}

func Test_VM_Render_Fast_Mixed_Plan_All_Optimized_Op_Kinds(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"user": vmAccessUser{Profile: &vmAccessProfile{Name: "<Bender>"}},
		"upper": func(value string) string {
			return strings.ToUpper(value)
		},
		"items": []string{"a", "b"},
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name", "unsupported", "user", "upper", "items"}}, ctx)
	var out strings.Builder

	ok, err := renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{
		{
			kind:      fastMixedOpAccessChain,
			prefix:    " access=",
			valuePlan: *vmAccessProfileNamePlan(2),
			line:      12,
		},
		{
			kind: fastMixedOpCall,
			call: &compiler.FastCallPlan{
				Name:      "upper",
				NameIndex: 3,
				Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueString, Value: "go"}},
				Line:      13,
			},
		},
		{
			kind: fastMixedOpConditional,
			conditional: &compiler.FastConditionalPlan{
				Branches: []compiler.FastConditionalBranch{{
					Condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
					Segments:  []compiler.FastRenderSegment{{Kind: compiler.FastRenderSegmentStatic, Value: " yes"}},
					Line:      14,
				}},
			},
		},
		{
			kind:    fastMixedOpPartial,
			partial: &compiler.FastPartialPlan{Name: "edge_mixed_partial.plush", Line: 15},
		},
		{
			kind: fastMixedOpLoop,
			loop: &compiler.FastLoopPlan{
				IterableName:      "items",
				IterableNameIndex: 4,
				Parts: []compiler.FastLoopPart{
					{Kind: compiler.FastLoopPartValue},
				},
				Line: 16,
			},
		},
	}})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, ` access=&lt;Bender&gt;GO yes<span>Mido</span>ab`, out.String())
}

func Test_VM_Render_Fast_Mixed_Plan_Error_And_Fallback_Edges(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name":   "Mido",
		"user":   vmAccessUser{Profile: &vmAccessProfile{Name: "<Bender>"}},
		"hidden": vmFastPropertyUser{hidden: "secret"},
		"scalar": 12,
		"partialFeeder": func(string) (string, error) {
			return "", fmt.Errorf("missing partial")
		},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name", "user", "hidden", "scalar"}}, ctx)
	var out strings.Builder

	_, err := renderFastMixedPlan(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind:      fastMixedOpProperty,
		nameIndex: 1,
		value:     "user",
		property:  "Profile",
		line:      21,
	}}})
	require.ErrorContains(t, err, "line 21")

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpValue,
		valuePlan: compiler.FastValuePlan{
			Kind:     compiler.FastValueInfix,
			Operator: "??",
			Left:     &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 1},
			Right:    &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 2},
			Line:     22,
		},
		line: 22,
	}}})
	require.ErrorContains(t, err, "line 22")

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpValue,
		valuePlan: compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: 99,
			Value:     "missing",
			Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Name", Line: 23}},
		},
		line: 23,
	}}})
	require.ErrorContains(t, err, "line 23")

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpAccessChain,
		valuePlan: compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: 99,
			Value:     "missing",
			Path:      vmAccessProfileNamePlan(99).Path,
		},
		line: 24,
	}}})
	require.ErrorContains(t, err, "line 24")

	out.Reset()
	ok, err := renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind:      fastMixedOpAccessChain,
		valuePlan: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "fallback"},
		line:      25,
	}}})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "fallback", out.String())

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpCall,
		call: &compiler.FastCallPlan{Name: "missing", NameIndex: 99, Line: 26},
	}}})
	require.ErrorContains(t, err, "line 26")

	ok, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpConditional,
		simpleCond: &fastSimpleConditionalPlan{
			branches: []fastSimpleConditionalBranch{{
				condition: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"}},
				segments:  &fastSimplePlan{ops: []fastSimpleOp{{op: &fastMixedOp{kind: fastMixedOpStatic, prefix: "nope"}}}},
				line:      27,
			}},
		},
	}}})
	require.NoError(t, err)
	require.True(t, ok)

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind:    fastMixedOpPartial,
		partial: &compiler.FastPartialPlan{Name: "edge_mixed_missing_partial.plush", Line: 28},
	}}})
	require.ErrorContains(t, err, "line 28")

	ok, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{
		kind: fastMixedOpLoop,
		loop: &compiler.FastLoopPlan{IterableName: "scalar", IterableNameIndex: 3, Line: 29},
	}}})
	require.NoError(t, err)
	require.False(t, ok)
}

func Test_VM_Render_Fast_Segments_And_Mixed_Edge_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name":        "Mido",
		"unsupported": &object.Builtin{},
		"user":        vmSegmentUser{Name: "Bender"},
		"scalar":      3,
		"partialFeeder": func(string) (string, error) {
			return "", fmt.Errorf("partial boom")
		},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name", "unsupported", "user", "scalar"}}, ctx)
	var out strings.Builder

	ok, err := renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentName, NameIndex: 1, Value: "unsupported", Line: 1},
	})
	require.NoError(t, err)
	require.False(t, ok)

	_, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentProperty, NameIndex: 99, Value: "missing", Property: "Name", Line: 2},
	})
	require.ErrorContains(t, err, "line 2")

	_, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentProperty, NameIndex: 2, Value: "user", Property: "Missing", Receiver: "user", Full: "user.Missing", Line: 3},
	})
	require.ErrorContains(t, err, "line 3")

	_, err = renderFastSegments(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentProperty, NameIndex: 2, Value: "user", Property: "Name", Receiver: "user", Full: "user.Name", Line: 31},
	})
	require.ErrorContains(t, err, "line 31")

	out.Reset()
	ok, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{
			Kind: compiler.FastRenderSegmentValue,
			ValuePlan: compiler.FastValuePlan{
				Kind:     compiler.FastValueInfix,
				Operator: "==",
				Left:     &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "x"},
				Right:    &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "x"},
				Line:     32,
			},
			Line: 32,
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "true", out.String())

	_, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{
			Kind: compiler.FastRenderSegmentValue,
			ValuePlan: compiler.FastValuePlan{
				Kind:     compiler.FastValueInfix,
				Operator: "??",
				Left:     &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "x"},
				Right:    &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "y"},
				Line:     33,
			},
			Line: 33,
		},
	})
	require.ErrorContains(t, err, "line 33")

	_, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{
			Kind: compiler.FastRenderSegmentValue,
			ValuePlan: compiler.FastValuePlan{
				Kind:      compiler.FastValuePath,
				NameIndex: 99,
				Value:     "missingPath",
				Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Name", Line: 34}},
			},
			Line: 34,
		},
	})
	require.ErrorContains(t, err, "line 34")

	_, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentValue, ValuePlan: compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"}, Line: 4},
	})
	require.ErrorContains(t, err, "line 4")

	_, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{
			Kind: compiler.FastRenderSegmentValue,
			ValuePlan: compiler.FastValuePlan{
				Kind: compiler.FastValueCall,
				Call: &compiler.FastCallPlan{Name: "scalar", NameIndex: 3, Line: 35},
			},
			Line: 35,
		},
	})
	require.ErrorContains(t, err, "line 35")

	_, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentCall, Call: &compiler.FastCallPlan{Name: "missing", NameIndex: 99, Line: 36}},
	})
	require.ErrorContains(t, err, "line 36")

	ok, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentConditional},
	})
	require.NoError(t, err)
	require.False(t, ok)

	_, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentPartial, Partial: &compiler.FastPartialPlan{Name: "missing.plush", Line: 37}},
	})
	require.ErrorContains(t, err, "line 37")

	ok, err = renderFastSegments(&out, ctx, bindings, []compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentLoop, Loop: &compiler.FastLoopPlan{IterableName: "scalar", IterableNameIndex: 3, Line: 5}},
	})
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = renderFastMixedPlan(&out, ctx, bindings, nil)
	require.NoError(t, err)
	require.False(t, ok)

	out.Reset()
	ok, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{
		{kind: fastMixedOpStatic, prefix: "prefix"},
		{kind: fastMixedOpName, nameIndex: 99, value: "missing", nullOnMissing: true, line: 6},
		{kind: fastMixedOpName, nameIndex: 0, value: "name", line: 6},
	}})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "prefixMido", out.String())

	ok, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{
		{kind: fastMixedOpName, nameIndex: 1, value: "unsupported", line: 7},
	}})
	require.NoError(t, err)
	require.False(t, ok)

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{
		{kind: fastMixedOpName, nameIndex: 99, value: "missing", line: 8},
	}})
	require.ErrorContains(t, err, "line 8")

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{
		{kind: fastMixedOpProperty, nameIndex: 99, value: "missing", property: "Name", line: 9},
	}})
	require.ErrorContains(t, err, "line 9")

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{
		{kind: fastMixedOpProperty, nameIndex: 2, value: "user", property: "Missing", receiver: "user", full: "user.Missing", line: 10},
	}})
	require.ErrorContains(t, err, "line 10")

	_, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{
		{kind: fastMixedOpValue, valuePlan: compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing"}, line: 11},
	}})
	require.ErrorContains(t, err, "line 11")

	ok, err = renderFastMixedPlan(&out, ctx, bindings, &fastMixedPlan{ops: []fastMixedOp{{kind: fastMixedOpKind(255)}}})
	require.NoError(t, err)
	require.False(t, ok)
}

func Test_VM_Render_Fast_Data_Partial_Slow_Branches(t *testing.T) {
	var out strings.Builder
	handled, err := renderFastDataPartialInto(&out, &compiler.FastPartialPlan{Name: "row.plush", Line: 7}, nil, fastRenderBindings{}, nil)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 7: invalid context")

	ctx := plush.NewContext()
	handled, err = renderFastDataPartialInto(&out, &compiler.FastPartialPlan{Name: "row.plush", Line: 7}, ctx, fastRenderBindings{}, nil)
	require.True(t, handled)
	require.ErrorContains(t, err, "could not find partial feeder")

	ctx = plush.NewContextWith(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})
	out.Reset()
	partial := &compiler.FastPartialPlan{
		Name: "row.html",
		Data: []compiler.FastPartialDataPair{
			{Key: "name", Value: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "Mido"}, Line: 7},
		},
		Line: 7,
	}
	handled, err = renderFastDataPartialInto(&out, partial, ctx, fastRenderBindings{}, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, template.JSEscapeString("<span>Mido</span>"), out.String())

	ctx = plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return "", fmt.Errorf("missing")
		},
	})
	out.Reset()
	handled, err = renderFastDataPartialInto(&out, partial, ctx, fastRenderBindings{}, nil)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 7: missing")
}

func Test_VM_Fast_Iterable_Len_And_Grow_Size_Branches(t *testing.T) {
	length, ok := fastIterableLen(&object.Array{Elements: []object.Object{&object.String{Value: "a"}}})
	require.True(t, ok)
	require.Equal(t, 1, length)

	length, ok = fastIterableLen(&object.Hash{Pairs: map[object.HashKey]object.HashPair{
		(&object.String{Value: "k"}).HashKey(): {Key: &object.String{Value: "k"}, Value: &object.String{Value: "v"}},
	}})
	require.True(t, ok)
	require.Equal(t, 1, length)

	length, ok = fastIterableLen([]interface{}{"a", "b"})
	require.True(t, ok)
	require.Equal(t, 2, length)

	length, ok = fastIterableLen([]string{"a", "b", "c"})
	require.True(t, ok)
	require.Equal(t, 3, length)

	length, ok = fastIterableLen([]object.Object{&object.String{Value: "a"}, &object.String{Value: "b"}})
	require.True(t, ok)
	require.Equal(t, 2, length)

	length, ok = fastIterableLen([2]string{"a", "b"})
	require.True(t, ok)
	require.Equal(t, 2, length)

	length, ok = fastIterableLen(map[string]int{"a": 1})
	require.True(t, ok)
	require.Equal(t, 1, length)

	length, ok = fastIterableLen(nil)
	require.True(t, ok)
	require.Zero(t, length)

	length, ok = fastIterableLen(&object.String{Value: "scalar"})
	require.False(t, ok)
	require.Zero(t, length)

	length, ok = fastIterableLen(struct{}{})
	require.False(t, ok)
	require.Zero(t, length)

	require.Zero(t, fastLoopConditionalGrowSize(nil))
	require.Equal(t, 48, fastLoopConditionalGrowSize(&compiler.FastLoopConditionalPlan{
		Branches: []compiler.FastLoopConditionalBranch{{
			Parts: []compiler.FastLoopPart{
				{Kind: compiler.FastLoopPartStatic, Value: "ignored"},
				{Kind: compiler.FastLoopPartKey},
				{Kind: compiler.FastLoopPartValue},
				{Kind: compiler.FastLoopPartConditional, Conditional: &compiler.FastLoopConditionalPlan{
					ElseParts: []compiler.FastLoopPart{{Kind: compiler.FastLoopPartKey}},
				}},
			},
		}},
		ElseParts: []compiler.FastLoopPart{
			{Kind: compiler.FastLoopPartValueProperty},
			{Kind: compiler.FastLoopPartValuePath},
			{Kind: compiler.FastLoopPartCall},
		},
	}))
	require.Zero(t, fastLoopPartsGrowSize([]compiler.FastLoopPart{{Kind: compiler.FastLoopPartKind(255)}}))

	ctx := plush.NewContextWith(map[string]interface{}{"items": []string{"a", "b"}})
	plan := &compiler.FastRenderPlan{Bindings: []string{"items"}}
	bindings := newFastRenderBindings(plan, ctx)
	size := fastOutputGrowSize(&fastMixedPlan{
		staticSize: 3,
		nameCount:  1,
		ops: []fastMixedOp{{
			loop: &compiler.FastLoopPlan{
				IterableNameIndex: 0,
				StaticSize:        1,
				Parts: []compiler.FastLoopPart{
					{Kind: compiler.FastLoopPartKey},
					{Kind: compiler.FastLoopPartValue},
				},
			},
		}},
	}, bindings)
	require.Positive(t, size)
	require.Zero(t, fastOutputGrowSize(nil, bindings))
}

func Test_VM_Fast_Mixed_Plan_Builder_Branches(t *testing.T) {
	require.Nil(t, prepareFastMixedPlan(nil))

	cachedPlan := &compiler.FastRenderPlan{}
	cachedMixed := &fastMixedPlan{staticSize: 7}
	cachedPlan.Prepared.Store(cachedMixed)
	require.Same(t, cachedMixed, prepareFastMixedPlan(cachedPlan))

	require.Nil(t, buildFastSimplePlanFromSegments(nil))
	require.Empty(t, fastMixedOpsFromSegments(nil))

	ops := fastMixedOpsFromSegments([]compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentStatic, Value: "prefix"},
		{Kind: compiler.FastRenderSegmentName, NameIndex: 0, Value: "name"},
		{Kind: compiler.FastRenderSegmentProperty, NameIndex: 1, Value: "user", Property: "Name"},
		{Kind: compiler.FastRenderSegmentValue, ValuePlan: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "value"}},
		{Kind: compiler.FastRenderSegmentValue, ValuePlan: *vmAccessProfileNamePlan(2)},
		{Kind: compiler.FastRenderSegmentCall, Call: &compiler.FastCallPlan{Name: "helper"}},
		{Kind: compiler.FastRenderSegmentConditional, Conditional: &compiler.FastConditionalPlan{
			Branches: []compiler.FastConditionalBranch{{Condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true}}},
		}},
		{Kind: compiler.FastRenderSegmentPartial, Partial: &compiler.FastPartialPlan{Name: "row.plush"}},
		{Kind: compiler.FastRenderSegmentLoop, Loop: &compiler.FastLoopPlan{IterableName: "items"}},
		{Kind: compiler.FastRenderSegmentKind(255)},
	})
	require.Len(t, ops, 9)
	require.Equal(t, fastMixedOpName, ops[0].kind)
	require.Equal(t, "prefix", ops[0].prefix)
	require.Equal(t, fastMixedOpProperty, ops[1].kind)
	require.Equal(t, fastMixedOpValue, ops[2].kind)
	require.Equal(t, fastMixedOpAccessChain, ops[3].kind)
	require.Equal(t, fastMixedOpCall, ops[4].kind)
	require.Equal(t, fastMixedOpConditional, ops[5].kind)
	require.NotNil(t, ops[5].simpleCond)
	require.Equal(t, fastMixedOpPartial, ops[6].kind)
	require.Equal(t, fastMixedOpLoop, ops[7].kind)
	require.Equal(t, fastMixedOpStatic, ops[8].kind)

	staticOps := fastMixedOpsFromSegments([]compiler.FastRenderSegment{{Kind: compiler.FastRenderSegmentStatic, Value: "only"}})
	require.Len(t, staticOps, 1)
	require.Equal(t, fastMixedOpStatic, staticOps[0].kind)
	require.Equal(t, "only", staticOps[0].prefix)

	simple := buildFastSimplePlanFromSegments([]compiler.FastRenderSegment{
		{Kind: compiler.FastRenderSegmentName, NameIndex: 0, Value: "name"},
		{Kind: compiler.FastRenderSegmentProperty, NameIndex: 1, Value: "user", Property: "Name"},
	})
	require.NotNil(t, simple)
	require.Equal(t, []int{0, 1}, simple.nameIndexes)

	binder := &fastSimplePlan{}
	require.Nil(t, buildFastSimpleValuePlan(nil, &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "x"}))
	require.Nil(t, buildFastSimpleValuePlan(binder, nil))
	require.Nil(t, buildFastSimpleValuePlan(binder, &compiler.FastValuePlan{Kind: compiler.FastValueInfix, Left: &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 1}}))
	require.Nil(t, buildFastSimpleValuePlan(binder, &compiler.FastValuePlan{Kind: compiler.FastValueCall}))
	require.Nil(t, buildFastSimpleValuePlan(binder, &compiler.FastValuePlan{
		Kind: compiler.FastValueCall,
		Call: &compiler.FastCallPlan{NameIndex: 0, Args: []compiler.FastValuePlan{{Kind: compiler.FastValueKind(255)}}},
	}))
	require.Nil(t, buildFastSimpleValuePlan(binder, &compiler.FastValuePlan{Kind: compiler.FastValueKind(255)}))

	binder = &fastSimplePlan{}
	value := buildFastSimpleValuePlan(binder, &compiler.FastValuePlan{
		Kind:  compiler.FastValueCall,
		Call:  &compiler.FastCallPlan{NameIndex: 3, Args: []compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 4, Value: "arg"}}},
		Value: "helper",
	})
	require.NotNil(t, value)
	require.Equal(t, 0, value.lookupIndex)
	require.Len(t, value.args, 1)
	require.Equal(t, []int{3, 4}, binder.nameIndexes)
}
