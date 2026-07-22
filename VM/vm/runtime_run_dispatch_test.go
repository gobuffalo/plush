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

func Test_VM_Run_Dispatch_Core_Opcodes(t *testing.T) {
	array := &object.Array{Elements: []object.Object{&object.Integer{Value: 1}}}
	constants := []object.Object{
		&object.Integer{Value: 1},
		&object.Integer{Value: 2},
		&object.Integer{Value: 5},
		&object.Integer{Value: 3},
		&object.Integer{Value: 4},
		&object.String{Value: "abc"},
		&object.String{Value: "a.c"},
		&object.String{Value: "k"},
		&object.Integer{Value: 7},
		array,
		&object.Integer{Value: 0},
		&object.Integer{Value: 42},
	}

	instructions := code.Instructions{}
	instructions = append(instructions, code.Make(code.OpConstant, 0)...)
	instructions = append(instructions, code.Make(code.OpConstant, 1)...)
	instructions = append(instructions, code.Make(code.OpAdd)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 2)...)
	instructions = append(instructions, code.Make(code.OpConstant, 3)...)
	instructions = append(instructions, code.Make(code.OpSub)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 1)...)
	instructions = append(instructions, code.Make(code.OpConstant, 4)...)
	instructions = append(instructions, code.Make(code.OpMul)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 4)...)
	instructions = append(instructions, code.Make(code.OpConstant, 1)...)
	instructions = append(instructions, code.Make(code.OpDiv)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpTrue)...)
	instructions = append(instructions, code.Make(code.OpFalse)...)
	instructions = append(instructions, code.Make(code.OpEqual)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpTrue)...)
	instructions = append(instructions, code.Make(code.OpFalse)...)
	instructions = append(instructions, code.Make(code.OpNotEqual)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 1)...)
	instructions = append(instructions, code.Make(code.OpConstant, 0)...)
	instructions = append(instructions, code.Make(code.OpGreaterThan)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 0)...)
	instructions = append(instructions, code.Make(code.OpConstant, 0)...)
	instructions = append(instructions, code.Make(code.OpGreaterEqual)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 5)...)
	instructions = append(instructions, code.Make(code.OpConstant, 6)...)
	instructions = append(instructions, code.Make(code.OpMatches)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpTrue)...)
	instructions = append(instructions, code.Make(code.OpFalse)...)
	instructions = append(instructions, code.Make(code.OpAnd)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpFalse)...)
	instructions = append(instructions, code.Make(code.OpTrue)...)
	instructions = append(instructions, code.Make(code.OpOr)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpTrue)...)
	instructions = append(instructions, code.Make(code.OpBang)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 0)...)
	instructions = append(instructions, code.Make(code.OpMinus)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpNull)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 0)...)
	instructions = append(instructions, code.Make(code.OpSetGlobal, 0)...)
	instructions = append(instructions, code.Make(code.OpGetGlobal, 0)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 0)...)
	instructions = append(instructions, code.Make(code.OpConstant, 1)...)
	instructions = append(instructions, code.Make(code.OpArray, 2)...)
	instructions = append(instructions, code.Make(code.OpConstant, 0)...)
	instructions = append(instructions, code.Make(code.OpIndex)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 7)...)
	instructions = append(instructions, code.Make(code.OpConstant, 8)...)
	instructions = append(instructions, code.Make(code.OpHash, 2)...)
	instructions = append(instructions, code.Make(code.OpConstant, 7)...)
	instructions = append(instructions, code.Make(code.OpIndex)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpConstant, 9)...)
	instructions = append(instructions, code.Make(code.OpConstant, 10)...)
	instructions = append(instructions, code.Make(code.OpConstant, 11)...)
	instructions = append(instructions, code.Make(code.OpSetIndex)...)
	instructions = append(instructions, code.Make(code.OpPop)...)

	machine := NewWithContext(&compiler.Bytecode{
		Instructions: instructions,
		Constants:    constants,
		NumGlobals:   1,
	}, plush.NewContext())

	require.NoError(t, machine.Run())
	require.Equal(t, &object.Integer{Value: 42}, array.Elements[0])
	require.Zero(t, machine.sp)
}

func Test_VM_Compile_Compiler_Error_Branch(t *testing.T) {
	_, err := templateFromBytecode(compileProgramBytecode(vmCoverageBadProgram()))
	require.ErrorContains(t, err, "unknown operator ??")
}

