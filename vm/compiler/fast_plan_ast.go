package compiler

import (
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/token"
	"github.com/gobuffalo/plush/v5/vm/code"
	"github.com/gobuffalo/plush/v5/vm/object"
)

// fastRenderPlanFromProgram builds the richest fast render plan directly from
// the Plush AST. It only returns a plan when every top-level statement has a
// supported render shape; unsupported shapes intentionally fall back to
// bytecode/VM execution. See vm/FAST_PATHS.md.
func fastRenderPlanFromProgram(program *ast.Program) *FastRenderPlan {
	plan, _ := fastRenderPlanAnalysisFromProgram(program)
	return plan
}

func fastRenderPlanAnalysisFromProgram(program *ast.Program) (*FastRenderPlan, FastRenderReject) {
	if program == nil {
		return nil, FastRenderReject{}
	}
	plan := &FastRenderPlan{}
	if !appendFastStatements(plan, &plan.Segments, program.Statements) {
		reject := firstFastRenderReject(program)
		if reject.Reason == "" {
			reject = firstFastRenderBuildReject(program)
		}
		if reject.Reason == "" {
			reject = FastRenderReject{Line: 1, Reason: "fast render plan builder declined after analyzer accepted"}
		}
		return nil, reject
	}
	if len(plan.Segments) == 0 {
		return nil, FastRenderReject{}
	}
	return plan, FastRenderReject{}
}

func firstFastRenderReject(program *ast.Program) FastRenderReject {
	if program == nil {
		return FastRenderReject{}
	}
	plan := &FastRenderPlan{}
	return firstFastRenderStatementReject(plan, nil, program.Statements, false)
}

func firstFastRenderBuildReject(program *ast.Program) FastRenderReject {
	if program == nil {
		return FastRenderReject{}
	}
	plan := &FastRenderPlan{}
	return fastRenderBuildStatementsReject(plan, nil, nil, program.Statements, false)
}

func fastRenderBuildStatementsReject(plan *FastRenderPlan, segments *[]FastRenderSegment, loop *FastLoopPlan, statements []ast.Statement, inLoop bool) FastRenderReject {
	for _, stmt := range statements {
		if reject := fastRenderBuildStatementReject(plan, segments, loop, stmt, inLoop); reject.Reason != "" {
			return reject
		}
	}
	return FastRenderReject{}
}

func fastRenderBuildStatementReject(plan *FastRenderPlan, segments *[]FastRenderSegment, loop *FastLoopPlan, stmt ast.Statement, inLoop bool) FastRenderReject {
	if inLoop {
		parts := []FastLoopPart{}
		if appendFastLoopStatement(plan, loop, &parts, stmt) {
			return FastRenderReject{}
		}
		return fastRenderBuildLoopStatementReject(plan, loop, stmt)
	}
	if segments == nil {
		local := []FastRenderSegment{}
		segments = &local
	}
	if appendFastStatement(plan, segments, stmt) {
		return FastRenderReject{}
	}
	switch stmt := stmt.(type) {
	case *ast.ExpressionStatement:
		if ifExpression, ok := stmt.Expression.(*ast.IfExpression); ok {
			return fastRenderBuildConditionalReject(plan, ifExpression, lineForNode(stmt), true)
		}
		return rejectFastRender(stmt, "fast render builder declined expression statement: "+fastExpressionSummary(stmt.Expression))
	case *ast.ReturnStatement:
		if stmt.Type != token.E_START {
			return rejectFastRender(stmt, "fast render builder declined non-output return")
		}
		return fastRenderBuildOutputReject(plan, stmt.ReturnValue, lineForNode(stmt))
	case *ast.LetStatement:
		return rejectFastRender(stmt, "fast render builder declined let statement")
	default:
		return rejectFastRender(stmt, "fast render builder declined statement")
	}
}

func fastRenderBuildOutputReject(plan *FastRenderPlan, expr ast.Expression, line int) FastRenderReject {
	segments := []FastRenderSegment{}
	if appendFastOutputExpression(plan, &segments, expr, line) {
		return FastRenderReject{}
	}
	switch expr := expr.(type) {
	case *ast.IfExpression:
		return fastRenderBuildConditionalReject(plan, expr, line, false)
	case *ast.ForExpression:
		return fastRenderBuildLoopReject(plan, nil, expr, line)
	default:
		return FastRenderReject{Line: line, Reason: "fast render builder declined output expression: " + fastExpressionSummary(expr)}
	}
}

func fastRenderBuildConditionalReject(plan *FastRenderPlan, expr *ast.IfExpression, line int, silent bool) FastRenderReject {
	var conditional *FastConditionalPlan
	var ok bool
	if silent {
		conditional, ok = fastSilentConditionalPlanFromExpression(plan, expr, line)
	} else {
		conditional, ok = fastConditionalPlanFromExpression(plan, expr, line)
	}
	if ok && conditional != nil {
		return FastRenderReject{}
	}
	if expr == nil || expr.Block == nil {
		return FastRenderReject{Line: line, Reason: "fast render builder declined if without block"}
	}
	if reject := fastRenderBuildStatementsReject(plan, nil, nil, expr.Block.Statements, false); reject.Reason != "" {
		return reject
	}
	for _, elseIf := range expr.ElseIf {
		if elseIf != nil && elseIf.Block != nil {
			if reject := fastRenderBuildStatementsReject(plan, nil, nil, elseIf.Block.Statements, false); reject.Reason != "" {
				return reject
			}
		}
	}
	if expr.ElseBlock != nil {
		return fastRenderBuildStatementsReject(plan, nil, nil, expr.ElseBlock.Statements, false)
	}
	return FastRenderReject{Line: line, Reason: "fast render builder declined if expression"}
}

func fastRenderBuildLoopReject(plan *FastRenderPlan, parent *FastLoopPlan, expr *ast.ForExpression, line int) FastRenderReject {
	loop, ok := fastLoopPlanFromExpressionWithOuterNames(plan, fastLoopOuterNames(parent), expr, line)
	if ok && loop != nil {
		return FastRenderReject{}
	}
	if expr == nil || expr.Block == nil {
		return FastRenderReject{Line: line, Reason: "fast render builder declined loop without block"}
	}
	iterable, _ := fastValuePlanFromExpression(plan, expr.Iterable, false, lineForNode(expr.Iterable))
	loop = &FastLoopPlan{
		IterableName:      iterable.Value,
		IterableNameIndex: iterable.NameIndex,
		Iterable:          iterable,
		KeyName:           expr.KeyName,
		ValueName:         expr.ValueName,
		OuterNames:        fastLoopOuterNames(parent),
		Line:              line,
	}
	return fastRenderBuildStatementsReject(plan, nil, loop, expr.Block.Statements, true)
}

func fastRenderBuildLoopStatementReject(plan *FastRenderPlan, loop *FastLoopPlan, stmt ast.Statement) FastRenderReject {
	switch stmt := stmt.(type) {
	case *ast.ExpressionStatement:
		if ifExpression, ok := stmt.Expression.(*ast.IfExpression); ok {
			return fastRenderBuildLoopConditionalReject(plan, loop, ifExpression, lineForNode(stmt), true)
		}
		return rejectFastRender(stmt, "fast render builder declined loop expression statement: "+fastExpressionSummary(stmt.Expression))
	case *ast.ReturnStatement:
		return fastRenderBuildLoopOutputReject(plan, loop, stmt.ReturnValue, lineForNode(stmt))
	default:
		return rejectFastRender(stmt, "fast render builder declined loop statement")
	}
}

func fastRenderBuildLoopOutputReject(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, line int) FastRenderReject {
	parts := []FastLoopPart{}
	if appendFastLoopOutputParts(plan, loop, &parts, expr, line) {
		return FastRenderReject{}
	}
	switch expr := expr.(type) {
	case *ast.IfExpression:
		return fastRenderBuildLoopConditionalReject(plan, loop, expr, line, false)
	case *ast.ForExpression:
		return fastRenderBuildLoopReject(plan, loop, expr, line)
	default:
		return FastRenderReject{Line: line, Reason: "fast render builder declined loop output expression: " + fastExpressionSummary(expr)}
	}
}

func fastRenderBuildLoopConditionalReject(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.IfExpression, line int, silent bool) FastRenderReject {
	var conditional *FastLoopConditionalPlan
	var ok bool
	if silent {
		conditional, ok = fastSilentLoopConditionalPlanFromExpression(plan, loop, expr, line)
	} else {
		conditional, ok = fastLoopConditionalPlanFromExpression(plan, loop, expr, line)
	}
	if ok && conditional != nil {
		return FastRenderReject{}
	}
	if expr == nil || expr.Block == nil {
		return FastRenderReject{Line: line, Reason: "fast render builder declined loop if without block"}
	}
	if reject := fastRenderBuildStatementsReject(plan, nil, loop, expr.Block.Statements, true); reject.Reason != "" {
		return reject
	}
	for _, elseIf := range expr.ElseIf {
		if elseIf != nil && elseIf.Block != nil {
			if reject := fastRenderBuildStatementsReject(plan, nil, loop, elseIf.Block.Statements, true); reject.Reason != "" {
				return reject
			}
		}
	}
	if expr.ElseBlock != nil {
		return fastRenderBuildStatementsReject(plan, nil, loop, expr.ElseBlock.Statements, true)
	}
	return FastRenderReject{Line: line, Reason: "fast render builder declined loop if expression"}
}

func firstFastRenderStatementReject(plan *FastRenderPlan, loop *FastLoopPlan, statements []ast.Statement, inLoop bool) FastRenderReject {
	for _, stmt := range statements {
		if reject := fastRenderStatementReject(plan, loop, stmt, inLoop); reject.Reason != "" {
			return reject
		}
	}
	return FastRenderReject{}
}

func fastRenderStatementReject(plan *FastRenderPlan, loop *FastLoopPlan, stmt ast.Statement, inLoop bool) FastRenderReject {
	switch stmt := stmt.(type) {
	case *ast.ExpressionStatement:
		if isFastCommentStatement(stmt) {
			return FastRenderReject{}
		}
		if _, ok := stmt.Expression.(*ast.HTMLLiteral); ok {
			return FastRenderReject{}
		}
		if inLoop {
			switch expr := stmt.Expression.(type) {
			case *ast.BreakExpression, *ast.ContinueExpression:
				return FastRenderReject{}
			case *ast.CallExpression:
				if expr.Block != nil {
					if _, ok := fastSilentLoopBlockCallPlanFromExpression(plan, loop, expr, lineForNode(stmt)); ok {
						return FastRenderReject{}
					}
					return fastRenderLoopBlockCallReject(plan, loop, expr, lineForNode(stmt))
				}
				if _, ok := fastSilentLoopCallPlanFromExpression(plan, loop, expr, lineForNode(stmt)); ok {
					return FastRenderReject{}
				}
				return fastRenderCallReject(plan, expr, lineForNode(stmt))
			case *ast.ForExpression:
				if _, ok := fastSilentNestedLoopPlanFromExpression(plan, loop, expr, lineForNode(stmt)); ok {
					return FastRenderReject{}
				}
				return fastRenderLoopReject(plan, loop, expr, lineForNode(stmt))
			case *ast.AssignExpression:
				parts := []FastLoopPart{}
				if appendFastLoopAssignExpression(plan, loop, &parts, expr, lineForNode(stmt)) {
					return FastRenderReject{}
				}
				return fastRenderValueReject(plan, expr.Value, "loop assignment value")
			case *ast.IndexExpression:
				if expr.Value != nil {
					parts := []FastLoopPart{}
					if appendFastLoopIndexAssignExpression(plan, loop, &parts, expr, lineForNode(stmt)) {
						return FastRenderReject{}
					}
					return fastRenderIndexAssignReject(plan, loop, expr, lineForNode(stmt), true)
				}
			}
		}
		if ifExpression, ok := stmt.Expression.(*ast.IfExpression); ok {
			if inLoop {
				if _, ok := fastSilentLoopConditionalPlanFromExpression(plan, loop, ifExpression, lineForNode(stmt)); ok {
					return FastRenderReject{}
				}
				return fastRenderLoopConditionalReject(plan, loop, ifExpression, lineForNode(stmt))
			}
			if _, ok := fastSilentConditionalPlanFromExpression(plan, ifExpression, lineForNode(stmt)); ok {
				return FastRenderReject{}
			}
			if reject := fastSilentConditionalReject(plan, ifExpression, lineForNode(stmt)); reject.Reason != "" {
				return reject
			}
		}
		if !inLoop {
			if assign, ok := stmt.Expression.(*ast.AssignExpression); ok {
				segments := []FastRenderSegment{}
				if appendFastAssignExpression(plan, &segments, assign, lineForNode(stmt)) {
					return FastRenderReject{}
				}
				return fastRenderValueReject(plan, assign.Value, "assignment value")
			}
			if index, ok := stmt.Expression.(*ast.IndexExpression); ok && index.Value != nil {
				segments := []FastRenderSegment{}
				if appendFastIndexAssignExpression(plan, &segments, index, lineForNode(stmt)) {
					return FastRenderReject{}
				}
				return fastRenderIndexAssignReject(plan, nil, index, lineForNode(stmt), false)
			}
			if forExpression, ok := stmt.Expression.(*ast.ForExpression); ok {
				if _, ok := fastSilentLoopPlanFromExpression(plan, forExpression, lineForNode(stmt)); ok {
					return FastRenderReject{}
				}
				return fastRenderLoopReject(plan, nil, forExpression, lineForNode(stmt))
			}
			if callExpression, ok := stmt.Expression.(*ast.CallExpression); ok && callExpression.Block != nil {
				if _, ok := fastSilentBlockCallPlanFromExpression(plan, callExpression, lineForNode(stmt)); ok {
					return FastRenderReject{}
				}
				return fastRenderBlockCallReject(plan, callExpression, lineForNode(stmt))
			}
			if callExpression, ok := stmt.Expression.(*ast.CallExpression); ok {
				if _, ok := fastSilentCallPlanFromExpression(plan, callExpression, lineForNode(stmt)); ok {
					return FastRenderReject{}
				}
				return fastRenderCallReject(plan, callExpression, lineForNode(stmt))
			}
		}
		return rejectFastRender(stmt, "script expression statements are not fast-planned: "+fastExpressionSummary(stmt.Expression))
	case *ast.ReturnStatement:
		if inLoop {
			return fastRenderLoopOutputReject(plan, loop, stmt.ReturnValue, lineForNode(stmt))
		}
		if stmt.Type != token.E_START {
			if _, ok := fastValuePlanFromExpression(plan, stmt.ReturnValue, false, lineForNode(stmt.ReturnValue)); ok {
				return FastRenderReject{}
			}
			return fastRenderValueReject(plan, stmt.ReturnValue, "return value")
		}
		return fastRenderOutputReject(plan, stmt.ReturnValue, lineForNode(stmt))
	case *ast.LetStatement:
		if stmt.Name == nil || stmt.Name.Callee != nil || stmt.Name.Value == "" {
			return rejectFastRender(stmt, "unsupported let target")
		}
		if inLoop {
			if _, ok := fastValuePlanFromLoopOperand(plan, loop, stmt.Value, false, lineForNode(stmt.Value)); ok {
				return FastRenderReject{}
			}
			return rejectFastRender(stmt.Value, "loop let value is not fast-planned: "+fastExpressionSummary(stmt.Value))
		}
		if _, ok := fastValuePlanFromExpression(plan, stmt.Value, false, lineForNode(stmt.Value)); ok {
			return FastRenderReject{}
		}
		return fastRenderValueReject(plan, stmt.Value, "let value")
	default:
		return rejectFastRender(stmt, "unsupported statement type for fast render")
	}
}

func isFastCommentStatement(stmt *ast.ExpressionStatement) bool {
	if stmt == nil {
		return false
	}
	literal, ok := stmt.Expression.(*ast.StringLiteral)
	if !ok || literal.Value != "" {
		return false
	}
	return stmt.Token.Type == token.C_START || literal.Token.Type == token.E_END
}

