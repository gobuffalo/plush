package compiler

import (
	"html/template"
	"testing"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/parser"
	"github.com/gobuffalo/plush/v5/token"
	"github.com/stretchr/testify/require"
)

type compilerTestCase struct {
	input                string
	expectedConstants    []interface{}
	expectedInstructions []code.Instructions
}

func Test_Integer_Arithmetic(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             "1 + 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpAdd),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 - 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSub),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "-1",
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpMinus),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func Test_Comparison_Operand_Optimization(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             "1 < 2",
			expectedConstants: []interface{}{2, 1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGreaterThan),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 <= 2",
			expectedConstants: []interface{}{2, 1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGreaterEqual),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 > 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGreaterThan),
				code.Make(code.OpPop),
			},
		},
		{
			input:             "1 >= 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGreaterEqual),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func Test_Infix_Operator_Opcode_Branches(t *testing.T) {
	tests := []struct {
		input string
		op    code.Opcode
	}{
		{input: `2 * 3`, op: code.OpMul},
		{input: `6 / 2`, op: code.OpDiv},
		{input: `1 == 1`, op: code.OpEqual},
		{input: `1 != 2`, op: code.OpNotEqual},
		{input: `"abc" ~= "a"`, op: code.OpMatches},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program, err := ParseScript(tt.input)
			require.NoError(t, err)

			compiler := New()
			require.NoError(t, compiler.Compile(program))
			require.Truef(t, bytecodeContainsOpcode(compiler.Bytecode(), tt.op), "expected opcode %s in:\n%s", tt.op, compiler.Bytecode().Instructions.String())
		})
	}
}

func Test_Compiler_Hash_Index_And_Receiver_Branch_Opcodes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		opcodes []code.Opcode
	}{
		{
			name:    "hash literal identifier and string keys",
			input:   `{name: 1, "age": 2}`,
			opcodes: []code.Opcode{code.OpHash},
		},
		{
			name:    "index read",
			input:   `items[0]`,
			opcodes: []code.Opcode{code.OpIndex},
		},
		{
			name:    "index write",
			input:   `items[0] = "x"`,
			opcodes: []code.Opcode{code.OpSetIndex},
		},
		{
			name:    "nested indexed receiver call",
			input:   `robots[0].Name.Echo()`,
			opcodes: []code.Opcode{code.OpIndex, code.OpGetProperty, code.OpCall},
		},
		{
			name:    "call result receiver with index",
			input:   `factory().Robots()[0].Name`,
			opcodes: []code.Opcode{code.OpCall, code.OpIndex, code.OpGetProperty},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := ParseScript(tt.input)
			require.NoError(t, err)

			compiler := New()
			require.NoError(t, compiler.Compile(program))
			instructions := compiler.Bytecode().Instructions

			for _, op := range tt.opcodes {
				require.Truef(t, instructionContainsOpcode(instructions, op), "expected %s in:\n%s", op, instructions.String())
			}
		})
	}
}

func Test_Compiler_Call_Name_And_Partial_Data_Key_Helpers(t *testing.T) {
	require.Equal(t, anonymousCallName, callExpressionName(nil))
	require.Equal(t, "helper", callExpressionName(&ast.CallExpression{Function: &ast.Identifier{Value: "helper"}}))
	require.Equal(t, "Name", expressionCallName(&ast.Identifier{Callee: &ast.Identifier{Value: "robot"}, Value: "Name"}))
	require.Equal(t, "robots", expressionCallName(&ast.IndexExpression{Left: &ast.Identifier{Value: "robots"}}))
	require.Equal(t, "helper", expressionCallName(&ast.CallExpression{Function: &ast.Identifier{Value: "helper"}}))
	require.Equal(t, "", expressionCallName(&ast.StringLiteral{Value: "not-callable"}))

	key, ok := fastPartialDataKey(&ast.Identifier{Value: "label"})
	require.True(t, ok)
	require.Equal(t, "label", key)

	key, ok = fastPartialDataKey(&ast.StringLiteral{Value: "label"})
	require.True(t, ok)
	require.Equal(t, "label", key)

	_, ok = fastPartialDataKey(&ast.Identifier{Value: ""})
	require.False(t, ok)

	_, ok = fastPartialDataKey(&ast.Identifier{Callee: &ast.Identifier{Value: "data"}, Value: "label"})
	require.False(t, ok)

	_, ok = fastPartialDataKey(&ast.IntegerLiteral{Value: 1})
	require.False(t, ok)
}

func Test_Compiler_Receiver_Chain_Helper_Branches(t *testing.T) {
	robotName := &ast.Identifier{Callee: &ast.Identifier{Value: "robot"}, Value: "Name"}
	require.Equal(t, []string{"robot", "Name"}, identifierParts(robotName))
	require.Nil(t, identifierParts(nil))

	require.Equal(t, []string{"Name"}, trimReceiverParts([]string{"robot", "Name"}, "robot"))
	require.Equal(t, []string{"Name"}, trimReceiverParts([]string{"robots[0]", "Name"}, "robots[0]"))
	require.Equal(t, []string{"robot", "Name"}, trimReceiverParts([]string{"robot", "Name"}, "factory"))
	require.Empty(t, trimReceiverParts(nil, "robot"))

	require.Equal(t, []string{"robot", "Name", "Echo"}, splitChainBase("robot.Name[0].Echo()"))
	require.Equal(t, []string{"robot", "Name"}, splitChainBase("(robot).Name"))
	require.Empty(t, splitChainBase(""))

	require.Equal(t, "Name", lastChainPart(robotName))
	require.Equal(t, "", lastChainPart(&ast.Identifier{}))
	require.Equal(t, "robots", lastChainPart(&ast.IndexExpression{Left: &ast.Identifier{Value: "robots"}}))
	require.Equal(t, "factory", lastChainPart(&ast.CallExpression{Function: &ast.Identifier{Value: "factory"}}))
	require.Equal(t, "Name", lastChainPart(&ast.IntegerLiteral{TokenAble: ast.TokenAble{Token: token.Token{Literal: "robot.Name"}}}))
	require.Equal(t, "Name", lastStringPart("factory().Robots()[0].Name"))
	require.Equal(t, "", lastStringPart(""))

	value := FastValuePlan{}
	require.True(t, appendFastReceiverCallee(&value, robotName, "robot", 7))
	require.Equal(t, []FastPathStep{{
		Kind:     FastPathStepProperty,
		Value:    "Name",
		Receiver: "robot",
		Full:     "robot.Name",
		Line:     7,
	}}, value.Path)

	value = FastValuePlan{}
	require.True(t, appendFastReceiverCallee(&value, &ast.IndexExpression{
		Left:   &ast.Identifier{Value: "robots"},
		Index:  &ast.IntegerLiteral{Value: 0},
		Callee: &ast.Identifier{Value: "Name"},
	}, "", 8))
	require.Equal(t, []FastPathStepKind{FastPathStepProperty, FastPathStepIndexInteger, FastPathStepProperty}, []FastPathStepKind{
		value.Path[0].Kind,
		value.Path[1].Kind,
		value.Path[2].Kind,
	})
	require.Equal(t, "robots", value.Path[0].Value)
	require.Equal(t, 0, value.Path[1].Index)
	require.Equal(t, "Name", value.Path[2].Value)

	value = FastValuePlan{}
	require.True(t, appendFastReceiverCallee(&value, &ast.CallExpression{
		Function:    &ast.Identifier{Value: "Robots"},
		ChainCallee: &ast.Identifier{Value: "Name"},
	}, "factory", 9))
	require.Equal(t, []FastPathStepKind{FastPathStepProperty, FastPathStepCall, FastPathStepProperty}, []FastPathStepKind{
		value.Path[0].Kind,
		value.Path[1].Kind,
		value.Path[2].Kind,
	})
	require.True(t, value.Path[0].Method)
	require.Equal(t, "Robots", value.Path[1].Value)
	require.Equal(t, "Name", value.Path[2].Value)

	value = FastValuePlan{}
	require.False(t, appendFastReceiverCallee(&value, &ast.IndexExpression{
		Left:  &ast.Identifier{Value: "robots"},
		Index: &ast.Identifier{Value: "dynamic"},
	}, "", 10))

	value = FastValuePlan{}
	require.False(t, appendFastReceiverCallee(&value, &ast.CallExpression{
		Function:  &ast.Identifier{Value: "helper"},
		Arguments: []ast.Expression{&ast.Identifier{Value: "name"}},
	}, "", 11))

	value = FastValuePlan{}
	require.False(t, appendFastReceiverCallee(&value, &ast.StringLiteral{Value: "nope"}, "", 12))
}

func Test_Compiler_Line_Helpers(t *testing.T) {
	require.Equal(t, 1, lineForNode(nil))
	require.Equal(t, 1, lineForNode(&ast.Identifier{}))
	require.Equal(t, 23, lineForNode(&ast.Identifier{TokenAble: ast.TokenAble{Token: token.Token{LineNumber: 23}}}))
	require.Equal(t, 1, lineForToken(ast.TokenAble{}))
	require.Equal(t, 24, lineForToken(ast.TokenAble{Token: token.Token{LineNumber: 24}}))
}

func Test_Compiler_Emit_And_Symbol_Helper_Branches(t *testing.T) {
	compiler := New()
	require.Nil(t, compiler.currentLineNumbers())
	require.Zero(t, compiler.globalCount())
	compiler.globalNames[3] = "late"
	require.Equal(t, 4, compiler.globalCount())

	require.False(t, compiler.lastInstructionIs(code.OpAdd))

	pos := compiler.emit(code.OpAdd)
	require.True(t, compiler.lastInstructionIs(code.OpAdd))
	require.False(t, compiler.lastInstructionIs(code.OpSub))

	compiler.recordCallName(pos, "")
	require.Equal(t, anonymousCallName, compiler.scopes[compiler.scopeIndex].callNames[pos])

	compiler.recordLocalName(Symbol{Name: "global", Scope: GlobalScope, Index: 2})
	require.Empty(t, compiler.scopes[compiler.scopeIndex].localNames)
	compiler.recordLocalName(Symbol{Name: "local", Scope: LocalScope, Index: 3})
	require.Equal(t, "local", compiler.scopes[compiler.scopeIndex].localNames[3])
	require.Equal(t, 4, compiler.scopes[compiler.scopeIndex].numLocals)

	compiler.markLastPropertyAsMethod()
	propertyPos := compiler.emitProperty("Name", "robot", "robot.Name")
	compiler.markLastPropertyAsMethod()
	property := compiler.scopes[compiler.scopeIndex].properties[propertyPos]
	require.True(t, property.Method)
	require.Equal(t, "robot.Name", property.Full)

	for _, symbol := range []Symbol{
		{Name: "global", Scope: GlobalScope, Index: 0},
		{Name: "local", Scope: LocalScope, Index: 1},
		{Name: "builtin", Scope: BuiltinScope, Index: 2},
		{Name: "free", Scope: FreeScope, Index: 3},
		{Name: "fn", Scope: FunctionScope},
	} {
		compiler.loadSymbol(symbol)
	}
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetGlobal))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetLocal))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetBuiltin))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetFree))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpCurrentClosure))

	require.Equal(t, anonymousCallName, callExpressionName(nil))
	require.Equal(t, anonymousCallName, callExpressionName(&ast.CallExpression{Function: &ast.StringLiteral{Value: "literal"}}))
	require.Empty(t, expressionCallName(&ast.Identifier{}))
	require.Equal(t, "items", expressionCallName(&ast.IndexExpression{Left: &ast.Identifier{Value: "items"}}))
	require.Equal(t, "helper", expressionCallName(&ast.CallExpression{Function: &ast.Identifier{Value: "helper"}}))
}

func Test_Compiler_Direct_Expression_Helper_Branches(t *testing.T) {
	compiler := New()
	err := compiler.compileInfixExpression(&ast.InfixExpression{
		Left:     &ast.IntegerLiteral{Value: 1},
		Operator: "??",
		Right:    &ast.IntegerLiteral{Value: 2},
	})
	require.ErrorContains(t, err, "unknown operator ??")

	compiler = New()
	nameKey := &ast.Identifier{TokenAble: ast.TokenAble{Token: token.Token{Literal: "name"}}, Value: "name"}
	ageKey := &ast.StringLiteral{TokenAble: ast.TokenAble{Token: token.Token{Literal: "age"}}, Value: "age"}
	hash := &ast.HashLiteral{
		Pairs: map[ast.Expression]ast.Expression{
			nameKey: &ast.IntegerLiteral{Value: 1},
			ageKey:  &ast.IntegerLiteral{Value: 2},
		},
	}
	require.NoError(t, compiler.compileHashLiteral(hash))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpHash))

	compiler = New()
	firstKey := &ast.StringLiteral{Value: "first"}
	orderedHash := &ast.HashLiteral{
		Order: []ast.Expression{firstKey},
		Pairs: map[ast.Expression]ast.Expression{
			firstKey: &ast.Boolean{Value: true},
		},
	}
	require.NoError(t, compiler.compileHashLiteral(orderedHash))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpTrue))

	compiler = New()
	err = compiler.compileReceiverCallee(&ast.StringLiteral{Value: "bad"}, "")
	require.ErrorContains(t, err, "unsupported chained callee")

	compiler = New()
	require.NoError(t, compiler.compileReceiverCallee(&ast.IndexExpression{
		Left:   &ast.Identifier{Value: "robots"},
		Index:  &ast.IntegerLiteral{Value: 0},
		Value:  &ast.StringLiteral{Value: "updated"},
		Callee: &ast.Identifier{Value: "Name"},
	}, ""))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpSetIndex))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetProperty))

	compiler = New()
	require.NoError(t, compiler.compileReceiverCallee(&ast.CallExpression{
		Function: &ast.CallExpression{Function: &ast.Identifier{Value: "factory"}},
		Arguments: []ast.Expression{
			&ast.StringLiteral{Value: "arg"},
		},
		ChainCallee: &ast.Identifier{Value: "Name"},
	}, ""))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpCall))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetProperty))
}

