package vm

import (
	"errors"
	"html/template"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

func Test_VM_Write_Fast_Direct_String_Call_Segment_Variants(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"name": "<Mido>"})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name"}}, ctx)
	call := &compiler.FastCallPlan{
		Name: "helper",
		Args: []compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 0, Value: "name", Line: 3}},
		Line: 3,
	}

	tests := []struct {
		name     string
		raw      interface{}
		expected string
	}{
		{"string", func(value string) string { return value + "!" }, "&lt;Mido&gt;!"},
		{"string_error", func(value string) (string, error) { return value + "?", nil }, "&lt;Mido&gt;?"},
		{"html", func(value string) template.HTML { return template.HTML("<b>" + value + "</b>") }, "<b><Mido></b>"},
		{"html_error", func(value string) (template.HTML, error) { return template.HTML("<i>" + value + "</i>"), nil }, "<i><Mido></i>"},
		{"object", func(value string) object.Object { return &object.String{Value: value} }, "&lt;Mido&gt;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			handled, err := writeFastDirectStringCallSegment(&out, ctx, bindings, call, tt.raw)
			require.NoError(t, err)
			require.True(t, handled)
			require.Equal(t, tt.expected, out.String())
		})
	}

	handled, err := writeFastDirectStringCallSegment(&strings.Builder{}, ctx, bindings, call, func(string) (string, error) {
		return "", errors.New("boom")
	})
	require.True(t, handled)
	require.ErrorContains(t, err, "line 3: could not call helper function")

	handled, err = writeFastDirectStringCallSegment(&strings.Builder{}, ctx, bindings, call, func(int) string { return "" })
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = writeFastDirectStringCallSegment(nil, ctx, bindings, call, func(string) string { return "" })
	require.NoError(t, err)
	require.False(t, handled)

	var out strings.Builder
	handled, err = writeFastDirectStringCallSegment(&out, ctx, bindings, call, &object.Native{Value: func(value string) string { return value + " native" }})
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "&lt;Mido&gt; native", out.String())

	handled, err = writeFastDirectStringCallSegment(&strings.Builder{}, ctx, bindings, call, func(string) (template.HTML, error) {
		return "", errors.New("html boom")
	})
	require.True(t, handled)
	require.ErrorContains(t, err, "line 3: could not call helper function")

	missingArgCall := &compiler.FastCallPlan{
		Name: "helper",
		Args: []compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 11}},
		Line: 11,
	}
	handled, err = writeFastDirectStringCallSegment(&strings.Builder{}, ctx, bindings, missingArgCall, func(string) string { return "" })
	require.True(t, handled)
	require.ErrorContains(t, err, `line 11: "missing": unknown identifier`)

	countArgCall := &compiler.FastCallPlan{
		Name: "helper",
		Args: []compiler.FastValuePlan{{Kind: compiler.FastValueInteger, IntValue: 3, Line: 12}},
		Line: 12,
	}
	handled, err = writeFastDirectStringCallSegment(&strings.Builder{}, ctx, bindings, countArgCall, func(string) string { return "" })
	require.NoError(t, err)
	require.False(t, handled)
}

func Test_VM_Fast_Writer_Write_Go_Value_Edges(t *testing.T) {
	require.False(t, FastWriter{}.WriteGoValue("ignored"))

	var out strings.Builder
	writer := FastWriter{out: &out, ctx: plush.NewContext()}
	require.True(t, writer.WriteGoValue("<value>"))
	require.Equal(t, "&lt;value&gt;", out.String())
}

