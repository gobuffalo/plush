package compiler

import (
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/parser"
	"github.com/gobuffalo/plush/v5/token"
)

type Compiler struct {
	constants []object.Object

	symbolTable *SymbolTable
	globalNames map[int]string
	program     *ast.Program

	scopes     []CompilationScope
	scopeIndex int

	softNames int
	line      int

	suppressOutput             int
	topLevelBlockReturnAsValue int
}

const anonymousCallName = "<anonymous>"

func New() *Compiler {
	mainScope := CompilationScope{
		instructions:        code.Instructions{},
		callNames:           map[int]string{},
		localNames:          map[int]string{},
		lineNumbers:         map[int]int{},
		properties:          map[int]object.PropertyAccess{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	symbolTable := NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	return &Compiler{
		constants:   []object.Object{},
		symbolTable: symbolTable,
		globalNames: map[int]string{},
		scopes:      []CompilationScope{mainScope},
		scopeIndex:  0,
	}
}

func NewWithState(s *SymbolTable, constants []object.Object) *Compiler {
	compiler := New()
	compiler.symbolTable = s
	compiler.constants = constants
	return compiler
}

// ParseScript parses a plain Plush expression script by wrapping it in script
// tags before handing it to Plush's parser.
func ParseScript(input string) (*ast.Program, error) {
	return parser.Parse("<% " + input + " %>")
}

func (c *Compiler) Compile(node interface{}) error {
	switch node := node.(type) {
	case *ast.Program:
		if c.scopeIndex == 0 {
			c.program = node
		}
		for _, s := range node.Statements {
			if err := c.compileStatement(s); err != nil {
				return err
			}
		}

	case *ast.ExpressionStatement:
		return c.compileExpressionStatement(node)

	case *ast.BlockStatement:
		for _, s := range node.Statements {
			if err := c.compileStatement(s); err != nil {
				return err
			}
		}

	case *ast.HoleStatement:
		c.emit(code.OpHole, c.addStringConstant("<%= "+node.String()+" %>"))

	case *ast.LetStatement:
		return c.compileLetStatement(node)

	case *ast.ReturnStatement:
		writeReturn := node.Type == token.E_START && c.suppressOutput == 0
		if writeReturn {
			handled, err := c.compileWriteExpression(node.ReturnValue)
			if handled || err != nil {
				return err
			}
		}
		if err := c.Compile(node.ReturnValue); err != nil {
			return err
		}
		if writeReturn {
			c.emit(code.OpWrite)
		} else if node.Type == token.RETURN && c.scopeIndex > 0 {
			c.emit(code.OpReturnValue)
		} else if node.Type == token.RETURN && c.topLevelBlockReturnAsValue == 0 && c.suppressOutput == 0 {
			c.emit(code.OpReturnValue)
		} else {
			c.emit(code.OpPop)
		}

	case *ast.InfixExpression:
		return c.compileInfixExpression(node)

	case *ast.IntegerLiteral:
		c.emitConstant(&object.Integer{Value: int64(node.Value)})

	case *ast.FloatLiteral:
		c.emitConstant(&object.Float{Value: node.Value})

	case *ast.Boolean:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	case *ast.PrefixExpression:
		switch node.Operator {
		case "!":
			if err := c.compileSoft(node.Right); err != nil {
				return err
			}
			c.emit(code.OpBang)
		case "-":
			if err := c.Compile(node.Right); err != nil {
				return err
			}
			c.emit(code.OpMinus)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.IfExpression:
		return c.compileIfExpression(node)

	case *ast.Identifier:
		return c.compileIdentifier(node)

	case *ast.StringLiteral:
		c.emitConstant(&object.String{Value: node.Value})

	case *ast.HTMLLiteral:
		c.emitConstant(&object.Native{Value: template.HTML(node.Value)})

	case *ast.ArrayLiteral:
		for _, el := range node.Elements {
			if err := c.Compile(el); err != nil {
				return err
			}
		}
		c.emit(code.OpArray, len(node.Elements))

	case *ast.HashLiteral:
		return c.compileHashLiteral(node)

	case *ast.IndexExpression:
		return c.compileIndexExpression(node)

	case *ast.AssignExpression:
		return c.compileAssignExpression(node)

	case *ast.FunctionLiteral:
		return c.compileFunctionLiteral(node, "")

	case *ast.CallExpression:
		return c.compileCallExpression(node)

	case *ast.ForExpression:
		return c.compileForExpression(node)

	case *ast.BreakExpression:
		c.emit(code.OpBreak)

	case *ast.ContinueExpression:
		c.emit(code.OpContinue)

	case nil:
		c.emit(code.OpNull)
	}

	return nil
}

func (c *Compiler) compileStatement(stmt ast.Statement) error {
	previousLine := c.line
	if stmt != nil && stmt.T().LineNumber > 0 {
		c.line = stmt.T().LineNumber
	}
	err := c.Compile(stmt)
	c.line = previousLine
	return err
}

func (c *Compiler) compileExpressionStatement(node *ast.ExpressionStatement) error {
	if html, ok := node.Expression.(*ast.HTMLLiteral); ok {
		if c.suppressOutput > 0 {
			return nil
		}
		c.emitWriteHTML(html.Value)
		return nil
	}

	switch node.Expression.(type) {
	case *ast.BreakExpression, *ast.ContinueExpression:
		return c.Compile(node.Expression)
	}

	suppressOutput := true
	if ifExpression, ok := node.Expression.(*ast.IfExpression); ok && ifExpressionHasLoopControl(ifExpression) {
		suppressOutput = false
	}

	if suppressOutput {
		c.suppressOutput++
	}
	err := c.Compile(node.Expression)
	if suppressOutput {
		c.suppressOutput--
	}
	if err != nil {
		return err
	}
	c.emit(code.OpPop)
	return nil
}

func (c *Compiler) compileWriteExpression(expr ast.Expression) (bool, error) {
	switch node := expr.(type) {
	case *ast.Identifier:
		if node.Callee != nil {
			return c.compileWriteIdentifierProperty(node)
		}
		if node.Value == "nil" {
			return true, nil
		}
		symbol, ok := c.symbolTable.Resolve(node.Value)
		if !ok {
			c.emit(code.OpWriteName, c.addStringConstant(node.Value))
			return true, nil
		}
		switch symbol.Scope {
		case GlobalScope:
			c.emit(code.OpWriteGlobal, symbol.Index)
			return true, nil
		case LocalScope:
			c.emit(code.OpWriteLocal, symbol.Index)
			return true, nil
		}
	case *ast.StringLiteral:
		c.emitWriteString(node.Value)
		return true, nil
	case *ast.HTMLLiteral:
		c.emitWriteHTML(node.Value)
		return true, nil
	case *ast.CallExpression:
		return c.compileWriteNameCallExpression(node)
	}
	return false, nil
}

func ifExpressionHasLoopControl(expr *ast.IfExpression) bool {
	if expr == nil {
		return false
	}
	if blockHasLoopControl(expr.Block) || blockHasLoopControl(expr.ElseBlock) {
		return true
	}
	for _, elseIf := range expr.ElseIf {
		if blockHasLoopControl(elseIf.Block) {
			return true
		}
	}
	return false
}

func blockHasLoopControl(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Statements {
		if statementHasLoopControl(stmt) {
			return true
		}
	}
	return false
}

func statementHasLoopControl(stmt ast.Statement) bool {
	exprStmt, ok := stmt.(*ast.ExpressionStatement)
	if !ok {
		return false
	}
	switch expr := exprStmt.Expression.(type) {
	case *ast.BreakExpression, *ast.ContinueExpression:
		return true
	case *ast.IfExpression:
		return ifExpressionHasLoopControl(expr)
	default:
		return false
	}
}

func (c *Compiler) compileWriteNameCallExpression(node *ast.CallExpression) (bool, error) {
	if node == nil || node.Block != nil || node.ChainCallee != nil {
		return false, nil
	}
	ident, ok := node.Function.(*ast.Identifier)
	if !ok || ident.Callee != nil {
		return false, nil
	}
	if _, ok := c.symbolTable.Resolve(ident.Value); ok {
		return false, nil
	}

	for _, arg := range node.Arguments {
		if err := c.compileHard(arg); err != nil {
			return true, err
		}
	}
	c.emitCall(code.OpWriteNameCall, ident.Value, c.addStringConstant(ident.Value), len(node.Arguments))
	return true, nil
}

func (c *Compiler) compileWriteIdentifierProperty(node *ast.Identifier) (bool, error) {
	if node == nil || node.Callee == nil || node.Callee.Callee != nil {
		return false, nil
	}

	base := node.Callee.Value
	if base == "" || strings.Contains(base, ".") || strings.Contains(node.Value, ".") {
		return false, nil
	}
	propertyIndex := c.addStringConstant(node.Value)
	receiver := node.Callee.String()
	full := node.String()

	symbol, ok := c.symbolTable.Resolve(base)
	if !ok {
		pos := c.emit(code.OpWriteNameProperty, c.addStringConstant(base), propertyIndex)
		c.scopes[c.scopeIndex].properties[pos] = object.PropertyAccess{
			Receiver: receiver,
			Full:     full,
		}
		return true, nil
	}

	switch symbol.Scope {
	case LocalScope:
		pos := c.emit(code.OpWriteLocalProperty, symbol.Index, propertyIndex)
		c.scopes[c.scopeIndex].properties[pos] = object.PropertyAccess{
			Receiver: receiver,
			Full:     full,
		}
		return true, nil
	case GlobalScope:
		pos := c.emit(code.OpWriteGlobalProperty, symbol.Index, propertyIndex)
		c.scopes[c.scopeIndex].properties[pos] = object.PropertyAccess{
			Receiver: receiver,
			Full:     full,
		}
		return true, nil
	}

	return false, nil
}

func (c *Compiler) compileLetStatement(node *ast.LetStatement) error {
	if fn, ok := node.Value.(*ast.FunctionLiteral); ok {
		symbol := c.symbolTable.Define(node.Name.Value)
		c.recordLocalName(symbol)
		if symbol.Scope == GlobalScope {
			c.globalNames[symbol.Index] = node.Name.Value
		}
		if err := c.compileFunctionLiteral(fn, node.Name.Value); err != nil {
			return err
		}
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}
		return nil
	}

	if err := c.Compile(node.Value); err != nil {
		return err
	}
	symbol := c.symbolTable.Define(node.Name.Value)
	c.recordLocalName(symbol)
	if symbol.Scope == GlobalScope {
		c.globalNames[symbol.Index] = node.Name.Value
	}
	if symbol.Scope == GlobalScope {
		c.emit(code.OpSetGlobal, symbol.Index)
	} else {
		c.emit(code.OpSetLocal, symbol.Index)
	}

	return nil
}

func (c *Compiler) compileInfixExpression(node *ast.InfixExpression) error {
	switch node.Operator {
	case "&&":
		return c.compileLogicalAnd(node)
	case "||":
		return c.compileLogicalOr(node)
	}

	switch node.Operator {
	case "<":
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		c.emit(code.OpGreaterThan)
		return nil
	case "<=":
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		c.emit(code.OpGreaterEqual)
		return nil
	}

	switch node.Operator {
	case "==", "!=":
		if err := c.compileSoft(node.Left); err != nil {
			return err
		}
		if err := c.compileSoft(node.Right); err != nil {
			return err
		}
	default:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		if err := c.Compile(node.Right); err != nil {
			return err
		}
	}

	switch node.Operator {
	case "+":
		c.emit(code.OpAdd)
	case "-":
		c.emit(code.OpSub)
	case "*":
		c.emit(code.OpMul)
	case "/":
		c.emit(code.OpDiv)
	case ">":
		c.emit(code.OpGreaterThan)
	case ">=":
		c.emit(code.OpGreaterEqual)
	case "==":
		c.emit(code.OpEqual)
	case "!=":
		c.emit(code.OpNotEqual)
	case "~=":
		c.emit(code.OpMatches)
	default:
		return fmt.Errorf("unknown operator %s", node.Operator)
	}

	return nil
}

func (c *Compiler) compileLogicalAnd(node *ast.InfixExpression) error {
	if err := c.compileSoft(node.Left); err != nil {
		return err
	}

	jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

	if err := c.compileSoft(node.Right); err != nil {
		return err
	}
	c.emit(code.OpBang)
	c.emit(code.OpBang)

	jumpEndPos := c.emit(code.OpJump, 9999)
	falsePos := len(c.currentInstructions())
	c.changeOperand(jumpNotTruthyPos, falsePos)
	c.emit(code.OpFalse)

	afterPos := len(c.currentInstructions())
	c.changeOperand(jumpEndPos, afterPos)
	return nil
}

func (c *Compiler) compileLogicalOr(node *ast.InfixExpression) error {
	if err := c.compileSoft(node.Left); err != nil {
		return err
	}

	evalRightPos := c.emit(code.OpJumpNotTruthy, 9999)
	c.emit(code.OpTrue)

	jumpEndPos := c.emit(code.OpJump, 9999)
	rightPos := len(c.currentInstructions())
	c.changeOperand(evalRightPos, rightPos)

	if err := c.compileSoft(node.Right); err != nil {
		return err
	}
	c.emit(code.OpBang)
	c.emit(code.OpBang)

	afterPos := len(c.currentInstructions())
	c.changeOperand(jumpEndPos, afterPos)
	return nil
}

func (c *Compiler) compileIfExpression(node *ast.IfExpression) error {
	if err := c.compileCondition(node.Condition); err != nil {
		return err
	}

	jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

	blockStart := len(c.currentInstructions())
	if err := c.compileScopedBlockStatement(node.Block); err != nil {
		return err
	}
	c.ensureBranchValue(blockStart)

	jumpPositions := []int{c.emit(code.OpJump, 9999)}
	afterBlockPos := len(c.currentInstructions())
	c.changeOperand(jumpNotTruthyPos, afterBlockPos)

	for _, elseIf := range node.ElseIf {
		if err := c.compileCondition(elseIf.Condition); err != nil {
			return err
		}

		elseIfJumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)
		elseIfBlockStart := len(c.currentInstructions())
		if err := c.compileScopedBlockStatement(elseIf.Block); err != nil {
			return err
		}
		c.ensureBranchValue(elseIfBlockStart)

		jumpPositions = append(jumpPositions, c.emit(code.OpJump, 9999))
		afterElseIfPos := len(c.currentInstructions())
		c.changeOperand(elseIfJumpNotTruthyPos, afterElseIfPos)
	}

	if node.ElseBlock == nil {
		c.emit(code.OpNull)
	} else {
		elseBlockStart := len(c.currentInstructions())
		if err := c.compileScopedBlockStatement(node.ElseBlock); err != nil {
			return err
		}
		c.ensureBranchValue(elseBlockStart)
	}

	afterAlternativePos := len(c.currentInstructions())
	for _, pos := range jumpPositions {
		c.changeOperand(pos, afterAlternativePos)
	}

	return nil
}

func (c *Compiler) compileScopedBlockStatement(block *ast.BlockStatement) error {
	outer := c.symbolTable
	c.symbolTable = NewInlineBlockSymbolTable(c.symbolTable)
	if c.scopeIndex == 0 {
		c.topLevelBlockReturnAsValue++
		defer func() {
			c.topLevelBlockReturnAsValue--
		}()
	}
	defer func() {
		c.symbolTable = outer
	}()
	return c.Compile(block)
}

func (c *Compiler) ensureBranchValue(startPos int) {
	if len(c.currentInstructions()) == startPos {
		c.emit(code.OpNull)
		return
	}
	if c.lastInstructionIs(code.OpPop) {
		c.removeLastPop()
		return
	}

	switch c.scopes[c.scopeIndex].lastInstruction.Opcode {
	case code.OpSetGlobal, code.OpSetLocal, code.OpSetName,
		code.OpAssignName, code.OpWrite, code.OpWriteConstant, code.OpWriteName,
		code.OpWriteNameOrNull, code.OpWriteLocal, code.OpWriteGlobal,
		code.OpWriteString, code.OpWriteHTML, code.OpWriteLocalProperty,
		code.OpWriteGlobalProperty, code.OpWriteNameProperty, code.OpWriteCall,
		code.OpWriteNameCall,
		code.OpHole, code.OpBreak, code.OpContinue:
		c.emit(code.OpNull)
	}
}

func (c *Compiler) compileIdentifier(node *ast.Identifier) error {
	if node.Callee != nil {
		_ = c.compileIdentifier(node.Callee)
		c.emitProperty(node.Value, node.Callee.String(), node.String())
		return nil
	}

	symbol, ok := c.symbolTable.Resolve(node.Value)
	if !ok {
		if node.Value == "nil" {
			c.emit(code.OpNull)
			return nil
		}
		if c.softNames > 0 {
			c.emit(code.OpGetNameOrNull, c.addStringConstant(node.Value))
			return nil
		}
		c.emit(code.OpGetName, c.addStringConstant(node.Value))
		return nil
	}

	if symbol.Scope == BuiltinScope && c.softNames > 0 {
		c.emit(code.OpGetNameOrNull, c.addStringConstant(symbol.Name))
		return nil
	}

	c.loadSymbol(symbol)
	return nil
}

func (c *Compiler) compileHashLiteral(node *ast.HashLiteral) error {
	keys := append([]ast.Expression(nil), node.Order...)
	if len(keys) == 0 {
		for k := range node.Pairs {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})
	}

	for _, k := range keys {
		if ident, ok := k.(*ast.Identifier); ok && ident.Callee == nil {
			c.emitConstant(&object.String{Value: ident.TokenLiteral()})
		} else if err := c.Compile(k); err != nil {
			return err
		}

		if err := c.Compile(node.Pairs[k]); err != nil {
			return err
		}
	}

	c.emit(code.OpHash, len(node.Pairs)*2)
	return nil
}