func Test_Compiler_Fast_Value_And_Partial_Data_Helper_Edges(t *testing.T) {
	plan := &FastRenderPlan{}
	require.True(t, fastInfixOperator("=="))
	require.False(t, fastInfixOperator("??"))

	_, ok := fastValuePlanFromInfixExpression(plan, nil, 1)
	require.False(t, ok)

	value, ok := fastValuePlanFromInfixExpression(plan, parseCompilerExpression(t, `name + 1`).(*ast.InfixExpression), 1)
	require.True(t, ok)
	require.Equal(t, FastValueConcat, value.Kind)

	value, ok = fastValuePlanFromInfixExpression(plan, parseCompilerExpression(t, `name == 1`).(*ast.InfixExpression), 1)
	require.True(t, ok)
	require.Equal(t, FastValueInfix, value.Kind)
	require.Equal(t, "==", value.Operator)

	_, ok = fastValuePlanFromInfixExpression(plan, &ast.InfixExpression{
		Operator: "==",
		Left:     &ast.FunctionLiteral{},
		Right:    &ast.IntegerLiteral{Value: 1},
	}, 1)
	require.False(t, ok)

	_, ok = fastValuePlanFromInfixExpression(plan, &ast.InfixExpression{
		Operator: "==",
		Left:     &ast.IntegerLiteral{Value: 1},
		Right:    &ast.FunctionLiteral{},
	}, 1)
	require.False(t, ok)

	value, ok = fastValuePlanFromIndexExpression(plan, parseCompilerExpression(t, `items[0]`).(*ast.IndexExpression), false, 1)
	require.True(t, ok)
	require.Equal(t, FastValuePath, value.Kind)
	require.Equal(t, FastPathStepIndexInteger, value.Path[0].Kind)

	_, ok = fastValuePlanFromIndexExpression(plan, parseCompilerExpression(t, `1[0]`).(*ast.IndexExpression), false, 1)
	require.False(t, ok)

	_, ok = fastValuePlanFromIndexExpression(plan, parseCompilerExpression(t, `items[key]`).(*ast.IndexExpression), false, 1)
	require.False(t, ok)

	_, ok = fastPartialDataPlanFromExpression(plan, &ast.Identifier{Value: "name"}, 1)
	require.False(t, ok)

	data, ok := fastPartialDataPlanFromExpression(plan, &ast.HashLiteral{
		Pairs: map[ast.Expression]ast.Expression{
			&ast.StringLiteral{Value: "b"}: &ast.StringLiteral{Value: "B"},
			&ast.StringLiteral{Value: "a"}: &ast.IntegerLiteral{Value: 1},
		},
	}, 1)
	require.True(t, ok)
	require.ElementsMatch(t, []string{"a", "b"}, []string{data[0].Key, data[1].Key})
	dataByKey := map[string]FastValuePlan{}
	for _, pair := range data {
		dataByKey[pair.Key] = pair.Value
	}
	require.Equal(t, FastValueInteger, dataByKey["a"].Kind)
	require.Equal(t, FastValueString, dataByKey["b"].Kind)

	layoutKey := &ast.Identifier{Value: "layout"}
	_, ok = fastPartialDataPlanFromExpression(plan, &ast.HashLiteral{
		Order: []ast.Expression{layoutKey},
		Pairs: map[ast.Expression]ast.Expression{
			layoutKey: &ast.StringLiteral{Value: "shell"},
		},
	}, 1)
	require.False(t, ok)

	invalidKey := &ast.IntegerLiteral{Value: 1}
	_, ok = fastPartialDataPlanFromExpression(plan, &ast.HashLiteral{
		Order: []ast.Expression{invalidKey},
		Pairs: map[ast.Expression]ast.Expression{
			invalidKey: &ast.StringLiteral{Value: "bad"},
		},
	}, 1)
	require.False(t, ok)

	unsupportedValueKey := &ast.Identifier{Value: "bad"}
	_, ok = fastPartialDataPlanFromExpression(plan, &ast.HashLiteral{
		Order: []ast.Expression{unsupportedValueKey},
		Pairs: map[ast.Expression]ast.Expression{
			unsupportedValueKey: &ast.FunctionLiteral{},
		},
	}, 1)
	require.False(t, ok)
}

func Test_Compiler_Fast_Loop_Value_Helper_Edges(t *testing.T) {
	plan := &FastRenderPlan{}
	loop := &FastLoopPlan{KeyName: "i", ValueName: "product"}

	_, ok := fastValuePlanFromLoopOperand(plan, nil, &ast.Identifier{Value: "product"}, false, 1)
	require.False(t, ok)

	_, ok = fastValuePlanFromLoopInfix(plan, loop, nil, 1)
	require.False(t, ok)

	value, ok := fastValuePlanFromLoopInfix(plan, loop, parseCompilerExpression(t, `product.Name + "x"`).(*ast.InfixExpression), 1)
	require.True(t, ok)
	require.Equal(t, FastValueConcat, value.Kind)

	_, ok = fastValuePlanFromLoopInfix(plan, loop, parseCompilerExpression(t, `i.Name == "x"`).(*ast.InfixExpression), 1)
	require.False(t, ok)

	value, ok = fastValuePlanFromLoopInfix(plan, loop, parseCompilerExpression(t, `product.Name == "x"`).(*ast.InfixExpression), 1)
	require.True(t, ok)
	require.Equal(t, FastValueInfix, value.Kind)

	root, ok := fastLoopExpressionRootName(parseCompilerExpression(t, `product.Tags[0]`))
	require.True(t, ok)
	require.Equal(t, "product", root)

	_, ok = fastLoopExpressionRootName(&ast.StringLiteral{Value: "nope"})
	require.False(t, ok)

	require.False(t, isFastLoopKeyIdentifier(loop, parseCompilerExpression(t, `i.Name`)))
	require.True(t, isFastLoopKeyIdentifier(loop, &ast.Identifier{Value: "i"}))

	_, ok = fastValuePlanFromLoopIndex(loop, &ast.Identifier{Value: "product"}, 1)
	require.False(t, ok)

	_, ok = fastValuePlanFromLoopIndex(loop, parseCompilerExpression(t, `other[0]`), 1)
	require.False(t, ok)

	_, ok = fastValuePlanFromLoopIndex(loop, parseCompilerExpression(t, `product[dynamic]`), 1)
	require.False(t, ok)

	value, ok = fastValuePlanFromLoopIndex(loop, parseCompilerExpression(t, `product.Tags[0].Name`), 1)
	require.True(t, ok)
	require.Equal(t, FastValuePath, value.Kind)
	require.Equal(t, []FastPathStepKind{FastPathStepProperty, FastPathStepIndexInteger, FastPathStepProperty}, []FastPathStepKind{
		value.Path[0].Kind,
		value.Path[1].Kind,
		value.Path[2].Kind,
	})

	_, ok = fastValuePlanFromLoopCall(loop, nil, 1)
	require.False(t, ok)

	_, ok = fastValuePlanFromLoopCall(loop, &ast.CallExpression{
		Function:  &ast.Identifier{Value: "Echo"},
		Arguments: []ast.Expression{&ast.StringLiteral{Value: "arg"}},
	}, 1)
	require.False(t, ok)

	_, ok = fastValuePlanFromLoopCall(loop, &ast.CallExpression{
		Function: &ast.Identifier{Value: "Echo"},
		Block:    &ast.BlockStatement{},
	}, 1)
	require.False(t, ok)

	value, ok = fastValuePlanFromLoopCall(loop, parseCompilerExpression(t, `product.Name.Echo()`).(*ast.CallExpression), 1)
	require.True(t, ok)
	require.Equal(t, FastPathStepCall, value.Path[len(value.Path)-1].Kind)

	_, ok = fastLoopCallPlanFromExpression(plan, loop, nil, 1)
	require.False(t, ok)

	_, ok = fastLoopCallPlanFromExpression(plan, loop, &ast.CallExpression{
		Function:    &ast.Identifier{Value: "label"},
		ChainCallee: &ast.Identifier{Value: "Echo"},
	}, 1)
	require.False(t, ok)

	_, ok = fastLoopCallPlanFromExpression(plan, loop, &ast.CallExpression{
		Function: &ast.Identifier{Callee: &ast.Identifier{Value: "helpers"}, Value: "label"},
	}, 1)
	require.False(t, ok)
}

func Test_Compiler_Fast_Loop_And_Conditional_Plan_Edges(t *testing.T) {
	plan := &FastRenderPlan{}
	loop := &FastLoopPlan{KeyName: "i", ValueName: "product"}

	_, ok := fastLoopPlanFromExpression(plan, nil, 1)
	require.False(t, ok)

	_, ok = fastLoopPlanFromExpression(plan, &ast.ForExpression{}, 1)
	require.False(t, ok)

	_, ok = fastLoopPlanFromExpression(plan, &ast.ForExpression{
		Iterable: &ast.StringLiteral{Value: "products"},
		Block:    &ast.BlockStatement{},
	}, 1)
	require.False(t, ok)

	_, ok = fastLoopPlanFromExpression(plan, &ast.ForExpression{
		Iterable: &ast.Identifier{Value: "nil"},
		Block:    &ast.BlockStatement{},
	}, 1)
	require.False(t, ok)

	fastLoop, ok := fastLoopPlanFromExpression(plan, &ast.ForExpression{
		KeyName:   "i",
		ValueName: "product",
		Iterable:  &ast.Identifier{Value: "products"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.HTMLLiteral{Value: "<li>"}},
			&ast.ReturnStatement{Type: token.E_START, ReturnValue: &ast.Identifier{Value: "product"}},
		}},
	}, 7)
	require.True(t, ok)
	require.Equal(t, "products", fastLoop.IterableName)
	require.Equal(t, []FastLoopPartKind{FastLoopPartStatic, FastLoopPartValue}, []FastLoopPartKind{
		fastLoop.Parts[0].Kind,
		fastLoop.Parts[1].Kind,
	})

	parts := []FastLoopPart{}
	require.False(t, appendFastLoopStatements(plan, loop, &parts, []ast.Statement{&ast.BlockStatement{}}))
	require.False(t, appendFastLoopStatement(plan, loop, &parts, &ast.ReturnStatement{Type: token.RETURN, ReturnValue: &ast.Identifier{Value: "product"}}))
	require.False(t, appendFastLoopStatement(plan, loop, &parts, &ast.ExpressionStatement{Expression: &ast.Identifier{Value: "notHTML"}}))

	parts = nil
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.StringLiteral{Value: "<x>"}, 1))
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.HTMLLiteral{Value: "<b>"}, 1))
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.IntegerLiteral{Value: 7}, 1))
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.FloatLiteral{Value: 1.5}, 1))
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.Boolean{Value: true}, 1))
	require.Equal(t, FastLoopPartStatic, parts[0].Kind)
	require.Contains(t, parts[0].Value, "&lt;x&gt;")
	require.Contains(t, parts[0].Value, "<b>")
	require.Contains(t, parts[0].Value, "71.5true")

	parts = nil
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.Identifier{Value: "i"}, 2))
	require.Equal(t, FastLoopPartKey, parts[0].Kind)

	parts = nil
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.Identifier{Value: "product"}, 2))
	require.Equal(t, FastLoopPartValue, parts[0].Kind)

	parts = nil
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, parseCompilerExpression(t, `product.Name`), 2))
	require.Equal(t, FastLoopPartValueProperty, parts[0].Kind)

	parts = nil
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, parseCompilerExpression(t, `product.Profile.Name`), 2))
	require.Equal(t, FastLoopPartValuePath, parts[0].Kind)

	parts = nil
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, parseCompilerExpression(t, `product.Name.Echo()`), 2))
	require.Equal(t, FastLoopPartValuePath, parts[0].Kind)

	parts = nil
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, parseCompilerExpression(t, `label(product.Name, i)`), 2))
	require.Equal(t, FastLoopPartCall, parts[0].Kind)

	parts = nil
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.Identifier{Value: "other"}, 2))
	require.Equal(t, FastLoopPartValuePath, parts[0].Kind)

	_, ok = fastConditionalPlanFromExpression(plan, nil, 1)
	require.False(t, ok)

	_, ok = fastConditionalPlanFromExpression(plan, &ast.IfExpression{
		Condition: &ast.FunctionLiteral{},
		Block:     &ast.BlockStatement{},
	}, 1)
	require.False(t, ok)

	_, ok = fastConditionalPlanFromExpression(plan, &ast.IfExpression{
		Condition: &ast.Boolean{Value: true},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.BlockStatement{},
		}},
	}, 1)
	require.False(t, ok)

	_, ok = fastConditionalPlanFromExpression(plan, &ast.IfExpression{
		Condition: &ast.Boolean{Value: true},
		Block:     &ast.BlockStatement{},
		ElseIf:    []*ast.ElseIfExpression{nil},
	}, 1)
	require.False(t, ok)

	conditional, ok := fastConditionalPlanFromExpression(plan, &ast.IfExpression{
		Condition: &ast.Boolean{Value: true},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.HTMLLiteral{Value: "yes"}},
		}},
		ElseIf: []*ast.ElseIfExpression{{
			Condition: &ast.Identifier{Value: "fallback"},
			Block: &ast.BlockStatement{Statements: []ast.Statement{
				&ast.ExpressionStatement{Expression: &ast.HTMLLiteral{Value: "fallback"}},
			}},
		}},
		ElseBlock: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.HTMLLiteral{Value: "no"}},
		}},
	}, 1)
	require.True(t, ok)
	require.Len(t, conditional.Branches, 2)
	require.Len(t, conditional.ElseSegments, 1)

	_, ok = fastLoopConditionalPlanFromExpression(plan, loop, nil, 1)
	require.False(t, ok)

	_, ok = fastLoopConditionalPlanFromExpression(plan, loop, &ast.IfExpression{
		Condition: &ast.Boolean{Value: true},
	}, 1)
	require.False(t, ok)

	_, ok = fastLoopConditionalPlanFromExpression(plan, loop, &ast.IfExpression{
		Condition: &ast.FunctionLiteral{},
		Block:     &ast.BlockStatement{},
	}, 1)
	require.False(t, ok)

	_, ok = fastLoopConditionalPlanFromExpression(plan, loop, &ast.IfExpression{
		Condition: &ast.Boolean{Value: true},
		Block:     &ast.BlockStatement{},
		ElseIf:    []*ast.ElseIfExpression{nil},
	}, 1)
	require.False(t, ok)

	_, ok = fastLoopConditionalPlanFromExpression(plan, loop, &ast.IfExpression{
		Condition: &ast.Boolean{Value: true},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.BlockStatement{},
		}},
	}, 1)
	require.False(t, ok)

	loopConditional, ok := fastLoopConditionalPlanFromExpression(plan, loop, &ast.IfExpression{
		Condition: &ast.Identifier{Value: "product"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ReturnStatement{Type: token.E_START, ReturnValue: &ast.Identifier{Value: "product"}},
		}},
		ElseIf: []*ast.ElseIfExpression{{
			Condition: &ast.Identifier{Value: "i"},
			Block: &ast.BlockStatement{Statements: []ast.Statement{
				&ast.ReturnStatement{Type: token.E_START, ReturnValue: &ast.Identifier{Value: "i"}},
			}},
		}},
		ElseBlock: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.HTMLLiteral{Value: "none"}},
		}},
	}, 1)
	require.True(t, ok)
	require.Len(t, loopConditional.Branches, 2)
	require.Len(t, loopConditional.ElseParts, 1)
}