func Test_VM_Run_Dispatch_Jump_Write_And_Name_Opcodes(t *testing.T) {
	constants := []object.Object{
		&object.String{Value: "<skip>"},
		&object.String{Value: "name"},
		&object.String{Value: "missing"},
		&object.String{Value: "<text>"},
		&object.Native{Value: template.HTML("<raw>")},
		&object.String{Value: "user"},
		&object.String{Value: "Prop"},
		&object.String{Value: "updated"},
		&object.String{Value: "assigned"},
		&object.String{Value: "local"},
		&object.Native{Value: vmHelperStruct{Prop: "local-prop"}},
		&object.String{Value: "global"},
		&object.Native{Value: vmHelperStruct{Prop: "global-prop"}},
		&object.String{Value: `<%= name %>`},
		&object.String{Value: "<constant>"},
	}
	instructions := code.Instructions{}
	instructions = append(instructions, code.Make(code.OpJump, 6)...)
	instructions = append(instructions, code.Make(code.OpWriteString, 0)...)
	instructions = append(instructions, code.Make(code.OpFalse)...)
	instructions = append(instructions, code.Make(code.OpJumpNotTruthy, 13)...)
	instructions = append(instructions, code.Make(code.OpWriteString, 0)...)
	instructions = append(instructions, code.Make(code.OpWriteName, 1)...)
	instructions = append(instructions, code.Make(code.OpWriteNameOrNull, 2)...)
	instructions = append(instructions, code.Make(code.OpWriteString, 3)...)
	instructions = append(instructions, code.Make(code.OpWriteHTML, 4)...)
	instructions = append(instructions, code.Make(code.OpWriteConstant, 14)...)
	instructions = append(instructions, code.Make(code.OpGetName, 1)...)
	instructions = append(instructions, code.Make(code.OpWrite)...)
	instructions = append(instructions, code.Make(code.OpGetNameOrNull, 2)...)
	instructions = append(instructions, code.Make(code.OpWrite)...)
	instructions = append(instructions, code.Make(code.OpGetName, 5)...)
	instructions = append(instructions, code.Make(code.OpGetProperty, 6)...)
	instructions = append(instructions, code.Make(code.OpWrite)...)
	instructions = append(instructions, code.Make(code.OpWriteNameProperty, 5, 6)...)
	instructions = append(instructions, code.Make(code.OpConstant, 7)...)
	instructions = append(instructions, code.Make(code.OpSetName, 1)...)
	instructions = append(instructions, code.Make(code.OpWriteName, 1)...)
	instructions = append(instructions, code.Make(code.OpConstant, 8)...)
	instructions = append(instructions, code.Make(code.OpAssignName, 1)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpWriteName, 1)...)
	instructions = append(instructions, code.Make(code.OpConstant, 9)...)
	instructions = append(instructions, code.Make(code.OpSetLocal, 0)...)
	instructions = append(instructions, code.Make(code.OpWriteLocal, 0)...)
	instructions = append(instructions, code.Make(code.OpConstant, 10)...)
	instructions = append(instructions, code.Make(code.OpSetLocal, 1)...)
	instructions = append(instructions, code.Make(code.OpWriteLocalProperty, 1, 6)...)
	instructions = append(instructions, code.Make(code.OpConstant, 11)...)
	instructions = append(instructions, code.Make(code.OpSetGlobal, 0)...)
	instructions = append(instructions, code.Make(code.OpWriteGlobal, 0)...)
	instructions = append(instructions, code.Make(code.OpConstant, 12)...)
	instructions = append(instructions, code.Make(code.OpSetGlobal, 1)...)
	instructions = append(instructions, code.Make(code.OpWriteGlobalProperty, 1, 6)...)
	instructions = append(instructions, code.Make(code.OpConstant, 13)...)
	instructions = append(instructions, code.Make(code.OpRenderTemplate)...)
	instructions = append(instructions, code.Make(code.OpWrite)...)

	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"user": vmHelperStruct{Prop: "user-prop"},
	})
	machine := NewWithContext(&compiler.Bytecode{
		Instructions: instructions,
		Constants:    constants,
		NumLocals:    2,
		NumGlobals:   2,
	}, ctx)

	require.NoError(t, machine.Run())
	require.Equal(t, "Mido&lt;text&gt;<raw>&lt;constant&gt;Midouser-propuser-propupdatedassignedlocallocal-propglobalglobal-propassigned", machine.Rendered())
}

