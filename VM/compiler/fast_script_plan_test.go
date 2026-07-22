package compiler

import (
	"testing"

	"github.com/gobuffalo/plush/v5/parser"
	"github.com/stretchr/testify/require"
)

func Test_Fast_Render_Plan_Literal_Lets_And_Path_Loops(t *testing.T) {
	input := `<% let title = "Default" %><title><%= title %></title><%= for (item) in menu.Items { %><%= item.Name %>;<% } %>`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	plan := compiler.Bytecode().FastRenderPlan
	require.NotNil(t, plan)
	require.Len(t, plan.Segments, 5)
	require.Equal(t, FastRenderSegmentLet, plan.Segments[0].Kind)
	require.Equal(t, FastValueString, plan.Segments[0].ValuePlan.Kind)
	require.Equal(t, FastRenderSegmentLoop, plan.Segments[4].Kind)
	require.Equal(t, FastValuePath, plan.Segments[4].Loop.Iterable.Kind)
	require.Equal(t, "menu", plan.Segments[4].Loop.Iterable.Value)
	require.Len(t, plan.Segments[4].Loop.Iterable.Path, 1)
	require.Equal(t, "Items", plan.Segments[4].Loop.Iterable.Path[0].Value)
}

func Test_Fast_Render_Plan_Loop_Let_With_Helper_And_Arithmetic_Arg(t *testing.T) {
	input := `<%= for (_, product) in products { %><% let categorySeo = replace(category.CategorySeoUrl, "-outofstock", "", 0 - 1) %><%= categorySeo %>:<%= product.Name %>;<% } %>`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan)
	require.Empty(t, bytecode.FastReject)
	require.Len(t, bytecode.FastRenderPlan.Segments, 1)

	loop := bytecode.FastRenderPlan.Segments[0].Loop
	require.NotNil(t, loop)
	require.GreaterOrEqual(t, len(loop.Parts), 4)
	require.Equal(t, FastLoopPartLet, loop.Parts[0].Kind)
	require.Equal(t, "categorySeo", loop.Parts[0].Value)
	require.Equal(t, FastValueCall, loop.Parts[0].ValuePlan.Kind)
	require.NotNil(t, loop.Parts[0].ValuePlan.Call)
	require.Equal(t, "replace", loop.Parts[0].ValuePlan.Call.Name)
	require.Len(t, loop.Parts[0].ValuePlan.Call.Args, 4)
	require.Equal(t, FastValueInfix, loop.Parts[0].ValuePlan.Call.Args[3].Kind)
	require.Equal(t, "-", loop.Parts[0].ValuePlan.Call.Args[3].Operator)

	require.Equal(t, FastLoopPartValuePath, loop.Parts[1].Kind)
	require.Equal(t, FastValueName, loop.Parts[1].ValuePlan.Kind)
	require.Equal(t, "categorySeo", loop.Parts[1].ValuePlan.Value)
}

func Test_Fast_Render_Plan_Prefix_Condition_And_Loop_Concat(t *testing.T) {
	input := `<%= if (!userSignedIn) { %>Guest<% } else { %>User<% } %><%= for (item) in menu.Items { %><%= item.Name + " x " + item.Count %>;<% } %>`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	plan := compiler.Bytecode().FastRenderPlan
	require.NotNil(t, plan)
	require.Len(t, plan.Segments, 2)
	require.Equal(t, FastRenderSegmentConditional, plan.Segments[0].Kind)
	require.NotNil(t, plan.Segments[0].Conditional)
	require.Len(t, plan.Segments[0].Conditional.Branches, 1)
	require.Equal(t, FastValuePrefix, plan.Segments[0].Conditional.Branches[0].Condition.Kind)
	require.Equal(t, "!", plan.Segments[0].Conditional.Branches[0].Condition.Operator)

	require.Equal(t, FastRenderSegmentLoop, plan.Segments[1].Kind)
	require.NotNil(t, plan.Segments[1].Loop)
	require.Len(t, plan.Segments[1].Loop.Parts, 2)
	require.Equal(t, FastLoopPartValuePath, plan.Segments[1].Loop.Parts[0].Kind)
	require.Equal(t, FastValueConcat, plan.Segments[1].Loop.Parts[0].ValuePlan.Kind)
	require.Equal(t, "+", plan.Segments[1].Loop.Parts[0].ValuePlan.Operator)
	require.Equal(t, FastLoopPartStatic, plan.Segments[1].Loop.Parts[1].Kind)
	require.Equal(t, ";", plan.Segments[1].Loop.Parts[1].Value)
}