func fastRenderOutputReject(plan *FastRenderPlan, expr ast.Expression, line int) FastRenderReject {
	switch expr := expr.(type) {
	case *ast.StringLiteral, *ast.HTMLLiteral, *ast.IntegerLiteral, *ast.FloatLiteral, *ast.Boolean:
		return FastRenderReject{}
	case *ast.ForExpression:
		return fastRenderLoopReject(plan, nil, expr, line)
	case *ast.IfExpression:
		return fastRenderConditionalReject(plan, expr, line)
	case *ast.CallExpression:
		if _, ok := fastBlockCallPlanFromExpression(plan, expr, line); ok {
			return FastRenderReject{}
		}
		if _, ok := fastPartialPlanFromCall(plan, expr, line); ok {
			return FastRenderReject{}
		}
		if _, ok := fastCallPlanFromExpression(plan, expr, line, false); ok {
			return FastRenderReject{}
		}
		if _, ok := fastValuePlanFromExpression(plan, expr, false, line); ok {
			return FastRenderReject{}
		}
		return fastRenderCallReject(plan, expr, line)
	default:
		if _, ok := fastValuePlanFromExpression(plan, expr, false, line); ok {
			return FastRenderReject{}
		}
		return fastRenderValueReject(plan, expr, "output expression")
	}
}

func fastRenderConditionalReject(plan *FastRenderPlan, expr *ast.IfExpression, line int) FastRenderReject {
	if expr == nil || expr.Block == nil {
		return FastRenderReject{Line: line, Reason: "if expressions without a block are not fast-planned"}
	}
	if _, ok := fastValuePlanFromExpression(plan, expr.Condition, true, lineForNode(expr.Condition)); !ok {
		return fastRenderValueReject(plan, expr.Condition, "if condition")
	}
	if reject := firstFastRenderStatementReject(plan, nil, expr.Block.Statements, false); reject.Reason != "" {
		return reject
	}
	for _, elseIf := range expr.ElseIf {
		if elseIf == nil || elseIf.Block == nil {
			return FastRenderReject{Line: line, Reason: "else-if expressions without a block are not fast-planned"}
		}
		if _, ok := fastValuePlanFromExpression(plan, elseIf.Condition, true, lineForNode(elseIf.Condition)); !ok {
			return fastRenderValueReject(plan, elseIf.Condition, "else-if condition")
		}
		if reject := firstFastRenderStatementReject(plan, nil, elseIf.Block.Statements, false); reject.Reason != "" {
			return reject
		}
	}
	if expr.ElseBlock != nil {
		return firstFastRenderStatementReject(plan, nil, expr.ElseBlock.Statements, false)
	}
	return FastRenderReject{}
}

func fastRenderLoopReject(plan *FastRenderPlan, parent *FastLoopPlan, expr *ast.ForExpression, line int) FastRenderReject {
	if expr == nil || expr.Block == nil {
		return FastRenderReject{Line: line, Reason: "for expressions without a block are not fast-planned"}
	}
	iterable, ok := fastValuePlanFromExpression(plan, expr.Iterable, false, lineForNode(expr.Iterable))
	if !ok {
		return fastRenderValueReject(plan, expr.Iterable, "for iterable")
	}
	if !fastLoopIterableValueSupported(iterable) {
		return FastRenderReject{Line: lineForNode(expr.Iterable), Reason: "unsupported for iterable for fast render: " + fastValuePlanSummary(iterable)}
	}
	loop := &FastLoopPlan{
		IterableName:      iterable.Value,
		IterableNameIndex: iterable.NameIndex,
		Iterable:          iterable,
		KeyName:           expr.KeyName,
		ValueName:         expr.ValueName,
		OuterNames:        fastLoopOuterNames(parent),
		Line:              line,
	}
	return firstFastRenderStatementReject(plan, loop, expr.Block.Statements, true)
}

func fastRenderLoopOutputReject(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, line int) FastRenderReject {
	switch expr := expr.(type) {
	case *ast.StringLiteral, *ast.HTMLLiteral, *ast.IntegerLiteral, *ast.FloatLiteral, *ast.Boolean:
		return FastRenderReject{}
	case *ast.BreakExpression, *ast.ContinueExpression:
		return FastRenderReject{}
	case *ast.IfExpression:
		if _, ok := fastLoopConditionalPlanFromExpression(plan, loop, expr, line); ok {
			return FastRenderReject{}
		}
		return fastRenderLoopConditionalReject(plan, loop, expr, line)
	case *ast.ForExpression:
		return fastRenderLoopReject(plan, loop, expr, line)
	case *ast.CallExpression:
		if expr.Block != nil {
			if _, ok := fastLoopBlockCallPlanFromExpression(plan, loop, expr, line); ok {
				return FastRenderReject{}
			}
			return fastRenderLoopBlockCallReject(plan, loop, expr, line)
		}
		if _, ok := fastPartialPlanFromCall(plan, expr, line); ok {
			return FastRenderReject{}
		}
		if _, ok := fastValuePlanFromLoopCallWithPlan(plan, loop, expr, line, false); ok {
			return FastRenderReject{}
		}
		if root, ok := fastLoopExpressionRootName(expr); ok && fastLoopHasOuterName(loop, root) {
			if _, ok := fastValuePlanFromExpression(plan, expr, false, line); ok {
				return FastRenderReject{}
			}
		}
		if _, ok := fastLoopCallPlanFromExpression(plan, loop, expr, line, false); ok {
			return FastRenderReject{}
		}
		return fastRenderCallReject(plan, expr, line)
	default:
		if _, ok := fastValuePlanFromLoopOperand(plan, loop, expr, false, line); ok {
			return FastRenderReject{}
		}
		if _, ok := fastValuePlanFromLoopIndexWithPlan(plan, loop, expr, line, false); ok {
			return FastRenderReject{}
		}
		return fastRenderValueReject(plan, expr, "loop output expression")
	}
}

func fastRenderLoopBlockCallReject(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.CallExpression, line int) FastRenderReject {
	if expr == nil {
		return FastRenderReject{Line: line, Reason: "nil loop block helper calls are not fast-planned"}
	}
	if loop == nil {
		return FastRenderReject{Line: line, Reason: "loop block helper call without loop context"}
	}
	if expr.ChainCallee != nil {
		return rejectFastRender(expr, "chained loop block helper calls are not fast-planned")
	}
	ident, ok := expr.Function.(*ast.Identifier)
	if !ok || !fastPlainHelperIdentifier(ident) || ident.Value == "nil" {
		return rejectFastRender(expr, "unsupported loop block helper callee for fast render")
	}
	if !fastBlockCanRenderFromSource(expr.Block) {
		return rejectFastRender(expr.Block, "loop block helper body contains statements that cannot be source-rendered")
	}
	for _, arg := range expr.Arguments {
		if _, ok := fastValuePlanFromLoopCallArgument(plan, loop, arg, lineForNode(arg), false); !ok {
			return fastRenderValueReject(plan, arg, "loop block helper argument")
		}
	}
	return rejectFastRender(expr, "unsupported loop block helper call shape for fast render")
}

func fastRenderLoopConditionalReject(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.IfExpression, line int) FastRenderReject {
	if expr == nil || expr.Block == nil {
		return FastRenderReject{Line: line, Reason: "loop if expressions without a block are not fast-planned"}
	}
	if _, ok := fastValuePlanFromLoopCondition(plan, loop, expr.Condition, lineForNode(expr.Condition)); !ok {
		return fastRenderValueReject(plan, expr.Condition, "loop if condition")
	}
	if reject := firstFastRenderStatementReject(plan, loop, expr.Block.Statements, true); reject.Reason != "" {
		return reject
	}
	for _, elseIf := range expr.ElseIf {
		if elseIf == nil || elseIf.Block == nil {
			return FastRenderReject{Line: line, Reason: "loop else-if expressions without a block are not fast-planned"}
		}
		if _, ok := fastValuePlanFromLoopCondition(plan, loop, elseIf.Condition, lineForNode(elseIf.Condition)); !ok {
			return fastRenderValueReject(plan, elseIf.Condition, "loop else-if condition")
		}
		if reject := firstFastRenderStatementReject(plan, loop, elseIf.Block.Statements, true); reject.Reason != "" {
			return reject
		}
	}
	if expr.ElseBlock != nil {
		return firstFastRenderStatementReject(plan, loop, expr.ElseBlock.Statements, true)
	}
	return FastRenderReject{}
}

func fastRenderCallReject(plan *FastRenderPlan, expr *ast.CallExpression, line int) FastRenderReject {
	if expr == nil {
		return FastRenderReject{Line: line, Reason: "nil call expressions are not fast-planned"}
	}
	if expr.Block != nil {
		return fastRenderBlockCallReject(plan, expr, line)
	}
	if expr.ChainCallee != nil {
		return rejectFastRender(expr, "chained helper calls with arguments are not fast-planned")
	}
	for _, arg := range expr.Arguments {
		if _, ok := fastValuePlanFromExpression(plan, arg, false, lineForNode(arg)); !ok {
			return fastRenderValueReject(plan, arg, "helper argument")
		}
	}
	return rejectFastRender(expr, "unsupported helper call shape for fast render: "+fastExpressionSummary(expr))
}

func fastRenderBlockCallReject(plan *FastRenderPlan, expr *ast.CallExpression, line int) FastRenderReject {
	if expr == nil {
		return FastRenderReject{Line: line, Reason: "nil block helper calls are not fast-planned"}
	}
	if expr.ChainCallee != nil {
		return rejectFastRender(expr, "chained block helper calls are not fast-planned")
	}
	ident, ok := expr.Function.(*ast.Identifier)
	if !ok || !fastPlainHelperIdentifier(ident) || ident.Value == "nil" {
		return rejectFastRender(expr, "unsupported block helper callee for fast render")
	}
	if !fastBlockCanRenderFromSource(expr.Block) {
		return rejectFastRender(expr.Block, "block helper body contains statements that cannot be source-rendered")
	}
	for _, arg := range expr.Arguments {
		if _, ok := fastValuePlanFromExpression(plan, arg, false, lineForNode(arg)); !ok {
			return fastRenderValueReject(plan, arg, "block helper argument")
		}
	}
	return rejectFastRender(expr, "unsupported block helper call shape for fast render")
}

func fastRenderValueReject(plan *FastRenderPlan, expr ast.Expression, role string) FastRenderReject {
	line := lineForNode(expr)
	switch expr := expr.(type) {
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if _, ok := fastValuePlanFromExpression(plan, element, false, lineForNode(element)); !ok {
				return fastRenderValueReject(plan, element, role+": array literal element")
			}
		}
		return FastRenderReject{Line: line, Reason: role + ": unsupported array literal for fast render"}
	case *ast.HashLiteral:
		keys := append([]ast.Expression(nil), expr.Order...)
		if len(keys) == 0 {
			for key := range expr.Pairs {
				keys = append(keys, key)
			}
			sort.Slice(keys, func(i, j int) bool {
				return keys[i].String() < keys[j].String()
			})
		}
		for _, key := range keys {
			if _, _, ok := fastHashLiteralKeyPlan(plan, key, lineForNode(key)); !ok {
				return FastRenderReject{Line: lineForNode(key), Reason: role + ": unsupported hash literal key for fast render"}
			}
			value := expr.Pairs[key]
			if _, ok := fastValuePlanFromExpression(plan, value, false, lineForNode(value)); !ok {
				return fastRenderValueReject(plan, value, role+": hash literal value")
			}
		}
		return FastRenderReject{Line: line, Reason: role + ": unsupported hash literal for fast render"}
	case *ast.IndexExpression:
		if _, ok := fastIndexStepFromExpression(expr.Index, line); !ok {
			return FastRenderReject{Line: lineForNode(expr.Index), Reason: role + ": dynamic index expressions are not fast-planned"}
		}
		return FastRenderReject{Line: line, Reason: role + ": unsupported index expression for fast render"}
	case *ast.AssignExpression:
		return FastRenderReject{Line: line, Reason: role + ": assignment expressions are not fast-planned"}
	case *ast.FunctionLiteral:
		return FastRenderReject{Line: line, Reason: role + ": function literals are not fast-planned"}
	case *ast.CallExpression:
		return fastRenderCallReject(plan, expr, line)
	default:
		return FastRenderReject{Line: line, Reason: role + ": unsupported expression type for fast render: " + fastExpressionSummary(expr)}
	}
}

func rejectFastRender(node ast.Node, reason string) FastRenderReject {
	return FastRenderReject{Line: lineForNode(node), Reason: reason}
}

func fastExpressionSummary(expr ast.Expression) string {
	if expr == nil {
		return "<nil>"
	}
	text := strings.TrimSpace(expr.String())
	if len(text) > 80 {
		text = text[:80] + "..."
	}
	return fmt.Sprintf("%T %q", expr, text)
}

func fastValuePlanSummary(value FastValuePlan) string {
	switch value.Kind {
	case FastValueName:
		return fmt.Sprintf("name(%s)", value.Value)
	case FastValueString:
		return "string"
	case FastValueInteger:
		return "integer"
	case FastValueFloat:
		return "float"
	case FastValueBool:
		return "bool"
	case FastValuePath:
		return fmt.Sprintf("path(%s)", value.Value)
	case FastValueLoopKey:
		return fmt.Sprintf("loop-key(%s)", value.Value)
	case FastValueInfix:
		return fmt.Sprintf("infix(%s)", value.Operator)
	case FastValueCall:
		if value.Call != nil {
			return fmt.Sprintf("call(%s)", value.Call.Name)
		}
		return "call"
	case FastValuePrefix:
		return fmt.Sprintf("prefix(%s)", value.Operator)
	case FastValueConcat:
		return "concat"
	case FastValueArray:
		return "array"
	case FastValueHash:
		return "hash"
	default:
		return fmt.Sprintf("kind(%d)", value.Kind)
	}
}

type bytecodeFeatures struct {
	HasHoles         bool
	HasPartials      bool
	HasContextWrites bool
}

// bytecodeFeaturesFromInstructions records features that affect whether a fast
// render shortcut is safe, such as holes and partial calls. The VM uses these
// flags to keep cache, partial, and punch-hole behavior aligned with the
// interpreter.
func bytecodeFeaturesFromInstructions(instructions code.Instructions, constants []object.Object, callNames map[int]string) bytecodeFeatures {
	features := scanInstructionFeatures(instructions, constants, callNames)
	seen := map[*object.CompiledFunction]bool{}
	for _, constant := range constants {
		fn, ok := constant.(*object.CompiledFunction)
		if !ok {
			continue
		}
		features.merge(scanFunctionFeatures(fn, constants, seen))
	}
	return features
}

func scanFunctionFeatures(fn *object.CompiledFunction, constants []object.Object, seen map[*object.CompiledFunction]bool) bytecodeFeatures {
	if fn == nil || seen[fn] {
		return bytecodeFeatures{}
	}
	seen[fn] = true
	features := scanInstructionFeatures(fn.Instructions, constants, fn.CallNames)
	for _, constant := range constants {
		nested, ok := constant.(*object.CompiledFunction)
		if !ok || seen[nested] {
			continue
		}
		features.merge(scanFunctionFeatures(nested, constants, seen))
	}
	return features
}

func scanInstructionFeatures(instructions code.Instructions, constants []object.Object, callNames map[int]string) bytecodeFeatures {
	var features bytecodeFeatures
	for i := 0; i < len(instructions); {
		op, operands, read, ok := instructionAt(instructions, i)
		if !ok {
			return features
		}

		if op == code.OpHole {
			features.HasHoles = true
		}
		switch op {
		case code.OpSetGlobal, code.OpSetLocal, code.OpSetName, code.OpAssignName, code.OpSetIndex:
			features.HasContextWrites = true
		}
		switch op {
		case code.OpGetName, code.OpGetNameOrNull, code.OpGetNameOrJumpMissing, code.OpSetName, code.OpAssignName,
			code.OpWriteName, code.OpWriteNameOrNull:
			if len(operands) > 0 && stringConstantEquals(constants, operands[0], "partial") {
				features.HasPartials = true
			}
		case code.OpWriteNameCall:
			if len(operands) > 0 && stringConstantEquals(constants, operands[0], "partial") {
				features.HasPartials = true
			}
		case code.OpCall, code.OpWriteCall, code.OpCallBlock:
			if callNames != nil && callNames[i] == "partial" {
				features.HasPartials = true
			}
		}

		if features.HasHoles && features.HasPartials && features.HasContextWrites {
			return features
		}
		i += 1 + read
	}
	return features
}