func Test_VM_Run_Dispatch_Builtin_Context_Override(t *testing.T) {
	lenIndex := -1
	for i, definition := range object.Builtins {
		if definition.Name == "len" {
			lenIndex = i
			break
		}
	}
	require.GreaterOrEqual(t, lenIndex, 0)

	overrideInstructions := code.Instructions{}
	overrideInstructions = append(overrideInstructions, code.Make(code.OpGetBuiltin, lenIndex)...)
	overrideInstructions = append(overrideInstructions, code.Make(code.OpPop)...)

	defaultInstructions := code.Instructions{}
	defaultInstructions = append(defaultInstructions, code.Make(code.OpGetBuiltin, lenIndex)...)
	defaultInstructions = append(defaultInstructions, code.Make(code.OpArray, 0)...)
	defaultInstructions = append(defaultInstructions, code.Make(code.OpCall, 1)...)
	defaultInstructions = append(defaultInstructions, code.Make(code.OpPop)...)

	overrideMachine := NewWithContext(&compiler.Bytecode{Instructions: overrideInstructions}, plush.NewContextWith(map[string]interface{}{
		"len": "override",
	}))
	require.NoError(t, overrideMachine.Run())
	require.Equal(t, &object.String{Value: "override"}, overrideMachine.LastPoppedStackElem())

	defaultMachine := NewWithContext(&compiler.Bytecode{Instructions: defaultInstructions}, plush.NewContext())
	require.NoError(t, defaultMachine.Run())
	require.Equal(t, &object.Integer{Value: 0}, defaultMachine.LastPoppedStackElem())
}

func newVMDispatchAssignmentBudgetContext() *plush.Context {
	costs := plush.ZeroCosts()
	costs.Assignment = 1
	return plush.NewContext().WithBudget(plush.NewBudgetWithCosts(0, costs))
}

func Test_VM_Run_Dispatch_Error_Branches(t *testing.T) {
	tests := []struct {
		name         string
		constants    []object.Object
		instructions code.Instructions
		ctx          *plush.Context
		numLocals    int
		numGlobals   int
		expected     string
	}{
		{
			name:         "binary_operation",
			instructions: append(append(code.Make(code.OpNull), code.Make(code.OpNull)...), code.Make(code.OpAdd)...),
			expected:     "unsupported types",
		},
		{
			name: "comparison",
			constants: []object.Object{
				&object.Integer{Value: 1},
				&object.String{Value: "one"},
			},
			instructions: append(append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1)...,
			), code.Make(code.OpEqual)...),
			expected: "unable to operate",
		},
		{
			name: "minus",
			constants: []object.Object{
				&object.String{Value: "bad"},
			},
			instructions: append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpMinus)...,
			),
			expected: "unsupported type for negation",
		},
		{
			name: "condition_budget",
			ctx:  plush.NewContext().WithBudget(plush.NewBudget(0)),
			instructions: append(append(
				code.Make(code.OpFalse),
				code.Make(code.OpJumpNotTruthy, 4)...,
			), code.Make(code.OpNull)...),
			expected: "budget",
		},
		{
			name: "assignment_budget",
			ctx:  newVMDispatchAssignmentBudgetContext(),
			constants: []object.Object{
				&object.Integer{Value: 1},
			},
			instructions: append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0)...,
			),
			numGlobals: 1,
			expected:   "budget",
		},
		{
			name: "hash_key",
			constants: []object.Object{
				&object.Array{},
				&object.Integer{Value: 1},
			},
			instructions: append(append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1)...,
			), code.Make(code.OpHash, 2)...),
			expected: "unusable as hash key",
		},
		{
			name: "index",
			constants: []object.Object{
				&object.Integer{Value: 1},
				&object.Integer{Value: 0},
			},
			instructions: append(append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1)...,
			), code.Make(code.OpIndex)...),
			expected: "could not index",
		},
		{
			name: "set_index",
			constants: []object.Object{
				&object.Integer{Value: 1},
				&object.Integer{Value: 0},
				&object.Integer{Value: 9},
			},
			instructions: append(append(append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1)...,
			), code.Make(code.OpConstant, 2)...), code.Make(code.OpSetIndex)...),
			expected: "could not index",
		},
		{
			name: "call",
			constants: []object.Object{
				&object.Integer{Value: 1},
			},
			instructions: append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpCall, 0)...,
			),
			expected: "invalid function",
		},
		{
			name: "write_call",
			constants: []object.Object{
				&object.Integer{Value: 1},
			},
			instructions: append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpWriteCall, 0)...,
			),
			expected: "invalid function",
		},
		{
			name: "write_name_call",
			constants: []object.Object{
				&object.String{Value: "missing"},
			},
			instructions: code.Make(code.OpWriteNameCall, 0, 0),
			expected:     `"missing": unknown identifier`,
		},
		{
			name: "call_block",
			constants: []object.Object{
				&object.Integer{Value: 1},
			},
			instructions: code.Make(code.OpCallBlock, 0, 0, 0),
			expected:     "not a function",
		},
		{
			name: "closure",
			constants: []object.Object{
				&object.Integer{Value: 1},
			},
			instructions: code.Make(code.OpClosure, 0, 0),
			expected:     "not a function",
		},
		{
			name: "get_name",
			constants: []object.Object{
				&object.String{Value: "missing"},
			},
			instructions: code.Make(code.OpGetName, 0),
			expected:     `"missing": unknown identifier`,
		},
		{
			name: "assign_name",
			constants: []object.Object{
				&object.String{Value: "missing"},
				&object.Integer{Value: 1},
			},
			instructions: append(
				code.Make(code.OpConstant, 1),
				code.Make(code.OpAssignName, 0)...,
			),
			expected: `"missing": unknown identifier`,
		},
		{
			name: "get_property",
			constants: []object.Object{
				&object.Integer{Value: 1},
				&object.String{Value: "Prop"},
			},
			instructions: append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpGetProperty, 1)...,
			),
			expected: "Prop",
		},
		{
			name: "write_name",
			constants: []object.Object{
				&object.String{Value: "missing"},
			},
			instructions: code.Make(code.OpWriteName, 0),
			expected:     `"missing": unknown identifier`,
		},
		{
			name: "write_local_property",
			constants: []object.Object{
				&object.Integer{Value: 1},
				&object.String{Value: "Prop"},
			},
			instructions: append(append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetLocal, 0)...,
			), code.Make(code.OpWriteLocalProperty, 0, 1)...),
			numLocals: 1,
			expected:  "Prop",
		},
		{
			name: "write_global_property",
			constants: []object.Object{
				&object.Integer{Value: 1},
				&object.String{Value: "Prop"},
			},
			instructions: append(append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0)...,
			), code.Make(code.OpWriteGlobalProperty, 0, 1)...),
			numGlobals: 1,
			expected:   "Prop",
		},
		{
			name: "write_name_property",
			ctx: plush.NewContextWith(map[string]interface{}{
				"user": 1,
			}),
			constants: []object.Object{
				&object.String{Value: "user"},
				&object.String{Value: "Prop"},
			},
			instructions: code.Make(code.OpWriteNameProperty, 0, 1),
			expected:     "Prop",
		},
		{
			name: "render_template",
			constants: []object.Object{
				&object.String{Value: `<%= missing %>`},
			},
			instructions: append(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpRenderTemplate)...,
			),
			expected: "missing",
		},
		{
			name: "for_block",
			constants: []object.Object{
				&object.Integer{Value: 1},
				&object.String{Value: "k"},
				&object.String{Value: "v"},
			},
			instructions: code.Make(code.OpFor, 0, 1, 2, 0),
			expected:     "not a function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx
			if ctx == nil {
				ctx = plush.NewContext()
			}
			machine := NewWithContext(&compiler.Bytecode{
				Instructions: tt.instructions,
				Constants:    tt.constants,
				NumLocals:    tt.numLocals,
				NumGlobals:   tt.numGlobals,
			}, ctx)
			require.ErrorContains(t, machine.Run(), tt.expected)
		})
	}
}