func Test_VM_Write_Fast_Call_Segment_Branches(t *testing.T) {
	var out strings.Builder
	ctx := plush.NewContextWith(map[string]interface{}{
		"registered": "raw",
		"direct":     func(value string) string { return value + "!" },
		"fallback":   func(value string) template.HTML { return template.HTML("<" + value + ">") },
		"bad":        "raw",
		"name":       "<Mido>",
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{
		Bindings: []string{"registered", "direct", "fallback", "bad", "name"},
	}, ctx)

	require.NoError(t, writeFastCallSegment(&out, ctx, bindings, nil))
	require.Empty(t, out.String())

	SetFastHelper(ctx, "registered", func(w FastWriter, args FastArgs) error {
		value, ok := args.String(0)
		require.True(t, ok)
		w.WriteEscapedString(value + "?")
		return nil
	})
	require.NoError(t, writeFastCallSegment(&out, ctx, bindings, &compiler.FastCallPlan{
		Name:      "registered",
		NameIndex: 0,
		Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 4, Value: "name", Line: 2}},
		Line:      2,
	}))
	require.Equal(t, "&lt;Mido&gt;?", out.String())

	SetFastHelper(ctx, "bad", func(FastWriter, FastArgs) error {
		return errors.New("bad helper")
	})
	err := writeFastCallSegment(&out, ctx, bindings, &compiler.FastCallPlan{
		Name:      "bad",
		NameIndex: 3,
		Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 4, Value: "name", Line: 3}},
		Line:      3,
	})
	require.ErrorContains(t, err, "line 3: bad helper")

	SetFastHelper(ctx, "direct", func(FastWriter, FastArgs) error {
		return ErrFastUnsupported
	})
	out.Reset()
	require.NoError(t, writeFastCallSegment(&out, ctx, bindings, &compiler.FastCallPlan{
		Name:      "direct",
		NameIndex: 1,
		Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 4, Value: "name", Line: 4}},
		Line:      4,
	}))
	require.Equal(t, "&lt;Mido&gt;!", out.String())

	out.Reset()
	require.NoError(t, writeFastCallSegment(&out, ctx, bindings, &compiler.FastCallPlan{
		Name:      "fallback",
		NameIndex: 2,
		Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueString, Value: "b", Line: 5}},
		Line:      5,
	}))
	require.Equal(t, "<b>", out.String())

	err = writeFastCallSegment(&out, ctx, bindings, &compiler.FastCallPlan{Name: "missing", NameIndex: 99, Line: 6})
	require.ErrorContains(t, err, `line 6: "missing": unknown identifier`)

	budgetCtx := plush.NewContextWith(map[string]interface{}{
		"direct": func(value string) string { return value },
		"name":   "Mido",
	}).WithBudget(plush.NewBudget(0))
	budgetBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"direct", "name"}}, budgetCtx)
	err = writeFastCallSegment(&out, budgetCtx, budgetBindings, &compiler.FastCallPlan{
		Name:      "direct",
		NameIndex: 0,
		Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 1, Value: "name", Line: 7}},
		Line:      7,
	})
	require.ErrorContains(t, err, "line 7")

	SetFastHelper(ctx, "registered", func(FastWriter, FastArgs) error {
		return nil
	})
	err = writeFastCallSegment(&out, ctx, bindings, &compiler.FastCallPlan{
		Name:      "registered",
		NameIndex: 0,
		Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 8}},
		Line:      8,
	})
	require.ErrorContains(t, err, `line 8: "missing": unknown identifier`)

	err = writeFastCallSegment(&out, ctx, bindings, &compiler.FastCallPlan{
		Name:      "bad",
		NameIndex: 3,
		Args: []compiler.FastValuePlan{
			{Kind: compiler.FastValueString, Value: "ok", Line: 9},
			{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 9},
		},
		Line: 9,
	})
	require.ErrorContains(t, err, `line 9: "missing": unknown identifier`)

	err = writeFastCallSegment(&out, ctx, bindings, &compiler.FastCallPlan{
		Name:      "fallback",
		NameIndex: 2,
		Args:      []compiler.FastValuePlan{{Kind: compiler.FastValueBool, BoolValue: true, Line: 10}},
		Line:      10,
	})
	require.ErrorContains(t, err, "line 10")
}

