package vm

import (
	"html/template"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

func newWriteNameCallTestVM(ctx *plush.Context, names ...string) *VM {
	constants := make([]object.Object, len(names))
	for i, name := range names {
		constants[i] = &object.String{Value: name}
	}
	return NewWithContext(&compiler.Bytecode{
		Constants:    constants,
		Instructions: code.Make(code.OpNull),
	}, ctx)
}

func Test_VM_Execute_Write_Name_Call_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"raw": func() string {
			return "<raw>"
		},
		"native": &object.Native{Value: func() string {
			return "<native>"
		}},
		"len": object.GetBuiltinByName("len"),
		"closure": &object.Closure{Fn: &object.CompiledFunction{
			Instructions: code.Instructions(append(
				code.Make(code.OpConstant, 4),
				code.Make(code.OpReturnValue)...,
			)),
			NumParameters: 0,
			NumLocals:     0,
		}},
		"bad": 7,
		"fast": func() string {
			return "<slow>"
		},
		"fastNative": &object.Native{Value: func() string {
			return "<slow-native>"
		}},
	})
	SetFastHelper(ctx, "fast", func(w FastWriter, args FastArgs) error {
		w.WriteEscapedString("<fast>")
		return nil
	})
	SetFastHelper(ctx, "fastNative", func(w FastWriter, args FastArgs) error {
		w.WriteEscapedString("<fast-native>")
		return nil
	})
	machine := newWriteNameCallTestVM(ctx, "raw", "native", "len", "closure", "<closure>", "missing", "bad", "fast", "fastNative")

	var rawSlot object.InlineCacheSlot
	require.NoError(t, machine.executeWriteNameCall(0, 0, &rawSlot))
	require.Equal(t, "&lt;raw&gt;", machine.currentFrame().output.String())

	machine.currentFrame().output.Reset()
	var nativeSlot object.InlineCacheSlot
	require.NoError(t, machine.executeWriteNameCall(1, 0, &nativeSlot))
	require.Equal(t, "&lt;native&gt;", machine.currentFrame().output.String())

	machine.currentFrame().output.Reset()
	require.NoError(t, machine.push(&object.Array{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}}))
	require.NoError(t, machine.executeWriteNameCall(2, 1, nil))
	require.Equal(t, "2", machine.currentFrame().output.String())

	machine.currentFrame().output.Reset()
	require.NoError(t, machine.executeWriteNameCall(3, 0, nil))
	require.Equal(t, 2, machine.framesIndex)
	require.NoError(t, machine.Run())
	require.Equal(t, "&lt;closure&gt;", machine.currentFrame().output.String())

	machine.currentFrame().output.Reset()
	require.NoError(t, machine.executeWriteNameCall(7, 0, nil))
	require.Equal(t, "&lt;fast&gt;", machine.currentFrame().output.String())

	machine.currentFrame().output.Reset()
	require.NoError(t, machine.executeWriteNameCall(8, 0, nil))
	require.Equal(t, "&lt;fast-native&gt;", machine.currentFrame().output.String())

	require.ErrorContains(t, machine.executeWriteNameCall(5, 0, nil), `"missing": unknown identifier`)
	require.ErrorContains(t, machine.executeWriteNameCall(6, 0, nil), "invalid function")

	budgetCtx := plush.NewContextWith(map[string]interface{}{"raw": func() string { return "raw" }}).WithBudget(plush.NewBudget(0))
	budgetMachine := newWriteNameCallTestVM(budgetCtx, "raw")
	require.ErrorContains(t, budgetMachine.executeWriteNameCall(0, 0, nil), "budget")
}

func Test_VM_Execute_Write_Call_Branches(t *testing.T) {
	ctx := plush.NewContext()
	machine := newRuntimeHelperTestVM(ctx)

	machine.stack[0] = object.GetBuiltinByName("len")
	machine.stack[1] = &object.Array{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}}
	machine.sp = 2
	require.NoError(t, machine.executeWriteCall("len", 1, nil))
	require.Equal(t, "2", machine.currentFrame().output.String())

	SetFastHelper(ctx, "fast", func(w FastWriter, args FastArgs) error {
		w.WriteEscapedString("<fast>")
		return nil
	})
	machine.currentFrame().output.Reset()
	machine.stack[0] = &object.Native{Value: func() string { return "<slow>" }}
	machine.sp = 1
	require.NoError(t, machine.executeWriteCall("fast", 0, nil))
	require.Equal(t, "&lt;fast&gt;", machine.currentFrame().output.String())

	machine.currentFrame().output.Reset()
	machine.stack[0] = &object.Integer{Value: 7}
	machine.sp = 1
	require.ErrorContains(t, machine.executeWriteCall("bad", 0, nil), "invalid function")
}

func Test_VM_Execute_Write_Name_Call_Direct_Literal_Partial(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"partial": vmPartialHelper,
		"partialFeeder": func(name string) (string, error) {
			require.Equal(t, "row.plush", name)
			return `<span>ok</span>`, nil
		},
	})
	machine := newWriteNameCallTestVM(ctx, "partial")

	require.NoError(t, machine.push(&object.String{Value: "row.plush"}))
	require.NoError(t, machine.executeWriteNameCall(0, 1, nil))
	require.Equal(t, `<span>ok</span>`, machine.currentFrame().output.String())
	require.True(t, machine.currentFrame().hasOutput)
	require.Equal(t, 0, machine.sp)
}

func Test_VM_Execute_Write_Name_Call_Direct_Literal_Partial_Respects_Custom_Helper(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"partial": func(string) template.HTML {
			return template.HTML("<custom>")
		},
		"partialFeeder": func(string) (string, error) {
			require.FailNow(t, "custom partial helper should bypass direct partial rendering")
			return "", nil
		},
	})
	machine := newWriteNameCallTestVM(ctx, "partial")

	require.NoError(t, machine.push(&object.String{Value: "row.plush"}))
	require.NoError(t, machine.executeWriteNameCall(0, 1, nil))
	require.Equal(t, `<custom>`, machine.currentFrame().output.String())
	require.True(t, machine.currentFrame().hasOutput)
	require.Equal(t, 0, machine.sp)
}

func Test_VM_Execute_Write_Name_Call_Direct_Literal_Partial_Skips_When_Frame_Has_Locals(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"partial": vmPartialHelper,
		"partialFeeder": func(name string) (string, error) {
			require.Equal(t, "fallback_skip.plush", name)
			return `<span>fallback</span>`, nil
		},
	})
	machine := newWriteNameCallTestVM(ctx, "partial")
	machine.currentFrame().cl.Fn.NumLocals = 1

	require.NoError(t, machine.push(&object.String{Value: "fallback_skip.plush"}))
	require.NoError(t, machine.executeWriteNameCall(0, 1, nil))
	require.Equal(t, `<span>fallback</span>`, machine.currentFrame().output.String())
	require.True(t, machine.currentFrame().hasOutput)
	require.Equal(t, 0, machine.sp)
}
