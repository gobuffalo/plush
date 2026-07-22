package compiler

import (
	"html/template"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
)

// fastRenderPlanFromInstructions is the narrower fallback planner. It recovers
// fast render segments from already-optimized bytecode when the AST planner did
// not produce a plan.
func fastRenderPlanFromInstructions(instructions code.Instructions, constants []object.Object, lineNumbers map[int]int) *FastRenderPlan {
	plan := &FastRenderPlan{}
	hasDynamic := false

	for i := 0; i < len(instructions); {
		op := code.Opcode(instructions[i])
		def, err := code.Lookup(instructions[i])
		if err != nil {
			return nil
		}
		operands, read := code.ReadOperands(def, instructions[i+1:])
		nextPos := i + 1 + read

		switch op {
		case code.OpWriteHTML:
			value, ok := htmlConstantValue(constants, operands[0])
			if !ok {
				return nil
			}
			plan.appendStatic(value)
		case code.OpWriteString:
			value, ok := stringConstantValue(constants, operands[0])
			if !ok {
				return nil
			}
			plan.appendStatic(template.HTMLEscapeString(value))
		case code.OpWriteConstant:
			value, ok := staticConstantOutput(constants, operands[0])
			if !ok {
				return nil
			}
			plan.appendStatic(value)
		case code.OpWriteName, code.OpWriteNameOrNull:
			name, ok := stringConstantValue(constants, operands[0])
			if !ok {
				return nil
			}
			if name != "nil" {
				hasDynamic = true
				plan.Segments = append(plan.Segments, FastRenderSegment{
					Kind:          FastRenderSegmentName,
					Value:         name,
					NameIndex:     plan.bindName(name),
					NullOnMissing: op == code.OpWriteNameOrNull,
					Line:          lineNumberAt(lineNumbers, i),
				})
				plan.NameCount++
			}
		case code.OpWriteNameProperty:
			base, ok := stringConstantValue(constants, operands[0])
			if !ok {
				return nil
			}
			property, ok := stringConstantValue(constants, operands[1])
			if !ok {
				return nil
			}
			if base != "nil" {
				hasDynamic = true
				plan.Segments = append(plan.Segments, FastRenderSegment{
					Kind:      FastRenderSegmentProperty,
					Value:     base,
					NameIndex: plan.bindName(base),
					Property:  property,
					Receiver:  base,
					Full:      base + "." + property,
					Line:      lineNumberAt(lineNumbers, i),
				})
				plan.NameCount++
			}
		case code.OpGetName:
			loop, writeEnd, ok := fastLoopPlanAt(instructions, nextPos, constants, lineNumbers)
			if !ok {
				return nil
			}
			iterable, ok := stringConstantValue(constants, operands[0])
			if !ok || iterable == "nil" {
				return nil
			}
			hasDynamic = true
			loop.IterableName = iterable
			loop.IterableNameIndex = plan.bindName(iterable)
			plan.Segments = append(plan.Segments, FastRenderSegment{
				Kind: FastRenderSegmentLoop,
				Loop: loop,
				Line: loop.Line,
			})
			i = writeEnd
			continue
		default:
			return nil
		}

		i = nextPos
	}

	if !hasDynamic {
		return nil
	}
	return plan
}

func fastLoopPlanAt(instructions code.Instructions, pos int, constants []object.Object, lineNumbers map[int]int) (*FastLoopPlan, int, bool) {
	op, operands, read, ok := instructionAt(instructions, pos)
	if !ok || op != code.OpFor || len(operands) != 4 || operands[3] != 0 {
		return nil, 0, false
	}
	writePos := pos + 1 + read
	writeOp, _, writeRead, ok := instructionAt(instructions, writePos)
	if !ok || writeOp != code.OpWrite {
		return nil, 0, false
	}

	keyName, ok := stringConstantValue(constants, operands[1])
	if !ok {
		return nil, 0, false
	}
	valueName, ok := stringConstantValue(constants, operands[2])
	if !ok {
		return nil, 0, false
	}
	fn, ok := compiledFunctionConstant(constants, operands[0])
	if !ok {
		return nil, 0, false
	}
	loop, ok := fastLoopPlanFromFunction(fn, constants, keyName, valueName)
	if !ok {
		return nil, 0, false
	}
	loop.Line = lineNumberAt(lineNumbers, pos)
	return loop, writePos + 1 + writeRead, true
}

