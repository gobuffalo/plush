package compiler

import (
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/token"
)

// fastRenderPlanFromProgram builds the richest fast render plan directly from
// the Plush AST. It only returns a plan when every top-level statement has a
// supported render shape; unsupported shapes intentionally fall back to
// bytecode/VM execution. See VM/FAST_PATHS.md.
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
	if plan.NameCount == 0 {
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
		if stmt.Type != token.E_START {
			return rejectFastRender(stmt, "fast render builder declined loop non-output return")
		}
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
		if _, ok := stmt.Expression.(*ast.HTMLLiteral); ok {
			return FastRenderReject{}
		}
		if inLoop {
			switch stmt.Expression.(type) {
			case *ast.BreakExpression, *ast.ContinueExpression:
				return FastRenderReject{}
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
		}
		return rejectFastRender(stmt, "script expression statements are not fast-planned: "+fastExpressionSummary(stmt.Expression))
	case *ast.ReturnStatement:
		if stmt.Type != token.E_START {
			return rejectFastRender(stmt, "non-output return statements are not fast-planned")
		}
		if inLoop {
			return fastRenderLoopOutputReject(plan, loop, stmt.ReturnValue, lineForNode(stmt))
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
		if _, ok := fastCallPlanFromExpression(plan, expr, line); ok {
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
	if iterable.Value == "nil" || !fastLoopIterableValueSupported(iterable) {
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
		if _, ok := fastValuePlanFromLoopCall(loop, expr, line); ok {
			return FastRenderReject{}
		}
		if root, ok := fastLoopExpressionRootName(expr); ok && fastLoopHasOuterName(loop, root) {
			if _, ok := fastValuePlanFromExpression(plan, expr, false, line); ok {
				return FastRenderReject{}
			}
		}
		if _, ok := fastLoopCallPlanFromExpression(plan, loop, expr, line); ok {
			return FastRenderReject{}
		}
		return fastRenderCallReject(plan, expr, line)
	default:
		if _, ok := fastValuePlanFromLoopOperand(plan, loop, expr, false, line); ok {
			return FastRenderReject{}
		}
		if _, ok := fastValuePlanFromLoopIndex(loop, expr, line); ok {
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
		if _, ok := fastValuePlanFromLoopCallArgument(plan, loop, arg, lineForNode(arg)); !ok {
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
			if _, ok := fastPartialDataKey(key); !ok {
				return FastRenderReject{Line: lineForNode(key), Reason: role + ": hash literal keys must be identifiers or strings"}
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
	HasHoles    bool
	HasPartials bool
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

		switch op {
		case code.OpHole:
			features.HasHoles = true
		case code.OpGetName, code.OpGetNameOrNull, code.OpSetName, code.OpAssignName,
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

		if features.HasHoles && features.HasPartials {
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
}

func appendFastStatements(plan *FastRenderPlan, segments *[]FastRenderSegment, statements []ast.Statement) bool {
	for _, stmt := range statements {
		if !appendFastStatement(plan, segments, stmt) {
			return false
		}
	}
	return true
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
		if html, ok := stmt.Expression.(*ast.HTMLLiteral); ok {
			appendFastStatic(plan, segments, html.Value)
			return true
		}
		if assign, ok := stmt.Expression.(*ast.AssignExpression); ok {
			return appendFastAssignExpression(plan, segments, assign, lineForNode(stmt))
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
	default:
		return false
	}
}

func appendFastStatement(plan *FastRenderPlan, segments *[]FastRenderSegment, stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case *ast.ExpressionStatement:
		if html, ok := stmt.Expression.(*ast.HTMLLiteral); ok {
			appendFastStatic(plan, segments, html.Value)
			return true
		}
		if assign, ok := stmt.Expression.(*ast.AssignExpression); ok {
			return appendFastAssignExpression(plan, segments, assign, lineForNode(stmt))
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
		if stmt.Type != token.E_START {
			return false
		}
		return appendFastOutputExpression(plan, segments, stmt.ReturnValue, lineForNode(stmt))
	case *ast.LetStatement:
		return appendFastLetStatement(plan, segments, stmt)
	default:
		return false
	}
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
		Line:      line,
	})
	plan.NameCount++
	return true
}

func fastAssignValueSupported(value FastValuePlan) bool {
	switch value.Kind {
	case FastValueName, FastValueString, FastValueInteger, FastValueFloat, FastValueBool, FastValuePath, FastValueCall, FastValuePrefix:
		return true
	default:
		return false
	}
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
		if call, ok := fastCallPlanFromExpression(plan, expr, line); ok {
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
		key, ok := fastPartialDataKey(keyExpr)
		if !ok {
			return FastValuePlan{}, false
		}
		valueExpr := expr.Pairs[keyExpr]
		value, ok := fastValuePlanFromExpression(plan, valueExpr, false, lineForNode(valueExpr))
		if !ok {
			return FastValuePlan{}, false
		}
		pairs = append(pairs, FastValuePair{
			Key:   key,
			Value: value,
			Line:  lineForNode(valueExpr),
		})
	}
	return FastValuePlan{
		Kind:  FastValueHash,
		Pairs: pairs,
		Line:  line,
	}, true
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
		Kind:     FastValueInfix,
		Operator: expr.Operator,
		Left:     &left,
		Right:    &right,
		Line:     line,
	}, true
}

func fastValuePlanFromPrefixExpression(plan *FastRenderPlan, expr *ast.PrefixExpression, line int) (FastValuePlan, bool) {
	if expr == nil || expr.Operator != "!" || expr.Right == nil {
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
		return FastValuePlan{}, false
	}
	indexStep, ok := fastIndexStepFromExpression(exp.Index, line)
	if !ok {
		return FastValuePlan{}, false
	}
	value.Kind = FastValuePath
	value.Path = append(value.Path, indexStep)
	if exp.Callee != nil {
		if !appendFastReceiverCallee(&value, exp.Callee, lastChainPart(exp.Left), line) {
			return FastValuePlan{}, false
		}
	}
	return value, true
}

func fastValuePlanFromCallExpression(plan *FastRenderPlan, exp *ast.CallExpression, nullOnMissing bool, line int) (FastValuePlan, bool) {
	if exp.Block != nil {
		return FastValuePlan{}, false
	}
	if ident, ok := exp.Function.(*ast.Identifier); ok {
		parts := identifierParts(ident)
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
			if exp.ChainCallee != nil && !appendFastReceiverCallee(&value, exp.ChainCallee, lastChainPart(exp.Function), line) {
				return FastValuePlan{}, false
			}
			return value, true
		}
	}
	if call, ok := fastCallPlanFromExpression(plan, exp, line); ok {
		if call.Name == "partial" {
			return FastValuePlan{}, false
		}
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
		value.Kind = FastValuePath
		value.Path = append(value.Path, FastPathStep{
			Kind:  FastPathStepCall,
			Value: callExpressionName(exp),
			Line:  line,
		})
		if exp.ChainCallee != nil && !appendFastReceiverCallee(&value, exp.ChainCallee, lastChainPart(exp.Function), line) {
			return FastValuePlan{}, false
		}
		return value, true
	}
	return FastValuePlan{}, false
}

func (v FastValuePlan) canUsePath() bool {
	return v.Kind == FastValueName || v.Kind == FastValuePath
}

func appendFastReceiverCallee(value *FastValuePlan, exp ast.Expression, base string, line int) bool {
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
		if !appendFastReceiverCallee(value, exp.Left, base, line) {
			return false
		}
		indexStep, ok := fastIndexStepFromExpression(exp.Index, line)
		if !ok {
			return false
		}
		value.Path = append(value.Path, indexStep)
		if exp.Callee != nil {
			return appendFastReceiverCallee(value, exp.Callee, lastChainPart(exp.Left), line)
		}
		return true
	case *ast.CallExpression:
		if exp.Block != nil || len(exp.Arguments) != 0 {
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
		} else if !appendFastReceiverCallee(value, exp.Function, base, line) {
			return false
		}
		value.Path = append(value.Path, FastPathStep{
			Kind:  FastPathStepCall,
			Value: callExpressionName(exp),
			Line:  line,
		})
		if exp.ChainCallee != nil {
			return appendFastReceiverCallee(value, exp.ChainCallee, lastChainPart(exp.Function), line)
		}
		return true
	default:
		return false
	}
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

func fastCallPlanFromExpression(plan *FastRenderPlan, exp *ast.CallExpression, line int) (*FastCallPlan, bool) {
	if exp == nil || exp.Block != nil || exp.ChainCallee != nil {
		return nil, false
	}
	ident, ok := exp.Function.(*ast.Identifier)
	if !ok || ident.Callee != nil || ident.Value == "" || ident.Value == "nil" || ident.Value == "partial" {
		return nil, false
	}
	call := &FastCallPlan{
		Name:      ident.Value,
		NameIndex: plan.bindName(ident.Value),
		Line:      line,
	}
	for _, arg := range exp.Arguments {
		value, ok := fastValuePlanFromExpression(plan, arg, false, line)
		if !ok {
			return nil, false
		}
		call.Args = append(call.Args, value)
	}
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

func fastBlockCanRenderFromSource(block *ast.BlockStatement) bool {
	return block != nil && !fastBlockStatementsHaveAssignment(block.Statements)
}

func fastBlockStatementsHaveAssignment(statements []ast.Statement) bool {
	for _, stmt := range statements {
		if fastBlockStatementHasAssignment(stmt) {
			return true
		}
	}
	return false
}

func fastBlockStatementHasAssignment(stmt ast.Statement) bool {
	switch stmt := stmt.(type) {
	case nil:
		return false
	case *ast.ExpressionStatement:
		return fastBlockExpressionHasAssignment(stmt.Expression)
	case *ast.ReturnStatement:
		return fastBlockExpressionHasAssignment(stmt.ReturnValue)
	case *ast.LetStatement:
		return fastBlockExpressionHasAssignment(stmt.Value)
	case *ast.BlockStatement:
		return fastBlockStatementsHaveAssignment(stmt.Statements)
	default:
		return false
	}
}

func fastBlockExpressionHasAssignment(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case nil:
		return false
	case *ast.AssignExpression:
		return true
	case *ast.PrefixExpression:
		return fastBlockExpressionHasAssignment(expr.Right)
	case *ast.InfixExpression:
		return fastBlockExpressionHasAssignment(expr.Left) || fastBlockExpressionHasAssignment(expr.Right)
	case *ast.IndexExpression:
		return fastBlockExpressionHasAssignment(expr.Left) || fastBlockExpressionHasAssignment(expr.Index) || fastBlockExpressionHasAssignment(expr.Callee)
	case *ast.CallExpression:
		if fastBlockExpressionHasAssignment(expr.Function) || fastBlockExpressionHasAssignment(expr.ChainCallee) {
			return true
		}
		for _, arg := range expr.Arguments {
			if fastBlockExpressionHasAssignment(arg) {
				return true
			}
		}
		return expr.Block != nil && fastBlockStatementsHaveAssignment(expr.Block.Statements)
	case *ast.IfExpression:
		if fastBlockExpressionHasAssignment(expr.Condition) || (expr.Block != nil && fastBlockStatementsHaveAssignment(expr.Block.Statements)) {
			return true
		}
		for _, elseIf := range expr.ElseIf {
			if elseIf != nil && (fastBlockExpressionHasAssignment(elseIf.Condition) || (elseIf.Block != nil && fastBlockStatementsHaveAssignment(elseIf.Block.Statements))) {
				return true
			}
		}
		return expr.ElseBlock != nil && fastBlockStatementsHaveAssignment(expr.ElseBlock.Statements)
	case *ast.ForExpression:
		return fastBlockExpressionHasAssignment(expr.Iterable) || (expr.Block != nil && fastBlockStatementsHaveAssignment(expr.Block.Statements))
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			if fastBlockExpressionHasAssignment(element) {
				return true
			}
		}
		return false
	case *ast.HashLiteral:
		for key, value := range expr.Pairs {
			if fastBlockExpressionHasAssignment(key) || fastBlockExpressionHasAssignment(value) {
				return true
			}
		}
		return false
	default:
		return false
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
	if !appendFastStatements(plan, &firstSegments, expr.Block.Statements) {
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
		if !appendFastStatements(plan, &segments, elseIf.Block.Statements) {
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
		if !appendFastStatements(plan, &segments, expr.ElseBlock.Statements) {
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
		return rejectFastRender(stmt, "let statements inside script if bodies are not fast-planned")
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

func fastNestedLoopPlanFromExpression(plan *FastRenderPlan, parent *FastLoopPlan, expr *ast.ForExpression, line int) (*FastLoopPlan, bool) {
	return fastLoopPlanFromExpressionWithOuterNames(plan, fastLoopOuterNames(parent), expr, line)
}

func fastLoopPlanFromExpressionWithOuterNames(plan *FastRenderPlan, outerNames []string, expr *ast.ForExpression, line int) (*FastLoopPlan, bool) {
	if expr == nil || expr.Block == nil {
		return nil, false
	}
	iterable, ok := fastValuePlanFromExpression(plan, expr.Iterable, false, lineForNode(expr.Iterable))
	if !ok || iterable.Value == "nil" || !fastLoopIterableValueSupported(iterable) {
		return nil, false
	}
	loop := &FastLoopPlan{
		IterableName:      iterable.Value,
		IterableNameIndex: iterable.NameIndex,
		Iterable:          iterable,
		KeyName:           expr.KeyName,
		ValueName:         expr.ValueName,
		OuterNames:        append([]string(nil), outerNames...),
		Line:              line,
	}
	if !appendFastLoopStatements(plan, loop, &loop.Parts, expr.Block.Statements) {
		return nil, false
	}
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
			return false
		}
		return appendFastLoopOutputParts(plan, loop, parts, stmt.ReturnValue, lineForNode(stmt))
	case *ast.LetStatement:
		return appendFastLoopLetStatement(plan, loop, parts, stmt)
	default:
		return false
	}
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
		if value, ok := fastValuePlanFromLoopCall(loop, expr, line); ok {
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
		if call, ok := fastLoopCallPlanFromExpression(plan, loop, expr, line); ok {
			*parts = append(*parts, FastLoopPart{
				Kind:  FastLoopPartCall,
				Value: call.Name,
				Call:  call,
				Line:  line,
			})
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
		Kind:     FastValueInfix,
		Operator: expr.Operator,
		Left:     &left,
		Right:    &right,
		Line:     line,
	}, true
}

func fastValuePlanFromLoopPrefix(plan *FastRenderPlan, loop *FastLoopPlan, expr *ast.PrefixExpression, line int) (FastValuePlan, bool) {
	if expr == nil || expr.Operator != "!" || expr.Right == nil {
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
	case "-", "*", "/", "==", "!=", "<", ">", "<=", ">=", "&&", "||":
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
	}
	if call, ok := expr.(*ast.CallExpression); ok {
		if value, ok := fastValuePlanFromLoopCall(loop, call, line); ok {
			return value, true
		}
		if planned, ok := fastLoopCallPlanFromExpression(plan, loop, call, line); ok {
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
			return fastValuePlanFromLoopExpression(loop, expr, line)
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
		key, ok := fastPartialDataKey(keyExpr)
		if !ok {
			return FastValuePlan{}, false
		}
		valueExpr := expr.Pairs[keyExpr]
		value, ok := fastValuePlanFromLoopOperand(plan, loop, valueExpr, false, lineForNode(valueExpr))
		if !ok {
			return FastValuePlan{}, false
		}
		pairs = append(pairs, FastValuePair{
			Key:   key,
			Value: value,
			Line:  lineForNode(valueExpr),
		})
	}
	return FastValuePlan{
		Kind:  FastValueHash,
		Pairs: pairs,
		Line:  line,
	}, true
}

func fastLoopCallPlanFromExpression(plan *FastRenderPlan, loop *FastLoopPlan, exp *ast.CallExpression, line int) (*FastCallPlan, bool) {
	if plan == nil || loop == nil || exp == nil || exp.Block != nil || exp.ChainCallee != nil {
		return nil, false
	}
	ident, ok := exp.Function.(*ast.Identifier)
	if !ok || ident.Callee != nil || ident.Value == "" || ident.Value == "nil" || ident.Value == "partial" {
		return nil, false
	}
	call := &FastCallPlan{
		Name:      ident.Value,
		NameIndex: plan.bindName(ident.Value),
		Line:      line,
	}
	for _, arg := range exp.Arguments {
		value, ok := fastValuePlanFromLoopCallArgument(plan, loop, arg, line)
		if !ok {
			return nil, false
		}
		call.Args = append(call.Args, value)
	}
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
		value, ok := fastValuePlanFromLoopCallArgument(plan, loop, arg, lineForNode(arg))
		if !ok {
			return nil, false
		}
		call.Args = append(call.Args, value)
	}
	return call, true
}

func fastPlainHelperIdentifier(ident *ast.Identifier) bool {
	if ident == nil || ident.Callee != nil || ident.Value == "" {
		return false
	}
	return len(identifierParts(ident)) == 1
}

func fastValuePlanFromLoopCallArgument(plan *FastRenderPlan, loop *FastLoopPlan, expr ast.Expression, line int) (FastValuePlan, bool) {
	return fastValuePlanFromLoopOperand(plan, loop, expr, false, line)
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
	index, ok := expr.(*ast.IndexExpression)
	if !ok {
		return FastValuePlan{}, false
	}
	value, ok := fastValuePlanFromLoopExpressionWithMethod(loop, index.Left, line, false)
	if !ok {
		return FastValuePlan{}, false
	}
	indexStep, ok := fastIndexStepFromExpression(index.Index, line)
	if !ok {
		return FastValuePlan{}, false
	}
	value.Path = append(value.Path, indexStep)
	if index.Callee != nil && !appendFastReceiverCallee(&value, index.Callee, lastChainPart(index.Left), line) {
		return FastValuePlan{}, false
	}
	return value, true
}

func fastValuePlanFromLoopCall(loop *FastLoopPlan, exp *ast.CallExpression, line int) (FastValuePlan, bool) {
	if exp == nil || exp.Block != nil || len(exp.Arguments) != 0 {
		return FastValuePlan{}, false
	}
	value, ok := fastValuePlanFromLoopExpressionWithMethod(loop, exp.Function, line, true)
	if !ok {
		return FastValuePlan{}, false
	}
	value.Path = append(value.Path, FastPathStep{Kind: FastPathStepCall, Value: callExpressionName(exp), Line: line})
	if exp.ChainCallee != nil && !appendFastReceiverCallee(&value, exp.ChainCallee, lastChainPart(exp.Function), line) {
		return FastValuePlan{}, false
	}
	return value, true
}

func fastValuePlanFromLoopExpression(loop *FastLoopPlan, expr ast.Expression, line int) (FastValuePlan, bool) {
	return fastValuePlanFromLoopExpressionWithMethod(loop, expr, line, false)
}

func fastValuePlanFromLoopExpressionWithMethod(loop *FastLoopPlan, expr ast.Expression, line int, markLastPropertyAsMethod bool) (FastValuePlan, bool) {
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
		return fastValuePlanFromLoopIndex(loop, expr, line)
	case *ast.CallExpression:
		return fastValuePlanFromLoopCall(loop, expr, line)
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