func Test_Compiler_Core_Helper_Branch_Edges(t *testing.T) {
	loopControlBlock := func(expr ast.Expression) *ast.BlockStatement {
		return &ast.BlockStatement{
			Statements: []ast.Statement{
				&ast.ExpressionStatement{Expression: expr},
			},
		}
	}

	require.False(t, ifExpressionHasLoopControl(nil))
	require.False(t, ifExpressionHasLoopControl(&ast.IfExpression{
		Block: &ast.BlockStatement{
			Statements: []ast.Statement{
				&ast.LetStatement{Name: &ast.Identifier{Value: "x"}, Value: &ast.IntegerLiteral{Value: 1}},
			},
		},
	}))
	require.True(t, ifExpressionHasLoopControl(&ast.IfExpression{
		Block: loopControlBlock(&ast.BreakExpression{}),
	}))
	require.True(t, ifExpressionHasLoopControl(&ast.IfExpression{
		ElseBlock: loopControlBlock(&ast.ContinueExpression{}),
	}))
	require.True(t, ifExpressionHasLoopControl(&ast.IfExpression{
		ElseIf: []*ast.ElseIfExpression{
			{Block: loopControlBlock(&ast.ContinueExpression{})},
		},
	}))
	require.False(t, statementHasLoopControl(nil))
	require.False(t, statementHasLoopControl(&ast.LetStatement{Name: &ast.Identifier{Value: "x"}, Value: &ast.IntegerLiteral{Value: 1}}))
	require.True(t, statementHasLoopControl(&ast.ExpressionStatement{Expression: &ast.BreakExpression{}}))
	require.True(t, statementHasLoopControl(&ast.ExpressionStatement{Expression: &ast.ContinueExpression{}}))
	require.True(t, statementHasLoopControl(&ast.ExpressionStatement{Expression: &ast.IfExpression{
		Block: loopControlBlock(&ast.BreakExpression{}),
	}}))
	require.False(t, statementHasLoopControl(&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 1}}))

	compiler := New()
	require.NoError(t, compiler.compileExpressionStatement(&ast.ExpressionStatement{Expression: &ast.HTMLLiteral{Value: "<b>"}}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpWriteHTML))

	compiler = New()
	require.NoError(t, compiler.compileExpressionStatement(&ast.ExpressionStatement{Expression: &ast.IfExpression{
		Condition: &ast.Boolean{Value: true},
		Block:     loopControlBlock(&ast.BreakExpression{}),
	}}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpBreak))

	compiler = New()
	require.NoError(t, compiler.compileExpressionStatement(&ast.ExpressionStatement{Expression: &ast.BreakExpression{}}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpBreak))
	require.False(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpPop))

	compiler = New()
	require.NoError(t, compiler.compileExpressionStatement(&ast.ExpressionStatement{Expression: &ast.ContinueExpression{}}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpContinue))
	require.False(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpPop))

	compiler = New()
	require.NoError(t, compiler.compileIdentifier(&ast.Identifier{Value: "nil"}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpNull))

	compiler = New()
	compiler.softNames = 1
	require.NoError(t, compiler.compileIdentifier(&ast.Identifier{Value: "unknown"}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetNameOrNull))

	compiler = New()
	compiler.softNames = 1
	require.NoError(t, compiler.compileIdentifier(&ast.Identifier{Value: "len"}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetNameOrNull))

	compiler = New()
	require.NoError(t, compiler.compileIdentifier(&ast.Identifier{Callee: &ast.Identifier{Value: "robot"}, Value: "Name"}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetProperty))

	compiler = New()
	start := len(compiler.currentInstructions())
	compiler.ensureBranchValue(start)
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpNull))

	compiler = New()
	start = len(compiler.currentInstructions())
	compiler.emitConstant(&object.Integer{Value: 1})
	compiler.emit(code.OpPop)
	compiler.ensureBranchValue(start)
	require.False(t, compiler.lastInstructionIs(code.OpPop))

	compiler = New()
	start = len(compiler.currentInstructions())
	compiler.emit(code.OpWrite)
	compiler.ensureBranchValue(start)
	require.True(t, compiler.lastInstructionIs(code.OpNull))

	compiler = New()
	_, ok := compiler.lastWriteConstantIndex(code.OpWriteString)
	require.False(t, ok)

	pos := compiler.emitWriteString("hello")
	index, ok := compiler.lastWriteConstantIndex(code.OpWriteString)
	require.True(t, ok)
	require.Zero(t, index)

	compiler.scopes[compiler.scopeIndex].lastInstruction = EmittedInstruction{Opcode: code.OpWriteString, Position: len(compiler.currentInstructions()) + 10}
	_, ok = compiler.lastWriteConstantIndex(code.OpWriteString)
	require.False(t, ok)

	compiler.scopes[compiler.scopeIndex].lastInstruction = EmittedInstruction{Opcode: code.OpAdd, Position: pos}
	_, ok = compiler.lastWriteConstantIndex(code.OpAdd)
	require.False(t, ok)

	_, ok = compiledFunctionConstant(nil, 0)
	require.False(t, ok)

	_, ok = compiledFunctionConstant([]object.Object{&object.String{Value: "nope"}}, 0)
	require.False(t, ok)

	fn := &object.CompiledFunction{}
	gotFn, ok := compiledFunctionConstant([]object.Object{fn}, 0)
	require.True(t, ok)
	require.Same(t, fn, gotFn)

	require.Nil(t, NewSymbolTable().localDefinitionOwner())
}

func Test_Compile_For_Expression_Error_Branches(t *testing.T) {
	compiler := New()
	err := compiler.compileForExpression(&ast.ForExpression{
		KeyName:   "i",
		ValueName: "item",
		Iterable: &ast.PrefixExpression{
			Operator: "??",
			Right:    &ast.Boolean{Value: true},
		},
		Block: &ast.BlockStatement{},
	})
	require.ErrorContains(t, err, "unknown operator ??")

	compiler = New()
	err = compiler.compileForExpression(&ast.ForExpression{
		KeyName:   "i",
		ValueName: "item",
		Iterable:  &ast.ArrayLiteral{},
		Block: &ast.BlockStatement{
			Statements: []ast.Statement{
				&ast.ExpressionStatement{
					Expression: &ast.PrefixExpression{
						Operator: "??",
						Right:    &ast.Boolean{Value: true},
					},
				},
			},
		},
	})
	require.ErrorContains(t, err, "unknown operator ??")
}

func Test_Compiler_Expression_Error_Branches(t *testing.T) {
	badExpression := func() ast.Expression {
		return &ast.PrefixExpression{
			Operator: "??",
			Right:    &ast.Boolean{Value: true},
		}
	}
	badBlock := func() *ast.BlockStatement {
		return &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: badExpression()},
		}}
	}

	for _, tt := range []struct {
		name string
		node *ast.InfixExpression
	}{
		{
			name: "less-than right compile error",
			node: &ast.InfixExpression{Operator: "<", Left: &ast.IntegerLiteral{Value: 1}, Right: badExpression()},
		},
		{
			name: "less-than left compile error",
			node: &ast.InfixExpression{Operator: "<", Left: badExpression(), Right: &ast.IntegerLiteral{Value: 1}},
		},
		{
			name: "less-equal right compile error",
			node: &ast.InfixExpression{Operator: "<=", Left: &ast.IntegerLiteral{Value: 1}, Right: badExpression()},
		},
		{
			name: "less-equal left compile error",
			node: &ast.InfixExpression{Operator: "<=", Left: badExpression(), Right: &ast.IntegerLiteral{Value: 1}},
		},
		{
			name: "soft equality left compile error",
			node: &ast.InfixExpression{Operator: "==", Left: badExpression(), Right: &ast.IntegerLiteral{Value: 1}},
		},
		{
			name: "soft equality right compile error",
			node: &ast.InfixExpression{Operator: "==", Left: &ast.IntegerLiteral{Value: 1}, Right: badExpression()},
		},
		{
			name: "default left compile error",
			node: &ast.InfixExpression{Operator: "+", Left: badExpression(), Right: &ast.IntegerLiteral{Value: 1}},
		},
		{
			name: "default right compile error",
			node: &ast.InfixExpression{Operator: "+", Left: &ast.IntegerLiteral{Value: 1}, Right: badExpression()},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := New().compileInfixExpression(tt.node)
			require.ErrorContains(t, err, "unknown operator ??")
		})
	}

	require.NoError(t, New().compileInfixExpression(&ast.InfixExpression{
		Operator: "~=",
		Left:     &ast.StringLiteral{Value: "abc"},
		Right:    &ast.StringLiteral{Value: "a"},
	}))

	err := New().compileIfExpression(&ast.IfExpression{Condition: badExpression(), Block: &ast.BlockStatement{}})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileIfExpression(&ast.IfExpression{Condition: &ast.Boolean{Value: true}, Block: badBlock()})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileIfExpression(&ast.IfExpression{
		Condition: &ast.Boolean{Value: true},
		Block:     &ast.BlockStatement{},
		ElseIf: []*ast.ElseIfExpression{
			{Condition: badExpression(), Block: &ast.BlockStatement{}},
		},
	})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileIfExpression(&ast.IfExpression{
		Condition: &ast.Boolean{Value: true},
		Block:     &ast.BlockStatement{},
		ElseIf: []*ast.ElseIfExpression{
			{Condition: &ast.Boolean{Value: true}, Block: badBlock()},
		},
	})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileIfExpression(&ast.IfExpression{
		Condition: &ast.Boolean{Value: true},
		Block:     &ast.BlockStatement{},
		ElseBlock: badBlock(),
	})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileIndexExpression(&ast.IndexExpression{
		Left:  badExpression(),
		Index: &ast.IntegerLiteral{Value: 0},
	})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileIndexExpression(&ast.IndexExpression{
		Left:  &ast.Identifier{Value: "items"},
		Index: badExpression(),
	})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileIndexExpression(&ast.IndexExpression{
		Left:  &ast.Identifier{Value: "items"},
		Index: &ast.IntegerLiteral{Value: 0},
		Value: badExpression(),
	})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileIndexExpression(&ast.IndexExpression{
		Left:   &ast.Identifier{Value: "items"},
		Index:  &ast.IntegerLiteral{Value: 0},
		Callee: &ast.StringLiteral{Value: "bad"},
	})
	require.ErrorContains(t, err, "unsupported chained callee")

	err = New().compileAssignExpression(&ast.AssignExpression{
		Name:  &ast.Identifier{Value: "name"},
		Value: badExpression(),
	})
	require.ErrorContains(t, err, "unknown operator ??")

	compiler := New()
	require.NoError(t, compiler.compileAssignExpression(&ast.AssignExpression{
		Name:  &ast.Identifier{Value: "len"},
		Value: &ast.IntegerLiteral{Value: 1},
	}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpAssignName))
}

func Test_Compiler_Call_And_Receiver_Error_Branches(t *testing.T) {
	badExpression := func() ast.Expression {
		return &ast.PrefixExpression{
			Operator: "??",
			Right:    &ast.Boolean{Value: true},
		}
	}
	badBlock := func() *ast.BlockStatement {
		return &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: badExpression()},
		}}
	}

	err := New().compileReceiverCallee(&ast.IndexExpression{
		Left:  &ast.StringLiteral{Value: "bad"},
		Index: &ast.IntegerLiteral{Value: 0},
	}, "")
	require.ErrorContains(t, err, "unsupported chained callee")

	err = New().compileReceiverCallee(&ast.IndexExpression{
		Left:  &ast.Identifier{Value: "items"},
		Index: badExpression(),
	}, "")
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileReceiverCallee(&ast.IndexExpression{
		Left:  &ast.Identifier{Value: "items"},
		Index: &ast.IntegerLiteral{Value: 0},
		Value: badExpression(),
	}, "")
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileReceiverCallee(&ast.IndexExpression{
		Left:   &ast.Identifier{Value: "items"},
		Index:  &ast.IntegerLiteral{Value: 0},
		Callee: &ast.StringLiteral{Value: "bad"},
	}, "")
	require.ErrorContains(t, err, "unsupported chained callee")

	err = New().compileReceiverCallee(&ast.CallExpression{
		Function: &ast.StringLiteral{Value: "bad"},
	}, "")
	require.ErrorContains(t, err, "unsupported chained callee")

	err = New().compileReceiverCallee(&ast.CallExpression{
		Function:  &ast.Identifier{Value: "label"},
		Arguments: []ast.Expression{badExpression()},
	}, "")
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileReceiverCallee(&ast.CallExpression{
		Function: &ast.Identifier{Value: "label"},
		Block:    badBlock(),
	}, "")
	require.ErrorContains(t, err, "unknown operator ??")

	compiler := New()
	err = compiler.compileReceiverCallee(&ast.CallExpression{
		Function: &ast.Identifier{Value: "label"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "ok"}),
		}},
	}, "")
	require.NoError(t, err)
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpCallBlock))

	err = New().compileReceiverCallee(&ast.CallExpression{
		Function:    &ast.Identifier{Value: "label"},
		ChainCallee: &ast.StringLiteral{Value: "bad"},
	}, "")
	require.ErrorContains(t, err, "unsupported chained callee")

	err = New().compileCallExpression(&ast.CallExpression{Function: badExpression()})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileCallExpression(&ast.CallExpression{
		Function:  &ast.Identifier{Value: "label"},
		Arguments: []ast.Expression{badExpression()},
	})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileCallExpression(&ast.CallExpression{
		Function: &ast.Identifier{Value: "label"},
		Block:    badBlock(),
	})
	require.ErrorContains(t, err, "unknown operator ??")

	err = New().compileCallExpression(&ast.CallExpression{
		Function:    &ast.Identifier{Value: "label"},
		ChainCallee: &ast.StringLiteral{Value: "bad"},
	})
	require.ErrorContains(t, err, "unsupported chained callee")
}