func (c *Compiler) compileIndexExpression(node *ast.IndexExpression) error {
	if err := c.Compile(node.Left); err != nil {
		return err
	}
	if err := c.Compile(node.Index); err != nil {
		return err
	}

	if node.Value != nil {
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		c.emit(code.OpSetIndex)
		return nil
	}

	c.emit(code.OpIndex)

	if node.Callee != nil {
		return c.compileReceiverCallee(node.Callee, lastChainPart(node.Left))
	}

	return nil
}

func (c *Compiler) compileReceiverCallee(exp ast.Expression, base string) error {
	switch exp := exp.(type) {
	case *ast.Identifier:
		for _, property := range trimReceiverParts(identifierParts(exp), base) {
			c.emit(code.OpGetProperty, c.addStringConstant(property))
		}
		return nil
	case *ast.IndexExpression:
		if err := c.compileReceiverCallee(exp.Left, base); err != nil {
			return err
		}
		if err := c.Compile(exp.Index); err != nil {
			return err
		}
		if exp.Value != nil {
			if err := c.Compile(exp.Value); err != nil {
				return err
			}
			c.emit(code.OpSetIndex)
		} else {
			c.emit(code.OpIndex)
		}
		if exp.Callee != nil {
			return c.compileReceiverCallee(exp.Callee, lastChainPart(exp.Left))
		}
		return nil
	case *ast.CallExpression:
		if ident, ok := exp.Function.(*ast.Identifier); ok {
			for _, property := range trimReceiverParts(identifierParts(ident), base) {
				c.emit(code.OpGetProperty, c.addStringConstant(property))
			}
		} else if err := c.compileReceiverCallee(exp.Function, base); err != nil {
			return err
		}

		for _, a := range exp.Arguments {
			if err := c.compileHard(a); err != nil {
				return err
			}
		}

		if exp.Block != nil {
			blockIndex, numFree, err := c.compileBlockConstant(exp.Block, nil)
			if err != nil {
				return err
			}
			c.emitCall(code.OpCallBlock, callExpressionName(exp), len(exp.Arguments), blockIndex, numFree)
		} else {
			c.emitCall(code.OpCall, callExpressionName(exp), len(exp.Arguments))
		}

		if exp.ChainCallee != nil {
			return c.compileReceiverCallee(exp.ChainCallee, lastChainPart(exp.Function))
		}
		return nil
	default:
		return fmt.Errorf("unsupported chained callee %T", exp)
	}
}

