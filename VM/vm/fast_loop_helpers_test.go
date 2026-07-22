package vm

import (
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

func Test_VM_Render_Fast_Loop_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"items":   []interface{}{"<a>", "b"},
		"objects": []object.Object{&object.String{Value: "<obj>"}},
		"map":     map[string]string{"one": "<1>"},
		"users":   &[]vmFastPropertyUser{{Name: "<mido>"}},
		"scalar":  3,
		"strings": []string{"<s>"},
		"nilPtr":  (*[]string)(nil),
		"array":   [1]string{"<array>"},
		"badList": []interface{}{struct{}{}},
		"badObjs": []object.Object{&object.Native{Value: struct{}{}}},
		"badMap":  map[string]struct{}{"bad": {}},
		"badInts": []int{1},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"items", "objects", "map", "users", "missing", "scalar", "strings", "nilPtr", "array", "badList", "badObjs", "badMap", "badInts"}}, ctx)

	loop := &compiler.FastLoopPlan{
		IterableName:      "items",
		IterableNameIndex: 0,
		Line:              1,
		Parts: []compiler.FastLoopPart{
			{Kind: compiler.FastLoopPartKey},
			{Kind: compiler.FastLoopPartStatic, Value: ":"},
			{Kind: compiler.FastLoopPartValue},
			{Kind: compiler.FastLoopPartStatic, Value: ";"},
		},
	}
	var out strings.Builder
	handled, err := renderFastLoop(&out, ctx, bindings, loop)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "0:&lt;a&gt;;1:b;", out.String())

	out.Reset()
	loop.IterableName = "objects"
	loop.IterableNameIndex = 1
	handled, err = renderFastLoop(&out, ctx, bindings, loop)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "0:&lt;obj&gt;;", out.String())

	out.Reset()
	loop.IterableName = "map"
	loop.IterableNameIndex = 2
	handled, err = renderFastLoop(&out, ctx, bindings, loop)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "one:&lt;1&gt;;", out.String())

	out.Reset()
	loop.IterableName = "strings"
	loop.IterableNameIndex = 6
	handled, err = renderFastLoop(&out, ctx, bindings, loop)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "0:&lt;s&gt;;", out.String())

	out.Reset()
	loop.IterableName = "users"
	loop.IterableNameIndex = 3
	loop.Parts = []compiler.FastLoopPart{{Kind: compiler.FastLoopPartValueProperty, Value: "Name", Receiver: "user", Full: "user.Name", Line: 1}}
	handled, err = renderFastLoop(&out, ctx, bindings, loop)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "&lt;mido&gt;", out.String())

	out.Reset()
	loop.IterableName = "nilPtr"
	loop.IterableNameIndex = 7
	handled, err = renderFastLoop(&out, ctx, bindings, loop)
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, out.String())

	out.Reset()
	loop.IterableName = "array"
	loop.IterableNameIndex = 8
	loop.Parts = []compiler.FastLoopPart{
		{Kind: compiler.FastLoopPartKey},
		{Kind: compiler.FastLoopPartStatic, Value: ":"},
		{Kind: compiler.FastLoopPartValue},
	}
	handled, err = renderFastLoop(&out, ctx, bindings, loop)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "0:&lt;array&gt;", out.String())

	loop.Parts = []compiler.FastLoopPart{{Kind: compiler.FastLoopPartValueProperty, Value: "Missing", Receiver: "item", Full: "item.Missing", Line: 4}}
	loop.IterableName = "badList"
	loop.IterableNameIndex = 9
	_, err = renderFastLoop(&out, ctx, bindings, loop)
	require.ErrorContains(t, err, "line 4")

	loop.IterableName = "badObjs"
	loop.IterableNameIndex = 10
	_, err = renderFastLoop(&out, ctx, bindings, loop)
	require.ErrorContains(t, err, "line 4")

	loop.IterableName = "badInts"
	loop.IterableNameIndex = 12
	_, err = renderFastLoop(&out, ctx, bindings, loop)
	require.ErrorContains(t, err, "line 4")

	loop.IterableName = "badMap"
	loop.IterableNameIndex = 11
	_, err = renderFastLoop(&out, ctx, bindings, loop)
	require.ErrorContains(t, err, "line 4")

	loop.IterableName = "strings"
	loop.IterableNameIndex = 6
	_, err = renderFastLoop(&out, ctx, bindings, loop)
	require.ErrorContains(t, err, "line 4")

	out.Reset()
	loop.Parts = []compiler.FastLoopPart{{Kind: compiler.FastLoopPartValuePath, ValuePlan: compiler.FastValuePlan{Kind: 255}}}
	loop.IterableName = "items"
	loop.IterableNameIndex = 0
	handled, err = renderFastLoop(&out, ctx, bindings, loop)
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, out.String())

	out.Reset()
	loop.Parts = []compiler.FastLoopPart{{Kind: compiler.FastLoopPartKey}}

	out.Reset()
	loop.IterableName = "missing"
	loop.IterableNameIndex = 4
	_, err = renderFastLoop(&out, ctx, bindings, loop)
	require.ErrorContains(t, err, `"missing": unknown identifier`)

	handled, err = renderFastLoop(&out, ctx, bindings, nil)
	require.NoError(t, err)
	require.False(t, handled)

	ctx.Set("items", nil)
	bindings = newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"items", "scalar"}}, ctx)
	handled, err = renderFastLoop(&out, ctx, bindings, &compiler.FastLoopPlan{IterableName: "items", IterableNameIndex: 0, Line: 1})
	require.NoError(t, err)
	require.True(t, handled)

	nilBindings := fastRenderBindings{names: []string{"items"}, localOK: []bool{true}, localVals: []interface{}{nil}}
	handled, err = renderFastLoop(&out, ctx, nilBindings, &compiler.FastLoopPlan{IterableName: "items", IterableNameIndex: 0, Line: 1})
	require.NoError(t, err)
	require.True(t, handled)

	handled, err = renderFastLoop(&out, ctx, bindings, &compiler.FastLoopPlan{IterableName: "scalar", IterableNameIndex: 1, Line: 1})
	require.NoError(t, err)
	require.False(t, handled)

	err = renderFastLoopIteration(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, &compiler.FastLoopPlan{
		Line: 9,
		Parts: []compiler.FastLoopPart{
			{Kind: compiler.FastLoopPartStatic, Value: "never"},
		},
	}, 0, "value")
	require.ErrorContains(t, err, "line 9")
}