func stringConstantEquals(constants []object.Object, index int, want string) bool {
	value, ok := stringConstantValue(constants, index)
	return ok && value == want
}

func (f *bytecodeFeatures) merge(other bytecodeFeatures) {
	f.HasHoles = f.HasHoles || other.HasHoles
	f.HasPartials = f.HasPartials || other.HasPartials
	f.HasContextWrites = f.HasContextWrites || other.HasContextWrites
}

func appendFastStatements(plan *FastRenderPlan, segments *[]FastRenderSegment, statements []ast.Statement) bool {
	for _, stmt := range statements {
		if !appendFastStatement(plan, segments, stmt) {
			return false
		}
	}
	return true
}

func appendFastOutputBlockStatements(plan *FastRenderPlan, segments *[]FastRenderSegment, statements []ast.Statement) bool {
	for _, stmt := range statements {
		if !appendFastOutputBlockStatement(plan, segments, stmt) {
			return false
		}
	}
	return true
}

func appendFastOutputBlockStatement(plan *FastRenderPlan, segments *[]FastRenderSegment, stmt ast.Statement) bool {
	if ret, ok := stmt.(*ast.ReturnStatement); ok {
		return appendFastOutputExpression(plan, segments, ret.ReturnValue, lineForNode(ret))
	}
	return appendFastStatement(plan, segments, stmt)
}

func appendFastSilentStatements(plan *FastRenderPlan, segments *[]FastRenderSegment, statements []ast.Statement) bool {
	for _, stmt := range statements {
		if !appendFastSilentStatement(plan, segments, stmt) {
			return false
		}
	}
	return true
}

func appendFastSilentStatement(plan *FastRenderPlan, segments *[]FastRenderSegment, stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.ExpressionStatement:
		if isFastCommentStatement(stmt) {
			return true
		}
		if html, ok := stmt.Expression.(*ast.HTMLLiteral); ok {
			appendFastStatic(plan, segments, html.Value)
			return true
		}
		if assign, ok := stmt.Expression.(*ast.AssignExpression); ok {
			return appendFastAssignExpression(plan, segments, assign, lineForNode(stmt))
		}
		if index, ok := stmt.Expression.(*ast.IndexExpression); ok && index.Value != nil {
			return appendFastIndexAssignExpression(plan, segments, index, lineForNode(stmt))
		}
		if forExpression, ok := stmt.Expression.(*ast.ForExpression); ok {
			loop, ok := fastSilentLoopPlanFromExpression(plan, forExpression, lineForNode(stmt))
			if !ok {
				return false
			}
			*segments = append(*segments, FastRenderSegment{
				Kind: FastRenderSegmentLoop,
				Loop: loop,
				Line: lineForNode(stmt),
			})
			plan.NameCount++
			return true
		}
		if callExpression, ok := stmt.Expression.(*ast.CallExpression); ok {
			if callExpression.Block != nil {
				call, ok := fastSilentBlockCallPlanFromExpression(plan, callExpression, lineForNode(stmt))
				if !ok {
					return false
				}
				*segments = append(*segments, FastRenderSegment{
					Kind:      FastRenderSegmentBlockCall,
					Value:     call.Name,
					BlockCall: call,
					Line:      lineForNode(stmt),
				})
				plan.NameCount++
				return true
			}
			call, ok := fastSilentCallPlanFromExpression(plan, callExpression, lineForNode(stmt))
			if !ok {
				return false
			}
			*segments = append(*segments, FastRenderSegment{
				Kind:  FastRenderSegmentCall,
				Value: call.Name,
				Call:  call,
				Line:  lineForNode(stmt),
			})
			plan.NameCount++
			return true
		}
		if ifExpression, ok := stmt.Expression.(*ast.IfExpression); ok {
			conditional, ok := fastSilentConditionalPlanFromExpression(plan, ifExpression, lineForNode(stmt))
			if !ok {
				return false
			}
			*segments = append(*segments, FastRenderSegment{
				Kind:        FastRenderSegmentConditional,
				Conditional: conditional,
				Line:        lineForNode(stmt),
			})
			plan.NameCount++
			return true
		}
		return false
	case *ast.ReturnStatement:
		return appendFastOutputExpression(plan, segments, stmt.ReturnValue, lineForNode(stmt))
	case *ast.LetStatement:
		return appendFastLetStatement(plan, segments, stmt)
	default:
		return false
	}
}

func appendFastStatement(plan *FastRenderPlan, segments *[]FastRenderSegment, stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.ExpressionStatement:
		if isFastCommentStatement(stmt) {
			return true
		}
		if html, ok := stmt.Expression.(*ast.HTMLLiteral); ok {
			appendFastStatic(plan, segments, html.Value)
			return true
		}
		if assign, ok := stmt.Expression.(*ast.AssignExpression); ok {
			return appendFastAssignExpression(plan, segments, assign, lineForNode(stmt))
		}
		if index, ok := stmt.Expression.(*ast.IndexExpression); ok && index.Value != nil {
			return appendFastIndexAssignExpression(plan, segments, index, lineForNode(stmt))
		}
		if forExpression, ok := stmt.Expression.(*ast.ForExpression); ok {
			loop, ok := fastSilentLoopPlanFromExpression(plan, forExpression, lineForNode(stmt))
			if !ok {
				return false
			}
			*segments = append(*segments, FastRenderSegment{
				Kind: FastRenderSegmentLoop,
				Loop: loop,
				Line: lineForNode(stmt),
			})
			plan.NameCount++
			return true
		}
		if callExpression, ok := stmt.Expression.(*ast.CallExpression); ok {
			if callExpression.Block != nil {
				call, ok := fastSilentBlockCallPlanFromExpression(plan, callExpression, lineForNode(stmt))
				if !ok {
					return false
				}
				*segments = append(*segments, FastRenderSegment{
					Kind:      FastRenderSegmentBlockCall,
					Value:     call.Name,
					BlockCall: call,
					Line:      lineForNode(stmt),
				})
				plan.NameCount++
				return true
			}
			call, ok := fastSilentCallPlanFromExpression(plan, callExpression, lineForNode(stmt))
			if !ok {
				return false
			}
			*segments = append(*segments, FastRenderSegment{
				Kind:  FastRenderSegmentCall,
				Value: call.Name,
				Call:  call,
				Line:  lineForNode(stmt),
			})
			plan.NameCount++
			return true
		}
		if ifExpression, ok := stmt.Expression.(*ast.IfExpression); ok {
			conditional, ok := fastSilentConditionalPlanFromExpression(plan, ifExpression, lineForNode(stmt))
			if !ok {
				return false
			}
			*segments = append(*segments, FastRenderSegment{
				Kind:        FastRenderSegmentConditional,
				Conditional: conditional,
				Line:        lineForNode(stmt),
			})
			plan.NameCount++
			return true
		}
		return false
	case *ast.ReturnStatement:
		if stmt.Type == token.E_START {
			return appendFastOutputExpression(plan, segments, stmt.ReturnValue, lineForNode(stmt))
		}
		return appendFastReturnStatement(plan, segments, stmt)
	case *ast.LetStatement:
		return appendFastLetStatement(plan, segments, stmt)
	default:
		return false
	}
}

func appendFastReturnStatement(plan *FastRenderPlan, segments *[]FastRenderSegment, stmt *ast.ReturnStatement) bool {
	if stmt == nil {
		return false
	}
	value, ok := fastValuePlanFromExpression(plan, stmt.ReturnValue, false, lineForNode(stmt.ReturnValue))
	if !ok {
		return false
	}
	*segments = append(*segments, FastRenderSegment{
		Kind:      FastRenderSegmentReturn,
		ValuePlan: value,
		Line:      lineForNode(stmt),
	})
	plan.NameCount++
	return true
}

func appendFastLetStatement(plan *FastRenderPlan, segments *[]FastRenderSegment, stmt *ast.LetStatement) bool {
	if stmt == nil || stmt.Name == nil || stmt.Name.Callee != nil || stmt.Name.Value == "" || stmt.Value == nil {
		return false
	}
	value, ok := fastValuePlanFromExpression(plan, stmt.Value, false, lineForNode(stmt.Value))
	if !ok {
		return false
	}
	*segments = append(*segments, FastRenderSegment{
		Kind:      FastRenderSegmentLet,
		Value:     stmt.Name.Value,
		NameIndex: plan.bindName(stmt.Name.Value),
		ValuePlan: value,
		Line:      lineForNode(stmt),
	})
	plan.NameCount++
	return true
}

func appendFastAssignExpression(plan *FastRenderPlan, segments *[]FastRenderSegment, expr *ast.AssignExpression, line int) bool {
	if expr == nil || expr.Name == nil || expr.Name.Callee != nil || expr.Name.Value == "" || expr.Value == nil {
		return false
	}
	value, ok := fastValuePlanFromExpression(plan, expr.Value, false, lineForNode(expr.Value))
	if !ok || !fastAssignValueSupported(value) {
		return false
	}
	*segments = append(*segments, FastRenderSegment{
		Kind:      FastRenderSegmentAssign,
		Value:     expr.Name.Value,
		NameIndex: plan.bindName(expr.Name.Value),
		ValuePlan: value,
		AssignTarget: &FastAssignTarget{
			Kind:      FastAssignTargetName,
			Name:      expr.Name.Value,
			NameIndex: plan.bindName(expr.Name.Value),
			Line:      line,
		},
		Line: line,
	})
	plan.NameCount++
	return true
}

func appendFastIndexAssignExpression(plan *FastRenderPlan, segments *[]FastRenderSegment, expr *ast.IndexExpression, line int) bool {
	target, ok := fastAssignIndexTargetFromExpression(plan, nil, expr, line, false)
	if !ok {
		return false
	}
	value, ok := fastValuePlanFromExpression(plan, expr.Value, false, lineForNode(expr.Value))
	if !ok || !fastAssignValueSupported(value) {
		return false
	}
	*segments = append(*segments, FastRenderSegment{
		Kind:         FastRenderSegmentAssign,
		ValuePlan:    value,
		AssignTarget: &target,
		Line:         line,
	})
	plan.NameCount++
	return true
}

func fastAssignValueSupported(value FastValuePlan) bool {
	switch value.Kind {
	case FastValueName, FastValueString, FastValueInteger, FastValueFloat, FastValueBool, FastValuePath, FastValueCall, FastValuePrefix, FastValueInfix, FastValueConcat, FastValueArray, FastValueHash, FastValueIndex:
		return true
	default:
		return false
	}
}

func fastAssignIndexTargetFromExpression(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.IndexExpression, line int, inLoop bool) (FastAssignTarget, bool) {
	if expr == nil || expr.Left == nil || expr.Index == nil || expr.Value == nil || expr.Callee != nil {
		return FastAssignTarget{}, false
	}
	container, ok := fastValuePlanForAssignOperand(plan, loop, expr.Left, false, lineForNode(expr.Left), inLoop)
	if !ok {
		return FastAssignTarget{}, false
	}
	index, ok := fastValuePlanForAssignOperand(plan, loop, expr.Index, false, lineForNode(expr.Index), inLoop)
	if !ok || !fastIndexOperandSupported(index) {
		return FastAssignTarget{}, false
	}
	return FastAssignTarget{
		Kind:      FastAssignTargetIndex,
		Container: container,
		Index:     index,
		Line:      line,
	}, true
}

func fastValuePlanForAssignOperand(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, nullOnMissing bool, line int, inLoop bool) (FastValuePlan, bool) {
	if inLoop {
		return fastValuePlanFromLoopOperand(plan, loop, expr, nullOnMissing, line)
	}
	return fastValuePlanFromExpression(plan, expr, nullOnMissing, line)
}

func fastIndexOperandSupported(value FastValuePlan) bool {
	switch value.Kind {
	case FastValueName, FastValueString, FastValueInteger, FastValueFloat, FastValueBool, FastValuePath, FastValueCall, FastValuePrefix, FastValueInfix, FastValueConcat, FastValueIndex, FastValueLoopKey:
		return true
	default:
		return false
	}
}

func fastIndexContainerSupported(value FastValuePlan) bool {
	switch value.Kind {
	case FastValueName, FastValuePath, FastValueCall, FastValueArray, FastValueHash, FastValueIndex:
		return true
	default:
		return false
	}
}

func fastRenderIndexAssignReject(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.IndexExpression, line int, inLoop bool) FastRenderReject {
	if expr == nil {
		return FastRenderReject{Line: line, Reason: "nil index assignment is not fast-planned"}
	}
	if expr.Callee != nil {
		return rejectFastRender(expr, "assignment target: chained index assignments are not fast-planned")
	}
	if _, ok := fastValuePlanForAssignOperand(plan, loop, expr.Left, false, lineForNode(expr.Left), inLoop); !ok {
		return fastRenderValueReject(plan, expr.Left, "assignment target")
	}
	index, ok := fastValuePlanForAssignOperand(plan, loop, expr.Index, false, lineForNode(expr.Index), inLoop)
	if !ok || !fastIndexOperandSupported(index) {
		return fastRenderValueReject(plan, expr.Index, "assignment index")
	}
	value, ok := fastValuePlanForAssignOperand(plan, loop, expr.Value, false, lineForNode(expr.Value), inLoop)
	if !ok || !fastAssignValueSupported(value) {
		return fastRenderValueReject(plan, expr.Value, "assignment value")
	}
	return rejectFastRender(expr, "unsupported index assignment target for fast render")
}

func appendFastOutputExpression(plan *FastRenderPlan, segments *[]FastRenderSegment, expr ast.Expression, line int) bool {
	switch expr := expr.(type) {
	case *ast.StringLiteral:
		appendFastStatic(plan, segments, template.HTMLEscapeString(expr.Value))
		return true
	case *ast.HTMLLiteral:
		appendFastStatic(plan, segments, expr.Value)
		return true
	case *ast.IntegerLiteral:
		appendFastStatic(plan, segments, fmt.Sprint(expr.Value))
		return true
	case *ast.FloatLiteral:
		appendFastStatic(plan, segments, fmt.Sprint(expr.Value))
		return true
	case *ast.Boolean:
		appendFastStatic(plan, segments, fmt.Sprint(expr.Value))
		return true
	case *ast.ForExpression:
		loop, ok := fastLoopPlanFromExpression(plan, expr, line)
		if !ok {
			return false
		}
		*segments = append(*segments, FastRenderSegment{
			Kind: FastRenderSegmentLoop,
			Loop: loop,
			Line: line,
		})
		plan.NameCount++
		return true
	case *ast.IfExpression:
		conditional, ok := fastConditionalPlanFromExpression(plan, expr, line)
		if !ok {
			return false
		}
		*segments = append(*segments, FastRenderSegment{
			Kind:        FastRenderSegmentConditional,
			Conditional: conditional,
			Line:        line,
		})
		plan.NameCount++
		return true
	case *ast.CallExpression:
		if blockCall, ok := fastBlockCallPlanFromExpression(plan, expr, line); ok {
			*segments = append(*segments, FastRenderSegment{
				Kind:      FastRenderSegmentBlockCall,
				BlockCall: blockCall,
				Line:      line,
				Value:     blockCall.Name,
			})
			plan.NameCount++
			return true
		}
		if partial, ok := fastPartialPlanFromCall(plan, expr, line); ok {
			*segments = append(*segments, FastRenderSegment{
				Kind:    FastRenderSegmentPartial,
				Partial: partial,
				Line:    line,
			})
			plan.NameCount++
			return true
		}
		if call, ok := fastCallPlanFromExpression(plan, expr, line, false); ok {
			*segments = append(*segments, FastRenderSegment{
				Kind:  FastRenderSegmentCall,
				Call:  call,
				Line:  line,
				Value: call.Name,
			})
			plan.NameCount++
			return true
		}
	}

	value, ok := fastValuePlanFromExpression(plan, expr, false, line)
	if !ok {
		return false
	}
	switch value.Kind {
	case FastValueName:
		if value.Value != "nil" {
			*segments = append(*segments, FastRenderSegment{
				Kind:          FastRenderSegmentName,
				Value:         value.Value,
				NameIndex:     value.NameIndex,
				NullOnMissing: value.NullOnMissing,
				Line:          line,
			})
			plan.NameCount++
		}
	case FastValuePath:
		if len(value.Path) == 1 && value.Path[0].Kind == FastPathStepProperty {
			step := value.Path[0]
			*segments = append(*segments, FastRenderSegment{
				Kind:          FastRenderSegmentProperty,
				Value:         value.Value,
				NameIndex:     value.NameIndex,
				Property:      step.Value,
				Receiver:      step.Receiver,
				Full:          step.Full,
				Line:          line,
				PropertyCache: object.InlineCacheSlot{},
			})
		} else {
			*segments = append(*segments, FastRenderSegment{
				Kind:      FastRenderSegmentValue,
				ValuePlan: value,
				Line:      line,
			})
		}
		plan.NameCount++
	default:
		*segments = append(*segments, FastRenderSegment{
			Kind:      FastRenderSegmentValue,
			ValuePlan: value,
			Line:      line,
		})
		plan.NameCount++
	}
	return true
}

