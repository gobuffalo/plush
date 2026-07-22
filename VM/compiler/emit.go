package compiler

import (
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
)

func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)
	c.recordLineNumber(pos)

	return pos
}

func (c *Compiler) emitCall(op code.Opcode, name string, operands ...int) int {
	pos := c.emit(op, operands...)
	c.recordCallName(pos, name)
	return pos
}

func (c *Compiler) emitProperty(name, receiver, full string) int {
	pos := c.emit(code.OpGetProperty, c.addStringConstant(name))
	c.scopes[c.scopeIndex].properties[pos] = object.PropertyAccess{
		Receiver: receiver,
		Full:     full,
	}
	return pos
}

func (c *Compiler) markLastPropertyAsMethod() {
	last := c.scopes[c.scopeIndex].lastInstruction
	if last.Opcode != code.OpGetProperty {
		return
	}
	info := c.scopes[c.scopeIndex].properties[last.Position]
	info.Method = true
	c.scopes[c.scopeIndex].properties[last.Position] = info
}

func (c *Compiler) recordCallName(pos int, name string) {
	if name == "" {
		name = anonymousCallName
	}
	c.scopes[c.scopeIndex].callNames[pos] = name
}

func (c *Compiler) recordLocalName(symbol Symbol) {
	if symbol.Scope == LocalScope {
		c.scopes[c.scopeIndex].localNames[symbol.Index] = symbol.Name
		if symbol.Index+1 > c.scopes[c.scopeIndex].numLocals {
			c.scopes[c.scopeIndex].numLocals = symbol.Index + 1
		}
	}
}

func (c *Compiler) recordLineNumber(pos int) {
	if c.line > 0 {
		c.scopes[c.scopeIndex].lineNumbers[pos] = c.line
	}
}

func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.currentInstructions())
	updatedInstructions := append(c.currentInstructions(), ins...)

	c.scopes[c.scopeIndex].instructions = updatedInstructions

	return posNewInstruction
}

func (c *Compiler) setLastInstruction(op code.Opcode, pos int) {
	previous := c.scopes[c.scopeIndex].lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}

	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {
	if len(c.currentInstructions()) == 0 {
		return false
	}

	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

func (c *Compiler) removeLastPop() {
	last := c.scopes[c.scopeIndex].lastInstruction
	previous := c.scopes[c.scopeIndex].previousInstruction

	old := c.currentInstructions()
	newInstructions := old[:last.Position]

	c.scopes[c.scopeIndex].instructions = newInstructions
	c.scopes[c.scopeIndex].lastInstruction = previous
}

func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {
	ins := c.currentInstructions()
	for i := 0; i < len(newInstruction); i++ {
		ins[pos+i] = newInstruction[i]
	}
}

func (c *Compiler) changeOperand(opPos int, operand int) {
	op := code.Opcode(c.currentInstructions()[opPos])
	newInstruction := code.Make(op, operand)

	c.replaceInstruction(opPos, newInstruction)
}

func (c *Compiler) currentInstructions() code.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

func (c *Compiler) currentCallNames() map[int]string {
	callNames := c.scopes[c.scopeIndex].callNames
	if len(callNames) == 0 {
		return nil
	}
	copied := make(map[int]string, len(callNames))
	for pos, name := range callNames {
		copied[pos] = name
	}
	return copied
}

func (c *Compiler) currentLocalNames() map[int]string {
	localNames := c.scopes[c.scopeIndex].localNames
	if len(localNames) == 0 {
		return nil
	}
	copied := make(map[int]string, len(localNames))
	for index, name := range localNames {
		copied[index] = name
	}
	return copied
}

func (c *Compiler) currentLineNumbers() map[int]int {
	lineNumbers := c.scopes[c.scopeIndex].lineNumbers
	if len(lineNumbers) == 0 {
		return nil
	}
	copied := make(map[int]int, len(lineNumbers))
	for pos, line := range lineNumbers {
		copied[pos] = line
	}
	return copied
}

func (c *Compiler) currentProperties() map[int]object.PropertyAccess {
	properties := c.scopes[c.scopeIndex].properties
	if len(properties) == 0 {
		return nil
	}
	copied := make(map[int]object.PropertyAccess, len(properties))
	for pos, property := range properties {
		copied[pos] = property
	}
	return copied
}

func (c *Compiler) currentLocalCount() int {
	return c.scopes[c.scopeIndex].numLocals
}

func (c *Compiler) globalCount() int {
	count := 0
	if c.symbolTable != nil && c.symbolTable.Outer == nil {
		count = c.symbolTable.numDefinitions
	}
	for index := range c.globalNames {
		if index+1 > count {
			count = index + 1
		}
	}
	return count
}

func (c *Compiler) enterScope() {
	scope := CompilationScope{
		instructions:        code.Instructions{},
		callNames:           map[int]string{},
		localNames:          map[int]string{},
		lineNumbers:         map[int]int{},
		properties:          map[int]object.PropertyAccess{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}
	c.scopes = append(c.scopes, scope)
	c.scopeIndex++

	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

func (c *Compiler) leaveScope() code.Instructions {
	instructions := c.currentInstructions()

	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--

	c.symbolTable = c.symbolTable.Outer
	return instructions
}

func (c *Compiler) replaceLastPopWithReturn() {
	lastPos := c.scopes[c.scopeIndex].lastInstruction.Position
	c.replaceInstruction(lastPos, code.Make(code.OpReturnValue))

	c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpReturnValue
}

func (c *Compiler) loadSymbol(s Symbol) {
	switch s.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobal, s.Index)
	case LocalScope:
		c.emit(code.OpGetLocal, s.Index)
	case BuiltinScope:
		c.emit(code.OpGetBuiltin, s.Index)
	case FreeScope:
		c.emit(code.OpGetFree, s.Index)
	case FunctionScope:
		c.emit(code.OpCurrentClosure)
	}
}