func Test_VM_Run_Dispatch_Stack_Overflow_Error_Branches(t *testing.T) {
	tests := []struct {
		name         string
		constants    []object.Object
		instructions code.Instructions
		numGlobals   int
	}{
		{
			name: "constant",
			constants: []object.Object{
				&object.Integer{Value: 1},
			},
			instructions: code.Make(code.OpConstant, 0),
		},
		{
			name:         "true",
			instructions: code.Make(code.OpTrue),
		},
		{
			name:         "false",
			instructions: code.Make(code.OpFalse),
		},
		{
			name:         "null",
			instructions: code.Make(code.OpNull),
		},
		{
			name:         "global",
			instructions: code.Make(code.OpGetGlobal, 0),
			numGlobals:   1,
		},
		{
			name:         "name_or_null",
			constants:    []object.Object{&object.String{Value: "missing"}},
			instructions: code.Make(code.OpGetNameOrNull, 0),
		},
		{
			name:         "array",
			instructions: code.Make(code.OpArray, 0),
		},
		{
			name:         "hash",
			instructions: code.Make(code.OpHash, 0),
		},
		{
			name: "closure",
			constants: []object.Object{
				&object.CompiledFunction{},
			},
			instructions: code.Make(code.OpClosure, 0, 0),
		},
		{
			name:         "current_closure",
			instructions: code.Make(code.OpCurrentClosure),
		},
		{
			name: "builtin",
			instructions: code.Make(code.OpGetBuiltin, func() int {
				for i, definition := range object.Builtins {
					if definition.Name == "len" {
						return i
					}
				}
				return 0
			}()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := NewWithContext(&compiler.Bytecode{
				Instructions: tt.instructions,
				Constants:    tt.constants,
				NumGlobals:   tt.numGlobals,
			}, plush.NewContext())
			machine.sp = StackSize
			require.EqualError(t, machine.Run(), "stack overflow")
		})
	}
}

