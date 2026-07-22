package compiler

import (
	"testing"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/parser"
	"github.com/gobuffalo/plush/v5/token"
	"github.com/stretchr/testify/require"
)

func parseFast_Test_Expression(t *testing.T, input string) ast.Expression {
	t.Helper()
	program, err := parser.Parse("<%= " + input + " %>")
	require.NoError(t, err)
	require.Len(t, program.Statements, 1)
	stmt, ok := program.Statements[0].(*ast.ReturnStatement)
	require.True(t, ok)
	require.NotNil(t, stmt.ReturnValue)
	return stmt.ReturnValue
}

func compilerFast_Test_Return(expr ast.Expression) *ast.ReturnStatement {
	return &ast.ReturnStatement{Type: token.E_START, ReturnValue: expr}
}

func Test_Fast_Render_AST_Program_And_Static_Edges(t *testing.T) {
	require.Nil(t, fastRenderPlanFromProgram(nil))
	require.Nil(t, fastRenderPlanFromProgram(&ast.Program{Statements: []ast.Statement{
		&ast.ExpressionStatement{Expression: &ast.HTMLLiteral{Value: "static"}},
	}}))

	plan := &FastRenderPlan{}
	segments := []FastRenderSegment{}
	require.False(t, appendFastStatement(plan, &segments, &ast.ReturnStatement{
		Type:        token.RETURN,
		ReturnValue: &ast.StringLiteral{Value: "nope"},
	}))
	require.False(t, appendFastStatement(plan, &segments, &ast.ExpressionStatement{
		Expression: &ast.IntegerLiteral{Value: 7},
	}))

	appendFastStatic(plan, &segments, "")
	require.Empty(t, segments)

	require.True(t, appendFastOutputExpression(plan, &segments, &ast.StringLiteral{Value: "<x>"}, 1))
	require.True(t, appendFastOutputExpression(plan, &segments, &ast.HTMLLiteral{Value: "<b>"}, 1))
	require.True(t, appendFastOutputExpression(plan, &segments, &ast.IntegerLiteral{Value: 7}, 1))
	require.True(t, appendFastOutputExpression(plan, &segments, &ast.FloatLiteral{Value: 1.5}, 1))
	require.True(t, appendFastOutputExpression(plan, &segments, &ast.Boolean{Value: true}, 1))
	require.Len(t, segments, 1)
	require.Equal(t, "&lt;x&gt;<b>71.5true", segments[0].Value)

	require.False(t, appendFastOutputExpression(plan, &segments, &ast.ForExpression{}, 1))
	require.False(t, appendFastOutputExpression(plan, &segments, &ast.IfExpression{}, 1))
	require.False(t, appendFastOutputExpression(plan, &segments, &ast.CallExpression{
		Function: &ast.StringLiteral{Value: "not-callable"},
	}, 1))
}

func Test_Fast_Render_AST_Feature_Scan_Edges(t *testing.T) {
	require.Equal(t, bytecodeFeatures{}, scanInstructionFeatures(code.Instructions{255}, nil, nil))

	constants := []object.Object{&object.String{Value: "partial"}}
	instructions := code.Instructions{}
	instructions = append(instructions, code.Make(code.OpHole, 0)...)
	instructions = append(instructions, code.Make(code.OpWriteNameCall, 0, 0)...)
	features := scanInstructionFeatures(instructions, constants, nil)
	require.True(t, features.HasHoles)
	require.True(t, features.HasPartials)

	features = scanInstructionFeatures(code.Make(code.OpCall, 0), constants, map[int]string{0: "partial"})
	require.True(t, features.HasPartials)

	fn := &object.CompiledFunction{Instructions: code.Make(code.OpHole, 0)}
	nested := &object.CompiledFunction{Instructions: code.Make(code.OpWriteName, 0)}
	features = bytecodeFeaturesFromInstructions(nil, []object.Object{constants[0], fn, nested}, nil)
	require.True(t, features.HasHoles)
	require.True(t, features.HasPartials)
}

