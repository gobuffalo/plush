package compiler

import (
	"html/template"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
)

// optimizeScope performs bytecode peephole rewrites for render hot paths, such
// as replacing OpGetName+OpWrite with OpWriteName. It also remaps jumps and
// metadata so errors, call names, and property access stay attached to the
// correct instruction after bytes are removed.
func optimizeScope(
	instructions code.Instructions,
	callNames map[int]string,
	lineNumbers map[int]int,
	properties map[int]object.PropertyAccess,
	constants []object.Object,
) (code.Instructions, map[int]string, map[int]int, map[int]object.PropertyAccess) {
	type instruction struct {
		oldPos   int
		op       code.Opcode
		operands []int
		width    int
	}

	parsed := []instruction{}
	for i := 0; i < len(instructions); {
		op := code.Opcode(instructions[i])
		def, err := code.Lookup(byte(op))
		if err != nil {
			parsed = append(parsed, instruction{oldPos: i, op: op, width: 1})
			i++
			continue
		}
		operands, read := code.ReadOperands(def, instructions[i+1:])
		parsed = append(parsed, instruction{oldPos: i, op: op, operands: operands, width: 1 + read})
		i += 1 + read
	}

	jumpTargets := map[int]bool{}
	for _, ins := range parsed {
		switch ins.op {
		case code.OpJump, code.OpJumpNotTruthy:
			if len(ins.operands) > 0 {
				jumpTargets[ins.operands[0]] = true
			}
		}
	}

	remove := map[int]bool{}
	replace := map[int]code.Instructions{}
	removedTarget := map[int]int{}
	for i := 0; i < len(parsed)-1; i++ {
		current := parsed[i]
		next := parsed[i+1]
		if current.op == code.OpNull && next.op == code.OpPop {
			replace[current.oldPos] = code.Make(code.OpPop)
			remove[next.oldPos] = true
			removedTarget[next.oldPos] = current.oldPos
			i++
			continue
		}
		if next.op == code.OpWrite {
			if jumpTargets[next.oldPos] {
				continue
			}
			if replacement, ok := writeReplacement(current.op, current.operands, constants); ok {
				replace[current.oldPos] = replacement
				remove[next.oldPos] = true
				removedTarget[next.oldPos] = current.oldPos
				i++
				continue
			}
		}
	}
	if len(remove) == 0 && len(replace) == 0 {
		return instructions, callNames, lineNumbers, properties
	}

	oldToNew := map[int]int{}
	removedBytes := make([]int, len(instructions)+1)
	out := code.Instructions{}
	removedSoFar := 0
	for _, ins := range parsed {
		for pos := ins.oldPos; pos <= ins.oldPos+ins.width && pos < len(removedBytes); pos++ {
			removedBytes[pos] = removedSoFar
		}
		if remove[ins.oldPos] {
			removedSoFar += ins.width
			continue
		}

		oldToNew[ins.oldPos] = len(out)
		if replacement, ok := replace[ins.oldPos]; ok {
			out = append(out, replacement...)
			continue
		}
		out = append(out, instructions[ins.oldPos:ins.oldPos+ins.width]...)
	}
	removedBytes[len(instructions)] = removedSoFar

	mapTarget := func(pos int) int {
		if newPos, ok := oldToNew[pos]; ok {
			return newPos
		}
		if target, ok := removedTarget[pos]; ok {
			return oldToNew[target]
		}
		if pos >= len(removedBytes) {
			return len(out)
		}
		return pos - removedBytes[pos]
	}

	for i := 0; i < len(out); {
		op := code.Opcode(out[i])
		def, err := code.Lookup(byte(op))
		if err != nil {
			i++
			continue
		}
		operands, read := code.ReadOperands(def, out[i+1:])
		switch op {
		case code.OpJump, code.OpJumpNotTruthy:
			copied := append([]int(nil), operands...)
			copied[0] = mapTarget(copied[0])
			replacement := code.Make(op, copied...)
			copy(out[i:i+len(replacement)], replacement)
		}
		i += 1 + read
	}

	return out,
		remapStringMap(callNames, oldToNew, remove),
		remapIntMap(lineNumbers, oldToNew, remove),
		remapPropertyMap(properties, oldToNew, remove)
}

func writeReplacement(op code.Opcode, operands []int, constants []object.Object) (code.Instructions, bool) {
	if len(operands) == 0 {
		return nil, false
	}
	switch op {
	case code.OpConstant:
		return code.Make(constantWriteOpcode(operands[0], constants), operands[0]), true
	case code.OpGetName:
		return code.Make(code.OpWriteName, operands[0]), true
	case code.OpGetNameOrNull:
		return code.Make(code.OpWriteNameOrNull, operands[0]), true
	case code.OpGetLocal:
		return code.Make(code.OpWriteLocal, operands[0]), true
	case code.OpGetGlobal:
		return code.Make(code.OpWriteGlobal, operands[0]), true
	case code.OpCall:
		return code.Make(code.OpWriteCall, operands[0]), true
	}
	return nil, false
}

func constantWriteOpcode(index int, constants []object.Object) code.Opcode {
	if index < 0 || index >= len(constants) {
		return code.OpWriteConstant
	}
	switch obj := constants[index].(type) {
	case *object.String:
		return code.OpWriteString
	case *object.Native:
		if _, ok := obj.Value.(template.HTML); ok {
			return code.OpWriteHTML
		}
	}
	return code.OpWriteConstant
}

func remapStringMap(input map[int]string, oldToNew map[int]int, remove map[int]bool) map[int]string {
	if len(input) == 0 {
		return nil
	}
	out := map[int]string{}
	for pos, value := range input {
		if remove[pos] {
			continue
		}
		if newPos, ok := oldToNew[pos]; ok {
			out[newPos] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func remapIntMap(input map[int]int, oldToNew map[int]int, remove map[int]bool) map[int]int {
	if len(input) == 0 {
		return nil
	}
	out := map[int]int{}
	for pos, value := range input {
		if remove[pos] {
			continue
		}
		if newPos, ok := oldToNew[pos]; ok {
			out[newPos] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func remapPropertyMap(input map[int]object.PropertyAccess, oldToNew map[int]int, remove map[int]bool) map[int]object.PropertyAccess {
	if len(input) == 0 {
		return nil
	}
	out := map[int]object.PropertyAccess{}
	for pos, value := range input {
		if remove[pos] {
			continue
		}
		if newPos, ok := oldToNew[pos]; ok {
			out[newPos] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
