package code

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Make(t *testing.T) {
	tests := []struct {
		op       Opcode
		operands []int
		expected []byte
	}{
		{OpConstant, []int{65534}, []byte{byte(OpConstant), 255, 254}},
		{OpAdd, []int{}, []byte{byte(OpAdd)}},
		{OpGetLocal, []int{255}, []byte{byte(OpGetLocal), 255}},
		{OpClosure, []int{65534, 255}, []byte{byte(OpClosure), 255, 254, 255}},
		{OpFor, []int{65534, 65533, 65532, 255}, []byte{byte(OpFor), 255, 254, 255, 253, 255, 252, 255}},
		{OpCallBlock, []int{7, 65534, 1}, []byte{byte(OpCallBlock), 7, 255, 254, 1}},
		{OpWriteNameCall, []int{65534, 2}, []byte{byte(OpWriteNameCall), 255, 254, 2}},
	}

	for _, tt := range tests {
		instruction := Make(tt.op, tt.operands...)
		require.Equal(t, tt.expected, []byte(instruction))
	}
}

func Test_Instructions_String(t *testing.T) {
	instructions := []Instructions{
		Make(OpAdd),
		Make(OpGetLocal, 1),
		Make(OpConstant, 2),
		Make(OpConstant, 65535),
		Make(OpClosure, 65535, 255),
		Make(OpFor, 10, 20, 30, 1),
	}

	concatted := Instructions{}
	for _, ins := range instructions {
		concatted = append(concatted, ins...)
	}

	expected := `0000 OpAdd
0001 OpGetLocal 1
0003 OpConstant 2
0006 OpConstant 65535
0009 OpClosure 65535 255
0013 OpFor 10 20 30 1
`

	require.Equal(t, expected, concatted.String())
}

func Test_Read_Operands(t *testing.T) {
	tests := []struct {
		op            Opcode
		operands      []int
		bytesRead     int
		expectedWidth int
	}{
		{OpConstant, []int{65535}, 2, 2},
		{OpGetLocal, []int{255}, 1, 1},
		{OpClosure, []int{65535, 255}, 3, 3},
		{OpFor, []int{1, 2, 3, 4}, 7, 7},
		{OpCallBlock, []int{1, 65535, 2}, 4, 4},
		{OpWriteNameCall, []int{65535, 3}, 3, 3},
	}

	for _, tt := range tests {
		instruction := Make(tt.op, tt.operands...)
		def, err := Lookup(byte(tt.op))
		require.NoError(t, err)

		operandsRead, n := ReadOperands(def, instruction[1:])
		require.Equal(t, tt.bytesRead, n)
		require.Equal(t, tt.expectedWidth, n)
		require.Equal(t, tt.operands, operandsRead)
	}
}

func Test_Lookup_Unknown_Opcode(t *testing.T) {
	_, err := Lookup(255)
	require.Error(t, err)
	require.Contains(t, err.Error(), "opcode 255 undefined")
}
