package code

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

type Instructions []byte

func (ins Instructions) String() string {
	var out bytes.Buffer

	i := 0
	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "ERROR: %s\n", err)
			i++
			continue
		}

		operands, read := ReadOperands(def, ins[i+1:])

		fmt.Fprintf(&out, "%04d %s\n", i, ins.fmtInstruction(def, operands))

		i += 1 + read
	}

	return out.String()
}

func (ins Instructions) fmtInstruction(def *Definition, operands []int) string {
	if len(operands) != len(def.OperandWidths) {
		return fmt.Sprintf("ERROR: operand len %d does not match defined %d\n",
			len(operands), len(def.OperandWidths))
	}

	if len(operands) == 0 {
		return def.Name
	}

	parts := make([]string, len(operands))
	for i, operand := range operands {
		parts[i] = fmt.Sprintf("%d", operand)
	}

	return fmt.Sprintf("%s %s", def.Name, strings.Join(parts, " "))
}

type Opcode byte

const (
	OpConstant Opcode = iota

	OpAdd

	OpPop

	OpSub
	OpMul
	OpDiv

	OpTrue
	OpFalse

	OpEqual
	OpNotEqual
	OpGreaterThan

	OpMinus
	OpBang

	OpJumpNotTruthy
	OpJump

	OpNull

	OpGetGlobal
	OpSetGlobal

	OpArray
	OpHash
	OpIndex

	OpCall

	OpReturnValue
	OpReturn

	OpGetLocal
	OpSetLocal

	OpGetBuiltin

	OpClosure

	OpGetFree

	OpCurrentClosure

	// Plush extensions. These are appended after the core VM opcodes so their
	// numeric values remain stable.
	OpGreaterEqual
	OpMatches
	OpAnd
	OpOr
	OpGetName
	OpSetName
	OpAssignName
	OpGetProperty
	OpSetIndex
	OpWrite
	OpFor
	OpBreak
	OpContinue
	OpCallBlock
	OpRenderTemplate
	OpGetNameOrNull
	OpHole

	// Direct write opcodes are Plush fast paths. They fuse common
	// "load or call, then OpWrite" instruction pairs so rendering can write to
	// the frame output without unnecessary stack traffic. See VM/FAST_PATHS.md.
	OpWriteConstant
	OpWriteName
	OpWriteNameOrNull
	OpWriteLocal
	OpWriteGlobal
	OpWriteString
	OpWriteHTML
	OpWriteLocalProperty
	OpWriteGlobalProperty
	OpWriteNameProperty
	OpWriteCall
	OpWriteNameCall
)

type Definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*Definition{
	OpConstant: {"OpConstant", []int{2}},

	OpAdd: {"OpAdd", []int{}},

	OpPop: {"OpPop", []int{}},

	OpSub: {"OpSub", []int{}},
	OpMul: {"OpMul", []int{}},
	OpDiv: {"OpDiv", []int{}},

	OpTrue:  {"OpTrue", []int{}},
	OpFalse: {"OpFalse", []int{}},

	OpEqual:       {"OpEqual", []int{}},
	OpNotEqual:    {"OpNotEqual", []int{}},
	OpGreaterThan: {"OpGreaterThan", []int{}},

	OpMinus: {"OpMinus", []int{}},
	OpBang:  {"OpBang", []int{}},

	OpJumpNotTruthy: {"OpJumpNotTruthy", []int{2}},
	OpJump:          {"OpJump", []int{2}},

	OpNull: {"OpNull", []int{}},

	OpGetGlobal: {"OpGetGlobal", []int{2}},
	OpSetGlobal: {"OpSetGlobal", []int{2}},

	OpArray: {"OpArray", []int{2}},
	OpHash:  {"OpHash", []int{2}},
	OpIndex: {"OpIndex", []int{}},

	OpCall: {"OpCall", []int{1}},

	OpReturnValue: {"OpReturnValue", []int{}},
	OpReturn:      {"OpReturn", []int{}},

	OpGetLocal: {"OpGetLocal", []int{1}},
	OpSetLocal: {"OpSetLocal", []int{1}},

	OpGetBuiltin: {"OpGetBuiltin", []int{1}},

	OpClosure: {"OpClosure", []int{2, 1}},

	OpGetFree: {"OpGetFree", []int{1}},

	OpCurrentClosure: {"OpCurrentClosure", []int{}},

	OpGreaterEqual:        {"OpGreaterEqual", []int{}},
	OpMatches:             {"OpMatches", []int{}},
	OpAnd:                 {"OpAnd", []int{}},
	OpOr:                  {"OpOr", []int{}},
	OpGetName:             {"OpGetName", []int{2}},
	OpSetName:             {"OpSetName", []int{2}},
	OpAssignName:          {"OpAssignName", []int{2}},
	OpGetProperty:         {"OpGetProperty", []int{2}},
	OpSetIndex:            {"OpSetIndex", []int{}},
	OpWrite:               {"OpWrite", []int{}},
	OpFor:                 {"OpFor", []int{2, 2, 2, 1}},
	OpBreak:               {"OpBreak", []int{}},
	OpContinue:            {"OpContinue", []int{}},
	OpCallBlock:           {"OpCallBlock", []int{1, 2, 1}},
	OpRenderTemplate:      {"OpRenderTemplate", []int{}},
	OpGetNameOrNull:       {"OpGetNameOrNull", []int{2}},
	OpHole:                {"OpHole", []int{2}},
	OpWriteConstant:       {"OpWriteConstant", []int{2}},
	OpWriteName:           {"OpWriteName", []int{2}},
	OpWriteNameOrNull:     {"OpWriteNameOrNull", []int{2}},
	OpWriteLocal:          {"OpWriteLocal", []int{1}},
	OpWriteGlobal:         {"OpWriteGlobal", []int{2}},
	OpWriteString:         {"OpWriteString", []int{2}},
	OpWriteHTML:           {"OpWriteHTML", []int{2}},
	OpWriteLocalProperty:  {"OpWriteLocalProperty", []int{1, 2}},
	OpWriteGlobalProperty: {"OpWriteGlobalProperty", []int{2, 2}},
	OpWriteNameProperty:   {"OpWriteNameProperty", []int{2, 2}},
	OpWriteCall:           {"OpWriteCall", []int{1}},
	OpWriteNameCall:       {"OpWriteNameCall", []int{2, 1}},
}

func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}

	return def, nil
}

func Make(op Opcode, operands ...int) []byte {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	instructionLen := 1
	for _, width := range def.OperandWidths {
		instructionLen += width
	}

	instruction := make([]byte, instructionLen)
	instruction[0] = byte(op)

	offset := 1
	for i, operand := range operands {
		width := def.OperandWidths[i]
		switch width {
		case 2:
			binary.BigEndian.PutUint16(instruction[offset:], uint16(operand))
		case 1:
			instruction[offset] = byte(operand)
		}
		offset += width
	}

	return instruction
}

func ReadOperands(def *Definition, ins Instructions) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, width := range def.OperandWidths {
		switch width {
		case 2:
			operands[i] = int(ReadUint16(ins[offset:]))
		case 1:
			operands[i] = int(ReadUint8(ins[offset:]))
		}

		offset += width
	}

	return operands, offset
}

func ReadUint8(ins Instructions) uint8 {
	return uint8(ins[0])
}

func ReadUint16(ins Instructions) uint16 {
	return binary.BigEndian.Uint16(ins)
}