func Test_VM_Run_Dispatch_Builtin_Default_Stack_Overflow_Branch(t *testing.T) {
	lenIndex := -1
	for i, definition := range object.Builtins {
		if definition.Name == "len" {
			lenIndex = i
			break
		}
	}
	require.GreaterOrEqual(t, lenIndex, 0)

	machine := New(&compiler.Bytecode{
		Instructions: code.Make(code.OpGetBuiltin, lenIndex),
	})
	machine.ctx = nil
	machine.sp = StackSize
	require.EqualError(t, machine.Run(), "stack overflow")
}

func Test_VM_Run_Dispatch_Get_Free_Stack_Overflow_Branch(t *testing.T) {
	machine := NewWithContext(&compiler.Bytecode{
		Instructions: code.Make(code.OpGetFree, 0),
	}, plush.NewContext())
	machine.currentFrame().cl.Free = []object.Object{&object.String{Value: "free"}}
	machine.sp = StackSize
	require.EqualError(t, machine.Run(), "stack overflow")
}

func Test_VM_Run_Dispatch_Call_Block_Execute_Error_Branch(t *testing.T) {
	instructions := append(
		code.Make(code.OpConstant, 0),
		code.Make(code.OpCallBlock, 0, 1, 0)...,
	)
	machine := NewWithContext(&compiler.Bytecode{
		Instructions: instructions,
		Constants: []object.Object{
			&object.Integer{Value: 1},
			&object.CompiledFunction{Instructions: code.Make(code.OpReturn)},
		},
	}, plush.NewContext())
	require.ErrorContains(t, machine.Run(), "invalid function")
}

func Test_VM_Run_Dispatch_Child_Frame_Return_Overflow_Branches(t *testing.T) {
	tests := []struct {
		name         string
		instructions code.Instructions
	}{
		{name: "return_value", instructions: code.Make(code.OpReturnValue)},
		{name: "return", instructions: code.Make(code.OpReturn)},
		{name: "break", instructions: code.Make(code.OpBreak)},
		{name: "continue", instructions: code.Make(code.OpContinue)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := NewFrame(&object.Closure{Fn: &object.CompiledFunction{Instructions: code.Make(code.OpNull)}}, 0)
			child := NewFrame(&object.Closure{Fn: &object.CompiledFunction{Instructions: tt.instructions}}, StackSize)
			machine := &VM{
				stack:       make([]object.Object, StackSize),
				sp:          StackSize,
				frames:      make([]*Frame, MaxFrames),
				framesIndex: 2,
				ctx:         plush.NewContext(),
			}
			machine.stack[StackSize-1] = &object.String{Value: "return"}
			machine.frames[0] = parent
			machine.frames[1] = child
			require.EqualError(t, machine.Run(), "stack overflow")
		})
	}
}

func Test_VM_Run_Dispatch_Set_And_Get_Local_Error_Branches(t *testing.T) {
	setMachine := NewWithContext(&compiler.Bytecode{
		Instructions: append(
			code.Make(code.OpConstant, 0),
			code.Make(code.OpSetLocal, 0)...,
		),
		Constants: []object.Object{&object.String{Value: "value"}},
		NumLocals: 1,
	}, newVMDispatchAssignmentBudgetContext())
	require.ErrorContains(t, setMachine.Run(), "budget")

	getMachine := NewWithContext(&compiler.Bytecode{
		Instructions: code.Make(code.OpGetLocal, 0),
		NumLocals:    1,
	}, plush.NewContext())
	getMachine.stack[0] = &object.String{Value: "value"}
	getMachine.sp = StackSize
	require.EqualError(t, getMachine.Run(), "stack overflow")
}

func Test_VM_Run_Dispatch_For_Execute_Error_Branch(t *testing.T) {
	instructions := append(
		code.Make(code.OpConstant, 0),
		code.Make(code.OpFor, 1, 2, 3, 0)...,
	)
	machine := NewWithContext(&compiler.Bytecode{
		Instructions: instructions,
		Constants: []object.Object{
			&object.Integer{Value: 7},
			&object.CompiledFunction{Instructions: code.Make(code.OpReturn)},
			&object.String{Value: "i"},
			&object.String{Value: "item"},
		},
	}, plush.NewContext())
	require.ErrorContains(t, machine.Run(), "could not iterate")
}