func appendFastStatic(plan *FastRenderPlan, segments *[]FastRenderSegment, value string) {
	if value == "" {
		return
	}
	last := len(*segments) - 1
	if last >= 0 && (*segments)[last].Kind == FastRenderSegmentStatic {
		(*segments)[last].Value += value
	} else {
		*segments = append(*segments, FastRenderSegment{
			Kind:  FastRenderSegmentStatic,
			Value: value,
		})
	}
	plan.StaticSize += len(value)
}

func fastValuePlanFromExpression(plan *FastRenderPlan, expr ast.Expression, nullOnMissing bool, line int) (FastValuePlan, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return fastValuePlanFromIdentifier(plan, expr, nullOnMissing, line)
	case *ast.IndexExpression:
		return fastValuePlanFromIndexExpression(plan, expr, nullOnMissing, line)
	case *ast.CallExpression:
		return fastValuePlanFromCallExpression(plan, expr, nullOnMissing, line)
	case *ast.PrefixExpression:
		return fastValuePlanFromPrefixExpression(plan, expr, line)
	case *ast.InfixExpression:
		return fastValuePlanFromInfixExpression(plan, expr, line)
	case *ast.ArrayLiteral:
		return fastValuePlanFromArrayLiteral(plan, expr, line)
	case *ast.HashLiteral:
		return fastValuePlanFromHashLiteral(plan, expr, line)
	case *ast.StringLiteral:
		return FastValuePlan{Kind: FastValueString, Value: expr.Value, Line: line}, true
	case *ast.IntegerLiteral:
		return FastValuePlan{Kind: FastValueInteger, IntValue: int64(expr.Value), Line: line}, true
	case *ast.FloatLiteral:
		return FastValuePlan{Kind: FastValueFloat, FloatValue: expr.Value, Line: line}, true
	case *ast.Boolean:
		return FastValuePlan{Kind: FastValueBool, BoolValue: expr.Value, Line: line}, true
	default:
		return FastValuePlan{}, false
	}
}

func fastValuePlanFromArrayLiteral(plan *FastRenderPlan, expr *ast.ArrayLiteral, line int) (FastValuePlan, bool) {
	if expr == nil {
		return FastValuePlan{}, false
	}
	elements := make([]FastValuePlan, 0, len(expr.Elements))
	for _, elementExpr := range expr.Elements {
		value, ok := fastValuePlanFromExpression(plan, elementExpr, false, lineForNode(elementExpr))
		if !ok {
			return FastValuePlan{}, false
		}
		elements = append(elements, value)
	}
	return FastValuePlan{
		Kind:     FastValueArray,
		Elements: elements,
		Line:     line,
	}, true
}

func fastValuePlanFromHashLiteral(plan *FastRenderPlan, expr *ast.HashLiteral, line int) (FastValuePlan, bool) {
	if expr == nil {
		return FastValuePlan{}, false
	}
	keys := append([]ast.Expression(nil), expr.Order...)
	if len(keys) == 0 {
		for key := range expr.Pairs {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})
	}
	pairs := make([]FastValuePair, 0, len(keys))
	for _, keyExpr := range keys {
		key, keyPlan, ok := fastHashLiteralKeyPlan(plan, keyExpr, lineForNode(keyExpr))
		if !ok {
			return FastValuePlan{}, false
		}
		valueExpr := expr.Pairs[keyExpr]
		value, ok := fastValuePlanFromExpression(plan, valueExpr, false, lineForNode(valueExpr))
		if !ok {
			return FastValuePlan{}, false
		}
		pairs = append(pairs, FastValuePair{
			Key:     key,
			KeyPlan: keyPlan,
			Value:   value,
			Line:    lineForNode(valueExpr),
		})
	}
	return FastValuePlan{
		Kind:  FastValueHash,
		Pairs: pairs,
		Line:  line,
	}, true
}

func fastHashLiteralKeyPlan(plan *FastRenderPlan, expr ast.Expression, line int) (string, *FastValuePlan, bool) {
	if key, ok := fastPartialDataKey(expr); ok {
		return key, nil, true
	}
	value, ok := fastValuePlanFromExpression(plan, expr, false, line)
	if !ok || !fastHashLiteralKeySupported(value) {
		return "", nil, false
	}
	return "", &value, true
}

func fastHashLiteralKeySupported(value FastValuePlan) bool {
	switch value.Kind {
	case FastValueString, FastValueInteger, FastValueBool:
		return true
	default:
		return false
	}
}

func fastValuePlanFromInfixExpression(plan *FastRenderPlan, expr *ast.InfixExpression, line int) (FastValuePlan, bool) {
	if expr == nil || !fastInfixOperator(expr.Operator) {
		if expr != nil && expr.Operator == "+" {
			return fastValuePlanFromConcatExpression(plan, expr, line)
		}
		return FastValuePlan{}, false
	}
	left, ok := fastValuePlanFromExpression(plan, expr.Left, true, lineForNode(expr.Left))
	if !ok {
		return FastValuePlan{}, false
	}
	right, ok := fastValuePlanFromExpression(plan, expr.Right, true, lineForNode(expr.Right))
	if !ok {
		return FastValuePlan{}, false
	}
	if !fastInfixOperandSupported(left) || !fastInfixOperandSupported(right) {
		return FastValuePlan{}, false
	}
	return FastValuePlan{
		Kind:       FastValueInfix,
		Operator:   expr.Operator,
		Left:       &left,
		Right:      &right,
		RegexCache: fastRegexCacheSlot(expr.Operator),
		Line:       line,
	}, true
}

func fastValuePlanFromPrefixExpression(plan *FastRenderPlan, expr *ast.PrefixExpression, line int) (FastValuePlan, bool) {
	if expr == nil || expr.Right == nil {
		return FastValuePlan{}, false
	}
	switch expr.Operator {
	case "!", "-":
	default:
		return FastValuePlan{}, false
	}
	right, ok := fastValuePlanFromExpression(plan, expr.Right, true, lineForNode(expr.Right))
	if !ok {
		return FastValuePlan{}, false
	}
	return FastValuePlan{
		Kind:     FastValuePrefix,
		Operator: expr.Operator,
		Right:    &right,
		Line:     line,
	}, true
}

func fastValuePlanFromConcatExpression(plan *FastRenderPlan, expr *ast.InfixExpression, line int) (FastValuePlan, bool) {
	if expr == nil || expr.Operator != "+" {
		return FastValuePlan{}, false
	}
	left, ok := fastValuePlanFromExpression(plan, expr.Left, false, lineForNode(expr.Left))
	if !ok {
		return FastValuePlan{}, false
	}
	right, ok := fastValuePlanFromExpression(plan, expr.Right, false, lineForNode(expr.Right))
	if !ok {
		return FastValuePlan{}, false
	}
	return FastValuePlan{
		Kind:     FastValueConcat,
		Operator: expr.Operator,
		Left:     &left,
		Right:    &right,
		Line:     line,
	}, true
}

func fastValuePlanFromIdentifier(plan *FastRenderPlan, ident *ast.Identifier, nullOnMissing bool, line int) (FastValuePlan, bool) {
	parts := identifierParts(ident)
	if len(parts) == 0 {
		return FastValuePlan{}, false
	}
	if parts[0] == "nil" {
		return FastValuePlan{
			Kind:          FastValueName,
			Value:         "nil",
			NameIndex:     -1,
			NullOnMissing: true,
			Line:          line,
		}, true
	}
	value := FastValuePlan{
		Kind:          FastValueName,
		Value:         parts[0],
		NameIndex:     plan.bindName(parts[0]),
		NullOnMissing: nullOnMissing,
		Line:          line,
	}
	if len(parts) == 1 {
		return value, true
	}
	value.Kind = FastValuePath
	for i, property := range parts[1:] {
		value.Path = append(value.Path, fastPropertyStep(property, strings.Join(parts[:i+1], "."), strings.Join(parts[:i+2], "."), line, false))
	}
	return value, true
}

func fastValuePlanFromIndexExpression(plan *FastRenderPlan, exp *ast.IndexExpression, nullOnMissing bool, line int) (FastValuePlan, bool) {
	value, ok := fastValuePlanFromExpression(plan, exp.Left, nullOnMissing, line)
	if !ok || !value.canUsePath() {
		return fastValuePlanFromDynamicIndexExpression(plan, exp, nullOnMissing, line)
	}
	indexStep, ok := fastIndexStepFromExpression(exp.Index, line)
	if !ok {
		return fastValuePlanFromDynamicIndexExpression(plan, exp, nullOnMissing, line)
	}
	appendFastValuePathStep(&value, indexStep)
	if exp.Callee != nil {
		if !appendFastReceiverCalleeWithArgumentPlanner(&value, exp.Callee, lastChainPart(exp.Left), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
			return fastValuePlanFromExpression(plan, arg, nullOnMissing, argLine)
		}) {
			return FastValuePlan{}, false
		}
	}
	return value, true
}

func fastValuePlanFromDynamicIndexExpression(plan *FastRenderPlan, exp *ast.IndexExpression, nullOnMissing bool, line int) (FastValuePlan, bool) {
	if exp == nil || exp.Left == nil || exp.Index == nil {
		return FastValuePlan{}, false
	}
	left, ok := fastValuePlanFromExpression(plan, exp.Left, nullOnMissing, lineForNode(exp.Left))
	if !ok || !fastIndexContainerSupported(left) {
		return FastValuePlan{}, false
	}
	index, ok := fastValuePlanFromExpression(plan, exp.Index, false, lineForNode(exp.Index))
	if !ok || !fastIndexOperandSupported(index) {
		return FastValuePlan{}, false
	}
	value := FastValuePlan{
		Kind:          FastValueIndex,
		Left:          &left,
		Right:         &index,
		NullOnMissing: nullOnMissing,
		Line:          line,
	}
	if exp.Callee != nil {
		if !appendFastReceiverCalleeWithArgumentPlanner(&value, exp.Callee, lastChainPart(exp.Left), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
			return fastValuePlanFromExpression(plan, arg, nullOnMissing, argLine)
		}) {
			return FastValuePlan{}, false
		}
	}
	return value, true
}

func fastValuePlanFromCallExpression(plan *FastRenderPlan, exp *ast.CallExpression, nullOnMissing bool, line int) (FastValuePlan, bool) {
	if exp.Block != nil {
		return FastValuePlan{}, false
	}
	if exp.ChainCallee != nil {
		root := *exp
		root.ChainCallee = nil
		if call, ok := fastCallPlanFromExpression(plan, &root, line, nullOnMissing); ok {
			value := FastValuePlan{
				Kind: FastValueCall,
				Call: call,
				Line: line,
			}
			if !appendFastReceiverCalleeWithArgumentPlanner(&value, exp.ChainCallee, lastChainPart(exp.Function), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
				return fastValuePlanFromExpression(plan, arg, nullOnMissing, argLine)
			}) {
				return FastValuePlan{}, false
			}
			return value, true
		}
	}
	if ident, ok := exp.Function.(*ast.Identifier); ok {
		parts := identifierParts(ident)
		if len(parts) > 1 && len(exp.Arguments) > 0 {
			return fastValuePlanFromReceiverCallExpression(plan, exp, parts, nullOnMissing, line)
		}
		if len(parts) > 1 && len(exp.Arguments) == 0 {
			value := FastValuePlan{
				Kind:          FastValuePath,
				Value:         parts[0],
				NameIndex:     plan.bindName(parts[0]),
				NullOnMissing: nullOnMissing,
				Line:          line,
			}
			for i, property := range parts[1:] {
				value.Path = append(value.Path, fastPropertyStep(property, strings.Join(parts[:i+1], "."), strings.Join(parts[:i+2], "."), line, i == len(parts[1:])-1))
			}
			value.Path = append(value.Path, FastPathStep{
				Kind:  FastPathStepCall,
				Value: callExpressionName(exp),
				Line:  line,
			})
			if exp.ChainCallee != nil && !appendFastReceiverCalleeWithArgumentPlanner(&value, exp.ChainCallee, lastChainPart(exp.Function), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
				return fastValuePlanFromExpression(plan, arg, nullOnMissing, argLine)
			}) {
				return FastValuePlan{}, false
			}
			return value, true
		}
	}
	if call, ok := fastCallPlanFromExpression(plan, exp, line, nullOnMissing); ok {
		return FastValuePlan{
			Kind: FastValueCall,
			Call: call,
			Line: line,
		}, true
	}
	if len(exp.Arguments) == 0 {
		value, ok := fastValuePlanFromExpression(plan, exp.Function, nullOnMissing, line)
		if !ok || !value.canUsePath() {
			return FastValuePlan{}, false
		}
		appendFastValuePathStep(&value, FastPathStep{
			Kind:  FastPathStepCall,
			Value: callExpressionName(exp),
			Line:  line,
		})
		if exp.ChainCallee != nil && !appendFastReceiverCalleeWithArgumentPlanner(&value, exp.ChainCallee, lastChainPart(exp.Function), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
			return fastValuePlanFromExpression(plan, arg, nullOnMissing, argLine)
		}) {
			return FastValuePlan{}, false
		}
		return value, true
	}
	value, ok := fastValuePlanFromExpression(plan, exp.Function, nullOnMissing, line)
	if !ok || !value.canUsePath() {
		return FastValuePlan{}, false
	}
	callStep := FastPathStep{
		Kind:  FastPathStepCall,
		Value: callExpressionName(exp),
		Line:  line,
	}
	for _, arg := range exp.Arguments {
		argPlan, ok := fastValuePlanFromExpression(plan, arg, nullOnMissing, lineForNode(arg))
		if !ok {
			return FastValuePlan{}, false
		}
		callStep.Args = append(callStep.Args, argPlan)
	}
	appendFastValuePathStep(&value, callStep)
	if exp.ChainCallee != nil && !appendFastReceiverCalleeWithArgumentPlanner(&value, exp.ChainCallee, lastChainPart(exp.Function), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
		return fastValuePlanFromExpression(plan, arg, nullOnMissing, argLine)
	}) {
		return FastValuePlan{}, false
	}
	return value, true
}