func Test_Compiler_Core_Compile_And_Helper_Error_Branches(t *testing.T) {
	badExpression := func() ast.Expression {
		return &ast.PrefixExpression{
			Operator: "??",
			Right:    &ast.Boolean{Value: true},
		}
	}
	badStatement := func() ast.Statement {
		return &ast.ExpressionStatement{Expression: badExpression()}
	}

	require.ErrorContains(t, New().Compile(&ast.Program{Statements: []ast.Statement{badStatement()}}), "unknown operator ??")
	require.ErrorContains(t, New().Compile(&ast.BlockStatement{Statements: []ast.Statement{badStatement()}}), "unknown operator ??")
	require.ErrorContains(t, New().Compile(&ast.ReturnStatement{Type: token.E_START, ReturnValue: badExpression()}), "unknown operator ??")

	compiler := New()
	require.NoError(t, compiler.Compile(&ast.ReturnStatement{Type: token.RETURN, ReturnValue: &ast.IntegerLiteral{Value: 7}}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpReturnValue))

	compiler = New()
	require.NoError(t, compiler.Compile(nil))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpNull))

	compiler = New()
	require.NoError(t, compiler.Compile(&ast.HTMLLiteral{Value: "<b>"}))
	require.Len(t, compiler.constants, 1)

	compiler = New()
	require.NoError(t, compiler.Compile(&ast.PrefixExpression{Operator: "!", Right: &ast.Boolean{Value: true}}))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpBang))

	require.ErrorContains(t, New().Compile(&ast.ArrayLiteral{Elements: []ast.Expression{badExpression()}}), "unknown operator ??")
	require.ErrorContains(t, New().Compile(&ast.PrefixExpression{Operator: "!", Right: badExpression()}), "unknown operator ??")
	require.ErrorContains(t, New().Compile(&ast.PrefixExpression{Operator: "-", Right: badExpression()}), "unknown operator ??")

	require.ErrorContains(t, New().compileLogicalAnd(&ast.InfixExpression{
		Operator: "&&",
		Left:     badExpression(),
		Right:    &ast.Boolean{Value: true},
	}), "unknown operator ??")
	require.ErrorContains(t, New().compileLogicalAnd(&ast.InfixExpression{
		Operator: "&&",
		Left:     &ast.Boolean{Value: true},
		Right:    badExpression(),
	}), "unknown operator ??")
	require.ErrorContains(t, New().compileLogicalOr(&ast.InfixExpression{
		Operator: "||",
		Left:     badExpression(),
		Right:    &ast.Boolean{Value: true},
	}), "unknown operator ??")
	require.ErrorContains(t, New().compileLogicalOr(&ast.InfixExpression{
		Operator: "||",
		Left:     &ast.Boolean{Value: true},
		Right:    badExpression(),
	}), "unknown operator ??")

	key := badExpression()
	require.ErrorContains(t, New().compileHashLiteral(&ast.HashLiteral{
		Order: []ast.Expression{key},
		Pairs: map[ast.Expression]ast.Expression{key: &ast.IntegerLiteral{Value: 1}},
	}), "unknown operator ??")

	key = &ast.StringLiteral{Value: "name"}
	require.ErrorContains(t, New().compileHashLiteral(&ast.HashLiteral{
		Order: []ast.Expression{key},
		Pairs: map[ast.Expression]ast.Expression{key: badExpression()},
	}), "unknown operator ??")

	compiler = New()
	handled, err := compiler.compileWriteExpression(&ast.Identifier{Value: "nil"})
	require.NoError(t, err)
	require.True(t, handled)

	compiler = New()
	handled, err = compiler.compileWriteExpression(&ast.HTMLLiteral{Value: "<b>"})
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpWriteHTML))

	compiler = New()
	handled, err = compiler.compileWriteNameCallExpression(&ast.CallExpression{
		Function:  &ast.Identifier{Value: "missing"},
		Arguments: []ast.Expression{badExpression()},
	})
	require.True(t, handled)
	require.ErrorContains(t, err, "unknown operator ??")

	compiler = New()
	handled, err = compiler.compileWriteIdentifierProperty(nil)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = compiler.compileWriteIdentifierProperty(&ast.Identifier{
		Callee: &ast.Identifier{Callee: &ast.Identifier{Value: "root"}, Value: "robot"},
		Value:  "Name",
	})
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = compiler.compileWriteIdentifierProperty(&ast.Identifier{
		Callee: &ast.Identifier{Value: "robot.name"},
		Value:  "Name",
	})
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = compiler.compileWriteIdentifierProperty(&ast.Identifier{
		Callee: &ast.Identifier{Value: "len"},
		Value:  "Name",
	})
	require.NoError(t, err)
	require.False(t, handled)

	require.ErrorContains(t, New().compileLetStatement(&ast.LetStatement{
		Name: &ast.Identifier{Value: "fn"},
		Value: &ast.FunctionLiteral{Block: &ast.BlockStatement{
			Statements: []ast.Statement{badStatement()},
		}},
	}), "unknown operator ??")

	require.ErrorContains(t, New().compileLetStatement(&ast.LetStatement{
		Name:  &ast.Identifier{Value: "x"},
		Value: badExpression(),
	}), "unknown operator ??")
}

func Test_Compiler_Free_Symbol_Loading_For_Functions_And_Blocks(t *testing.T) {
	compiler := New()
	require.NoError(t, compiler.compileFunctionLiteral(&ast.FunctionLiteral{Block: &ast.BlockStatement{}}, ""))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpClosure))

	compiler = New()
	compiler.enterScope()
	compiler.symbolTable.Define("name")

	require.NoError(t, compiler.compileFunctionLiteral(&ast.FunctionLiteral{
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.Identifier{Value: "name"}},
		}},
	}, ""))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetLocal))
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpClosure))

	compiler = New()
	compiler.enterScope()
	compiler.symbolTable.Define("name")

	blockIndex, numFree, err := compiler.compileBlockConstant(&ast.BlockStatement{Statements: []ast.Statement{
		compilerFast_Test_Return(&ast.Identifier{Value: "name"}),
	}}, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, blockIndex, 0)
	require.Equal(t, 1, numFree)
	require.True(t, instructionContainsOpcode(compiler.currentInstructions(), code.OpGetLocal))
}

func Test_Compiler_Script_Tag_Control_Blocks_Suppress_Nested_Output(t *testing.T) {
	program, err := parser.Parse(`<% if (true) { %>hidden<% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))
	instructions := compiler.Bytecode().Instructions
	require.Falsef(t, instructionContainsOpcode(instructions, code.OpWriteHTML), "script if should not write HTML:\n%s", instructions.String())
	require.Falsef(t, instructionContainsOpcode(instructions, code.OpWriteString), "script if should not write strings:\n%s", instructions.String())

	program, err = parser.Parse(`<% if (true) { %><%= "hidden" %><% } %>`)
	require.NoError(t, err)
	compiler = New()
	require.NoError(t, compiler.Compile(program))
	instructions = compiler.Bytecode().Instructions
	require.Falsef(t, instructionContainsOpcode(instructions, code.OpWriteString), "script if should not write nested eval output:\n%s", instructions.String())

	program, err = parser.Parse(`<%= if (true) { %>shown<% } %>`)
	require.NoError(t, err)
	compiler = New()
	require.NoError(t, compiler.Compile(program))
	instructions = compiler.Bytecode().Instructions
	require.Truef(t, instructionContainsOpcode(instructions, code.OpWrite), "output if should keep shared write:\n%s", instructions.String())
}

func Test_Compiler_Static_Write_Merge_Branches(t *testing.T) {
	compiler := New()
	first := compiler.emitWriteString("<")
	second := compiler.emitWriteString(">")
	require.Equal(t, first, second)
	require.Len(t, compiler.constants, 1)
	require.Equal(t, "<>", compiler.constants[0].(*object.String).Value)

	htmlPos := compiler.emitWriteHTML("<b>")
	require.Equal(t, first, htmlPos)
	require.Len(t, compiler.constants, 1)
	require.Equal(t, template.HTML("&lt;&gt;<b>"), compiler.constants[0].(*object.Native).Value)
	require.Equal(t, code.OpWriteHTML, compiler.scopes[compiler.scopeIndex].lastInstruction.Opcode)

	htmlCompiler := New()
	first = htmlCompiler.emitWriteHTML("<b>")
	second = htmlCompiler.emitWriteHTML("<i>")
	require.Equal(t, first, second)
	require.Len(t, htmlCompiler.constants, 1)
	require.Equal(t, template.HTML("<b><i>"), htmlCompiler.constants[0].(*object.Native).Value)

	mixedCompiler := New()
	first = mixedCompiler.emitWriteHTML("<b>")
	second = mixedCompiler.emitWriteString("<x>")
	require.Equal(t, first, second)
	require.Len(t, mixedCompiler.constants, 1)
	require.Equal(t, template.HTML("<b>&lt;x&gt;"), mixedCompiler.constants[0].(*object.Native).Value)
}

func Test_New_With_State_Keeps_Provided_State(t *testing.T) {
	symbols := NewSymbolTable()
	constants := []object.Object{&object.Integer{Value: 7}}

	compiler := NewWithState(symbols, constants)

	require.Same(t, symbols, compiler.symbolTable)
	require.Same(t, constants[0], compiler.constants[0])
}

func Test_Logical_Or_Compilation_Branch(t *testing.T) {
	program, err := ParseScript(`true || false`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	instructions := compiler.Bytecode().Instructions
	require.True(t, instructionContainsOpcode(instructions, code.OpJumpNotTruthy))
	require.True(t, instructionContainsOpcode(instructions, code.OpJump))
	require.True(t, instructionContainsOpcode(instructions, code.OpTrue))
}

func Test_Assign_Expression_Compilation_Branches(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  code.Opcode
	}{
		{
			name:  "unknown name assignment",
			input: `missing = 1`,
			want:  code.OpAssignName,
		},
		{
			name:  "global assignment",
			input: `let value = 1; value = 2`,
			want:  code.OpSetGlobal,
		},
		{
			name:  "local assignment",
			input: `fn() { let value = 1; value = 2 }`,
			want:  code.OpSetLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := ParseScript(tt.input)
			require.NoError(t, err)

			compiler := New()
			require.NoError(t, compiler.Compile(program))

			require.True(t, bytecodeContainsOpcode(compiler.Bytecode(), tt.want))
		})
	}
}

func Test_Soft_Undefined_Equality_Names(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             `3 == unknown; unknown != 3;`,
			expectedConstants: []interface{}{3, "unknown", "unknown", 3},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpGetNameOrNull, 1),
				code.Make(code.OpEqual),
				code.Make(code.OpPop),
				code.Make(code.OpGetNameOrNull, 2),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpNotEqual),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func Test_Conditionals(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             `if (true) { 10 }; 3333;`,
			expectedConstants: []interface{}{10, 3333},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpJumpNotTruthy, 10),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpJump, 10),
				code.Make(code.OpPop),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `if (true) { 10 } else { 20 }; 3333;`,
			expectedConstants: []interface{}{10, 20, 3333},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpJumpNotTruthy, 10),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpJump, 13),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpPop),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func Test_Peephole_Null_Pop_Optimization(t *testing.T) {
	program, err := ParseScript(`if (false) { 10 };`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.False(t, instructionContainsSequence(bytecode.Instructions, code.OpNull, code.OpPop))
	testInstructions(t, []code.Instructions{
		code.Make(code.OpFalse),
		code.Make(code.OpJumpNotTruthy, 10),
		code.Make(code.OpConstant, 0),
		code.Make(code.OpJump, 10),
		code.Make(code.OpPop),
	}, bytecode.Instructions)
}

func Test_Peephole_Write_Superinstructions(t *testing.T) {
	program, err := parser.Parse(`<%= "static" %><%= 10 %>prefix<%= name %><% let local = "local" %><%= local %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWriteConstant), "expected OpWriteConstant:\n%s", bytecode.Instructions.String())
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWriteString), "expected OpWriteString:\n%s", bytecode.Instructions.String())
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWriteHTML), "expected OpWriteHTML:\n%s", bytecode.Instructions.String())
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWriteName), "expected OpWriteName:\n%s", bytecode.Instructions.String())
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWriteGlobal), "expected OpWriteGlobal:\n%s", bytecode.Instructions.String())
	require.Falsef(t, instructionContainsSequence(bytecode.Instructions, code.OpConstant, code.OpWrite), "expected constant/write pair to be fused:\n%s", bytecode.Instructions.String())
	require.Falsef(t, instructionContainsSequence(bytecode.Instructions, code.OpGetName, code.OpWrite), "expected name/write pair to be fused:\n%s", bytecode.Instructions.String())
	require.Falsef(t, instructionContainsSequence(bytecode.Instructions, code.OpGetGlobal, code.OpWrite), "expected global/write pair to be fused:\n%s", bytecode.Instructions.String())
}