func identifierParts(id *ast.Identifier) []string {
	if id == nil {
		return nil
	}
	parts := identifierParts(id.Callee)
	for _, part := range strings.Split(id.Value, ".") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func trimReceiverParts(parts []string, base string) []string {
	if len(parts) == 0 {
		return parts
	}
	basePart := lastStringPart(base)
	if basePart == "" {
		return parts
	}
	for i, part := range parts {
		if part == basePart || lastStringPart(part) == basePart {
			return parts[i+1:]
		}
	}
	return parts
}

func splitChainBase(base string) []string {
	parts := []string{}
	for _, part := range strings.Split(base, ".") {
		part = strings.Trim(part, "()")
		if i := strings.Index(part, "["); i >= 0 {
			part = part[:i]
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func lastChainPart(exp ast.Expression) string {
	switch exp := exp.(type) {
	case *ast.Identifier:
		parts := identifierParts(exp)
		if len(parts) == 0 {
			return ""
		}
		return parts[len(parts)-1]
	case *ast.IndexExpression:
		return lastChainPart(exp.Left)
	case *ast.CallExpression:
		return lastChainPart(exp.Function)
	default:
		return lastStringPart(exp.String())
	}
}

func callExpressionName(exp *ast.CallExpression) string {
	if exp == nil {
		return anonymousCallName
	}
	name := expressionCallName(exp.Function)
	if name == "" {
		return anonymousCallName
	}
	return name
}

func expressionCallName(exp ast.Expression) string {
	switch exp := exp.(type) {
	case *ast.Identifier:
		parts := identifierParts(exp)
		if len(parts) == 0 {
			return exp.Value
		}
		return parts[len(parts)-1]
	case *ast.IndexExpression:
		return lastChainPart(exp.Left)
	case *ast.CallExpression:
		return callExpressionName(exp)
	default:
		return ""
	}
}

func lastStringPart(value string) string {
	parts := splitChainBase(value)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func (c *Compiler) compileAssignExpression(node *ast.AssignExpression) error {
	if err := c.Compile(node.Value); err != nil {
		return err
	}

	name := node.Name.Value
	if symbol, ok := c.symbolTable.Resolve(name); ok {
		switch symbol.Scope {
		case GlobalScope:
			c.emit(code.OpSetGlobal, symbol.Index)
		case LocalScope:
			c.emit(code.OpSetLocal, symbol.Index)
		default:
			c.emit(code.OpAssignName, c.addStringConstant(name))
			return nil
		}
		c.emit(code.OpNull)
		return nil
	}

	c.emit(code.OpAssignName, c.addStringConstant(name))
	return nil
}

func (c *Compiler) compileFunctionLiteral(node *ast.FunctionLiteral, name string) error {
	c.enterScope()
	previousSuppressOutput := c.suppressOutput
	c.suppressOutput = 0
	defer func() {
		c.suppressOutput = previousSuppressOutput
	}()

	if name != "" {
		c.symbolTable.DefineFunctionName(name)
	}

	for _, p := range node.Parameters {
		c.recordLocalName(c.symbolTable.Define(p.Value))
	}

	if err := c.Compile(node.Block); err != nil {
		return err
	}

	if c.lastInstructionIs(code.OpPop) {
		c.replaceLastPopWithReturn()
	}
	if !c.lastInstructionIs(code.OpReturnValue) {
		c.emit(code.OpReturn)
	}

	freeSymbols := c.symbolTable.FreeSymbols
	numLocals := c.symbolTable.numDefinitions
	callNames := c.currentCallNames()
	localNames := c.currentLocalNames()
	lineNumbers := c.currentLineNumbers()
	properties := c.currentProperties()
	instructions := c.leaveScope()
	instructions, callNames, lineNumbers, properties = optimizeScope(instructions, callNames, lineNumbers, properties, c.constants)

	for _, s := range freeSymbols {
		c.loadSymbol(s)
	}

	compiledFn := &object.CompiledFunction{
		Instructions:   instructions,
		CallNames:      callNames,
		LocalNames:     localNames,
		LineNumbers:    lineNumbers,
		Properties:     properties,
		PropertyCaches: object.NewInlineCacheSlots(len(instructions)),
		CallCaches:     object.NewInlineCacheSlots(len(instructions)),
		NumLocals:      numLocals,
		NumParameters:  len(node.Parameters),
	}

	fnIndex := c.addConstant(compiledFn)
	c.emit(code.OpClosure, fnIndex, len(freeSymbols))
	return nil
}

func (c *Compiler) compileCallExpression(node *ast.CallExpression) error {
	if err := c.compileHard(node.Function); err != nil {
		return err
	}
	c.markLastPropertyAsMethod()

	for _, a := range node.Arguments {
		if err := c.compileHard(a); err != nil {
			return err
		}
	}

	if node.Block != nil {
		blockIndex, numFree, err := c.compileOutputBlockConstant(node.Block, nil)
		if err != nil {
			return err
		}
		c.emitCall(code.OpCallBlock, callExpressionName(node), len(node.Arguments), blockIndex, numFree)
	} else {
		c.emitCall(code.OpCall, callExpressionName(node), len(node.Arguments))
	}

	if node.ChainCallee != nil {
		return c.compileReceiverCallee(node.ChainCallee, lastChainPart(node.Function))
	}

	return nil
}

func (c *Compiler) compileCondition(node ast.Expression) error {
	switch node.(type) {
	case *ast.Identifier:
		return c.compileSoft(node)
	default:
		return c.Compile(node)
	}
}

func (c *Compiler) compileSoft(node interface{}) error {
	c.softNames++
	defer func() { c.softNames-- }()
	return c.Compile(node)
}

func (c *Compiler) compileHard(node interface{}) error {
	softNames := c.softNames
	c.softNames = 0
	defer func() { c.softNames = softNames }()
	return c.Compile(node)
}

func (c *Compiler) compileForExpression(node *ast.ForExpression) error {
	if err := c.Compile(node.Iterable); err != nil {
		return err
	}

	blockIndex, numFree, err := c.compileBlockConstant(node.Block, []string{node.KeyName, node.ValueName})
	if err != nil {
		return err
	}

	c.emit(code.OpFor, blockIndex, c.addStringConstant(node.KeyName), c.addStringConstant(node.ValueName), numFree)
	return nil
}

func (c *Compiler) compileBlockConstant(block *ast.BlockStatement, params []string) (int, int, error) {
	c.enterScope()

	for _, name := range params {
		c.recordLocalName(c.symbolTable.Define(name))
	}

	if err := c.Compile(block); err != nil {
		return 0, 0, err
	}

	if !c.lastInstructionIs(code.OpReturnValue) {
		c.emit(code.OpReturn)
	}

	freeSymbols := c.symbolTable.FreeSymbols
	numLocals := c.symbolTable.numDefinitions
	callNames := c.currentCallNames()
	localNames := c.currentLocalNames()
	lineNumbers := c.currentLineNumbers()
	properties := c.currentProperties()
	instructions := c.leaveScope()
	instructions, callNames, lineNumbers, properties = optimizeScope(instructions, callNames, lineNumbers, properties, c.constants)

	for _, s := range freeSymbols {
		c.loadSymbol(s)
	}

	compiledFn := &object.CompiledFunction{
		Instructions:   instructions,
		CallNames:      callNames,
		LocalNames:     localNames,
		LineNumbers:    lineNumbers,
		Properties:     properties,
		PropertyCaches: object.NewInlineCacheSlots(len(instructions)),
		CallCaches:     object.NewInlineCacheSlots(len(instructions)),
		NumLocals:      numLocals,
		NumParameters:  len(params),
	}

	return c.addConstant(compiledFn), len(freeSymbols), nil
}

func (c *Compiler) compileOutputBlockConstant(block *ast.BlockStatement, params []string) (int, int, error) {
	previousSuppressOutput := c.suppressOutput
	c.suppressOutput = 0
	defer func() {
		c.suppressOutput = previousSuppressOutput
	}()
	return c.compileBlockConstant(block, params)
}

func (c *Compiler) Bytecode() *Bytecode {
	names := map[int]string{}
	for k, v := range c.globalNames {
		names[k] = v
	}
	instructions, callNames, lineNumbers, properties := optimizeScope(
		c.currentInstructions(),
		c.currentCallNames(),
		c.currentLineNumbers(),
		c.currentProperties(),
		c.constants,
	)
	staticOutput, static := staticOutputFromInstructions(instructions, c.constants)
	fastRenderPlan, fastReject := fastRenderPlanAnalysisFromProgram(c.program)
	if fastRenderPlan != nil {
		if reject := compileFastRenderPlanBlocks(fastRenderPlan); reject.Reason != "" {
			fastReject = reject
			fastRenderPlan = nil
		}
	}
	if fastRenderPlan == nil {
		fastRenderPlan = fastRenderPlanFromInstructions(instructions, c.constants, lineNumbers)
		if fastRenderPlan != nil {
			fastReject = FastRenderReject{}
		}
	}
	features := bytecodeFeaturesFromInstructions(instructions, c.constants, callNames)

	return &Bytecode{
		Instructions:   instructions,
		CallNames:      callNames,
		LocalNames:     c.currentLocalNames(),
		LineNumbers:    lineNumbers,
		Properties:     properties,
		PropertyCaches: object.NewInlineCacheSlots(len(instructions)),
		CallCaches:     object.NewInlineCacheSlots(len(instructions)),
		NumLocals:      c.currentLocalCount(),
		NumGlobals:     c.globalCount(),
		Constants:      c.constants,
		GlobalNames:    names,
		Static:         static,
		StaticOutput:   staticOutput,
		FastRenderPlan: fastRenderPlan,
		FastRejectLine: fastReject.Line,
		FastReject:     fastReject.Reason,
		HasHoles:       features.HasHoles,
		HasPartials:    features.HasPartials,
	}
}

func compileFastRenderPlanBlocks(plan *FastRenderPlan) FastRenderReject {
	if plan == nil {
		return FastRenderReject{}
	}
	return compileFastRenderSegmentBlocks(plan.Segments)
}

func compileFastRenderSegmentBlocks(segments []FastRenderSegment) FastRenderReject {
	for i := range segments {
		segment := &segments[i]
		switch segment.Kind {
		case FastRenderSegmentBlockCall:
			if segment.BlockCall == nil {
				return FastRenderReject{Line: segment.Line, Reason: "missing fast block helper call plan"}
			}
			if reject := compileFastBlockCall(segment.BlockCall); reject.Reason != "" {
				return reject
			}
		case FastRenderSegmentConditional:
			if segment.Conditional == nil {
				continue
			}
			for branchIndex := range segment.Conditional.Branches {
				if reject := compileFastRenderSegmentBlocks(segment.Conditional.Branches[branchIndex].Segments); reject.Reason != "" {
					return reject
				}
			}
			if reject := compileFastRenderSegmentBlocks(segment.Conditional.ElseSegments); reject.Reason != "" {
				return reject
			}
		case FastRenderSegmentLoop:
			if reject := compileFastLoopBlocks(segment.Loop); reject.Reason != "" {
				return reject
			}
		}
	}
	return FastRenderReject{}
}

func compileFastLoopBlocks(loop *FastLoopPlan) FastRenderReject {
	if loop == nil {
		return FastRenderReject{}
	}
	return compileFastLoopPartBlocks(loop.Parts)
}

func compileFastLoopPartBlocks(parts []FastLoopPart) FastRenderReject {
	for i := range parts {
		part := &parts[i]
		switch part.Kind {
		case FastLoopPartBlockCall:
			if part.BlockCall == nil {
				return FastRenderReject{Line: part.Line, Reason: "missing fast loop block helper call plan"}
			}
			if reject := compileFastBlockCall(part.BlockCall); reject.Reason != "" {
				return reject
			}
		case FastLoopPartConditional:
			if part.Conditional == nil {
				continue
			}
			for branchIndex := range part.Conditional.Branches {
				if reject := compileFastLoopPartBlocks(part.Conditional.Branches[branchIndex].Parts); reject.Reason != "" {
					return reject
				}
			}
			if reject := compileFastLoopPartBlocks(part.Conditional.ElseParts); reject.Reason != "" {
				return reject
			}
		case FastLoopPartLoop:
			if reject := compileFastLoopBlocks(part.Loop); reject.Reason != "" {
				return reject
			}
		}
	}
	return FastRenderReject{}
}

func compileFastBlockCall(call *FastBlockCallPlan) FastRenderReject {
	if call == nil || (call.Block == nil && call.BlockSource == "") {
		return FastRenderReject{Line: callLine(call), Reason: "empty fast block helper body"}
	}
	var program *ast.Program
	if call.Block != nil {
		program = &ast.Program{Statements: call.Block.Statements}
	} else {
		parsed, err := parser.Parse(call.BlockSource)
		if err != nil {
			return FastRenderReject{Line: call.Line, Reason: "could not parse fast block helper body: " + err.Error()}
		}
		program = parsed
	}
	comp := New()
	if err := comp.Compile(program); err != nil {
		return FastRenderReject{Line: call.Line, Reason: "could not compile fast block helper body: " + err.Error()}
	}
	call.BlockBytecode = comp.Bytecode()
	call.Block = nil
	return FastRenderReject{}
}

func callLine(call *FastBlockCallPlan) int {
	if call == nil {
		return 1
	}
	return call.Line
}

func (c *Compiler) emitConstant(obj object.Object) int {
	return c.emit(code.OpConstant, c.addConstant(obj))
}

func (c *Compiler) addStringConstant(value string) int {
	return c.addConstant(&object.String{Value: value})
}

func (c *Compiler) addHTMLConstant(value string) int {
	return c.addConstant(&object.Native{Value: template.HTML(value)})
}

func (c *Compiler) emitWriteString(value string) int {
	if index, ok := c.lastWriteConstantIndex(code.OpWriteString); ok {
		if previous, ok := stringConstantValue(c.constants, index); ok {
			c.constants[index] = &object.String{Value: previous + value}
			return c.scopes[c.scopeIndex].lastInstruction.Position
		}
	}
	if index, ok := c.lastWriteConstantIndex(code.OpWriteHTML); ok {
		if previous, ok := htmlConstantValue(c.constants, index); ok {
			c.constants[index] = &object.Native{
				Value: template.HTML(previous + template.HTMLEscapeString(value)),
			}
			return c.scopes[c.scopeIndex].lastInstruction.Position
		}
	}
	return c.emit(code.OpWriteString, c.addStringConstant(value))
}

func (c *Compiler) emitWriteHTML(value string) int {
	if index, ok := c.lastWriteConstantIndex(code.OpWriteHTML); ok {
		if previous, ok := htmlConstantValue(c.constants, index); ok {
			c.constants[index] = &object.Native{Value: template.HTML(previous + value)}
			return c.scopes[c.scopeIndex].lastInstruction.Position
		}
	}
	if index, ok := c.lastWriteConstantIndex(code.OpWriteString); ok {
		if previous, ok := stringConstantValue(c.constants, index); ok {
			c.constants[index] = &object.Native{
				Value: template.HTML(template.HTMLEscapeString(previous) + value),
			}
			c.currentInstructions()[c.scopes[c.scopeIndex].lastInstruction.Position] = byte(code.OpWriteHTML)
			c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpWriteHTML
			return c.scopes[c.scopeIndex].lastInstruction.Position
		}
	}
	return c.emit(code.OpWriteHTML, c.addHTMLConstant(value))
}

func (c *Compiler) lastWriteConstantIndex(op code.Opcode) (int, bool) {
	last := c.scopes[c.scopeIndex].lastInstruction
	if last.Opcode != op {
		return 0, false
	}
	instructions := c.currentInstructions()
	if last.Position < 0 || last.Position >= len(instructions) {
		return 0, false
	}
	def, err := code.Lookup(byte(last.Opcode))
	if err != nil || len(def.OperandWidths) != 1 {
		return 0, false
	}
	operands, _ := code.ReadOperands(def, instructions[last.Position+1:])
	return operands[0], true
}

func staticOutputFromInstructions(instructions code.Instructions, constants []object.Object) (string, bool) {
	var out strings.Builder
	for i := 0; i < len(instructions); {
		op := code.Opcode(instructions[i])
		def, err := code.Lookup(instructions[i])
		if err != nil {
			return "", false
		}
		operands, read := code.ReadOperands(def, instructions[i+1:])
		switch op {
		case code.OpWriteHTML:
			value, ok := htmlConstantValue(constants, operands[0])
			if !ok {
				return "", false
			}
			out.WriteString(value)
		case code.OpWriteString:
			value, ok := stringConstantValue(constants, operands[0])
			if !ok {
				return "", false
			}
			out.WriteString(template.HTMLEscapeString(value))
		case code.OpWriteConstant:
			value, ok := staticConstantOutput(constants, operands[0])
			if !ok {
				return "", false
			}
			out.WriteString(value)
		default:
			return "", false
		}
		i += 1 + read
	}
	return out.String(), true
}

func stringConstantValue(constants []object.Object, index int) (string, bool) {
	if index < 0 || index >= len(constants) {
		return "", false
	}
	obj, ok := constants[index].(*object.String)
	if !ok {
		return "", false
	}
	return obj.Value, true
}

func htmlConstantValue(constants []object.Object, index int) (string, bool) {
	if index < 0 || index >= len(constants) {
		return "", false
	}
	obj, ok := constants[index].(*object.Native)
	if !ok {
		return "", false
	}
	html, ok := obj.Value.(template.HTML)
	if !ok {
		return "", false
	}
	return string(html), true
}

func staticConstantOutput(constants []object.Object, index int) (string, bool) {
	if index < 0 || index >= len(constants) {
		return "", false
	}
	obj := constants[index]
	if object.IsNull(obj) {
		return "", true
	}
	switch obj := obj.(type) {
	case *object.Integer, *object.Float, *object.Boolean:
		return template.HTMLEscaper(obj.Inspect()), true
	case *object.String:
		return template.HTMLEscapeString(obj.Value), true
	case *object.Native:
		if html, ok := obj.Value.(template.HTML); ok {
			return string(html), true
		}
	}
	return "", false
}