func instructionAt(instructions code.Instructions, pos int) (code.Opcode, []int, int, bool) {
	if pos < 0 || pos >= len(instructions) {
		return 0, nil, 0, false
	}
	op := code.Opcode(instructions[pos])
	def, err := code.Lookup(instructions[pos])
	if err != nil {
		return 0, nil, 0, false
	}
	operands, read := code.ReadOperands(def, instructions[pos+1:])
	return op, operands, read, true
}

func compiledFunctionConstant(constants []object.Object, index int) (*object.CompiledFunction, bool) {
	if index < 0 || index >= len(constants) {
		return nil, false
	}
	fn, ok := constants[index].(*object.CompiledFunction)
	return fn, ok
}

func fastLoopPlanFromFunction(fn *object.CompiledFunction, constants []object.Object, keyName, valueName string) (*FastLoopPlan, bool) {
	if fn == nil || fn.NumParameters < 2 {
		return nil, false
	}

	loop := &FastLoopPlan{KeyName: keyName, ValueName: valueName}
	for i := 0; i < len(fn.Instructions); {
		op, operands, read, ok := instructionAt(fn.Instructions, i)
		if !ok {
			return nil, false
		}

		switch op {
		case code.OpReturn:
			return loop, true
		case code.OpWriteHTML:
			value, ok := htmlConstantValue(constants, operands[0])
			if !ok {
				return nil, false
			}
			loop.appendStatic(value)
		case code.OpWriteString:
			value, ok := stringConstantValue(constants, operands[0])
			if !ok {
				return nil, false
			}
			loop.appendStatic(template.HTMLEscapeString(value))
		case code.OpWriteConstant:
			value, ok := staticConstantOutput(constants, operands[0])
			if !ok {
				return nil, false
			}
			loop.appendStatic(value)
		case code.OpWriteLocal:
			switch operands[0] {
			case 0:
				loop.Parts = append(loop.Parts, FastLoopPart{Kind: FastLoopPartKey, Line: lineNumberAt(fn.LineNumbers, i)})
			case 1:
				loop.Parts = append(loop.Parts, FastLoopPart{Kind: FastLoopPartValue, Line: lineNumberAt(fn.LineNumbers, i)})
			default:
				return nil, false
			}
		case code.OpWriteLocalProperty:
			if operands[0] != 1 {
				return nil, false
			}
			property, ok := stringConstantValue(constants, operands[1])
			if !ok {
				return nil, false
			}
			loop.Parts = append(loop.Parts, FastLoopPart{
				Kind:     FastLoopPartValueProperty,
				Value:    property,
				Receiver: valueName,
				Full:     valueName + "." + property,
				Line:     lineNumberAt(fn.LineNumbers, i),
			})
		default:
			return nil, false
		}

		i += 1 + read
	}
	return nil, false
}

func lineNumberAt(lineNumbers map[int]int, pos int) int {
	if lineNumbers == nil {
		return 1
	}
	if line := lineNumbers[pos]; line > 0 {
		return line
	}
	return 1
}

func (p *FastRenderPlan) appendStatic(value string) {
	if value == "" {
		return
	}
	last := len(p.Segments) - 1
	if last >= 0 && p.Segments[last].Kind == FastRenderSegmentStatic {
		p.Segments[last].Value += value
	} else {
		p.Segments = append(p.Segments, FastRenderSegment{
			Kind:  FastRenderSegmentStatic,
			Value: value,
		})
	}
	p.StaticSize += len(value)
}

func (p *FastRenderPlan) bindName(name string) int {
	for i, bound := range p.Bindings {
		if bound == name {
			return i
		}
	}
	p.Bindings = append(p.Bindings, name)
	return len(p.Bindings) - 1
}

func (p *FastLoopPlan) appendStatic(value string) {
	if value == "" {
		return
	}
	last := len(p.Parts) - 1
	if last >= 0 && p.Parts[last].Kind == FastLoopPartStatic {
		p.Parts[last].Value += value
	} else {
		p.Parts = append(p.Parts, FastLoopPart{
			Kind:  FastLoopPartStatic,
			Value: value,
		})
	}
	p.StaticSize += len(value)
}