func Test_Fast_Render_AST_Value_Plan_Edges(t *testing.T) {
	plan := &FastRenderPlan{}

	value, ok := fastValuePlanFromIdentifier(plan, nil, false, 2)
	require.False(t, ok)
	require.Equal(t, FastValuePlan{}, value)

	value, ok = fastValuePlanFromIdentifier(plan, &ast.Identifier{Value: "nil"}, false, 2)
	require.True(t, ok)
	require.Equal(t, FastValueName, value.Kind)
	require.Equal(t, -1, value.NameIndex)
	require.True(t, value.NullOnMissing)

	value, ok = fastValuePlanFromIdentifier(plan, &ast.Identifier{Value: "robot.Name.Echo"}, false, 2)
	require.True(t, ok)
	require.Equal(t, FastValuePath, value.Kind)
	require.Len(t, value.Path, 2)

	_, ok = fastValuePlanFromIndexExpression(plan, &ast.IndexExpression{
		Left:  &ast.StringLiteral{Value: "not-path"},
		Index: &ast.IntegerLiteral{Value: 0},
	}, false, 3)
	require.False(t, ok)

	_, ok = fastValuePlanFromIndexExpression(plan, &ast.IndexExpression{
		Left:  &ast.Identifier{Value: "items"},
		Index: &ast.Identifier{Value: "dynamic"},
	}, false, 3)
	require.False(t, ok)
	_, ok = fastValuePlanFromIndexExpression(plan, &ast.IndexExpression{
		Left:   &ast.Identifier{Value: "items"},
		Index:  &ast.IntegerLiteral{Value: 0},
		Callee: &ast.StringLiteral{Value: "bad"},
	}, false, 3)
	require.False(t, ok)

	_, ok = fastValuePlanFromCallExpression(plan, &ast.CallExpression{
		Function: &ast.Identifier{Value: "helper"},
		Block:    &ast.BlockStatement{},
	}, false, 4)
	require.False(t, ok)

	_, ok = fastValuePlanFromCallExpression(plan, &ast.CallExpression{
		Function:  &ast.Identifier{Value: "partial"},
		Arguments: []ast.Expression{&ast.StringLiteral{Value: "row"}},
	}, false, 4)
	require.False(t, ok)

	_, ok = fastValuePlanFromCallExpression(plan, &ast.CallExpression{
		Function: &ast.StringLiteral{Value: "not-path"},
	}, false, 4)
	require.False(t, ok)

	_, ok = fastValuePlanFromCallExpression(plan, &ast.CallExpression{
		Function: &ast.Identifier{Value: "robot.Name"},
		ChainCallee: &ast.CallExpression{
			Function:  &ast.Identifier{Value: "Echo"},
			Arguments: []ast.Expression{&ast.StringLiteral{Value: "bad"}},
		},
	}, false, 4)
	require.False(t, ok)

	_, ok = fastValuePlanFromCallExpression(plan, &ast.CallExpression{
		Function:    &ast.Identifier{Value: "robot"},
		ChainCallee: &ast.StringLiteral{Value: "bad"},
	}, false, 4)
	require.False(t, ok)

	path := FastValuePlan{Kind: FastValuePath, Value: "robot"}
	require.False(t, appendFastReceiverCallee(&path, &ast.StringLiteral{Value: "bad"}, "robot", 5))
	require.False(t, appendFastReceiverCallee(&path, &ast.IndexExpression{
		Left:  &ast.StringLiteral{Value: "bad"},
		Index: &ast.IntegerLiteral{Value: 0},
	}, "robot", 5))
	require.True(t, appendFastReceiverCallee(&path, &ast.IndexExpression{
		Left:  &ast.Identifier{Value: "robot"},
		Index: &ast.IntegerLiteral{Value: 0},
	}, "robot", 5))
	require.False(t, appendFastReceiverCallee(&path, &ast.CallExpression{
		Function: &ast.StringLiteral{Value: "bad"},
	}, "robot", 5))
}