func Test_Fast_Render_Plan_Nested_Loop_Tracks_Outer_Names(t *testing.T) {
	input := `<%= for (i, row) in rows { %><%= for (j, col) in row { %><%= i %>,<%= j %>:<%= col %>;<% } %><% } %>`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	plan := compiler.Bytecode().FastRenderPlan
	require.NotNil(t, plan)
	require.Len(t, plan.Segments, 1)
	require.Equal(t, FastRenderSegmentLoop, plan.Segments[0].Kind)

	outer := plan.Segments[0].Loop
	require.NotNil(t, outer)
	require.Len(t, outer.Parts, 1)
	require.Equal(t, FastLoopPartLoop, outer.Parts[0].Kind)

	inner := outer.Parts[0].Loop
	require.NotNil(t, inner)
	require.ElementsMatch(t, []string{"i", "row"}, inner.OuterNames)
	require.Len(t, inner.Parts, 6)
	require.Equal(t, FastLoopPartValuePath, inner.Parts[0].Kind)
	require.Equal(t, FastValueName, inner.Parts[0].ValuePlan.Kind)
	require.Equal(t, "i", inner.Parts[0].ValuePlan.Value)
	require.Equal(t, FastLoopPartKey, inner.Parts[2].Kind)
	require.Equal(t, FastLoopPartValue, inner.Parts[4].Kind)
}

func Test_Fast_Render_Plan_Silent_Script_If(t *testing.T) {
	input := `<% if (show) { %>hidden<%= touch(name) %><% } else { %><%= touch("else") %><% } %>done`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	plan := compiler.Bytecode().FastRenderPlan
	require.NotNil(t, plan)
	require.Len(t, plan.Segments, 2)

	conditional := plan.Segments[0].Conditional
	require.NotNil(t, conditional)
	require.True(t, conditional.Silent)
	require.Len(t, conditional.Branches, 1)
	require.Len(t, conditional.Branches[0].Segments, 2)
	require.Equal(t, FastRenderSegmentStatic, conditional.Branches[0].Segments[0].Kind)
	require.Equal(t, FastRenderSegmentCall, conditional.Branches[0].Segments[1].Kind)
	require.Len(t, conditional.ElseSegments, 1)
	require.Equal(t, FastRenderSegmentCall, conditional.ElseSegments[0].Kind)
	require.Equal(t, FastRenderSegmentStatic, plan.Segments[1].Kind)
}

func Test_Fast_Render_Plan_Silent_Script_If_Inside_Loop(t *testing.T) {
	input := `<%= for (_, item) in items { %><% if (item.Hidden) { %><%= touch(item.Name) %><% } %><%= item.Name %><% } %>`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	plan := compiler.Bytecode().FastRenderPlan
	require.NotNil(t, plan)
	require.Len(t, plan.Segments, 1)

	loop := plan.Segments[0].Loop
	require.NotNil(t, loop)
	require.Len(t, loop.Parts, 2)
	require.Equal(t, FastLoopPartConditional, loop.Parts[0].Kind)
	require.True(t, loop.Parts[0].Conditional.Silent)
	require.Len(t, loop.Parts[0].Conditional.Branches, 1)
	require.Len(t, loop.Parts[0].Conditional.Branches[0].Parts, 1)
	require.Equal(t, FastLoopPartCall, loop.Parts[0].Conditional.Branches[0].Parts[0].Kind)
	require.Equal(t, FastLoopPartValueProperty, loop.Parts[1].Kind)
}

func Test_Fast_Render_Plan_Silent_Script_If_Allows_Assignments(t *testing.T) {
	input := `<% if (show) { name = "changed" } %><%= name %>`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	compiler := New()
	require.NoError(t, compiler.Compile(program))

	bytecode := compiler.Bytecode()
	require.NotNil(t, bytecode.FastRenderPlan, bytecode.FastReject)
	require.Empty(t, bytecode.FastReject)
	require.Len(t, bytecode.FastRenderPlan.Segments, 2)
	conditional := bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, FastRenderSegmentConditional, conditional.Kind)
	require.NotNil(t, conditional.Conditional)
	require.True(t, conditional.Conditional.Silent)
	require.Len(t, conditional.Conditional.Branches, 1)
	require.Len(t, conditional.Conditional.Branches[0].Segments, 1)
	require.Equal(t, FastRenderSegmentAssign, conditional.Conditional.Branches[0].Segments[0].Kind)
}