func Test_VM_Eval_Fast_Call_Args_Into_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"name": "Mido", "count": 3})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name", "count"}}, ctx)

	args, err := evalFastCallArgsInto([]compiler.FastValuePlan{
		{Kind: compiler.FastValueName, NameIndex: 0, Value: "name", Line: 1},
		{Kind: compiler.FastValueName, NameIndex: 1, Value: "count", Line: 1},
	}, ctx, bindings, nil)
	require.NoError(t, err)
	require.Equal(t, 2, args.Len())
	require.Equal(t, "Mido", args.Raw(0))
	require.Equal(t, 3, args.Raw(1))

	args, err = evalFastCallArgsInto(nil, ctx, bindings, &fastCallArgs{})
	require.NoError(t, err)
	require.Nil(t, args)

	_, err = evalFastCallArgsInto([]compiler.FastValuePlan{{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 5}}, ctx, bindings, nil)
	require.ErrorContains(t, err, `line 5: "missing": unknown identifier`)

	_, err = evalFastCallArgsInto([]compiler.FastValuePlan{{
		Kind: compiler.FastValueCall,
		Call: &compiler.FastCallPlan{Name: "name", NameIndex: 0, Line: 6},
		Line: 6,
	}}, plush.NewContextWith(map[string]interface{}{"name": func() string { return "blocked" }}).WithBudget(plush.NewBudget(0)), bindings, nil)
	require.ErrorContains(t, err, "line 6")
}

func Test_VM_Fast_Call_Args_Reset_And_String_Arg_Branches(t *testing.T) {
	var nilArgs *fastCallArgs
	nilArgs.Reset()
	require.Equal(t, 0, nilArgs.Len())
	require.Nil(t, nilArgs.Raw(0))
	require.Nil(t, nilArgs.Objects())

	args := &fastCallArgs{}
	for i := 0; i < len(args.inline)+2; i++ {
		args.Append(i)
	}
	require.Equal(t, len(args.inline)+2, args.Len())
	require.Equal(t, 2, len(args.extra))
	objects := args.Objects()
	require.NotNil(t, objects)
	require.Equal(t, objects, args.Objects())
	args.Reset()
	require.Zero(t, args.Len())
	require.Empty(t, args.extra)
	require.Nil(t, args.objects)

	ctx := plush.NewContextWith(map[string]interface{}{"name": "Mido", "count": 3})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name", "count"}}, ctx)
	value, ok, err := evalFastCallStringArg(&compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 0, Value: "name", Line: 8}, ctx, bindings)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "Mido", value)

	value, ok, err = evalFastCallStringArg(&compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 1, Value: "count", Line: 9}, ctx, bindings)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, value)

	_, _, err = evalFastCallStringArg(&compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 10}, ctx, bindings)
	require.ErrorContains(t, err, `line 10: "missing": unknown identifier`)

	_, _, err = evalFastCallStringArg(&compiler.FastValuePlan{
		Kind: compiler.FastValueCall,
		Call: &compiler.FastCallPlan{Name: "name", NameIndex: 0, Line: 11},
		Line: 11,
	}, plush.NewContextWith(map[string]interface{}{"name": func() string { return "blocked" }}).WithBudget(plush.NewBudget(0)), bindings)
	require.ErrorContains(t, err, "line 11")
}

func Test_VM_Try_Write_Registered_Fast_Helper(t *testing.T) {
	ctx := plush.NewContext()
	SetFastHelper(ctx, "fast", func(w FastWriter, args FastArgs) error {
		value, ok := args.String(0)
		require.True(t, ok)
		w.WriteEscapedString(value + "!")
		return nil
	})

	machine := newRuntimeHelperTestVM(ctx)
	machine.stack[0] = &object.Native{Value: func(string) string { return "" }}
	machine.stack[1] = &object.String{Value: "<name>"}
	machine.sp = 2
	handled, err := machine.tryWriteRegisteredFastHelper("fast", 1, true)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, 0, machine.sp)
	require.Equal(t, "&lt;name&gt;!", machine.currentFrame().output.String())

	handled, err = machine.tryWriteRegisteredFastHelper("missing", 0, false)
	require.NoError(t, err)
	require.False(t, handled)

	SetFastHelper(ctx, "fallback", func(FastWriter, FastArgs) error {
		return ErrFastUnsupported
	})
	machine.currentFrame().output.Reset()
	machine.sp = 0
	handled, err = machine.tryWriteRegisteredFastHelper("fallback", 0, false)
	require.NoError(t, err)
	require.False(t, handled)

	machine.frames[0] = nil
	handled, err = machine.tryWriteRegisteredFastHelper("fast", 0, false)
	require.NoError(t, err)
	require.False(t, handled)
}