func Test_VM_Render_Fast_Loop_Parts_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"label": func(value string, key int) string {
			return value + ":" + object.Wrap(key).Inspect()
		},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
	user := vmFastPropertyUser{Name: "<mido>"}
	loop := &compiler.FastLoopPlan{Line: 1}
	parts := []compiler.FastLoopPart{
		{Kind: compiler.FastLoopPartValuePath, ValuePlan: compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: -1,
			Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Name", Receiver: "user", Full: "user.Name", Line: 1}},
			Line:      1,
		}},
		{Kind: compiler.FastLoopPartStatic, Value: "|"},
		{Kind: compiler.FastLoopPartCall, Call: &compiler.FastCallPlan{
			Name:      "label",
			NameIndex: 0,
			Args: []compiler.FastValuePlan{
				{
					Kind:      compiler.FastValuePath,
					NameIndex: -1,
					Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Name", Receiver: "user", Full: "user.Name", Line: 1}},
					Line:      1,
				},
				{Kind: compiler.FastValueLoopKey, Value: "i", Line: 1},
			},
			Line: 1,
		}},
		{Kind: compiler.FastLoopPartStatic, Value: "|"},
		{Kind: compiler.FastLoopPartConditional, Conditional: &compiler.FastLoopConditionalPlan{
			Branches: []compiler.FastLoopConditionalBranch{
				{
					Condition: compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: false, Line: 1},
					Parts:     []compiler.FastLoopPart{{Kind: compiler.FastLoopPartStatic, Value: "no"}},
					Line:      1,
				},
			},
			ElseParts: []compiler.FastLoopPart{{Kind: compiler.FastLoopPartStatic, Value: "else"}},
		}},
	}

	var out strings.Builder
	require.NoError(t, renderFastLoopParts(&out, ctx, bindings, loop, parts, 3, user))
	require.Equal(t, "&lt;mido&gt;|&lt;mido&gt;:3|else", out.String())

	out.Reset()
	require.NoError(t, renderFastLoopParts(&out, ctx, bindings, loop, []compiler.FastLoopPart{{Kind: 255}}, 0, user))
	require.Empty(t, out.String())

	require.NoError(t, renderFastLoopParts(&out, ctx, bindings, loop, []compiler.FastLoopPart{{
		Kind: compiler.FastLoopPartValuePath,
		ValuePlan: compiler.FastValuePlan{
			Kind:          compiler.FastValueName,
			NameIndex:     99,
			Value:         "missing",
			NullOnMissing: true,
			Line:          3,
		},
	}}, 0, user))
	require.Empty(t, out.String())

	err := renderFastLoopParts(&out, ctx, bindings, loop, []compiler.FastLoopPart{{
		Kind:     compiler.FastLoopPartValueProperty,
		Value:    "Missing",
		Receiver: "user",
		Full:     "user.Missing",
		Line:     6,
	}}, 0, user)
	require.ErrorContains(t, err, "line 6")

	err = renderFastLoopParts(&out, ctx, bindings, loop, []compiler.FastLoopPart{{
		Kind: compiler.FastLoopPartCall,
		Call: &compiler.FastCallPlan{Name: "missing", NameIndex: 99, Line: 8},
	}}, 0, user)
	require.ErrorContains(t, err, "line 8")

	_, err = renderFastLoop(&out, ctx, bindings, &compiler.FastLoopPlan{IterableName: "label", IterableNameIndex: 99, Line: 7})
	require.ErrorContains(t, err, `line 7`)
}

func Test_VM_Fast_Struct_Loop_Static_Call_Arg_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"name": "Mido"})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name"}}, ctx)

	tests := []struct {
		name     string
		plan     *fastStructLoopCallArgPlan
		expected interface{}
		ok       bool
	}{
		{"nil", nil, nil, true},
		{"binding", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBinding, nameIndex: 0}, "Mido", true},
		{"nil literal", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgNil}, nil, true},
		{"string", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgString, stringVal: "x"}, "x", true},
		{"int", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgInt, intVal: 2}, 2, true},
		{"float", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgFloat, floatVal: 1.5}, 1.5, true},
		{"bool", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBool, boolVal: true}, true, true},
		{"unknown", &fastStructLoopCallArgPlan{kind: fastStructLoopCallArgKey}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := evalFastStructLoopStaticCallArgValue(tt.plan, bindings)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.expected, value)
		})
	}

	value, err := evalFastStructLoopStaticCallArgReflect(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgString, stringVal: "x"}, bindings, stringType, "helper", 0)
	require.NoError(t, err)
	require.Equal(t, "x", value.Interface())

	_, err = evalFastStructLoopStaticCallArgReflect(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBinding, nameIndex: 99, line: 4, value: compiler.FastValuePlan{Value: "missing"}}, bindings, stringType, "helper", 0)
	require.ErrorContains(t, err, `line 4`)
}
