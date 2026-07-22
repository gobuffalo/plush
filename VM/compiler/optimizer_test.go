package compiler

import (
	"html/template"
	"testing"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

func Test_Optimizer_Write_Replacement_Opcodes(t *testing.T) {
	constants := []object.Object{
		&object.String{Value: "text"},
		&object.Native{Value: template.HTML("<b>")},
		&object.Integer{Value: 3},
	}

	tests := []struct {
		name     string
		op       code.Opcode
		operands []int
		expected code.Instructions
		ok       bool
	}{
		{"missing operands", code.OpGetName, nil, nil, false},
		{"string constant", code.OpConstant, []int{0}, code.Make(code.OpWriteString, 0), true},
		{"html constant", code.OpConstant, []int{1}, code.Make(code.OpWriteHTML, 1), true},
		{"other constant", code.OpConstant, []int{2}, code.Make(code.OpWriteConstant, 2), true},
		{"bad constant index", code.OpConstant, []int{99}, code.Make(code.OpWriteConstant, 99), true},
		{"name", code.OpGetName, []int{4}, code.Make(code.OpWriteName, 4), true},
		{"name or null", code.OpGetNameOrNull, []int{5}, code.Make(code.OpWriteNameOrNull, 5), true},
		{"local", code.OpGetLocal, []int{6}, code.Make(code.OpWriteLocal, 6), true},
		{"global", code.OpGetGlobal, []int{7}, code.Make(code.OpWriteGlobal, 7), true},
		{"call", code.OpCall, []int{2}, code.Make(code.OpWriteCall, 2), true},
		{"unsupported", code.OpTrue, []int{1}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := writeReplacement(tt.op, tt.operands, constants)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.expected, got)
		})
	}
}

func Test_Optimize_Scope_Remaps_Jump_And_Metadata(t *testing.T) {
	constants := []object.Object{&object.String{Value: "text"}}
	instructions := code.Instructions{}
	instructions = append(instructions, code.Make(code.OpJump, 7)...)
	instructions = append(instructions, code.Make(code.OpConstant, 0)...)
	instructions = append(instructions, code.Make(code.OpWrite)...)
	instructions = append(instructions, code.Make(code.OpPop)...)

	optimized, callNames, lineNumbers, properties := optimizeScope(
		instructions,
		map[int]string{0: "jump", 3: "constant", 6: "removed"},
		map[int]int{0: 1, 3: 2, 6: 3},
		map[int]object.PropertyAccess{3: {Receiver: "x", Full: "x.y"}},
		constants,
	)

	def, err := code.Lookup(byte(code.OpJump))
	require.NoError(t, err)
	operands, _ := code.ReadOperands(def, optimized[1:])
	require.Equal(t, []int{6}, operands)
	require.True(t, instructionContainsOpcode(optimized, code.OpWriteString))
	require.False(t, instructionContainsOpcode(optimized, code.OpWrite))

	require.Equal(t, map[int]string{0: "jump", 3: "constant"}, callNames)
	require.Equal(t, map[int]int{0: 1, 3: 2}, lineNumbers)
	require.Equal(t, map[int]object.PropertyAccess{3: {Receiver: "x", Full: "x.y"}}, properties)
}

func Test_Optimize_Scope_Does_Not_Fuse_Shared_Branch_Write_Target(t *testing.T) {
	constants := []object.Object{
		&object.String{Value: "good"},
		&object.String{Value: "bad"},
	}
	instructions := code.Instructions{}
	instructions = append(instructions, code.Make(code.OpTrue)...)
	instructions = append(instructions, code.Make(code.OpJumpNotTruthy, 10)...)
	instructions = append(instructions, code.Make(code.OpConstant, 0)...)
	instructions = append(instructions, code.Make(code.OpJump, 13)...)
	instructions = append(instructions, code.Make(code.OpConstant, 1)...)
	instructions = append(instructions, code.Make(code.OpWrite)...)

	optimized, _, _, _ := optimizeScope(instructions, nil, nil, nil, constants)
	require.Equal(t, instructions, optimized)
}