func Test_Static_Only_Bytecode_Fast_Path(t *testing.T) {
	program, err := parser.Parse(`<section><p>Hello</p></section>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.True(t, bytecode.Static)
	require.Equal(t, `<section><p>Hello</p></section>`, bytecode.StaticOutput)
	require.Equal(t, 1, instructionOpcodeCount(bytecode.Instructions, code.OpWriteHTML))
}

func Test_Static_Only_Bytecode_Escapes_String_Literal(t *testing.T) {
	program, err := parser.Parse(`<strong><%= "<x>" %></strong>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.True(t, bytecode.Static)
	require.Equal(t, `<strong>&lt;x&gt;</strong>`, bytecode.StaticOutput)
	require.Equal(t, 1, instructionOpcodeCount(bytecode.Instructions, code.OpWriteHTML))
}

func Test_Static_Only_Bytecode_Includes_Scalar_Constants(t *testing.T) {
	program, err := parser.Parse(`<%= 10 %><span>items</span>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.True(t, bytecode.Static)
	require.Equal(t, `10<span>items</span>`, bytecode.StaticOutput)
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWriteConstant), "expected OpWriteConstant:\n%s", bytecode.Instructions.String())
}

func Test_Bytecode_Feature_Flags(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		hasHoles    bool
		hasPartials bool
	}{
		{
			name:  "plain",
			input: `<p><%= name %></p>`,
		},
		{
			name:        "partial call",
			input:       `<%= partial("row") %>`,
			hasPartials: true,
		},
		{
			name:        "partial alias",
			input:       `<% let p = partial %><%= p("row") %>`,
			hasPartials: true,
		},
		{
			name:     "hole",
			input:    `<%H "hole" %>`,
			hasHoles: true,
		},
		{
			name:        "partial in function",
			input:       `<% let render = fn() { return partial("row") } %><%= render() %>`,
			hasPartials: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := parser.Parse(tt.input)
			require.NoError(t, err)

			compiler := New()
			require.NoError(t, compiler.Compile(program))

			bytecode := compiler.Bytecode()
			require.Equal(t, tt.hasHoles, bytecode.HasHoles)
			require.Equal(t, tt.hasPartials, bytecode.HasPartials)
		})
	}
}

func Test_Mixed_Static_Runs_Are_Coalesced_Without_Marking_Template_Static(t *testing.T) {
	program, err := parser.Parse(`a<%= "b<c>" %>d<%= name %>e<%= "f" %>g`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.False(t, bytecode.Static)
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"name"}, bytecode.FastRenderPlan.Bindings)
	require.Equal(t, 15, bytecode.FastRenderPlan.StaticSize)
	require.Equal(t, 1, bytecode.FastRenderPlan.NameCount)
	require.Equal(t, []FastRenderSegment{
		{Kind: FastRenderSegmentStatic, Value: `ab&lt;c&gt;d`},
		{Kind: FastRenderSegmentName, Value: "name", Line: 1},
		{Kind: FastRenderSegmentStatic, Value: `efg`},
	}, bytecode.FastRenderPlan.Segments)
	require.Equal(t, 2, instructionOpcodeCount(bytecode.Instructions, code.OpWriteHTML))
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWriteName), "expected OpWriteName:\n%s", bytecode.Instructions.String())
}

func Test_Fast_Render_Plan_From_Instructions_Edges(t *testing.T) {
	require.Nil(t, fastRenderPlanFromInstructions(code.Instructions{255}, nil, nil))

	require.Nil(t, fastRenderPlanFromInstructions(code.Make(code.OpWriteString, 0), []object.Object{&object.String{Value: "static"}}, nil))
	require.Nil(t, fastRenderPlanFromInstructions(code.Make(code.OpWriteName, 0), []object.Object{&object.String{Value: "nil"}}, nil))
	require.Nil(t, fastRenderPlanFromInstructions(code.Make(code.OpWriteNameProperty, 0, 1), []object.Object{
		&object.String{Value: "nil"},
		&object.String{Value: "Name"},
	}, nil))

	require.Nil(t, fastRenderPlanFromInstructions(code.Make(code.OpWriteHTML, 0), []object.Object{&object.String{Value: "bad"}}, nil))
	require.Nil(t, fastRenderPlanFromInstructions(code.Make(code.OpWriteName, 0), []object.Object{&object.Integer{Value: 1}}, nil))
	require.Nil(t, fastRenderPlanFromInstructions(code.Make(code.OpWriteNameProperty, 0, 1), []object.Object{
		&object.Integer{Value: 1},
		&object.String{Value: "Name"},
	}, nil))
	require.Nil(t, fastRenderPlanFromInstructions(code.Make(code.OpWriteNameProperty, 0, 1), []object.Object{
		&object.String{Value: "user"},
		&object.Integer{Value: 1},
	}, nil))
	require.Nil(t, fastRenderPlanFromInstructions(code.Make(code.OpWriteString, 0), []object.Object{&object.Integer{Value: 1}}, nil))
	require.Nil(t, fastRenderPlanFromInstructions(code.Make(code.OpWriteConstant, 0), []object.Object{&object.Array{}}, nil))
	require.Nil(t, fastRenderPlanFromInstructions(code.Make(code.OpAdd), nil, nil))

	plan := fastRenderPlanFromInstructions(
		append(code.Make(code.OpWriteString, 0), code.Make(code.OpWriteName, 1)...),
		[]object.Object{&object.String{Value: "<"}, &object.String{Value: "name"}},
		map[int]int{len(code.Make(code.OpWriteString, 0)): 9},
	)
	require.NotNil(t, plan)
	require.Equal(t, []string{"name"}, plan.Bindings)
	require.Equal(t, []FastRenderSegment{
		{Kind: FastRenderSegmentStatic, Value: "&lt;"},
		{Kind: FastRenderSegmentName, Value: "name", Line: 9},
	}, plan.Segments)

	plan = fastRenderPlanFromInstructions(code.Make(code.OpWriteNameProperty, 0, 1), []object.Object{
		&object.String{Value: "user"},
		&object.String{Value: "Name"},
	}, map[int]int{0: 4})
	require.NotNil(t, plan)
	require.Equal(t, FastRenderSegmentProperty, plan.Segments[0].Kind)
	require.Equal(t, "user.Name", plan.Segments[0].Full)

	loopInstructions := code.Instructions(append(code.Make(code.OpWriteLocal, 0), code.Make(code.OpReturn)...))
	loopConstants := []object.Object{
		&object.CompiledFunction{Instructions: loopInstructions, NumParameters: 2},
		&object.String{Value: "nil"},
		&object.String{Value: "i"},
		&object.String{Value: "item"},
	}
	loopPlanInstructions := code.Instructions{}
	loopPlanInstructions = append(loopPlanInstructions, code.Make(code.OpGetName, 1)...)
	loopPlanInstructions = append(loopPlanInstructions, code.Make(code.OpFor, 0, 2, 3, 0)...)
	loopPlanInstructions = append(loopPlanInstructions, code.Make(code.OpWrite)...)
	require.Nil(t, fastRenderPlanFromInstructions(loopPlanInstructions, loopConstants, nil))

	loopConstants[1] = &object.Integer{Value: 1}
	require.Nil(t, fastRenderPlanFromInstructions(loopPlanInstructions, loopConstants, nil))

	output, ok := staticOutputFromInstructions(code.Instructions{255}, nil)
	require.False(t, ok)
	require.Empty(t, output)

	output, ok = staticOutputFromInstructions(code.Make(code.OpWriteHTML, 0), []object.Object{&object.String{Value: "bad"}})
	require.False(t, ok)
	require.Empty(t, output)

	output, ok = staticOutputFromInstructions(code.Make(code.OpWriteString, 0), []object.Object{&object.Integer{Value: 1}})
	require.False(t, ok)
	require.Empty(t, output)

	output, ok = staticOutputFromInstructions(code.Make(code.OpWriteConstant, 0), []object.Object{&object.Array{}})
	require.False(t, ok)
	require.Empty(t, output)
}

func Test_Fast_Render_Plan_Includes_Property_Access(t *testing.T) {
	program, err := parser.Parse(`<p><%= user.Name %></p>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"user"}, bytecode.FastRenderPlan.Bindings)
	require.Equal(t, []FastRenderSegment{
		{Kind: FastRenderSegmentStatic, Value: `<p>`},
		{Kind: FastRenderSegmentProperty, Value: "user", Property: "Name", Receiver: "user", Full: "user.Name", Line: 1},
		{Kind: FastRenderSegmentStatic, Value: `</p>`},
	}, bytecode.FastRenderPlan.Segments)
}

func Test_Fast_Render_Plan_Includes_Simple_Loops(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= i %>:<%= product.Name %>;<% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"products"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Equal(t, "products", segment.Loop.IterableName)
	require.Equal(t, "i", segment.Loop.KeyName)
	require.Equal(t, "product", segment.Loop.ValueName)
	require.Equal(t, []FastLoopPart{
		{Kind: FastLoopPartKey, Line: 1},
		{Kind: FastLoopPartStatic, Value: ":"},
		{Kind: FastLoopPartValueProperty, Value: "Name", Receiver: "product", Full: "product.Name", Line: 1},
		{Kind: FastLoopPartStatic, Value: ";"},
	}, segment.Loop.Parts)
}

func Test_Fast_Render_Plan_Includes_Loop_Indexed_Value_Paths(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= product.Friends[0].Name %>;<% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 2)
	part := segment.Loop.Parts[0]
	require.Equal(t, FastLoopPartValuePath, part.Kind)
	require.Equal(t, FastValuePath, part.ValuePlan.Kind)
	require.Equal(t, "product", part.ValuePlan.Value)
	require.Len(t, part.ValuePlan.Path, 3)
	require.Equal(t, FastPathStepProperty, part.ValuePlan.Path[0].Kind)
	require.Equal(t, "Friends", part.ValuePlan.Path[0].Value)
	require.False(t, part.ValuePlan.Path[0].Method)
	require.Equal(t, FastPathStepIndexInteger, part.ValuePlan.Path[1].Kind)
	require.Equal(t, 0, part.ValuePlan.Path[1].Index)
	require.Equal(t, FastPathStepProperty, part.ValuePlan.Path[2].Kind)
	require.Equal(t, "Name", part.ValuePlan.Path[2].Value)
	require.False(t, part.ValuePlan.Path[2].Method)
}

func Test_Fast_Render_Plan_Includes_Loop_String_Indexed_Value_Paths(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= product.Meta["label"] %>;<% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 2)
	part := segment.Loop.Parts[0]
	require.Equal(t, FastLoopPartValuePath, part.Kind)
	require.Equal(t, FastValuePath, part.ValuePlan.Kind)
	require.Len(t, part.ValuePlan.Path, 2)
	require.Equal(t, FastPathStepProperty, part.ValuePlan.Path[0].Kind)
	require.Equal(t, "Meta", part.ValuePlan.Path[0].Value)
	require.Equal(t, FastPathStepIndexString, part.ValuePlan.Path[1].Kind)
	require.Equal(t, "label", part.ValuePlan.Path[1].Value)
}

func Test_Fast_Render_Plan_Includes_Top_Level_Context_Paths_Inside_Loops(t *testing.T) {
	program, err := parser.Parse(`<%= for (_, product) in products { %><%= category.CategorySeoTitle %>/<%= product.ProductSeoUrl %><% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Empty(t, bytecode.FastReject)
	require.Equal(t, []string{"products", "category"}, bytecode.FastRenderPlan.Bindings)

	loop := bytecode.FastRenderPlan.Segments[0].Loop
	require.NotNil(t, loop)
	require.Len(t, loop.Parts, 3)
	require.Equal(t, FastLoopPartValuePath, loop.Parts[0].Kind)
	require.Equal(t, "category", loop.Parts[0].ValuePlan.Value)
	require.Equal(t, FastLoopPartStatic, loop.Parts[1].Kind)
	require.Equal(t, FastLoopPartValueProperty, loop.Parts[2].Kind)
}

func Test_Fast_Render_Plan_Includes_Loop_Method_Call_Value_Paths(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= product.Name.Echo() %>;<% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 2)
	part := segment.Loop.Parts[0]
	require.Equal(t, FastLoopPartValuePath, part.Kind)
	require.Equal(t, FastValuePath, part.ValuePlan.Kind)
	require.Len(t, part.ValuePlan.Path, 3)
	require.Equal(t, FastPathStepProperty, part.ValuePlan.Path[0].Kind)
	require.Equal(t, "Name", part.ValuePlan.Path[0].Value)
	require.Equal(t, FastPathStepProperty, part.ValuePlan.Path[1].Kind)
	require.Equal(t, "Echo", part.ValuePlan.Path[1].Value)
	require.True(t, part.ValuePlan.Path[1].Method)
	require.Equal(t, FastPathStepCall, part.ValuePlan.Path[2].Kind)
	require.Equal(t, "Echo", part.ValuePlan.Path[2].Value)
}

func Test_Fast_Render_Plan_Includes_Loop_Literal_Outputs(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= "x<y>" %><%= 3 %><%= 1.5 %><%= true %><% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 1)
	require.Equal(t, FastLoopPartStatic, segment.Loop.Parts[0].Kind)
	require.Equal(t, "x&lt;y&gt;31.5true", segment.Loop.Parts[0].Value)
}

func Test_Fast_Render_Plan_Includes_Loop_Helper_Calls(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= label(product.Name, prefix) %>;<% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"products", "label", "prefix"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 2)

	part := segment.Loop.Parts[0]
	require.Equal(t, FastLoopPartCall, part.Kind)
	require.NotNil(t, part.Call)
	require.Equal(t, "label", part.Call.Name)
	require.Equal(t, 1, part.Call.NameIndex)
	require.Len(t, part.Call.Args, 2)

	productArg := part.Call.Args[0]
	require.Equal(t, FastValuePath, productArg.Kind)
	require.Equal(t, "product", productArg.Value)
	require.Equal(t, -1, productArg.NameIndex)
	require.Len(t, productArg.Path, 1)
	require.Equal(t, FastPathStepProperty, productArg.Path[0].Kind)
	require.Equal(t, "Name", productArg.Path[0].Value)

	prefixArg := part.Call.Args[1]
	require.Equal(t, FastValueName, prefixArg.Kind)
	require.Equal(t, "prefix", prefixArg.Value)
	require.Equal(t, 2, prefixArg.NameIndex)
	require.Equal(t, FastLoopPartStatic, segment.Loop.Parts[1].Kind)
	require.Equal(t, ";", segment.Loop.Parts[1].Value)
}

func Test_Fast_Render_Plan_Includes_Loop_Helper_Call_Method_Arguments(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= label(product.Name.Echo(), prefix) %>;<% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"products", "label", "prefix"}, bytecode.FastRenderPlan.Bindings)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	part := segment.Loop.Parts[0]
	require.Equal(t, FastLoopPartCall, part.Kind)
	require.NotNil(t, part.Call)
	require.Len(t, part.Call.Args, 2)
	arg := part.Call.Args[0]
	require.Equal(t, FastValuePath, arg.Kind)
	require.Len(t, arg.Path, 3)
	require.Equal(t, FastPathStepProperty, arg.Path[0].Kind)
	require.Equal(t, "Name", arg.Path[0].Value)
	require.Equal(t, FastPathStepProperty, arg.Path[1].Kind)
	require.True(t, arg.Path[1].Method)
	require.Equal(t, FastPathStepCall, arg.Path[2].Kind)
}

func Test_Fast_Render_Plan_Includes_Loop_Conditionals(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= if product.Enabled { %><%= product.Name %><% } else if fallback { %>fallback<% } else { %>hidden<% } %>;<% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"products", "fallback"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 2)

	part := segment.Loop.Parts[0]
	require.Equal(t, FastLoopPartConditional, part.Kind)
	require.NotNil(t, part.Conditional)
	require.Len(t, part.Conditional.Branches, 2)

	first := part.Conditional.Branches[0]
	require.Equal(t, FastValuePath, first.Condition.Kind)
	require.Equal(t, "product", first.Condition.Value)
	require.Equal(t, -1, first.Condition.NameIndex)
	require.Len(t, first.Condition.Path, 1)
	require.Equal(t, "Enabled", first.Condition.Path[0].Value)
	require.Len(t, first.Parts, 1)
	require.Equal(t, FastLoopPartValueProperty, first.Parts[0].Kind)
	require.Equal(t, "Name", first.Parts[0].Value)

	second := part.Conditional.Branches[1]
	require.Equal(t, FastValueName, second.Condition.Kind)
	require.Equal(t, "fallback", second.Condition.Value)
	require.Equal(t, 1, second.Condition.NameIndex)
	require.Len(t, second.Parts, 1)
	require.Equal(t, FastLoopPartStatic, second.Parts[0].Kind)
	require.Equal(t, "fallback", second.Parts[0].Value)

	require.Len(t, part.Conditional.ElseParts, 1)
	require.Equal(t, FastLoopPartStatic, part.Conditional.ElseParts[0].Kind)
	require.Equal(t, "hidden", part.Conditional.ElseParts[0].Value)
	require.Equal(t, FastLoopPartStatic, segment.Loop.Parts[1].Kind)
	require.Equal(t, ";", segment.Loop.Parts[1].Value)
}

func Test_Fast_Render_Plan_Includes_Loop_Break_And_Continue(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= if product.Stop { %><%= break %><% } %><%= if product.Skip { %><%= continue %><% } %><%= product.Name %><% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Empty(t, bytecode.FastReject)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)

	loop := bytecode.FastRenderPlan.Segments[0].Loop
	require.NotNil(t, loop)
	require.Len(t, loop.Parts, 3)
	require.Equal(t, FastLoopPartConditional, loop.Parts[0].Kind)
	require.Len(t, loop.Parts[0].Conditional.Branches[0].Parts, 1)
	require.Equal(t, FastLoopPartBreak, loop.Parts[0].Conditional.Branches[0].Parts[0].Kind)
	require.Equal(t, FastLoopPartConditional, loop.Parts[1].Kind)
	require.Len(t, loop.Parts[1].Conditional.Branches[0].Parts, 1)
	require.Equal(t, FastLoopPartContinue, loop.Parts[1].Conditional.Branches[0].Parts[0].Kind)
	require.Equal(t, FastLoopPartValueProperty, loop.Parts[2].Kind)
}

func Test_Fast_Render_Plan_Includes_Script_Loop_Control(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { break } %><%= for (i, product) in products { continue } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Empty(t, bytecode.FastReject)
	require.Len(t, bytecode.FastRenderPlan.Segments, 2)
	require.Equal(t, FastLoopPartBreak, bytecode.FastRenderPlan.Segments[0].Loop.Parts[0].Kind)
	require.Equal(t, FastLoopPartContinue, bytecode.FastRenderPlan.Segments[1].Loop.Parts[0].Kind)
}

func Test_Fast_Render_Plan_Includes_Loop_Block_Helper_Calls(t *testing.T) {
	program, err := parser.Parse(`<%= for (_, product) in products { %><%= form({id: product.ID, path: cartPath()}) { %><span><%= product.Name %></span><% } %><% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Empty(t, bytecode.FastReject)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)

	loop := bytecode.FastRenderPlan.Segments[0].Loop
	require.NotNil(t, loop)
	require.Len(t, loop.Parts, 1)
	require.Equal(t, FastLoopPartBlockCall, loop.Parts[0].Kind)
	require.NotNil(t, loop.Parts[0].BlockCall)
	require.Equal(t, "form", loop.Parts[0].BlockCall.Name)
	require.Len(t, loop.Parts[0].BlockCall.Args, 1)
	require.NotNil(t, loop.Parts[0].BlockCall.BlockBytecode)
	require.NotEmpty(t, loop.Parts[0].BlockCall.BlockSource)
}

func Test_Fast_Render_Plan_Includes_Loop_Helper_Calls_Using_Key_Argument(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= label(i) %>;<% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"products", "label"}, bytecode.FastRenderPlan.Bindings)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 2)
	part := segment.Loop.Parts[0]
	require.Equal(t, FastLoopPartCall, part.Kind)
	require.NotNil(t, part.Call)
	require.Len(t, part.Call.Args, 1)
	require.Equal(t, FastValueLoopKey, part.Call.Args[0].Kind)
	require.Equal(t, "i", part.Call.Args[0].Value)
}