func Test_Fast_Render_AST_Partial_And_Conditional_Edges(t *testing.T) {
	plan := &FastRenderPlan{}

	partial, ok := fastPartialPlanFromCall(plan, nil, 1)
	require.False(t, ok)
	require.Nil(t, partial)

	partial, ok = fastPartialPlanFromCall(plan, &ast.CallExpression{
		Function:  &ast.Identifier{Value: "partial"},
		Arguments: []ast.Expression{&ast.IntegerLiteral{Value: 1}},
	}, 1)
	require.False(t, ok)
	require.Nil(t, partial)

	_, ok = fastPartialDataPlanFromExpression(plan, &ast.IntegerLiteral{Value: 1}, 1)
	require.False(t, ok)
	_, ok = fastPartialDataPlanFromExpression(plan, &ast.HashLiteral{
		Order: []ast.Expression{&ast.StringLiteral{Value: "layout"}},
		Pairs: map[ast.Expression]ast.Expression{
			&ast.StringLiteral{Value: "layout"}: &ast.StringLiteral{Value: "app"},
		},
	}, 1)
	require.False(t, ok)
	_, ok = fastPartialDataKey(&ast.Identifier{Value: ""})
	require.False(t, ok)
	_, ok = fastPartialDataKey(&ast.IntegerLiteral{Value: 1})
	require.False(t, ok)
	key := &ast.StringLiteral{Value: "label"}
	data, ok := fastPartialDataPlanFromExpression(plan, &ast.HashLiteral{
		Pairs: map[ast.Expression]ast.Expression{
			key: &ast.StringLiteral{Value: "ok"},
		},
	}, 1)
	require.True(t, ok)
	require.Len(t, data, 1)

	conditional, ok := fastConditionalPlanFromExpression(plan, nil, 1)
	require.False(t, ok)
	require.Nil(t, conditional)

	conditional, ok = fastConditionalPlanFromExpression(plan, &ast.IfExpression{
		Condition: &ast.Identifier{Value: "ok"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 1}},
		}},
	}, 1)
	require.False(t, ok)
	require.Nil(t, conditional)

	conditional, ok = fastConditionalPlanFromExpression(plan, &ast.IfExpression{
		Condition: &ast.Identifier{Value: "ok"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "yes"}),
		}},
		ElseIf: []*ast.ElseIfExpression{nil},
	}, 1)
	require.False(t, ok)
	require.Nil(t, conditional)

	conditional, ok = fastConditionalPlanFromExpression(plan, &ast.IfExpression{
		Condition: &ast.Identifier{Value: "ok"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "yes"}),
		}},
		ElseIf: []*ast.ElseIfExpression{{Condition: &ast.HashLiteral{}, Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "else-if"}),
		}}}},
	}, 1)
	require.False(t, ok)
	require.Nil(t, conditional)
	conditional, ok = fastConditionalPlanFromExpression(plan, &ast.IfExpression{
		Condition: &ast.Identifier{Value: "ok"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "yes"}),
		}},
		ElseIf: []*ast.ElseIfExpression{{Condition: &ast.Identifier{Value: "ok2"}, Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 1}},
		}}}},
	}, 1)
	require.False(t, ok)
	require.Nil(t, conditional)
	conditional, ok = fastConditionalPlanFromExpression(plan, &ast.IfExpression{
		Condition: &ast.Identifier{Value: "ok"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "yes"}),
		}},
		ElseBlock: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 1}},
		}},
	}, 1)
	require.False(t, ok)
	require.Nil(t, conditional)
}