func appendFastReceiverCalleeWithArgumentPlanner(value *FastValuePlan, exp ast.Expression, base string, line int, argPlan func(ast.Expression, int) (FastValuePlan, bool)) bool {
	switch exp := exp.(type) {
	case *ast.Identifier:
		receiver := base
		for _, property := range trimReceiverParts(identifierParts(exp), base) {
			full := property
			if receiver != "" {
				full = receiver + "." + property
			}
			value.Path = append(value.Path, fastPropertyStep(property, receiver, full, line, false))
			receiver = full
		}
		return true
	case *ast.IndexExpression:
		if !appendFastReceiverCalleeWithArgumentPlanner(value, exp.Left, base, line, argPlan) {
			return false
		}
		indexStep, ok := fastIndexStepFromExpression(exp.Index, line)
		if !ok {
			return false
		}
		value.Path = append(value.Path, indexStep)
		if exp.Callee != nil {
			return appendFastReceiverCalleeWithArgumentPlanner(value, exp.Callee, lastChainPart(exp.Left), line, argPlan)
		}
		return true
	case *ast.CallExpression:
		if exp.Block != nil {
			return false
		}
		if ident, ok := exp.Function.(*ast.Identifier); ok {
			receiver := base
			parts := trimReceiverParts(identifierParts(ident), base)
			for i, property := range parts {
				full := property
				if receiver != "" {
					full = receiver + "." + property
				}
				value.Path = append(value.Path, fastPropertyStep(property, receiver, full, line, i == len(parts)-1))
				receiver = full
			}
		} else if !appendFastReceiverCalleeWithArgumentPlanner(value, exp.Function, base, line, argPlan) {
			return false
		}
		callStep := FastPathStep{
			Kind:  FastPathStepCall,
			Value: callExpressionName(exp),
			Line:  line,
		}
		for _, arg := range exp.Arguments {
			if argPlan == nil {
				return false
			}
			planned, ok := argPlan(arg, lineForNode(arg))
			if !ok {
				return false
			}
			callStep.Args = append(callStep.Args, planned)
		}
		value.Path = append(value.Path, callStep)
		if exp.ChainCallee != nil {
			return appendFastReceiverCalleeWithArgumentPlanner(value, exp.ChainCallee, lastChainPart(exp.Function), line, argPlan)
		}
		return true
	default:
		return false
	}
}

func appendFastReceiverCallee(value *FastValuePlan, exp ast.Expression, base string, line int) bool {
	return appendFastReceiverCalleeWithArgumentPlanner(value, exp, base, line, nil)
}

func fastValuePlanFromReceiverCallExpression(plan *FastRenderPlan, exp *ast.CallExpression, parts []string, nullOnMissing bool, line int) (FastValuePlan, bool) {
	if exp == nil || exp.Block != nil || len(parts) < 2 || parts[0] == "" {
		return FastValuePlan{}, false
	}
	value := FastValuePlan{
		Kind:          FastValuePath,
		Value:         parts[0],
		NameIndex:     plan.bindName(parts[0]),
		NullOnMissing: nullOnMissing,
		Line:          line,
	}
	receiver := parts[0]
	for i, property := range parts[1:] {
		full := receiver + "." + property
		value.Path = append(value.Path, fastPropertyStep(property, receiver, full, line, i == len(parts[1:])-1))
		receiver = full
	}
	callStep := FastPathStep{
		Kind:  FastPathStepCall,
		Value: callExpressionName(exp),
		Line:  line,
	}
	for _, arg := range exp.Arguments {
		argPlan, ok := fastValuePlanFromExpression(plan, arg, nullOnMissing, lineForNode(arg))
		if !ok {
			return FastValuePlan{}, false
		}
		callStep.Args = append(callStep.Args, argPlan)
	}
	value.Path = append(value.Path, callStep)
	if exp.ChainCallee != nil && !appendFastReceiverCalleeWithArgumentPlanner(&value, exp.ChainCallee, lastChainPart(exp.Function), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
		return fastValuePlanFromExpression(plan, arg, nullOnMissing, argLine)
	}) {
		return FastValuePlan{}, false
	}
	return value, true
}

func (v FastValuePlan) canUsePath() bool {
	return v.Kind == FastValueName || v.Kind == FastValuePath || v.Kind == FastValueIndex || v.Kind == FastValueCall
}

func appendFastValuePathStep(value *FastValuePlan, step FastPathStep) {
	if value == nil {
		return
	}
	if value.Kind == FastValueName {
		value.Kind = FastValuePath
	}
	value.Path = append(value.Path, step)
}

func fastPropertyStep(property, receiver, full string, line int, method bool) FastPathStep {
	return FastPathStep{
		Kind:     FastPathStepProperty,
		Value:    property,
		Receiver: receiver,
		Full:     full,
		Method:   method,
		Line:     line,
	}
}

func fastIntegerIndex(expr ast.Expression) (int, bool) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		return expr.Value, true
	default:
		return 0, false
	}
}

func fastStringIndex(expr ast.Expression) (string, bool) {
	switch expr := expr.(type) {
	case *ast.StringLiteral:
		return expr.Value, true
	default:
		return "", false
	}
}

func fastIndexStepFromExpression(expr ast.Expression, line int) (FastPathStep, bool) {
	if index, ok := fastIntegerIndex(expr); ok {
		return FastPathStep{Kind: FastPathStepIndexInteger, Index: index, Line: line}, true
	}
	if index, ok := fastStringIndex(expr); ok {
		return FastPathStep{Kind: FastPathStepIndexString, Value: index, Line: line}, true
	}
	return FastPathStep{}, false
}

func fastCallPlanFromExpression(plan *FastRenderPlan, exp *ast.CallExpression, line int, nullOnMissing bool) (*FastCallPlan, bool) {
	if exp == nil || exp.Block != nil || exp.ChainCallee != nil {
		return nil, false
	}
	ident, ok := exp.Function.(*ast.Identifier)
	if !ok || !fastPlainHelperIdentifier(ident) || ident.Value == "nil" {
		return nil, false
	}
	call := &FastCallPlan{
		Name:      ident.Value,
		NameIndex: plan.bindName(ident.Value),
		Line:      line,
	}
	for _, arg := range exp.Arguments {
		value, ok := fastValuePlanFromExpression(plan, arg, nullOnMissing, line)
		if !ok {
			return nil, false
		}
		call.Args = append(call.Args, value)
	}
	return call, true
}

func fastSilentCallPlanFromExpression(plan *FastRenderPlan, exp *ast.CallExpression, line int) (*FastCallPlan, bool) {
	call, ok := fastCallPlanFromExpression(plan, exp, line, false)
	if !ok {
		return nil, false
	}
	call.Silent = true
	return call, true
}

func fastBlockCallPlanFromExpression(plan *FastRenderPlan, exp *ast.CallExpression, line int) (*FastBlockCallPlan, bool) {
	if exp == nil || exp.Block == nil || exp.ChainCallee != nil {
		return nil, false
	}
	ident, ok := exp.Function.(*ast.Identifier)
	if !ok || !fastPlainHelperIdentifier(ident) || ident.Value == "nil" {
		return nil, false
	}
	if !fastBlockCanRenderFromSource(exp.Block) {
		return nil, false
	}
	call := &FastBlockCallPlan{
		Name:        ident.Value,
		NameIndex:   plan.bindName(ident.Value),
		Block:       exp.Block,
		BlockSource: fastBlockSource(exp.Block),
		Line:        line,
	}
	for _, arg := range exp.Arguments {
		value, ok := fastValuePlanFromExpression(plan, arg, false, lineForNode(arg))
		if !ok {
			return nil, false
		}
		call.Args = append(call.Args, value)
	}
	return call, true
}

func fastSilentBlockCallPlanFromExpression(plan *FastRenderPlan, exp *ast.CallExpression, line int) (*FastBlockCallPlan, bool) {
	call, ok := fastBlockCallPlanFromExpression(plan, exp, line)
	if !ok {
		return nil, false
	}
	call.Silent = true
	return call, true
}

func fastBlockCanRenderFromSource(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}
	return true
}

func fastBlockStatementsAllowScopedAssignments(statements []ast.Statement, locals map[string]struct{}) (map[string]struct{}, bool) {
	scoped := cloneFastBlockLocals(locals)
	for _, stmt := range statements {
		var ok bool
		scoped, ok = fastBlockStatementAllowScopedAssignments(stmt, scoped)
		if !ok {
			return nil, false
		}
	}
	return scoped, true
}

func fastBlockStatementAllowScopedAssignments(stmt ast.Statement, locals map[string]struct{}) (map[string]struct{}, bool) {
	switch stmt := stmt.(type) {
	case nil:
		return locals, true
	case *ast.ExpressionStatement:
		return locals, fastBlockExpressionAssignmentsAreScoped(stmt.Expression, locals)
	case *ast.ReturnStatement:
		return locals, fastBlockExpressionAssignmentsAreScoped(stmt.ReturnValue, locals)
	case *ast.LetStatement:
		if !fastBlockExpressionAssignmentsAreScoped(stmt.Value, locals) {
			return nil, false
		}
		if stmt.Name != nil && stmt.Name.Value != "" && stmt.Name.Callee == nil {
			locals = cloneFastBlockLocals(locals)
			locals[stmt.Name.Value] = struct{}{}
		}
		return locals, true
	case *ast.BlockStatement:
		_, ok := fastBlockStatementsAllowScopedAssignments(stmt.Statements, locals)
		return locals, ok
	default:
		return locals, true
	}
}

func fastBlockExpressionAssignmentsAreScoped(expr ast.Expression, locals map[string]struct{}) bool {
	switch expr := expr.(type) {
	case nil:
		return true
	case *ast.AssignExpression:
		if expr.Name == nil || !fastBlockLocalExists(locals, expr.Name.Value) {
			return false
		}
		return fastBlockExpressionAssignmentsAreScoped(expr.Value, locals)
	case *ast.PrefixExpression:
		return fastBlockExpressionAssignmentsAreScoped(expr.Right, locals)
	case *ast.InfixExpression:
		return fastBlockExpressionAssignmentsAreScoped(expr.Left, locals) && fastBlockExpressionAssignmentsAreScoped(expr.Right, locals)
	case *ast.IndexExpression:
		if expr.Value != nil {
			root, ok := fastBlockAssignmentRoot(expr.Left)
			if !ok || !fastBlockLocalExists(locals, root) {
				return false
			}
			return fastBlockExpressionAssignmentsAreScoped(expr.Left, locals) &&
				fastBlockExpressionAssignmentsAreScoped(expr.Index, locals) &&
				fastBlockExpressionAssignmentsAreScoped(expr.Value, locals) &&
				fastBlockExpressionAssignmentsAreScoped(expr.Callee, locals)
		}
		return fastBlockExpressionAssignmentsAreScoped(expr.Left, locals) &&
			fastBlockExpressionAssignmentsAreScoped(expr.Index, locals) &&
			fastBlockExpressionAssignmentsAreScoped(expr.Callee, locals)
	case *ast.CallExpression:
		if !fastBlockExpressionAssignmentsAreScoped(expr.Function, locals) || !fastBlockExpressionAssignmentsAreScoped(expr.ChainCallee, locals) {
			return false
		}
		for _, arg := range expr.Arguments {
			if !fastBlockExpressionAssignmentsAreScoped(arg, locals) {
				return false
			}
		}
		if expr.Block != nil {
			_, ok := fastBlockStatementsAllowScopedAssignments(expr.Block.Statements, locals)
			return ok
		}
		return true
	case *ast.IfExpression:
		if !fastBlockExpressionAssignmentsAreScoped(expr.Condition, locals) {
			return false
		}
		if expr.Block != nil {
			if _, ok := fastBlockStatementsAllowScopedAssignments(expr.Block.Statements, locals); !ok {
				return false
			}
		}
		for _, elseIf := range expr.ElseIf {
			if elseIf == nil {
				continue
			}
			if !fastBlockExpressionAssignmentsAreScoped(elseIf.Condition, locals) {
				return false
			}
			if elseIf.Block != nil {
				if _, ok := fastBlockStatementsAllowScopedAssignments(elseIf.Block.Statements, locals); !ok {
					return false
				}
			}
		}
		if expr.ElseBlock != nil {
			_, ok := fastBlockStatementsAllowScopedAssignments(expr.ElseBlock.Statements, locals)
			return ok
		}
		return true
	case *ast.ForExpression:
		if !fastBlockExpressionAssignmentsAreScoped(expr.Iterable, locals) {
			return false
		}
		if expr.Block == nil {
			return true
		}
		loopLocals := cloneFastBlockLocals(locals)
		if expr.KeyName != "" && expr.KeyName != "_" {
			loopLocals[expr.KeyName] = struct{}{}
		}
		if expr.ValueName != "" && expr.ValueName != "_" {
			loopLocals[expr.ValueName] = struct{}{}
		}
		_, ok := fastBlockStatementsAllowScopedAssignments(expr.Block.Statements, loopLocals)
		return ok
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if !fastBlockExpressionAssignmentsAreScoped(element, locals) {
				return false
			}
		}
		return true
	case *ast.HashLiteral:
		for key, value := range expr.Pairs {
			if !fastBlockExpressionAssignmentsAreScoped(key, locals) || !fastBlockExpressionAssignmentsAreScoped(value, locals) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func cloneFastBlockLocals(locals map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(locals)+1)
	for name := range locals {
		clone[name] = struct{}{}
	}
	return clone
}

func fastBlockLocalExists(locals map[string]struct{}, name string) bool {
	if name == "" {
		return false
	}
	_, ok := locals[name]
	return ok
}

func fastBlockAssignmentRoot(expr ast.Expression) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		parts := identifierParts(expr)
		if len(parts) == 0 {
			return "", false
		}
		return parts[0], true
	case *ast.IndexExpression:
		return fastBlockAssignmentRoot(expr.Left)
	default:
		return "", false
	}
}

func fastBlockSource(block *ast.BlockStatement) string {
	if block == nil {
		return ""
	}
	var out strings.Builder
	for _, stmt := range block.Statements {
		out.WriteString(fastBlockStatementSource(stmt))
	}
	return out.String()
}

func fastBlockStatementSource(stmt ast.Statement) string {
	switch stmt := stmt.(type) {
	case nil:
		return ""
	case *ast.ExpressionStatement:
		if html, ok := stmt.Expression.(*ast.HTMLLiteral); ok {
			return html.Value
		}
		return "<% " + stmt.String() + " %>"
	case *ast.ReturnStatement:
		if stmt.Type == token.E_START {
			if stmt.ReturnValue == nil {
				return ""
			}
			return "<%= " + stmt.ReturnValue.String() + " %>"
		}
		return "<% " + stmt.String() + " %>"
	case *ast.LetStatement:
		return "<% " + stmt.String() + " %>"
	case *ast.BlockStatement:
		return fastBlockSource(stmt)
	default:
		return "<% " + stmt.String() + " %>"
	}
}

func fastPartialPlanFromCall(plan *FastRenderPlan, exp *ast.CallExpression, line int) (*FastPartialPlan, bool) {
	if exp == nil || exp.Block != nil || exp.ChainCallee != nil || len(exp.Arguments) == 0 || len(exp.Arguments) > 2 {
		return nil, false
	}
	ident, ok := exp.Function.(*ast.Identifier)
	if !ok || ident.Callee != nil || ident.Value != "partial" {
		return nil, false
	}
	name, ok := exp.Arguments[0].(*ast.StringLiteral)
	if !ok {
		return nil, false
	}
	partial := &FastPartialPlan{Name: name.Value, Line: line}
	if len(exp.Arguments) == 2 {
		data, ok := fastPartialDataPlanFromExpression(plan, exp.Arguments[1], line)
		if !ok {
			return nil, false
		}
		partial.Data = data
	}
	return partial, true
}

func fastPartialDataPlanFromExpression(plan *FastRenderPlan, expr ast.Expression, line int) ([]FastPartialDataPair, bool) {
	hash, ok := expr.(*ast.HashLiteral)
	if !ok {
		return nil, false
	}
	keys := append([]ast.Expression(nil), hash.Order...)
	if len(keys) == 0 {
		for key := range hash.Pairs {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})
	}
	data := make([]FastPartialDataPair, 0, len(keys))
	for _, keyExpr := range keys {
		key, ok := fastPartialDataKey(keyExpr)
		if !ok || key == "layout" {
			return nil, false
		}
		valueExpr := hash.Pairs[keyExpr]
		valueLine := lineForNode(valueExpr)
		value, ok := fastValuePlanFromExpression(plan, valueExpr, false, valueLine)
		if !ok {
			return nil, false
		}
		data = append(data, FastPartialDataPair{
			Key:   key,
			Value: value,
			Line:  valueLine,
		})
	}
	return data, true
}