func Test_Fast_Render_Plan_Includes_Loop_Conditionals_Using_Key_Condition(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= if i { %><%= product.Name %><% } %><% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 1)
	part := segment.Loop.Parts[0]
	require.Equal(t, FastLoopPartConditional, part.Kind)
	require.NotNil(t, part.Conditional)
	require.Len(t, part.Conditional.Branches, 1)
	require.Equal(t, FastValueLoopKey, part.Conditional.Branches[0].Condition.Kind)
	require.Equal(t, "i", part.Conditional.Branches[0].Condition.Value)
}

func Test_Fast_Render_Plan_Includes_Loop_Infix_Conditionals(t *testing.T) {
	program, err := parser.Parse(`<%= for (i, product) in products { %><%= if product.Stock > min { %><%= product.Name %><% } else if i == 0 { %>first<% } %><% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"products", "min"}, bytecode.FastRenderPlan.Bindings)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 1)
	part := segment.Loop.Parts[0]
	require.Equal(t, FastLoopPartConditional, part.Kind)
	require.NotNil(t, part.Conditional)
	require.Len(t, part.Conditional.Branches, 2)

	first := part.Conditional.Branches[0].Condition
	require.Equal(t, FastValueInfix, first.Kind)
	require.Equal(t, ">", first.Operator)
	require.NotNil(t, first.Left)
	require.NotNil(t, first.Right)
	require.Equal(t, FastValuePath, first.Left.Kind)
	require.Equal(t, "product", first.Left.Value)
	require.Len(t, first.Left.Path, 1)
	require.Equal(t, "Stock", first.Left.Path[0].Value)
	require.Equal(t, FastValueName, first.Right.Kind)
	require.Equal(t, "min", first.Right.Value)

	second := part.Conditional.Branches[1].Condition
	require.Equal(t, FastValueInfix, second.Kind)
	require.Equal(t, "==", second.Operator)
	require.Equal(t, FastValueLoopKey, second.Left.Kind)
	require.Equal(t, FastValueInteger, second.Right.Kind)
	require.Equal(t, int64(0), second.Right.IntValue)
}

func Test_Fast_Render_Plan_Includes_Top_Level_Infix_Output(t *testing.T) {
	program, err := parser.Parse(`<%= robot.Stock == 3.0 %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"robot"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)

	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentValue, segment.Kind)
	require.Equal(t, FastValueInfix, segment.ValuePlan.Kind)
	require.Equal(t, "==", segment.ValuePlan.Operator)
	require.NotNil(t, segment.ValuePlan.Left)
	require.NotNil(t, segment.ValuePlan.Right)
	require.Equal(t, FastValuePath, segment.ValuePlan.Left.Kind)
	require.Equal(t, "robot", segment.ValuePlan.Left.Value)
	require.Len(t, segment.ValuePlan.Left.Path, 1)
	require.Equal(t, "Stock", segment.ValuePlan.Left.Path[0].Value)
	require.Equal(t, FastValueFloat, segment.ValuePlan.Right.Kind)
	require.Equal(t, 3.0, segment.ValuePlan.Right.FloatValue)
}

func Test_Fast_Render_Plan_Includes_Top_Level_Logical_Infix_Output(t *testing.T) {
	program, err := parser.Parse(`<%= enabled && count == 1 %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"enabled", "count"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)

	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentValue, segment.Kind)
	require.Equal(t, FastValueInfix, segment.ValuePlan.Kind)
	require.Equal(t, "&&", segment.ValuePlan.Operator)
	require.NotNil(t, segment.ValuePlan.Left)
	require.NotNil(t, segment.ValuePlan.Right)
	require.Equal(t, FastValueName, segment.ValuePlan.Left.Kind)
	require.Equal(t, FastValueInfix, segment.ValuePlan.Right.Kind)
	require.Equal(t, "==", segment.ValuePlan.Right.Operator)
}

func Test_Fast_Render_Plan_From_Instructions_Includes_Loop_Function(t *testing.T) {
	loopInstructions := code.Instructions{}
	loopInstructions = append(loopInstructions, code.Make(code.OpWriteHTML, 1)...)
	loopInstructions = append(loopInstructions, code.Make(code.OpWriteLocal, 0)...)
	loopInstructions = append(loopInstructions, code.Make(code.OpWriteString, 2)...)
	loopInstructions = append(loopInstructions, code.Make(code.OpWriteLocalProperty, 1, 3)...)
	loopInstructions = append(loopInstructions, code.Make(code.OpReturn)...)

	constants := []object.Object{
		&object.CompiledFunction{
			Instructions:  loopInstructions,
			NumParameters: 2,
			LineNumbers: map[int]int{
				0: 3,
			},
		},
		&object.Native{Value: template.HTML("<b>")},
		&object.String{Value: ":"},
		&object.String{Value: "Name"},
		&object.String{Value: "items"},
		&object.String{Value: "i"},
		&object.String{Value: "product"},
	}
	instructions := code.Instructions{}
	instructions = append(instructions, code.Make(code.OpGetName, 4)...)
	instructions = append(instructions, code.Make(code.OpFor, 0, 5, 6, 0)...)
	instructions = append(instructions, code.Make(code.OpWrite)...)

	plan := fastRenderPlanFromInstructions(instructions, constants, map[int]int{0: 2})

	require.NotNil(t, plan)
	require.Equal(t, []string{"items"}, plan.Bindings)
	require.Len(t, plan.Segments, 1)
	loop := plan.Segments[0].Loop
	require.NotNil(t, loop)
	require.Equal(t, "items", loop.IterableName)
	require.Equal(t, "i", loop.KeyName)
	require.Equal(t, "product", loop.ValueName)
	require.Len(t, loop.Parts, 4)
	require.Equal(t, FastLoopPartStatic, loop.Parts[0].Kind)
	require.Equal(t, "<b>", loop.Parts[0].Value)
	require.Equal(t, FastLoopPartKey, loop.Parts[1].Kind)
	require.Equal(t, FastLoopPartStatic, loop.Parts[2].Kind)
	require.Equal(t, ":", loop.Parts[2].Value)
	require.Equal(t, FastLoopPartValueProperty, loop.Parts[3].Kind)
	require.Equal(t, "Name", loop.Parts[3].Value)
}

func Test_Fast_Loop_Plan_From_Function_Branches(t *testing.T) {
	constants := []object.Object{
		&object.String{Value: "<raw>"},
		&object.Integer{Value: 7},
		&object.Native{Value: template.HTML("<b>")},
		&object.String{Value: "Name"},
	}
	instructions := code.Instructions{}
	instructions = append(instructions, code.Make(code.OpWriteString, 0)...)
	instructions = append(instructions, code.Make(code.OpWriteConstant, 1)...)
	instructions = append(instructions, code.Make(code.OpWriteHTML, 2)...)
	instructions = append(instructions, code.Make(code.OpWriteLocal, 1)...)
	instructions = append(instructions, code.Make(code.OpWriteLocalProperty, 1, 3)...)
	instructions = append(instructions, code.Make(code.OpReturn)...)

	loop, ok := fastLoopPlanFromFunction(&object.CompiledFunction{
		Instructions:  instructions,
		NumParameters: 2,
		LineNumbers: map[int]int{
			0:  3,
			12: 4,
		},
	}, constants, "i", "product")
	require.True(t, ok)
	require.NotNil(t, loop)
	require.Equal(t, "i", loop.KeyName)
	require.Equal(t, "product", loop.ValueName)
	require.Len(t, loop.Parts, 3)
	require.Equal(t, FastLoopPartStatic, loop.Parts[0].Kind)
	require.Equal(t, "&lt;raw&gt;7<b>", loop.Parts[0].Value)
	require.Equal(t, FastLoopPartValue, loop.Parts[1].Kind)
	require.Equal(t, FastLoopPartValueProperty, loop.Parts[2].Kind)
	require.Equal(t, "Name", loop.Parts[2].Value)

	_, ok = fastLoopPlanFromFunction(nil, constants, "i", "product")
	require.False(t, ok)
	_, ok = fastLoopPlanFromFunction(&object.CompiledFunction{NumParameters: 1}, constants, "i", "product")
	require.False(t, ok)
	_, ok = fastLoopPlanFromFunction(&object.CompiledFunction{NumParameters: 2, Instructions: code.Instructions{byte(255)}}, constants, "i", "product")
	require.False(t, ok)
	_, ok = fastLoopPlanFromFunction(&object.CompiledFunction{NumParameters: 2, Instructions: code.Make(code.OpWriteHTML, 99)}, constants, "i", "product")
	require.False(t, ok)
	_, ok = fastLoopPlanFromFunction(&object.CompiledFunction{NumParameters: 2, Instructions: code.Make(code.OpWriteString, 2)}, constants, "i", "product")
	require.False(t, ok)
	_, ok = fastLoopPlanFromFunction(&object.CompiledFunction{NumParameters: 2, Instructions: code.Make(code.OpWriteConstant, 99)}, constants, "i", "product")
	require.False(t, ok)
	_, ok = fastLoopPlanFromFunction(&object.CompiledFunction{NumParameters: 2, Instructions: code.Make(code.OpWriteLocal, 2)}, constants, "i", "product")
	require.False(t, ok)
	_, ok = fastLoopPlanFromFunction(&object.CompiledFunction{NumParameters: 2, Instructions: code.Make(code.OpWriteLocalProperty, 0, 3)}, constants, "i", "product")
	require.False(t, ok)
	_, ok = fastLoopPlanFromFunction(&object.CompiledFunction{NumParameters: 2, Instructions: code.Make(code.OpWriteLocalProperty, 1, 1)}, constants, "i", "product")
	require.False(t, ok)
	_, ok = fastLoopPlanFromFunction(&object.CompiledFunction{NumParameters: 2, Instructions: code.Make(code.OpAdd)}, constants, "i", "product")
	require.False(t, ok)
	_, ok = fastLoopPlanFromFunction(&object.CompiledFunction{NumParameters: 2, Instructions: code.Make(code.OpWriteString, 0)}, constants, "i", "product")
	require.False(t, ok)

	renderPlan := &FastRenderPlan{}
	renderPlan.appendStatic("")
	require.Empty(t, renderPlan.Segments)

	emptyLoop := &FastLoopPlan{}
	emptyLoop.appendStatic("")
	require.Empty(t, emptyLoop.Parts)
}

func Test_Fast_Loop_Plan_At_Branches(t *testing.T) {
	fn := &object.CompiledFunction{
		NumParameters: 2,
		Instructions: code.Instructions(append(
			code.Make(code.OpWriteLocal, 0),
			code.Make(code.OpReturn)...,
		)),
	}
	constants := []object.Object{
		fn,
		&object.String{Value: "items"},
		&object.String{Value: "i"},
		&object.String{Value: "item"},
	}
	instructions := code.Instructions{}
	instructions = append(instructions, code.Make(code.OpFor, 0, 2, 3, 0)...)
	instructions = append(instructions, code.Make(code.OpWrite)...)

	loop, next, ok := fastLoopPlanAt(instructions, 0, constants, map[int]int{0: 9})
	require.True(t, ok)
	require.NotNil(t, loop)
	require.Equal(t, 9, loop.Line)
	require.Equal(t, len(instructions), next)

	_, _, ok = fastLoopPlanAt(code.Make(code.OpConstant, 0), 0, constants, nil)
	require.False(t, ok)
	_, _, ok = fastLoopPlanAt(code.Make(code.OpFor, 0, 2, 3, 1), 0, constants, nil)
	require.False(t, ok)
	_, _, ok = fastLoopPlanAt(code.Make(code.OpFor, 0, 2, 3, 0), 0, constants, nil)
	require.False(t, ok)
	_, _, ok = fastLoopPlanAt(instructions, 0, []object.Object{fn, constants[1], &object.Integer{Value: 1}, constants[3]}, nil)
	require.False(t, ok)
	_, _, ok = fastLoopPlanAt(instructions, 0, []object.Object{fn, constants[1], constants[2], &object.Integer{Value: 1}}, nil)
	require.False(t, ok)
	_, _, ok = fastLoopPlanAt(instructions, 0, []object.Object{&object.String{Value: "nope"}, constants[1], constants[2], constants[3]}, nil)
	require.False(t, ok)
	_, _, ok = fastLoopPlanAt(instructions, 0, []object.Object{
		&object.CompiledFunction{NumParameters: 2, Instructions: code.Make(code.OpAdd)},
		constants[1],
		constants[2],
		constants[3],
	}, nil)
	require.False(t, ok)
}

func Test_Fast_Render_Plan_Includes_Helper_Calls(t *testing.T) {
	program, err := parser.Parse(`<%= greet(name) %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"greet", "name"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentCall, segment.Kind)
	require.NotNil(t, segment.Call)
	require.Equal(t, "greet", segment.Call.Name)
	require.Len(t, segment.Call.Args, 1)
	require.Equal(t, FastValueName, segment.Call.Args[0].Kind)
	require.Equal(t, "name", segment.Call.Args[0].Value)
}