func Test_Optimize_Scope_Noop_And_Empty_Remaps(t *testing.T) {
	instructions := code.Make(code.OpTrue)
	optimized, callNames, lineNumbers, properties := optimizeScope(instructions, nil, nil, nil, nil)
	require.Equal(t, code.Instructions(instructions), optimized)
	require.Nil(t, callNames)
	require.Nil(t, lineNumbers)
	require.Nil(t, properties)

	require.Nil(t, remapStringMap(nil, nil, nil))
	require.Nil(t, remapIntMap(nil, nil, nil))
	require.Nil(t, remapPropertyMap(nil, nil, nil))

	require.Nil(t, remapStringMap(map[int]string{1: "removed"}, nil, map[int]bool{1: true}))
	require.Nil(t, remapIntMap(map[int]int{1: 7}, nil, map[int]bool{1: true}))
	require.Nil(t, remapPropertyMap(map[int]object.PropertyAccess{1: {Receiver: "x"}}, nil, map[int]bool{1: true}))
}

func Test_Optimize_Scope_Remaps_Unknown_Opcode_And_Out_Of_Range_Jump(t *testing.T) {
	instructions := code.Instructions{255}
	instructions = append(instructions, code.Make(code.OpNull)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpJump, 999)...)

	optimized, _, _, _ := optimizeScope(instructions, nil, nil, nil, nil)

	require.Equal(t, byte(255), optimized[0])
	require.True(t, instructionContainsOpcode(optimized, code.OpPop))

	def, err := code.Lookup(byte(code.OpJump))
	require.NoError(t, err)
	jumpPos := 2
	operands, _ := code.ReadOperands(def, optimized[jumpPos+1:])
	require.Equal(t, []int{len(optimized)}, operands)
}

func Test_Optimize_Scope_Remaps_Jump_Target_Inside_Instruction(t *testing.T) {
	instructions := code.Instructions{}
	instructions = append(instructions, code.Make(code.OpNull)...)
	instructions = append(instructions, code.Make(code.OpPop)...)
	instructions = append(instructions, code.Make(code.OpJump, 4)...)

	optimized, _, _, _ := optimizeScope(instructions, nil, nil, nil, nil)

	def, err := code.Lookup(byte(code.OpJump))
	require.NoError(t, err)
	operands, _ := code.ReadOperands(def, optimized[2:])
	require.Equal(t, []int{3}, operands)
}

func Test_Static_Constant_Output_Helpers(t *testing.T) {
	constants := []object.Object{
		&object.String{Value: "<text>"},
		&object.Native{Value: template.HTML("<b>")},
		&object.Integer{Value: 7},
		&object.Float{Value: 1.5},
		object.TrueObject,
		object.NullObject,
		&object.Native{Value: "plain native"},
	}

	value, ok := stringConstantValue(constants, 0)
	require.True(t, ok)
	require.Equal(t, "<text>", value)
	_, ok = stringConstantValue(constants, 1)
	require.False(t, ok)
	_, ok = stringConstantValue(constants, 99)
	require.False(t, ok)

	value, ok = htmlConstantValue(constants, 1)
	require.True(t, ok)
	require.Equal(t, "<b>", value)
	_, ok = htmlConstantValue(constants, 0)
	require.False(t, ok)
	_, ok = htmlConstantValue(constants, 6)
	require.False(t, ok)
	_, ok = htmlConstantValue(constants, -1)
	require.False(t, ok)

	tests := []struct {
		index    int
		expected string
		ok       bool
	}{
		{0, "&lt;text&gt;", true},
		{1, "<b>", true},
		{2, "7", true},
		{3, "1.5", true},
		{4, "true", true},
		{5, "", true},
		{6, "", false},
		{99, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			value, ok := staticConstantOutput(constants, tt.index)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.expected, value)
		})
	}
}