func fastPartialDataKey(expr ast.Expression) (string, bool) {
	switch key := expr.(type) {
	case *ast.Identifier:
		if key.Callee != nil || key.Value == "" {
			return "", false
		}
		return key.Value, true
	case *ast.StringLiteral:
		return key.Value, true
	default:
		return "", false
	}
}

func fastConditionalPlanFromExpression(plan *FastRenderPlan, expr *ast.IfExpression, line int) (*FastConditionalPlan, bool) {
	if expr == nil {
		return nil, false
	}
	conditional := &FastConditionalPlan{Line: line}
	first, ok := fastValuePlanFromExpression(plan, expr.Condition, true, lineForNode(expr.Condition))
	if !ok || !fastConditionValueSupported(first) {
		return nil, false
	}
	firstSegments := []FastRenderSegment{}
	if !appendFastOutputBlockStatements(plan, &firstSegments, expr.Block.Statements) {
		return nil, false
	}
	conditional.Branches = append(conditional.Branches, FastConditionalBranch{
		Condition: first,
		Segments:  firstSegments,
		Line:      line,
	})
	for _, elseIf := range expr.ElseIf {
		if elseIf == nil {
			return nil, false
		}
		condition, ok := fastValuePlanFromExpression(plan, elseIf.Condition, true, lineForNode(elseIf.Condition))
		if !ok || !fastConditionValueSupported(condition) {
			return nil, false
		}
		segments := []FastRenderSegment{}
		if !appendFastOutputBlockStatements(plan, &segments, elseIf.Block.Statements) {
			return nil, false
		}
		conditional.Branches = append(conditional.Branches, FastConditionalBranch{
			Condition: condition,
			Segments:  segments,
			Line:      lineForToken(elseIf.TokenAble),
		})
	}
	if expr.ElseBlock != nil {
		segments := []FastRenderSegment{}
		if !appendFastOutputBlockStatements(plan, &segments, expr.ElseBlock.Statements) {
			return nil, false
		}
		conditional.ElseSegments = segments
	}
	return conditional, true
}

func fastSilentConditionalPlanFromExpression(plan *FastRenderPlan, expr *ast.IfExpression, line int) (*FastConditionalPlan, bool) {
	conditional, ok := fastConditionalPlanFromExpressionWithAppender(plan, expr, line, appendFastSilentStatements)
	if !ok {
		return nil, false
	}
	conditional.Silent = true
	return conditional, true
}

func fastSilentConditionalReject(plan *FastRenderPlan, expr *ast.IfExpression, line int) FastRenderReject {
	if expr == nil || expr.Block == nil {
		return FastRenderReject{Line: line, Reason: "script if expressions without a block are not fast-planned"}
	}
	condition, ok := fastValuePlanFromExpression(plan, expr.Condition, true, lineForNode(expr.Condition))
	if !ok {
		return fastRenderValueReject(plan, expr.Condition, "script if condition")
	}
	if !fastConditionValueSupported(condition) {
		return FastRenderReject{Line: lineForNode(expr.Condition), Reason: "script if condition: unsupported literal container condition"}
	}
	if reject := firstFastSilentStatementReject(plan, expr.Block.Statements); reject.Reason != "" {
		return reject
	}
	for _, elseIf := range expr.ElseIf {
		if elseIf == nil || elseIf.Block == nil {
			return FastRenderReject{Line: line, Reason: "script else-if expressions without a block are not fast-planned"}
		}
		condition, ok := fastValuePlanFromExpression(plan, elseIf.Condition, true, lineForNode(elseIf.Condition))
		if !ok {
			return fastRenderValueReject(plan, elseIf.Condition, "script else-if condition")
		}
		if !fastConditionValueSupported(condition) {
			return FastRenderReject{Line: lineForNode(elseIf.Condition), Reason: "script else-if condition: unsupported literal container condition"}
		}
		if reject := firstFastSilentStatementReject(plan, elseIf.Block.Statements); reject.Reason != "" {
			return reject
		}
	}
	if expr.ElseBlock != nil {
		return firstFastSilentStatementReject(plan, expr.ElseBlock.Statements)
	}
	return FastRenderReject{}
}

func firstFastSilentStatementReject(plan *FastRenderPlan, statements []ast.Statement) FastRenderReject {
	for _, stmt := range statements {
		if reject := fastSilentStatementReject(plan, stmt); reject.Reason != "" {
			return reject
		}
	}
	return FastRenderReject{}
}

func fastSilentStatementReject(plan *FastRenderPlan, stmt ast.Statement) FastRenderReject {
	switch stmt := stmt.(type) {
	case *ast.ExpressionStatement:
		if isFastCommentStatement(stmt) {
			return FastRenderReject{}
		}
		if _, ok := stmt.Expression.(*ast.HTMLLiteral); ok {
			return FastRenderReject{}
		}
		if assign, ok := stmt.Expression.(*ast.AssignExpression); ok {
			segments := []FastRenderSegment{}
			if appendFastAssignExpression(plan, &segments, assign, lineForNode(stmt)) {
				return FastRenderReject{}
			}
			return fastRenderValueReject(plan, assign.Value, "script assignment value")
		}
		if index, ok := stmt.Expression.(*ast.IndexExpression); ok && index.Value != nil {
			segments := []FastRenderSegment{}
			if appendFastIndexAssignExpression(plan, &segments, index, lineForNode(stmt)) {
				return FastRenderReject{}
			}
			return fastRenderIndexAssignReject(plan, nil, index, lineForNode(stmt), false)
		}
		if forExpression, ok := stmt.Expression.(*ast.ForExpression); ok {
			if _, ok := fastSilentLoopPlanFromExpression(plan, forExpression, lineForNode(stmt)); ok {
				return FastRenderReject{}
			}
			return fastRenderLoopReject(plan, nil, forExpression, lineForNode(stmt))
		}
		if callExpression, ok := stmt.Expression.(*ast.CallExpression); ok && callExpression.Block != nil {
			if _, ok := fastSilentBlockCallPlanFromExpression(plan, callExpression, lineForNode(stmt)); ok {
				return FastRenderReject{}
			}
			return fastRenderBlockCallReject(plan, callExpression, lineForNode(stmt))
		}
		if callExpression, ok := stmt.Expression.(*ast.CallExpression); ok {
			if _, ok := fastSilentCallPlanFromExpression(plan, callExpression, lineForNode(stmt)); ok {
				return FastRenderReject{}
			}
			return fastRenderCallReject(plan, callExpression, lineForNode(stmt))
		}
		if ifExpression, ok := stmt.Expression.(*ast.IfExpression); ok {
			return fastSilentConditionalReject(plan, ifExpression, lineForNode(stmt))
		}
		return rejectFastRender(stmt, "script if body expression is not fast-planned: "+fastExpressionSummary(stmt.Expression))
	case *ast.ReturnStatement:
		if stmt.ReturnValue == nil {
			return FastRenderReject{}
		}
		return fastRenderOutputReject(plan, stmt.ReturnValue, lineForNode(stmt))
	case *ast.LetStatement:
		segments := []FastRenderSegment{}
		if appendFastLetStatement(plan, &segments, stmt) {
			return FastRenderReject{}
		}
		return fastRenderValueReject(plan, stmt.Value, "script if let value")
	default:
		return rejectFastRender(stmt, "unsupported script if body statement type for fast render")
	}
}

func fastConditionalPlanFromExpressionWithAppender(plan *FastRenderPlan, expr *ast.IfExpression, line int, appendStatements func(*FastRenderPlan, *[]FastRenderSegment, []ast.Statement) bool) (*FastConditionalPlan, bool) {
	if expr == nil || appendStatements == nil {
		return nil, false
	}
	conditional := &FastConditionalPlan{Line: line}
	first, ok := fastValuePlanFromExpression(plan, expr.Condition, true, lineForNode(expr.Condition))
	if !ok || !fastConditionValueSupported(first) {
		return nil, false
	}
	firstSegments := []FastRenderSegment{}
	if !appendStatements(plan, &firstSegments, expr.Block.Statements) {
		return nil, false
	}
	conditional.Branches = append(conditional.Branches, FastConditionalBranch{
		Condition: first,
		Segments:  firstSegments,
		Line:      line,
	})
	for _, elseIf := range expr.ElseIf {
		if elseIf == nil {
			return nil, false
		}
		condition, ok := fastValuePlanFromExpression(plan, elseIf.Condition, true, lineForNode(elseIf.Condition))
		if !ok || !fastConditionValueSupported(condition) {
			return nil, false
		}
		segments := []FastRenderSegment{}
		if !appendStatements(plan, &segments, elseIf.Block.Statements) {
			return nil, false
		}
		conditional.Branches = append(conditional.Branches, FastConditionalBranch{
			Condition: condition,
			Segments:  segments,
			Line:      lineForToken(elseIf.TokenAble),
		})
	}
	if expr.ElseBlock != nil {
		segments := []FastRenderSegment{}
		if !appendStatements(plan, &segments, expr.ElseBlock.Statements) {
			return nil, false
		}
		conditional.ElseSegments = segments
	}
	return conditional, true
}

func fastConditionValueSupported(value FastValuePlan) bool {
	return value.Kind != FastValueArray && value.Kind != FastValueHash
}

func fastInfixOperandSupported(value FastValuePlan) bool {
	return value.Kind != FastValueArray && value.Kind != FastValueHash
}

func fastLoopPlanFromExpression(plan *FastRenderPlan, expr *ast.ForExpression, line int) (*FastLoopPlan, bool) {
	return fastLoopPlanFromExpressionWithOuterNames(plan, nil, expr, line)
}

func fastSilentLoopPlanFromExpression(plan *FastRenderPlan, expr *ast.ForExpression, line int) (*FastLoopPlan, bool) {
	return fastLoopPlanFromExpressionWithOuterNamesAndSilent(plan, nil, expr, line, true)
}

func fastNestedLoopPlanFromExpression(plan *FastRenderPlan, parent *FastLoopPlan, expr *ast.ForExpression, line int) (*FastLoopPlan, bool) {
	return fastLoopPlanFromExpressionWithOuterNames(plan, fastLoopOuterNames(parent), expr, line)
}

func fastSilentNestedLoopPlanFromExpression(plan *FastRenderPlan, parent *FastLoopPlan, expr *ast.ForExpression, line int) (*FastLoopPlan, bool) {
	return fastLoopPlanFromExpressionWithOuterNamesAndSilent(plan, fastLoopOuterNames(parent), expr, line, true)
}

func fastLoopPlanFromExpressionWithOuterNames(plan *FastRenderPlan, outerNames []string, expr *ast.ForExpression, line int) (*FastLoopPlan, bool) {
	return fastLoopPlanFromExpressionWithOuterNamesAndSilent(plan, outerNames, expr, line, false)
}

func fastLoopPlanFromExpressionWithOuterNamesAndSilent(plan *FastRenderPlan, outerNames []string, expr *ast.ForExpression, line int, silent bool) (*FastLoopPlan, bool) {
	if expr == nil || expr.Block == nil {
		return nil, false
	}
	iterable, ok := fastValuePlanFromExpression(plan, expr.Iterable, false, lineForNode(expr.Iterable))
	if !ok || !fastLoopIterableValueSupported(iterable) {
		return nil, false
	}
	loop := &FastLoopPlan{
		IterableName:      iterable.Value,
		IterableNameIndex: iterable.NameIndex,
		Iterable:          iterable,
		KeyName:           expr.KeyName,
		ValueName:         expr.ValueName,
		OuterNames:        append([]string(nil), outerNames...),
		Silent:            silent,
		Line:              line,
		SizeStats:         &LoopSizeStats{},
	}
	if !appendFastLoopStatements(plan, loop, &loop.Parts, expr.Block.Statements) {
		return nil, false
	}
	loop.PartFlagsSet = true
	return loop, true
}

func fastLoopIterableValueSupported(value FastValuePlan) bool {
	switch value.Kind {
	case FastValueName, FastValuePath, FastValueCall, FastValueArray, FastValueHash:
		return true
	default:
		return false
	}
}

func appendFastLoopStatements(plan *FastRenderPlan, loop *FastLoopPlan, parts *[]FastLoopPart, statements []ast.Statement) bool {
	for _, stmt := range statements {
		if !appendFastLoopStatement(plan, loop, parts, stmt) {
			return false
		}
	}
	return true
}

func appendFastLoopStatement(plan *FastRenderPlan, loop *FastLoopPlan, parts *[]FastLoopPart, stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.ExpressionStatement:
		if isFastCommentStatement(stmt) {
			return true
		}
		switch expr := stmt.Expression.(type) {
		case *ast.HTMLLiteral:
			appendFastLoopStatic(plan, loop, parts, expr.Value)
			return true
		case *ast.BreakExpression:
			appendFastLoopControlPart(parts, FastLoopPartBreak, lineForNode(stmt))
			return true
		case *ast.ContinueExpression:
			appendFastLoopControlPart(parts, FastLoopPartContinue, lineForNode(stmt))
			return true
		case *ast.AssignExpression:
			return appendFastLoopAssignExpression(plan, loop, parts, expr, lineForNode(stmt))
		case *ast.IndexExpression:
			if expr.Value != nil {
				return appendFastLoopIndexAssignExpression(plan, loop, parts, expr, lineForNode(stmt))
			}
			return false
		case *ast.CallExpression:
			if expr.Block == nil {
				call, ok := fastSilentLoopCallPlanFromExpression(plan, loop, expr, lineForNode(stmt))
				if !ok {
					return false
				}
				*parts = append(*parts, FastLoopPart{
					Kind:  FastLoopPartCall,
					Value: call.Name,
					Call:  call,
					Line:  lineForNode(stmt),
				})
				return true
			}
			blockCall, ok := fastSilentLoopBlockCallPlanFromExpression(plan, loop, expr, lineForNode(stmt))
			if !ok {
				return false
			}
			*parts = append(*parts, FastLoopPart{
				Kind:      FastLoopPartBlockCall,
				Value:     blockCall.Name,
				BlockCall: blockCall,
				Line:      lineForNode(stmt),
			})
			return true
		case *ast.ForExpression:
			nested, ok := fastSilentNestedLoopPlanFromExpression(plan, loop, expr, lineForNode(stmt))
			if !ok {
				return false
			}
			*parts = append(*parts, FastLoopPart{
				Kind: FastLoopPartLoop,
				Loop: nested,
				Line: lineForNode(stmt),
			})
			return true
		case *ast.IfExpression:
			conditional, ok := fastSilentLoopConditionalPlanFromExpression(plan, loop, expr, lineForNode(stmt))
			if !ok {
				return false
			}
			*parts = append(*parts, FastLoopPart{
				Kind:        FastLoopPartConditional,
				Conditional: conditional,
				Line:        lineForNode(stmt),
			})
			return true
		default:
			return false
		}
	case *ast.ReturnStatement:
		if stmt.Type != token.E_START {
			return appendFastLoopReturnStatement(plan, loop, parts, stmt)
		}
		return appendFastLoopOutputParts(plan, loop, parts, stmt.ReturnValue, lineForNode(stmt))
	case *ast.LetStatement:
		return appendFastLoopLetStatement(plan, loop, parts, stmt)
	default:
		return false
	}
}

func appendFastLoopReturnStatement(plan *FastRenderPlan, loop *FastLoopPlan, parts *[]FastLoopPart, stmt *ast.ReturnStatement) bool {
	if stmt == nil {
		return false
	}
	value, ok := fastValuePlanFromLoopOperand(plan, loop, stmt.ReturnValue, false, lineForNode(stmt.ReturnValue))
	if !ok {
		return false
	}
	*parts = append(*parts, FastLoopPart{
		Kind:      FastLoopPartReturn,
		ValuePlan: value,
		Line:      lineForNode(stmt),
	})
	plan.NameCount++
	return true
}