func Test_Fast_Render_Plan_Includes_Helper_Call_Array_Arguments(t *testing.T) {
	program, err := parser.Parse(`<%= multiple_menu_items([menu_nav, menu_categories_string]) %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Empty(t, bytecode.FastReject)
	require.Equal(t, []string{"multiple_menu_items", "menu_nav", "menu_categories_string"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)

	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentCall, segment.Kind)
	require.NotNil(t, segment.Call)
	require.Len(t, segment.Call.Args, 1)

	arg := segment.Call.Args[0]
	require.Equal(t, FastValueArray, arg.Kind)
	require.Len(t, arg.Elements, 2)
	require.Equal(t, FastValueName, arg.Elements[0].Kind)
	require.Equal(t, "menu_nav", arg.Elements[0].Value)
	require.Equal(t, FastValueName, arg.Elements[1].Kind)
	require.Equal(t, "menu_categories_string", arg.Elements[1].Value)
}

func Test_Fast_Render_Plan_Includes_Block_Helper_Call_With_Hash_Arguments(t *testing.T) {
	program, err := parser.Parse(`<%= form({content_for: "head", count: count}) { %><span><%= name %></span><% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Empty(t, bytecode.FastReject)
	require.Equal(t, []string{"form", "count"}, bytecode.FastRenderPlan.Bindings[:2])
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)

	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentBlockCall, segment.Kind)
	require.NotNil(t, segment.BlockCall)
	require.Equal(t, "form", segment.BlockCall.Name)
	require.Len(t, segment.BlockCall.Args, 1)
	require.NotNil(t, segment.BlockCall.BlockBytecode)
	require.NotNil(t, segment.BlockCall.BlockBytecode.FastRenderPlan)

	arg := segment.BlockCall.Args[0]
	require.Equal(t, FastValueHash, arg.Kind)
	require.Len(t, arg.Pairs, 2)
	require.Equal(t, "content_for", arg.Pairs[0].Key)
	require.Equal(t, FastValueString, arg.Pairs[0].Value.Kind)
	require.Equal(t, "head", arg.Pairs[0].Value.Value)
	require.Equal(t, "count", arg.Pairs[1].Key)
	require.Equal(t, FastValueName, arg.Pairs[1].Value.Kind)
	require.Equal(t, "count", arg.Pairs[1].Value.Value)
}

func Test_Fast_Render_Plan_Block_Helper_Source_Preserves_Output_Tags(t *testing.T) {
	program, err := parser.Parse(`<%= form({action: path}) { %><%= f.InputTag({name: "A"}) %><%= f.InputTag({name: "B"}) %><button>Go</button><% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Empty(t, bytecode.FastReject)
	block := bytecode.FastRenderPlan.Segments[0].BlockCall
	require.NotNil(t, block)
	require.Contains(t, block.BlockSource, `<%= f.InputTag`)
	require.Contains(t, block.BlockSource, `<button>Go</button>`)
	require.NotNil(t, block.BlockBytecode)
}

func Test_Fast_Render_Plan_Block_Helper_Source_Preserves_Shipping_Form(t *testing.T) {
	input := `<%= form({action: shippingEstimatorPath(), method: "POST", id: "shippingEstimate"}) { %>
	<%= f.InputTag({name:"CartAdd[0].CartAddProductID", value: product.ProductId}) %>
	<%= f.InputTag({name: "CartAdd[0].CartAddVariantID[]", value: product.ProductVariants[0].VariantId}) %>
	<%= f.InputTag({name:"CartAdd[0].CartAddVariantAddQty", value: "1"}) %>
	<%= f.InputTag({type:"text",label:"Shipping First Name", name:"Shipping.FirstName", value:"test"} ) %>
<button class="btn btn-success" role="submit">Get ESTIMATED Rates</button>
<% } %>`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan, bytecode.FastReject)
	require.Empty(t, bytecode.FastReject)
	require.NotNil(t, bytecode.FastRenderPlan.Segments[0].BlockCall.BlockBytecode)
}

func Test_Fast_Render_Plan_Includes_Top_Level_Let(t *testing.T) {
	program, err := parser.Parse(`<% let message = greet(name) %><%= message %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"greet", "name", "message"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 2)

	assign := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLet, assign.Kind)
	require.Equal(t, "message", assign.Value)
	require.Equal(t, FastValueCall, assign.ValuePlan.Kind)
	require.NotNil(t, assign.ValuePlan.Call)
	require.Equal(t, "greet", assign.ValuePlan.Call.Name)

	output := bytecode.FastRenderPlan.Segments[1]
	require.Equal(t, FastRenderSegmentName, output.Kind)
	require.Equal(t, "message", output.Value)
}

func Test_Fast_Render_Plan_Includes_Silent_If_Assignment(t *testing.T) {
	program, err := parser.Parse(`<% let displayResult = listing.DisplayResult
if (listing.TotalResult < listing.DisplayResult) {
	displayResult = listing.TotalResult
}
%><%= displayResult %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan, bytecode.FastReject)
	require.Equal(t, []string{"listing", "displayResult"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 3)

	initial := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLet, initial.Kind)
	require.Equal(t, "displayResult", initial.Value)

	conditional := bytecode.FastRenderPlan.Segments[1]
	require.Equal(t, FastRenderSegmentConditional, conditional.Kind)
	require.NotNil(t, conditional.Conditional)
	require.True(t, conditional.Conditional.Silent)
	require.Len(t, conditional.Conditional.Branches, 1)
	require.Len(t, conditional.Conditional.Branches[0].Segments, 1)

	assign := conditional.Conditional.Branches[0].Segments[0]
	require.Equal(t, FastRenderSegmentAssign, assign.Kind)
	require.Equal(t, "displayResult", assign.Value)
	require.Equal(t, FastValuePath, assign.ValuePlan.Kind)

	output := bytecode.FastRenderPlan.Segments[2]
	require.Equal(t, FastRenderSegmentName, output.Kind)
	require.Equal(t, "displayResult", output.Value)
}

func Test_Fast_Render_Plan_Includes_Indexed_Property_Chains(t *testing.T) {
	program, err := parser.Parse(`<%= robots[0].Name.Echo() %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"robots"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)
	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentValue, segment.Kind)
	require.Equal(t, FastValuePath, segment.ValuePlan.Kind)
	require.Equal(t, "robots", segment.ValuePlan.Value)
	require.Len(t, segment.ValuePlan.Path, 4)
	require.Equal(t, []FastPathStepKind{
		FastPathStepIndexInteger,
		FastPathStepProperty,
		FastPathStepProperty,
		FastPathStepCall,
	}, []FastPathStepKind{
		segment.ValuePlan.Path[0].Kind,
		segment.ValuePlan.Path[1].Kind,
		segment.ValuePlan.Path[2].Kind,
		segment.ValuePlan.Path[3].Kind,
	})
}

func Test_Fast_Render_Plan_Includes_String_Indexed_Map_Chains(t *testing.T) {
	program, err := parser.Parse(`<%= labels["status"] %><%= robots["bender"].Name %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"labels", "robots"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 2)

	first := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentValue, first.Kind)
	require.Equal(t, FastValuePath, first.ValuePlan.Kind)
	require.Equal(t, "labels", first.ValuePlan.Value)
	require.Len(t, first.ValuePlan.Path, 1)
	require.Equal(t, FastPathStepIndexString, first.ValuePlan.Path[0].Kind)
	require.Equal(t, "status", first.ValuePlan.Path[0].Value)

	second := bytecode.FastRenderPlan.Segments[1]
	require.Equal(t, FastRenderSegmentValue, second.Kind)
	require.Equal(t, FastValuePath, second.ValuePlan.Kind)
	require.Equal(t, "robots", second.ValuePlan.Value)
	require.Len(t, second.ValuePlan.Path, 2)
	require.Equal(t, FastPathStepIndexString, second.ValuePlan.Path[0].Kind)
	require.Equal(t, "bender", second.ValuePlan.Path[0].Value)
	require.Equal(t, FastPathStepProperty, second.ValuePlan.Path[1].Kind)
	require.Equal(t, "Name", second.ValuePlan.Path[1].Value)
}

func Test_Fast_Render_Plan_Includes_Conditionals_And_Partials(t *testing.T) {
	program, err := parser.Parse(`<%= if enabled { %>yes<% } else if fallback { %><%= name %><% } else { %>no<% } %><%= partial("row.plush") %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Len(t, bytecode.FastRenderPlan.Segments, 2)
	conditional := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentConditional, conditional.Kind)
	require.NotNil(t, conditional.Conditional)
	require.Len(t, conditional.Conditional.Branches, 2)
	require.Len(t, conditional.Conditional.ElseSegments, 1)
	require.Equal(t, FastRenderSegmentPartial, bytecode.FastRenderPlan.Segments[1].Kind)
	require.Equal(t, "row.plush", bytecode.FastRenderPlan.Segments[1].Partial.Name)
}

func Test_Fast_Render_Plan_Includes_Loop_Partial(t *testing.T) {
	program, err := parser.Parse(`<%= for (_, product) in products { %><%= partial("partials/product-card.plush.html") %><% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)

	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 1)
	require.Equal(t, FastLoopPartPartial, segment.Loop.Parts[0].Kind)
	require.NotNil(t, segment.Loop.Parts[0].Partial)
	require.Equal(t, "partials/product-card.plush.html", segment.Loop.Parts[0].Partial.Name)
}

func Test_Fast_Render_Plan_Includes_Partial_Data_Map(t *testing.T) {
	program, err := parser.Parse(`<%= partial("row.plush", {name: name, title: product.Name, literal: "ok"}) %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)

	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentPartial, segment.Kind)
	require.NotNil(t, segment.Partial)
	require.Equal(t, "row.plush", segment.Partial.Name)
	require.Len(t, segment.Partial.Data, 3)
	require.Equal(t, "name", segment.Partial.Data[0].Key)
	require.Equal(t, FastValueName, segment.Partial.Data[0].Value.Kind)
	require.Equal(t, "title", segment.Partial.Data[1].Key)
	require.Equal(t, FastValuePath, segment.Partial.Data[1].Value.Kind)
	require.Equal(t, "literal", segment.Partial.Data[2].Key)
	require.Equal(t, FastValueString, segment.Partial.Data[2].Value.Kind)
}

func Test_Fast_Render_Plan_Includes_Partial_Data_Helper_Call(t *testing.T) {
	program, err := parser.Parse(`<%= partial("row.plush", {label: label(product.Name, prefix)}) %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Equal(t, []string{"label", "product", "prefix"}, bytecode.FastRenderPlan.Bindings)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)

	segment := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentPartial, segment.Kind)
	require.NotNil(t, segment.Partial)
	require.Len(t, segment.Partial.Data, 1)
	require.Equal(t, "label", segment.Partial.Data[0].Key)

	value := segment.Partial.Data[0].Value
	require.Equal(t, FastValueCall, value.Kind)
	require.NotNil(t, value.Call)
	require.Equal(t, "label", value.Call.Name)
	require.Len(t, value.Call.Args, 2)
	require.Equal(t, FastValuePath, value.Call.Args[0].Kind)
	require.Equal(t, "product", value.Call.Args[0].Value)
	require.Equal(t, []FastPathStep{{
		Kind:     FastPathStepProperty,
		Value:    "Name",
		Receiver: "product",
		Full:     "product.Name",
		Line:     1,
	}}, value.Call.Args[0].Path)
	require.Equal(t, FastValueName, value.Call.Args[1].Kind)
	require.Equal(t, "prefix", value.Call.Args[1].Value)
}

func Test_Fast_Render_Plan_Skips_Partial_Data_Map_Layout(t *testing.T) {
	program, err := parser.Parse(`<%= partial("row.plush", {name: name, layout: "shell"}) %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	require.Nil(t, compiler.Bytecode().FastRenderPlan)
}

func Test_Direct_Write_Call_Fusion_Chains(t *testing.T) {
	program, err := parser.Parse(`<%= greet(name) %><%= robot.Name.Echo() %><%= factory().Robots().Name().Echo() %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.Equal(t, 1, instructionOpcodeCount(bytecode.Instructions, code.OpWriteNameCall))
	require.Equal(t, 2, instructionOpcodeCount(bytecode.Instructions, code.OpWriteCall))
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpCall), "expected intermediate OpCall:\n%s", bytecode.Instructions.String())
	require.Falsef(t, instructionContainsSequence(bytecode.Instructions, code.OpCall, code.OpWrite), "expected call/write pair to be fused:\n%s", bytecode.Instructions.String())
	require.False(t, bytecode.Static)
}

func Test_Direct_Write_Call_Fusion_Skips_Block_Helpers(t *testing.T) {
	program, err := parser.Parse(`<%= wrap() { %>body<% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.Falsef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWriteCall), "block helper should stay on OpCallBlock:\n%s", bytecode.Instructions.String())
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpCallBlock), "expected OpCallBlock:\n%s", bytecode.Instructions.String())
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWrite), "expected OpWrite:\n%s", bytecode.Instructions.String())
}