func Test_Fast_Render_AST_Loop_Edge_Branches(t *testing.T) {
	plan := &FastRenderPlan{}
	loop := &FastLoopPlan{KeyName: "i", ValueName: "product"}

	gotLoop, ok := fastLoopPlanFromExpression(plan, nil, 1)
	require.False(t, ok)
	require.Nil(t, gotLoop)
	gotLoop, ok = fastLoopPlanFromExpression(plan, &ast.ForExpression{Iterable: &ast.Identifier{Value: "items"}}, 1)
	require.False(t, ok)
	require.Nil(t, gotLoop)
	gotLoop, ok = fastLoopPlanFromExpression(plan, &ast.ForExpression{Iterable: &ast.Identifier{Value: "nil"}, Block: &ast.BlockStatement{}}, 1)
	require.False(t, ok)
	require.Nil(t, gotLoop)
	gotLoop, ok = fastLoopPlanFromExpression(plan, &ast.ForExpression{
		Iterable: &ast.Identifier{Value: "items"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 1}},
		}},
	}, 1)
	require.False(t, ok)
	require.Nil(t, gotLoop)

	parts := []FastLoopPart{}
	require.False(t, appendFastLoopStatement(plan, loop, &parts, &ast.ReturnStatement{
		Type:        token.RETURN,
		ReturnValue: &ast.StringLiteral{Value: "bad"},
	}))
	require.False(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.CallExpression{
		Function: &ast.Identifier{Value: "product.Name"},
		Block:    &ast.BlockStatement{},
	}, 1))
	require.False(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.IfExpression{
		Condition: &ast.HashLiteral{},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "bad"}),
		}},
	}, 1))
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.Identifier{Value: "other"}, 1))

	parts = nil
	appendFastLoopStatic(nil, nil, &parts, "")
	require.Empty(t, parts)
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.StringLiteral{Value: "<x>"}, 1))
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.HTMLLiteral{Value: "<b>"}, 1))
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.IntegerLiteral{Value: 7}, 1))
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.FloatLiteral{Value: 1.5}, 1))
	require.True(t, appendFastLoopOutputParts(plan, loop, &parts, &ast.Boolean{Value: false}, 1))
	require.Equal(t, "&lt;x&gt;<b>71.5false", parts[0].Value)

	condition, ok := fastLoopConditionalPlanFromExpression(plan, loop, nil, 1)
	require.False(t, ok)
	require.Nil(t, condition)
	condition, ok = fastLoopConditionalPlanFromExpression(plan, loop, &ast.IfExpression{
		Condition: &ast.Identifier{Value: "product.Name"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 1}},
		}},
	}, 1)
	require.False(t, ok)
	require.Nil(t, condition)
	condition, ok = fastLoopConditionalPlanFromExpression(plan, loop, &ast.IfExpression{
		Condition: &ast.Identifier{Value: "product.Name"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "yes"}),
		}},
		ElseIf: []*ast.ElseIfExpression{{Condition: &ast.HashLiteral{}, Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "else-if"}),
		}}}},
	}, 1)
	require.False(t, ok)
	require.Nil(t, condition)
	condition, ok = fastLoopConditionalPlanFromExpression(plan, loop, &ast.IfExpression{
		Condition: &ast.Identifier{Value: "product.Name"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "yes"}),
		}},
		ElseIf: []*ast.ElseIfExpression{{Condition: &ast.Identifier{Value: "product.Name"}, Block: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 1}},
		}}}},
	}, 1)
	require.False(t, ok)
	require.Nil(t, condition)
	condition, ok = fastLoopConditionalPlanFromExpression(plan, loop, &ast.IfExpression{
		Condition: &ast.Identifier{Value: "product.Name"},
		Block: &ast.BlockStatement{Statements: []ast.Statement{
			compilerFast_Test_Return(&ast.StringLiteral{Value: "yes"}),
		}},
		ElseBlock: &ast.BlockStatement{Statements: []ast.Statement{
			&ast.ExpressionStatement{Expression: &ast.IntegerLiteral{Value: 1}},
		}},
	}, 1)
	require.False(t, ok)
	require.Nil(t, condition)

	value, ok := fastValuePlanFromLoopOperand(plan, nil, &ast.Identifier{Value: "product"}, false, 1)
	require.False(t, ok)
	require.Equal(t, FastValuePlan{}, value)
	value, ok = fastValuePlanFromLoopOperand(plan, loop, &ast.Identifier{Value: "i.Name"}, false, 1)
	require.False(t, ok)
	require.Equal(t, FastValuePlan{}, value)
	_, ok = fastValuePlanFromLoopInfix(plan, loop, &ast.InfixExpression{
		Left:     &ast.Identifier{Value: "product.Name"},
		Operator: "??",
		Right:    &ast.StringLiteral{Value: "x"},
	}, 1)
	require.False(t, ok)
	_, ok = fastValuePlanFromLoopInfix(plan, loop, &ast.InfixExpression{
		Left:     &ast.Identifier{Value: "product.Name"},
		Operator: "==",
		Right:    &ast.HashLiteral{},
	}, 1)
	require.False(t, ok)
	value, ok = fastValuePlanFromLoopOperand(plan, loop, &ast.InfixExpression{
		Left:     &ast.Identifier{Value: "product.Name"},
		Operator: "==",
		Right:    &ast.StringLiteral{Value: "x"},
	}, false, 1)
	require.True(t, ok)
	require.Equal(t, FastValueInfix, value.Kind)
	loopCallPlan, ok := fastLoopCallPlanFromExpression(plan, loop, &ast.CallExpression{
		Function:  &ast.Identifier{Value: "label"},
		Arguments: []ast.Expression{&ast.HashLiteral{}},
	}, 1)
	require.True(t, ok)
	require.NotNil(t, loopCallPlan)
	require.Len(t, loopCallPlan.Args, 1)
	require.Equal(t, FastValueHash, loopCallPlan.Args[0].Kind)

	root, ok := fastLoopExpressionRootName(&ast.Identifier{Value: "."})
	require.False(t, ok)
	require.Empty(t, root)
	root, ok = fastLoopExpressionRootName(&ast.IndexExpression{Left: &ast.Identifier{Value: "product"}, Index: &ast.IntegerLiteral{Value: 0}})
	require.True(t, ok)
	require.Equal(t, "product", root)

	_, ok = fastValuePlanFromLoopIndex(loop, &ast.IndexExpression{Left: &ast.Identifier{Value: "product.Name"}, Index: &ast.Identifier{Value: "bad"}}, 1)
	require.False(t, ok)
	_, ok = fastValuePlanFromLoopIndex(loop, &ast.IndexExpression{
		Left:   &ast.Identifier{Value: "product.Name"},
		Index:  &ast.IntegerLiteral{Value: 0},
		Callee: &ast.StringLiteral{Value: "bad"},
	}, 1)
	require.False(t, ok)
	_, ok = fastValuePlanFromLoopCall(loop, &ast.CallExpression{Block: &ast.BlockStatement{}}, 1)
	require.False(t, ok)
	_, ok = fastValuePlanFromLoopCall(loop, &ast.CallExpression{
		Function:  &ast.Identifier{Value: "product.Name"},
		Arguments: []ast.Expression{&ast.StringLiteral{Value: "bad"}},
	}, 1)
	require.False(t, ok)
	_, ok = fastValuePlanFromLoopCall(loop, &ast.CallExpression{Function: &ast.StringLiteral{Value: "bad"}}, 1)
	require.False(t, ok)
	_, ok = fastValuePlanFromLoopCall(loop, &ast.CallExpression{
		Function:    &ast.Identifier{Value: "product.Name"},
		ChainCallee: &ast.StringLiteral{Value: "bad"},
	}, 1)
	require.False(t, ok)
	_, ok = fastValuePlanFromLoopExpressionWithMethod(loop, &ast.StringLiteral{Value: "bad"}, 1, false)
	require.False(t, ok)
	value, ok = fastValuePlanFromLoopExpressionWithMethod(loop, &ast.IndexExpression{
		Left:  &ast.Identifier{Value: "product.Name"},
		Index: &ast.IntegerLiteral{Value: 0},
	}, 1, false)
	require.True(t, ok)
	require.Equal(t, FastValuePath, value.Kind)

	expr := parseFast_Test_Expression(t, `product.Name()`)
	call, ok := expr.(*ast.CallExpression)
	require.True(t, ok)
	value, ok = fastValuePlanFromLoopCall(loop, call, 1)
	require.True(t, ok)
	require.Equal(t, FastValuePath, value.Kind)
	require.NotEmpty(t, value.Path)
}