func appendFastLoopAssignExpression(plan *FastRenderPlan, loop *FastLoopPlan, parts *[]FastLoopPart, expr *ast.AssignExpression, line int) bool {
	if expr == nil || expr.Name == nil || expr.Name.Callee != nil || expr.Name.Value == "" || expr.Value == nil {
		return false
	}
	if !fastLoopAssignTargetSupported(loop, expr.Name.Value) {
		return false
	}
	value, ok := fastValuePlanFromLoopOperand(plan, loop, expr.Value, false, lineForNode(expr.Value))
	if !ok || !fastAssignValueSupported(value) {
		return false
	}
	*parts = append(*parts, FastLoopPart{
		Kind:      FastLoopPartAssign,
		Value:     expr.Name.Value,
		NameIndex: plan.bindName(expr.Name.Value),
		ValuePlan: value,
		AssignTarget: &FastAssignTarget{
			Kind:      FastAssignTargetName,
			Name:      expr.Name.Value,
			NameIndex: plan.bindName(expr.Name.Value),
			Line:      line,
		},
		Line: line,
	})
	loop.HasAssign = true
	plan.NameCount++
	return true
}

func appendFastLoopIndexAssignExpression(plan *FastRenderPlan, loop *FastLoopPlan, parts *[]FastLoopPart, expr *ast.IndexExpression, line int) bool {
	target, ok := fastAssignIndexTargetFromExpression(plan, loop, expr, line, true)
	if !ok {
		return false
	}
	value, ok := fastValuePlanFromLoopOperand(plan, loop, expr.Value, false, lineForNode(expr.Value))
	if !ok || !fastAssignValueSupported(value) {
		return false
	}
	*parts = append(*parts, FastLoopPart{
		Kind:         FastLoopPartAssign,
		ValuePlan:    value,
		AssignTarget: &target,
		Line:         line,
	})
	loop.HasAssign = true
	plan.NameCount++
	return true
}

func fastLoopAssignTargetSupported(loop *FastLoopPlan, name string) bool {
	if loop == nil || name == "" || name == "_" {
		return false
	}
	if fastLoopHasOuterName(loop, name) {
		return false
	}
	return true
}

func appendFastLoopLetStatement(plan *FastRenderPlan, loop *FastLoopPlan, parts *[]FastLoopPart, stmt *ast.LetStatement) bool {
	if stmt == nil || stmt.Name == nil || stmt.Name.Callee != nil || stmt.Name.Value == "" || stmt.Value == nil {
		return false
	}
	value, ok := fastValuePlanFromLoopOperand(plan, loop, stmt.Value, false, lineForNode(stmt.Value))
	if !ok {
		return false
	}
	*parts = append(*parts, FastLoopPart{
		Kind:      FastLoopPartLet,
		Value:     stmt.Name.Value,
		NameIndex: plan.bindName(stmt.Name.Value),
		ValuePlan: value,
		Line:      lineForNode(stmt),
	})
	loop.HasLet = true
	plan.NameCount++
	return true
}

func appendFastLoopOutputParts(plan *FastRenderPlan, loop *FastLoopPlan, parts *[]FastLoopPart, expr ast.Expression, line int) bool {
	switch expr := expr.(type) {
	case *ast.StringLiteral:
		value := template.HTMLEscapeString(expr.Value)
		appendFastLoopStatic(plan, loop, parts, value)
		return true
	case *ast.HTMLLiteral:
		appendFastLoopStatic(plan, loop, parts, expr.Value)
		return true
	case *ast.IntegerLiteral:
		value := fmt.Sprint(expr.Value)
		appendFastLoopStatic(plan, loop, parts, value)
		return true
	case *ast.FloatLiteral:
		value := fmt.Sprint(expr.Value)
		appendFastLoopStatic(plan, loop, parts, value)
		return true
	case *ast.Boolean:
		value := fmt.Sprint(expr.Value)
		appendFastLoopStatic(plan, loop, parts, value)
		return true
	case *ast.BreakExpression:
		appendFastLoopControlPart(parts, FastLoopPartBreak, line)
		return true
	case *ast.ContinueExpression:
		appendFastLoopControlPart(parts, FastLoopPartContinue, line)
		return true
	case *ast.IfExpression:
		conditional, ok := fastLoopConditionalPlanFromExpression(plan, loop, expr, line)
		if !ok {
			return false
		}
		*parts = append(*parts, FastLoopPart{
			Kind:        FastLoopPartConditional,
			Conditional: conditional,
			Line:        line,
		})
		return true
	case *ast.ForExpression:
		nested, ok := fastNestedLoopPlanFromExpression(plan, loop, expr, line)
		if !ok {
			return false
		}
		*parts = append(*parts, FastLoopPart{
			Kind: FastLoopPartLoop,
			Loop: nested,
			Line: line,
		})
		return true
	case *ast.Identifier:
		identParts := identifierParts(expr)
		if len(identParts) == 1 && identParts[0] == loop.KeyName {
			*parts = append(*parts, FastLoopPart{Kind: FastLoopPartKey, Line: line})
			return true
		}
		if len(identParts) == 1 && identParts[0] == loop.ValueName {
			*parts = append(*parts, FastLoopPart{Kind: FastLoopPartValue, Line: line})
			return true
		}
		if len(identParts) > 1 && identParts[0] == loop.ValueName {
			if len(identParts) == 2 {
				*parts = append(*parts, FastLoopPart{
					Kind:     FastLoopPartValueProperty,
					Value:    identParts[1],
					Receiver: loop.ValueName,
					Full:     loop.ValueName + "." + identParts[1],
					Line:     line,
				})
				return true
			}
			value := FastValuePlan{Kind: FastValuePath, Value: loop.ValueName, NameIndex: -1, Line: line}
			receiver := loop.ValueName
			for _, property := range identParts[1:] {
				full := receiver + "." + property
				value.Path = append(value.Path, fastPropertyStep(property, receiver, full, line, false))
				receiver = full
			}
			*parts = append(*parts, FastLoopPart{Kind: FastLoopPartValuePath, ValuePlan: value, Line: line})
			return true
		}
		if len(identParts) > 0 && fastLoopHasOuterName(loop, identParts[0]) {
			value, ok := fastValuePlanFromExpression(plan, expr, false, line)
			if !ok {
				return false
			}
			*parts = append(*parts, FastLoopPart{Kind: FastLoopPartValuePath, ValuePlan: value, Line: line})
			return true
		}
	case *ast.CallExpression:
		if blockCall, ok := fastLoopBlockCallPlanFromExpression(plan, loop, expr, line); ok {
			*parts = append(*parts, FastLoopPart{
				Kind:      FastLoopPartBlockCall,
				Value:     blockCall.Name,
				BlockCall: blockCall,
				Line:      line,
			})
			return true
		}
		if partial, ok := fastPartialPlanFromCall(plan, expr, line); ok {
			*parts = append(*parts, FastLoopPart{
				Kind:    FastLoopPartPartial,
				Value:   partial.Name,
				Partial: partial,
				Line:    line,
			})
			return true
		}
		if value, ok := fastValuePlanFromLoopCallWithPlan(plan, loop, expr, line, false); ok {
			*parts = append(*parts, FastLoopPart{Kind: FastLoopPartValuePath, ValuePlan: value, Line: line})
			return true
		}
		if root, ok := fastLoopExpressionRootName(expr); ok && fastLoopHasOuterName(loop, root) {
			value, ok := fastValuePlanFromExpression(plan, expr, false, line)
			if !ok {
				return false
			}
			*parts = append(*parts, FastLoopPart{Kind: FastLoopPartValuePath, ValuePlan: value, Line: line})
			return true
		}
		if call, ok := fastLoopCallPlanFromExpression(plan, loop, expr, line, false); ok {
			*parts = append(*parts, FastLoopPart{
				Kind:  FastLoopPartCall,
				Value: call.Name,
				Call:  call,
				Line:  line,
			})
			return true
		}
		if value, ok := fastValuePlanFromLoopOperand(plan, loop, expr, false, line); ok {
			*parts = append(*parts, FastLoopPart{Kind: FastLoopPartValuePath, ValuePlan: value, Line: line})
			return true
		}
		return false
	}
	switch expr.(type) {
	case *ast.PrefixExpression, *ast.InfixExpression:
		if value, ok := fastValuePlanFromLoopOperand(plan, loop, expr, false, line); ok {
			*parts = append(*parts, FastLoopPart{Kind: FastLoopPartValuePath, ValuePlan: value, Line: line})
			return true
		}
	}
	value, ok := fastValuePlanFromLoopOperand(plan, loop, expr, false, line)
	if !ok {
		return false
	}
	*parts = append(*parts, FastLoopPart{Kind: FastLoopPartValuePath, ValuePlan: value, Line: line})
	return true
}

func appendFastLoopControlPart(parts *[]FastLoopPart, kind FastLoopPartKind, line int) {
	*parts = append(*parts, FastLoopPart{Kind: kind, Line: line})
}

func appendFastLoopStatic(plan *FastRenderPlan, loop *FastLoopPlan, parts *[]FastLoopPart, value string) {
	if value == "" {
		return
	}
	last := len(*parts) - 1
	if last >= 0 && (*parts)[last].Kind == FastLoopPartStatic {
		(*parts)[last].Value += value
	} else {
		*parts = append(*parts, FastLoopPart{
			Kind:  FastLoopPartStatic,
			Value: value,
		})
	}
	if plan != nil {
		plan.StaticSize += len(value)
	}
	if loop != nil {
		loop.StaticSize += len(value)
	}
}

func fastLoopConditionalPlanFromExpression(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.IfExpression, line int) (*FastLoopConditionalPlan, bool) {
	if expr == nil || expr.Block == nil {
		return nil, false
	}
	conditional := &FastLoopConditionalPlan{Line: line}
	first, ok := fastValuePlanFromLoopCondition(plan, loop, expr.Condition, lineForNode(expr.Condition))
	if !ok || !fastConditionValueSupported(first) {
		return nil, false
	}
	firstParts := []FastLoopPart{}
	if !appendFastLoopStatements(plan, loop, &firstParts, expr.Block.Statements) {
		return nil, false
	}
	conditional.Branches = append(conditional.Branches, FastLoopConditionalBranch{
		Condition: first,
		Parts:     firstParts,
		Line:      line,
	})
	for _, elseIf := range expr.ElseIf {
		if elseIf == nil || elseIf.Block == nil {
			return nil, false
		}
		condition, ok := fastValuePlanFromLoopCondition(plan, loop, elseIf.Condition, lineForNode(elseIf.Condition))
		if !ok || !fastConditionValueSupported(condition) {
			return nil, false
		}
		branchParts := []FastLoopPart{}
		if !appendFastLoopStatements(plan, loop, &branchParts, elseIf.Block.Statements) {
			return nil, false
		}
		conditional.Branches = append(conditional.Branches, FastLoopConditionalBranch{
			Condition: condition,
			Parts:     branchParts,
			Line:      lineForToken(elseIf.TokenAble),
		})
	}
	if expr.ElseBlock != nil {
		elseParts := []FastLoopPart{}
		if !appendFastLoopStatements(plan, loop, &elseParts, expr.ElseBlock.Statements) {
			return nil, false
		}
		conditional.ElseParts = elseParts
	}
	return conditional, true
}

func fastSilentLoopConditionalPlanFromExpression(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.IfExpression, line int) (*FastLoopConditionalPlan, bool) {
	conditional, ok := fastLoopConditionalPlanFromExpression(plan, loop, expr, line)
	if !ok {
		return nil, false
	}
	conditional.Silent = true
	return conditional, true
}

func fastLoopPartsHaveAssignmentLetConflict(parts []FastLoopPart, inheritedLets map[string]struct{}) bool {
	letNames := inheritedLets
	for i := range parts {
		part := &parts[i]
		switch part.Kind {
		case FastLoopPartLet:
			if letNames == nil {
				letNames = make(map[string]struct{}, 1)
			}
			letNames[part.Value] = struct{}{}
		case FastLoopPartAssign:
			if _, ok := letNames[part.Value]; ok {
				return true
			}
		case FastLoopPartConditional:
			if part.Conditional == nil {
				continue
			}
			for branchIndex := range part.Conditional.Branches {
				if fastLoopPartsHaveAssignmentLetConflict(part.Conditional.Branches[branchIndex].Parts, cloneFastLoopLetNames(letNames)) {
					return true
				}
			}
			if fastLoopPartsHaveAssignmentLetConflict(part.Conditional.ElseParts, cloneFastLoopLetNames(letNames)) {
				return true
			}
		case FastLoopPartLoop:
			if part.Loop != nil && fastLoopPartsHaveAssignmentLetConflict(part.Loop.Parts, nil) {
				return true
			}
		}
	}
	return false
}

func cloneFastLoopLetNames(names map[string]struct{}) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}
	clone := make(map[string]struct{}, len(names))
	for name := range names {
		clone[name] = struct{}{}
	}
	return clone
}

func fastValuePlanFromLoopCondition(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, line int) (FastValuePlan, bool) {
	if prefix, ok := expr.(*ast.PrefixExpression); ok {
		return fastValuePlanFromLoopPrefix(plan, loop, prefix, line)
	}
	if infix, ok := expr.(*ast.InfixExpression); ok {
		return fastValuePlanFromLoopInfix(plan, loop, infix, line)
	}
	return fastValuePlanFromLoopOperand(plan, loop, expr, true, line)
}

func fastValuePlanFromLoopInfix(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.InfixExpression, line int) (FastValuePlan, bool) {
	if expr != nil && expr.Operator == "+" {
		return fastValuePlanFromLoopConcat(plan, loop, expr, line)
	}
	if expr == nil || !fastInfixOperator(expr.Operator) {
		return FastValuePlan{}, false
	}
	left, ok := fastValuePlanFromLoopOperand(plan, loop, expr.Left, true, lineForNode(expr.Left))
	if !ok {
		return FastValuePlan{}, false
	}
	right, ok := fastValuePlanFromLoopOperand(plan, loop, expr.Right, true, lineForNode(expr.Right))
	if !ok {
		return FastValuePlan{}, false
	}
	if !fastInfixOperandSupported(left) || !fastInfixOperandSupported(right) {
		return FastValuePlan{}, false
	}
	return FastValuePlan{
		Kind:       FastValueInfix,
		Operator:   expr.Operator,
		Left:       &left,
		Right:      &right,
		RegexCache: fastRegexCacheSlot(expr.Operator),
		Line:       line,
	}, true
}

func fastValuePlanFromLoopPrefix(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.PrefixExpression, line int) (FastValuePlan, bool) {
	if expr == nil || expr.Right == nil {
		return FastValuePlan{}, false
	}
	switch expr.Operator {
	case "!", "-":
	default:
		return FastValuePlan{}, false
	}
	right, ok := fastValuePlanFromLoopOperand(plan, loop, expr.Right, true, lineForNode(expr.Right))
	if !ok {
		return FastValuePlan{}, false
	}
	return FastValuePlan{
		Kind:     FastValuePrefix,
		Operator: expr.Operator,
		Right:    &right,
		Line:     line,
	}, true
}

func fastValuePlanFromLoopConcat(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.InfixExpression, line int) (FastValuePlan, bool) {
	if expr == nil || expr.Operator != "+" {
		return FastValuePlan{}, false
	}
	left, ok := fastValuePlanFromLoopOperand(plan, loop, expr.Left, false, lineForNode(expr.Left))
	if !ok {
		return FastValuePlan{}, false
	}
	right, ok := fastValuePlanFromLoopOperand(plan, loop, expr.Right, false, lineForNode(expr.Right))
	if !ok {
		return FastValuePlan{}, false
	}
	return FastValuePlan{
		Kind:     FastValueConcat,
		Operator: expr.Operator,
		Left:     &left,
		Right:    &right,
		Line:     line,
	}, true
}