func Test_Direct_Property_Write_Fusion(t *testing.T) {
	program, err := parser.Parse(`<%= user.Name %><% let robot = {Name: "Bender"} %><%= robot.Name %><%= for (i, product) in products { %><%= product.Name %><% } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWriteNameProperty), "expected OpWriteNameProperty:\n%s", bytecode.Instructions.String())
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWriteGlobalProperty), "expected OpWriteGlobalProperty:\n%s", bytecode.Instructions.String())

	var loopFn *object.CompiledFunction
	for _, constant := range bytecode.Constants {
		if fn, ok := constant.(*object.CompiledFunction); ok && instructionContainsOpcode(fn.Instructions, code.OpWriteLocalProperty) {
			loopFn = fn
			break
		}
	}
	require.NotNil(t, loopFn, "expected loop body to contain OpWriteLocalProperty")
	require.Falsef(t, instructionContainsSequence(loopFn.Instructions, code.OpGetLocal, code.OpGetProperty), "expected local/property/write path to be fused:\n%s", loopFn.Instructions.String())
}

func Test_Direct_Write_Call_Fusion_Keeps_Call_Names(t *testing.T) {
	program, err := parser.Parse(`<%= greet(name) %><%= robot.Name.Echo() %><%= wrap() { return "x" } %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.Equalf(t, 1, instructionOpcodeCount(bytecode.Instructions, code.OpWriteNameCall), "expected OpWriteNameCall:\n%s", bytecode.Instructions.String())
	require.Equalf(t, 1, instructionOpcodeCount(bytecode.Instructions, code.OpWriteCall), "expected OpWriteCall:\n%s", bytecode.Instructions.String())
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpCallBlock), "expected OpCallBlock:\n%s", bytecode.Instructions.String())
	require.Truef(t, instructionContainsOpcode(bytecode.Instructions, code.OpWrite), "expected block helper to keep OpWrite:\n%s", bytecode.Instructions.String())
	require.Falsef(t, instructionContainsSequence(bytecode.Instructions, code.OpCall, code.OpWrite), "expected call/write pair to be fused:\n%s", bytecode.Instructions.String())
	require.True(t, callNamesContain(bytecode.CallNames, "greet"))
	require.True(t, callNamesContain(bytecode.CallNames, "Echo"))
	require.True(t, callNamesContain(bytecode.CallNames, "wrap"))
}

func Test_Peephole_Optimizes_Compiled_Function_Constants(t *testing.T) {
	program, err := ParseScript(`fn() { if (false) { 10 }; 20 }`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	fn, ok := bytecode.Constants[2].(*object.CompiledFunction)
	require.True(t, ok)
	require.False(t, instructionContainsSequence(fn.Instructions, code.OpNull, code.OpPop))
}

func Test_Indexed_Receiver_Callee_Chain(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             `product_listing.Products[0].Name[0]`,
			expectedConstants: []interface{}{"product_listing", "Products", 0, "Name", 0},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetName, 0),
				code.Make(code.OpGetProperty, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 3),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpIndex),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `robots[0].Stats.Count == 0`,
			expectedConstants: []interface{}{"robots", 0, "Stats", "Count", 0},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetNameOrNull, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 2),
				code.Make(code.OpGetProperty, 3),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpEqual),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `getRobot().Name.Echo()`,
			expectedConstants: []interface{}{"getRobot", "Name", "Echo"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetName, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpGetProperty, 1),
				code.Make(code.OpGetProperty, 2),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `getRobots()[0].Name`,
			expectedConstants: []interface{}{"getRobots", 0, "Name"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetName, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 2),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `factory().Robots()[0].Name.Echo()`,
			expectedConstants: []interface{}{"factory", "Robots", 0, "Name", "Echo"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetName, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpGetProperty, 1),
				code.Make(code.OpCall, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 3),
				code.Make(code.OpGetProperty, 4),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `robot.Map[key].Name`,
			expectedConstants: []interface{}{"robot", "Map", "key", "Name"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetName, 0),
				code.Make(code.OpGetProperty, 1),
				code.Make(code.OpGetName, 2),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 3),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `robot.Map[key].Name.Echo()`,
			expectedConstants: []interface{}{"robot", "Map", "key", "Name", "Echo"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetName, 0),
				code.Make(code.OpGetProperty, 1),
				code.Make(code.OpGetName, 2),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 3),
				code.Make(code.OpGetProperty, 4),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `robot.Nested.Map[nestedKey].Items[0].Name`,
			expectedConstants: []interface{}{"robot", "Nested", "Map", "nestedKey", "Items", 0, "Name"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetName, 0),
				code.Make(code.OpGetProperty, 1),
				code.Make(code.OpGetProperty, 2),
				code.Make(code.OpGetName, 3),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 4),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 6),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `robot.Nested.Map[nestedKey].Items[0].Name.Echo()`,
			expectedConstants: []interface{}{"robot", "Nested", "Map", "nestedKey", "Items", 0, "Name", "Echo"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetName, 0),
				code.Make(code.OpGetProperty, 1),
				code.Make(code.OpGetProperty, 2),
				code.Make(code.OpGetName, 3),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 4),
				code.Make(code.OpConstant, 5),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 6),
				code.Make(code.OpGetProperty, 7),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `robot.GetFriends()[0].Name.Echo()`,
			expectedConstants: []interface{}{"robot", "GetFriends", 0, "Name", "Echo"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetName, 0),
				code.Make(code.OpGetProperty, 1),
				code.Make(code.OpCall, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpIndex),
				code.Make(code.OpGetProperty, 3),
				code.Make(code.OpGetProperty, 4),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `factory().Robots().Name().Echo()`,
			expectedConstants: []interface{}{"factory", "Robots", "Name", "Echo"},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetName, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpGetProperty, 1),
				code.Make(code.OpCall, 0),
				code.Make(code.OpGetProperty, 2),
				code.Make(code.OpCall, 0),
				code.Make(code.OpGetProperty, 3),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func Test_If_Inline_Scope_Does_Not_Capture_Function_Parameter(t *testing.T) {
	program, err := ParseScript(`fn(obj) { if (obj.Secret) { return obj.String } return obj.String }`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotEmpty(t, bytecode.Constants, "expected compiled function constant")

	fn, ok := bytecode.Constants[len(bytecode.Constants)-1].(*object.CompiledFunction)
	require.Truef(t, ok, "last constant is not compiled function: %T", bytecode.Constants[len(bytecode.Constants)-1])
	require.Equal(t, 1, fn.NumLocals)
	require.Falsef(t, instructionContainsOpcode(fn.Instructions, code.OpGetFree), "inline if scope should not turn function parameter into free symbol:\n%s", fn.Instructions.String())
	require.Truef(t, instructionContainsOpcode(fn.Instructions, code.OpGetLocal), "expected inline if scope to read function parameter as local:\n%s", fn.Instructions.String())
}

func Test_If_Inline_Scope_Let_Allocates_Function_Local(t *testing.T) {
	program, err := ParseScript(`fn(username) { if (username) { let username = "inner"; username } return username }`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	fn, ok := bytecode.Constants[len(bytecode.Constants)-1].(*object.CompiledFunction)
	require.Truef(t, ok, "last constant is not compiled function: %T", bytecode.Constants[len(bytecode.Constants)-1])
	require.Equal(t, 2, fn.NumLocals)
	require.Truef(t, instructionContainsOpcode(fn.Instructions, code.OpSetLocal), "expected branch-scoped let to compile to local assignment:\n%s", fn.Instructions.String())
}

func Test_Compiler_Hole_Statement(t *testing.T) {
	program, err := parser.Parse(`<%H "hello" %>`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	testInstructions(t, []code.Instructions{
		code.Make(code.OpHole, 0),
	}, bytecode.Instructions)
	testConstants(t, []interface{}{`<%= "hello" %>`}, bytecode.Constants)
}

func Test_Compiler_Hole_Statement_Inside_Rendered_Template(t *testing.T) {
	program, err := parser.Parse(`prefix<%H name %>suffix`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	testInstructions(t, []code.Instructions{
		code.Make(code.OpWriteHTML, 0),
		code.Make(code.OpHole, 1),
		code.Make(code.OpWriteHTML, 2),
	}, bytecode.Instructions)
	require.Len(t, bytecode.Constants, 3)
	prefix, ok := htmlConstantValue(bytecode.Constants, 0)
	require.True(t, ok)
	require.Equal(t, "prefix", prefix)
	testStringObject(t, `<%= name %>`, bytecode.Constants[1])
	suffix, ok := htmlConstantValue(bytecode.Constants, 2)
	require.True(t, ok)
	require.Equal(t, "suffix", suffix)
}

func Test_Phase_11_Call_Name_Metadata(t *testing.T) {
	program, err := ParseScript(`factory().Robots().Name().Echo(); len([]);`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.Equal(t, map[int]string{
		3:  "factory",
		8:  "Robots",
		13: "Name",
		18: "Echo",
		26: "len",
	}, bytecode.CallNames)
}

func Test_Phase_11_Nested_Function_Call_Name_Metadata(t *testing.T) {
	program, err := ParseScript(`let make = fn() { let inner = fn() { return greet() }; return inner() }; make();`)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.True(t, callNamesContain(bytecode.CallNames, "make"))

	var foundGreet bool
	var foundInner bool
	for _, constant := range bytecode.Constants {
		fn, ok := constant.(*object.CompiledFunction)
		if !ok {
			continue
		}
		foundGreet = foundGreet || callNamesContain(fn.CallNames, "greet")
		foundInner = foundInner || callNamesContain(fn.CallNames, "inner")
	}

	require.True(t, foundGreet, "expected nested function constant to record greet() call")
	require.True(t, foundInner, "expected outer function constant to record inner() call")
}

func Test_Global_Let_Statements(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             `let one = 1; one;`,
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func Test_Global_Count_Metadata(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int
	}{
		{
			name:          "no globals",
			input:         `1 + 2;`,
			expectedCount: 0,
		},
		{
			name:          "two globals",
			input:         `let one = 1; let two = 2; one + two;`,
			expectedCount: 2,
		},
		{
			name:          "top level inline block locals are not globals",
			input:         `if (true) { let local = 1; local; }`,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := ParseScript(tt.input)
			require.NoError(t, err)

			compiler := New()
			require.NoError(t, compiler.Compile(program))

			require.Equal(t, tt.expectedCount, compiler.Bytecode().NumGlobals)
		})
	}
}

func Test_Functions(t *testing.T) {
	tests := []compilerTestCase{
		{
			input: `fn() { 5 + 10 }`,
			expectedConstants: []interface{}{
				5,
				10,
				[]code.Instructions{
					code.Make(code.OpConstant, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input: `fn(a) { a }`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 0, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func Test_Builtins(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             `len;`,
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetBuiltin, 0),
				code.Make(code.OpPop),
			},
		},
		{
			input:             `len([]); push([], 1);`,
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetBuiltin, 0),
				code.Make(code.OpArray, 0),
				code.Make(code.OpCall, 1),
				code.Make(code.OpPop),
				code.Make(code.OpGetBuiltin, 5),
				code.Make(code.OpArray, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpCall, 2),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func parseCompilerExpression(t *testing.T, input string) ast.Expression {
	t.Helper()

	program, err := ParseScript(input)
	require.NoError(t, err)
	require.Len(t, program.Statements, 1)

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	require.True(t, ok)
	require.NotNil(t, stmt.Expression)
	return stmt.Expression
}

func runCompilerTests(t *testing.T, tests []compilerTestCase) {
	t.Helper()

	for _, tt := range tests {
		program, err := ParseScript(tt.input)
		require.NoError(t, err)

		compiler := New()
		require.NoError(t, compiler.Compile(program))

		bytecode := compiler.Bytecode()
		testInstructions(t, tt.expectedInstructions, bytecode.Instructions)
		testConstants(t, tt.expectedConstants, bytecode.Constants)
	}
}

func testInstructions(t *testing.T, expected []code.Instructions, actual code.Instructions) {
	t.Helper()

	concatted := concatInstructions(expected)

	require.Lenf(t, actual, len(concatted), "wrong instructions length.\nwant=%q\ngot =%q", concatted, actual)

	for i, ins := range concatted {
		require.Equalf(t, ins, actual[i], "wrong instruction at %d.\nwant=%q\ngot =%q", i, concatted, actual)
	}
}

func concatInstructions(s []code.Instructions) code.Instructions {
	out := code.Instructions{}
	for _, ins := range s {
		out = append(out, ins...)
	}
	return out
}

func instructionContainsOpcode(instructions code.Instructions, target code.Opcode) bool {
	for i := 0; i < len(instructions); {
		op := code.Opcode(instructions[i])
		if op == target {
			return true
		}

		def, err := code.Lookup(byte(op))
		if err != nil {
			i++
			continue
		}
		_, read := code.ReadOperands(def, instructions[i+1:])
		i += 1 + read
	}
	return false
}

func instructionOpcodeCount(instructions code.Instructions, target code.Opcode) int {
	count := 0
	for i := 0; i < len(instructions); {
		op := code.Opcode(instructions[i])
		if op == target {
			count++
		}

		def, err := code.Lookup(byte(op))
		if err != nil {
			i++
			continue
		}
		_, read := code.ReadOperands(def, instructions[i+1:])
		i += 1 + read
	}
	return count
}

func instructionContainsSequence(instructions code.Instructions, first, second code.Opcode) bool {
	for i := 0; i < len(instructions); {
		op := code.Opcode(instructions[i])
		def, err := code.Lookup(byte(op))
		if err != nil {
			i++
			continue
		}
		_, read := code.ReadOperands(def, instructions[i+1:])
		next := i + 1 + read
		if op == first && next < len(instructions) && code.Opcode(instructions[next]) == second {
			return true
		}
		i = next
	}
	return false
}

func callNamesContain(callNames map[int]string, target string) bool {
	for _, name := range callNames {
		if name == target {
			return true
		}
	}
	return false
}

func bytecodeContainsOpcode(bytecode *Bytecode, target code.Opcode) bool {
	if bytecode == nil {
		return false
	}
	if instructionContainsOpcode(bytecode.Instructions, target) {
		return true
	}
	for _, constant := range bytecode.Constants {
		fn, ok := constant.(*object.CompiledFunction)
		if !ok {
			continue
		}
		if instructionContainsOpcode(fn.Instructions, target) {
			return true
		}
	}
	return false
}

func testConstants(t *testing.T, expected []interface{}, actual []object.Object) {
	t.Helper()

	require.Lenf(t, actual, len(expected), "wrong number of constants. got=%d, want=%d", len(actual), len(expected))

	for i, constant := range expected {
		switch constant := constant.(type) {
		case string:
			testStringObject(t, constant, actual[i])
		case int:
			testIntegerObject(t, int64(constant), actual[i])
		case []code.Instructions:
			fn, ok := actual[i].(*object.CompiledFunction)
			require.Truef(t, ok, "constant %d - not a function: %T", i, actual[i])
			testInstructions(t, constant, fn.Instructions)
		}
	}
}

func testIntegerObject(t *testing.T, expected int64, actual object.Object) {
	t.Helper()

	result, ok := actual.(*object.Integer)
	require.Truef(t, ok, "object is not Integer. got=%T (%+v)", actual, actual)
	require.Equal(t, expected, result.Value)
}

func testStringObject(t *testing.T, expected string, actual object.Object) {
	t.Helper()

	result, ok := actual.(*object.String)
	require.Truef(t, ok, "object is not String. got=%T (%+v)", actual, actual)
	require.Equal(t, expected, result.Value)
}