func fastInfixOperator(operator string) bool {
	switch operator {
	case "-", "*", "/", "==", "!=", "~=", "<", ">", "<=", ">=", "&&", "||":
		return true
	default:
		return false
	}
}

func fastValuePlanFromLoopOperand(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, nullOnMissing bool, line int) (FastValuePlan, bool) {
	if loop == nil {
		return FastValuePlan{}, false
	}
	switch expr := expr.(type) {
	case *ast.ArrayLiteral:
		return fastValuePlanFromLoopArrayLiteral(plan, loop, expr, line)
	case *ast.HashLiteral:
		return fastValuePlanFromLoopHashLiteral(plan, loop, expr, line)
	case *ast.IndexExpression:
		return fastValuePlanFromLoopIndexWithPlan(plan, loop, expr, line, nullOnMissing)
	case *ast.StringLiteral:
		return FastValuePlan{Kind: FastValueString, Value: expr.Value, Line: line}, true
	case *ast.IntegerLiteral:
		return FastValuePlan{Kind: FastValueInteger, IntValue: int64(expr.Value), Line: line}, true
	case *ast.FloatLiteral:
		return FastValuePlan{Kind: FastValueFloat, FloatValue: expr.Value, Line: line}, true
	case *ast.Boolean:
		return FastValuePlan{Kind: FastValueBool, BoolValue: expr.Value, Line: line}, true
	}
	if call, ok := expr.(*ast.CallExpression); ok {
		if value, ok := fastValuePlanFromLoopCallWithPlan(plan, loop, call, line, nullOnMissing); ok {
			return value, true
		}
		if planned, ok := fastLoopCallPlanFromExpression(plan, loop, call, line, nullOnMissing); ok {
			return FastValuePlan{
				Kind: FastValueCall,
				Call: planned,
				Line: line,
			}, true
		}
	}
	if prefix, ok := expr.(*ast.PrefixExpression); ok {
		return fastValuePlanFromLoopPrefix(plan, loop, prefix, line)
	}
	if infix, ok := expr.(*ast.InfixExpression); ok {
		return fastValuePlanFromLoopInfix(plan, loop, infix, line)
	}
	if root, ok := fastLoopExpressionRootName(expr); ok {
		if root == loop.KeyName {
			if isFastLoopKeyIdentifier(loop, expr) {
				return FastValuePlan{Kind: FastValueLoopKey, Value: loop.KeyName, Line: line}, true
			}
			return FastValuePlan{}, false
		}
		if root == loop.ValueName {
			return fastValuePlanFromLoopExpression(plan, loop, expr, line)
		}
		if fastLoopHasOuterName(loop, root) {
			return fastValuePlanFromExpression(plan, expr, nullOnMissing, line)
		}
	}
	return fastValuePlanFromExpression(plan, expr, nullOnMissing, line)
}

func fastValuePlanFromLoopArrayLiteral(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.ArrayLiteral, line int) (FastValuePlan, bool) {
	if expr == nil {
		return FastValuePlan{}, false
	}
	elements := make([]FastValuePlan, 0, len(expr.Elements))
	for _, elementExpr := range expr.Elements {
		value, ok := fastValuePlanFromLoopOperand(plan, loop, elementExpr, false, lineForNode(elementExpr))
		if !ok {
			return FastValuePlan{}, false
		}
		elements = append(elements, value)
	}
	return FastValuePlan{
		Kind:     FastValueArray,
		Elements: elements,
		Line:     line,
	}, true
}

func fastValuePlanFromLoopHashLiteral(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.HashLiteral, line int) (FastValuePlan, bool) {
	if expr == nil {
		return FastValuePlan{}, false
	}
	keys := append([]ast.Expression(nil), expr.Order...)
	if len(keys) == 0 {
		for key := range expr.Pairs {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})
	}
	pairs := make([]FastValuePair, 0, len(keys))
	for _, keyExpr := range keys {
		key, keyPlan, ok := fastLoopHashLiteralKeyPlan(plan, loop, keyExpr, lineForNode(keyExpr))
		if !ok {
			return FastValuePlan{}, false
		}
		valueExpr := expr.Pairs[keyExpr]
		value, ok := fastValuePlanFromLoopOperand(plan, loop, valueExpr, false, lineForNode(valueExpr))
		if !ok {
			return FastValuePlan{}, false
		}
		pairs = append(pairs, FastValuePair{
			Key:     key,
			KeyPlan: keyPlan,
			Value:   value,
			Line:    lineForNode(valueExpr),
		})
	}
	return FastValuePlan{
		Kind:  FastValueHash,
		Pairs: pairs,
		Line:  line,
	}, true
}

func fastLoopHashLiteralKeyPlan(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, line int) (string, *FastValuePlan, bool) {
	if key, ok := fastPartialDataKey(expr); ok {
		return key, nil, true
	}
	value, ok := fastValuePlanFromLoopOperand(plan, loop, expr, false, line)
	if !ok || !fastHashLiteralKeySupported(value) {
		return "", nil, false
	}
	return "", &value, true
}

func fastLoopCallPlanFromExpression(plan *FastRenderPlan, loop *FastLoopPlan, exp *ast.CallExpression, line int, nullOnMissing bool) (*FastCallPlan, bool) {
	if plan == nil || loop == nil || exp == nil || exp.Block != nil || exp.ChainCallee != nil {
		return nil, false
	}
	ident, ok := exp.Function.(*ast.Identifier)
	if !ok || !fastPlainHelperIdentifier(ident) || ident.Value == "nil" {
		return nil, false
	}
	call := &FastCallPlan{
		Name:      ident.Value,
		NameIndex: plan.bindName(ident.Value),
		Line:      line,
	}
	for _, arg := range exp.Arguments {
		value, ok := fastValuePlanFromLoopCallArgument(plan, loop, arg, line, nullOnMissing)
		if !ok {
			return nil, false
		}
		call.Args = append(call.Args, value)
	}
	return call, true
}

func fastSilentLoopCallPlanFromExpression(plan *FastRenderPlan, loop *FastLoopPlan, exp *ast.CallExpression, line int) (*FastCallPlan, bool) {
	call, ok := fastLoopCallPlanFromExpression(plan, loop, exp, line, false)
	if !ok {
		return nil, false
	}
	call.Silent = true
	return call, true
}

func fastLoopBlockCallPlanFromExpression(plan *FastRenderPlan, loop *FastLoopPlan, exp *ast.CallExpression, line int) (*FastBlockCallPlan, bool) {
	if plan == nil || loop == nil || exp == nil || exp.Block == nil || exp.ChainCallee != nil {
		return nil, false
	}
	ident, ok := exp.Function.(*ast.Identifier)
	if !ok || !fastPlainHelperIdentifier(ident) || ident.Value == "nil" {
		return nil, false
	}
	if !fastBlockCanRenderFromSource(exp.Block) {
		return nil, false
	}
	call := &FastBlockCallPlan{
		Name:        ident.Value,
		NameIndex:   plan.bindName(ident.Value),
		Block:       exp.Block,
		BlockSource: fastBlockSource(exp.Block),
		Line:        line,
	}
	for _, arg := range exp.Arguments {
		value, ok := fastValuePlanFromLoopCallArgument(plan, loop, arg, lineForNode(arg), false)
		if !ok {
			return nil, false
		}
		call.Args = append(call.Args, value)
	}
	return call, true
}

func fastSilentLoopBlockCallPlanFromExpression(plan *FastRenderPlan, loop *FastLoopPlan, exp *ast.CallExpression, line int) (*FastBlockCallPlan, bool) {
	call, ok := fastLoopBlockCallPlanFromExpression(plan, loop, exp, line)
	if !ok {
		return nil, false
	}
	call.Silent = true
	return call, true
}

func fastPlainHelperIdentifier(ident *ast.Identifier) bool {
	if ident == nil || ident.Callee != nil || ident.Value == "" {
		return false
	}
	return len(identifierParts(ident)) == 1
}

func fastValuePlanFromLoopCallArgument(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, line int, nullOnMissing bool) (FastValuePlan, bool) {
	return fastValuePlanFromLoopOperand(plan, loop, expr, nullOnMissing, line)
}

func fastLoopOuterNames(parent *FastLoopPlan) []string {
	if parent == nil {
		return nil
	}
	names := append([]string(nil), parent.OuterNames...)
	names = appendFastLoopOuterName(names, parent.KeyName)
	names = appendFastLoopOuterName(names, parent.ValueName)
	return names
}

func appendFastLoopOuterName(names []string, name string) []string {
	if name == "" || name == "_" {
		return names
	}
	for _, existing := range names {
		if existing == name {
			return names
		}
	}
	return append(names, name)
}

func fastLoopHasOuterName(loop *FastLoopPlan, name string) bool {
	if loop == nil || name == "" {
		return false
	}
	for _, outer := range loop.OuterNames {
		if outer == name {
			return true
		}
	}
	return false
}

func fastLoopExpressionRootName(expr ast.Expression) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		parts := identifierParts(expr)
		if len(parts) == 0 {
			return "", false
		}
		return parts[0], true
	case *ast.IndexExpression:
		return fastLoopExpressionRootName(expr.Left)
	case *ast.CallExpression:
		return fastLoopExpressionRootName(expr.Function)
	default:
		return "", false
	}
}

func isFastLoopKeyIdentifier(loop *FastLoopPlan, expr ast.Expression) bool {
	ident, ok := expr.(*ast.Identifier)
	if !ok || ident.Callee != nil {
		return false
	}
	parts := identifierParts(ident)
	return len(parts) == 1 && parts[0] == loop.KeyName
}

func fastValuePlanFromLoopIndex(loop *FastLoopPlan, expr ast.Expression, line int) (FastValuePlan, bool) {
	return fastValuePlanFromLoopIndexWithPlan(&FastRenderPlan{}, loop, expr, line, false)
}

func fastValuePlanFromLoopIndexWithPlan(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, line int, nullOnMissing bool) (FastValuePlan, bool) {
	index, ok := expr.(*ast.IndexExpression)
	if !ok {
		return FastValuePlan{}, false
	}
	value, ok := fastValuePlanFromLoopExpressionWithMethodWithPlan(plan, loop, index.Left, line, false)
	if !ok {
		return fastValuePlanFromLoopDynamicIndex(plan, loop, index, line, nullOnMissing)
	}
	indexStep, ok := fastIndexStepFromExpression(index.Index, line)
	if !ok {
		return fastValuePlanFromLoopDynamicIndex(plan, loop, index, line, nullOnMissing)
	}
	value.Path = append(value.Path, indexStep)
	if index.Callee != nil {
		if !appendFastReceiverCalleeWithArgumentPlanner(&value, index.Callee, lastChainPart(index.Left), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
			return fastValuePlanFromLoopCallArgument(plan, loop, arg, argLine, nullOnMissing)
		}) {
			return FastValuePlan{}, false
		}
	}
	return value, true
}

func fastValuePlanFromLoopDynamicIndex(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.IndexExpression, line int, nullOnMissing bool) (FastValuePlan, bool) {
	if expr == nil || expr.Left == nil || expr.Index == nil {
		return FastValuePlan{}, false
	}
	left, ok := fastValuePlanFromLoopOperand(plan, loop, expr.Left, nullOnMissing, lineForNode(expr.Left))
	if !ok {
		return FastValuePlan{}, false
	}
	index, ok := fastValuePlanFromLoopOperand(plan, loop, expr.Index, nullOnMissing, lineForNode(expr.Index))
	if !ok || !fastIndexOperandSupported(index) {
		return FastValuePlan{}, false
	}
	value := FastValuePlan{
		Kind:  FastValueIndex,
		Left:  &left,
		Right: &index,
		Line:  line,
	}
	if expr.Callee != nil {
		if !appendFastReceiverCalleeWithArgumentPlanner(&value, expr.Callee, lastChainPart(expr.Left), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
			return fastValuePlanFromLoopCallArgument(plan, loop, arg, argLine, nullOnMissing)
		}) {
			return FastValuePlan{}, false
		}
	}
	return value, true
}

func fastValuePlanFromLoopCall(loop *FastLoopPlan, exp *ast.CallExpression, line int) (FastValuePlan, bool) {
	return fastValuePlanFromLoopCallWithPlan(&FastRenderPlan{}, loop, exp, line, false)
}

func fastValuePlanFromLoopCallWithPlan(plan *FastRenderPlan, loop *FastLoopPlan, exp *ast.CallExpression, line int, nullOnMissing bool) (FastValuePlan, bool) {
	if exp == nil || exp.Block != nil {
		return FastValuePlan{}, false
	}
	if exp.ChainCallee != nil {
		root := *exp
		root.ChainCallee = nil
		if call, ok := fastLoopCallPlanFromExpression(plan, loop, &root, line, nullOnMissing); ok {
			value := FastValuePlan{
				Kind: FastValueCall,
				Call: call,
				Line: line,
			}
			if !appendFastReceiverCalleeWithArgumentPlanner(&value, exp.ChainCallee, lastChainPart(exp.Function), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
				return fastValuePlanFromLoopCallArgument(plan, loop, arg, argLine, nullOnMissing)
			}) {
				return FastValuePlan{}, false
			}
			return value, true
		}
	}
	value, ok := fastValuePlanFromLoopExpressionWithMethodWithPlan(plan, loop, exp.Function, line, true)
	if !ok {
		return FastValuePlan{}, false
	}
	callStep := FastPathStep{Kind: FastPathStepCall, Value: callExpressionName(exp), Line: line}
	for _, arg := range exp.Arguments {
		argPlan, ok := fastValuePlanFromLoopCallArgument(plan, loop, arg, lineForNode(arg), nullOnMissing)
		if !ok {
			return FastValuePlan{}, false
		}
		callStep.Args = append(callStep.Args, argPlan)
	}
	appendFastValuePathStep(&value, callStep)
	if exp.ChainCallee != nil {
		if !appendFastReceiverCalleeWithArgumentPlanner(&value, exp.ChainCallee, lastChainPart(exp.Function), line, func(arg ast.Expression, argLine int) (FastValuePlan, bool) {
			return fastValuePlanFromLoopCallArgument(plan, loop, arg, argLine, nullOnMissing)
		}) {
			return FastValuePlan{}, false
		}
	}
	return value, true
}

func fastValuePlanFromLoopExpression(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, line int) (FastValuePlan, bool) {
	return fastValuePlanFromLoopExpressionWithMethodWithPlan(plan, loop, expr, line, false)
}

func fastValuePlanFromLoopExpressionWithMethod(loop *FastLoopPlan, expr ast.Expression, line int, markLastPropertyAsMethod bool) (FastValuePlan, bool) {
	return fastValuePlanFromLoopExpressionWithMethodWithPlan(&FastRenderPlan{}, loop, expr, line, markLastPropertyAsMethod)
}

func fastValuePlanFromLoopExpressionWithMethodWithPlan(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, line int, markLastPropertyAsMethod bool) (FastValuePlan, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		parts := identifierParts(expr)
		if len(parts) == 0 || parts[0] != loop.ValueName {
			return FastValuePlan{}, false
		}
		value := FastValuePlan{Kind: FastValuePath, Value: loop.ValueName, NameIndex: -1, Line: line}
		receiver := loop.ValueName
		for i, property := range parts[1:] {
			full := receiver + "." + property
			method := markLastPropertyAsMethod && i == len(parts[1:])-1
			value.Path = append(value.Path, fastPropertyStep(property, receiver, full, line, method))
			receiver = full
		}
		return value, true
	case *ast.IndexExpression:
		return fastValuePlanFromLoopIndexWithPlan(plan, loop, expr, line, false)
	case *ast.CallExpression:
		return fastValuePlanFromLoopCallWithPlan(plan, loop, expr, line, false)
	default:
		return FastValuePlan{}, false
	}
}

func lineForNode(node ast.Node) int {
	if node == nil {
		return 1
	}
	if line := node.T().LineNumber; line > 0 {
		return line
	}
	return 1
}

func lineForToken(tokenable ast.TokenAble) int {
	if line := tokenable.T().LineNumber; line > 0 {
		return line
	}
	return 1
}
