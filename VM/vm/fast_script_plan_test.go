package vm

import (
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

type fastScriptPlanMenu struct {
	Items []fastScriptPlanItem
}

type fastScriptPlanItem struct {
	Name  string
	Count int
}

func Test_VM_Fast_Render_Literal_Lets_And_Path_Loops(t *testing.T) {
	tmpl, err := Compile(`<% let title = "Default" %><title><%= title %></title><%= for (item) in menu.Items { %><%= item.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"menu": fastScriptPlanMenu{Items: []fastScriptPlanItem{
			{Name: "One"},
			{Name: "Two"},
		}},
	}))
	require.NoError(t, err)
	require.Equal(t, `<title>Default</title>One;Two;`, out)
}

func Test_VM_Fast_Render_Loop_Let_With_Helper_And_Arithmetic_Arg(t *testing.T) {
	tmpl, err := Compile(`<%= for (_, product) in products { %><% let categorySeo = replace(category.CategorySeoUrl, "-outofstock", "", 0 - 1) %><%= categorySeo %>:<%= product.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"category": struct {
			CategorySeoUrl string
		}{CategorySeoUrl: "pizza-outofstock"},
		"products": []fastScriptPlanItem{
			{Name: "One"},
			{Name: "Two"},
		},
		"replace": strings.Replace,
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `pizza:One;pizza:Two;`, out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
}

func Test_VM_Fast_Render_Loop_Let_Spends_Assignment_Budget(t *testing.T) {
	tmpl, err := Compile(`<%= for (_, item) in items { %><% let label = item.Name %><%= label %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Empty(t, tmpl.bytecode.FastReject)

	costs := plush.ZeroCosts()
	costs.Assignment = 1
	budget := plush.NewBudgetWithCosts(1, costs)
	ctx := plush.NewContextWith(map[string]interface{}{
		"items": []fastScriptPlanItem{
			{Name: "One"},
			{Name: "Two"},
		},
	}).WithBudget(budget)

	_, err = tmpl.Render(ctx)
	require.ErrorIs(t, err, plush.ErrBudgetExceeded)
	require.Equal(t, int64(2), budget.Stats().Assignments)
}

func Test_VM_Fast_Render_Prefix_Condition_And_Loop_Concat(t *testing.T) {
	tmpl, err := Compile(`<%= if (!userSignedIn) { %>Guest<% } else { %>User<% } %><%= for (item) in menu.Items { %><%= item.Name + " x " + item.Count %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"userSignedIn": false,
		"menu": fastScriptPlanMenu{Items: []fastScriptPlanItem{
			{Name: "One", Count: 2},
			{Name: "Two", Count: 3},
		}},
	}))
	require.NoError(t, err)
	require.Equal(t, `GuestOne x 2;Two x 3;`, out)
}

func Test_VM_Fast_Render_Nested_Loop_Uses_Outer_Key(t *testing.T) {
	tmpl, err := Compile(`<%= for (i, row) in rows { %><%= for (j, col) in row { %><%= i %>,<%= j %>:<%= col %>;<% } %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"rows": [][]string{{"a", "b"}, {"c"}},
	}))
	require.NoError(t, err)
	require.Equal(t, `0,0:a;0,1:b;1,0:c;`, out)
}

func Test_VM_Fast_Render_Nested_Loop_Condition_Uses_Outer_Value(t *testing.T) {
	tmpl, err := Compile(`<%= for (k, messages) in flash { %><%= for (msg) in messages { %><%= if (len(messages) && messages[0] != "skip") { %><%= k %>:<%= msg %>;<% } %><% } %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"flash": map[string][]string{"notice": {"Hello", "Bye"}},
	}))
	require.NoError(t, err)
	require.Equal(t, `notice:Hello;notice:Bye;`, out)
}

func Test_VM_Fast_Render_Silent_Script_If_Discards_Output_But_Evaluates_Selected_Branch(t *testing.T) {
	tmpl, err := Compile(`<% if (show) { %>hidden<%= touch(name) %><% } else { %><%= touch("else") %><% } %>done`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.True(t, tmpl.bytecode.FastRenderPlan.Segments[0].Conditional.Silent)

	calls := []string{}
	ctx := plush.NewContextWith(map[string]interface{}{
		"show": true,
		"name": "Mido",
		"touch": func(value string) string {
			calls = append(calls, value)
			return "called " + value
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "done", out)
	require.Equal(t, []string{"Mido"}, calls)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)

	calls = calls[:0]
	out, err = tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"show": false,
		"name": "Mido",
		"touch": func(value string) string {
			calls = append(calls, value)
			return "called " + value
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "done", out)
	require.Equal(t, []string{"else"}, calls)
}

func Test_VM_Fast_Render_Silent_Script_If_Inside_Loop(t *testing.T) {
	type item struct {
		Name   string
		Hidden bool
		Stop   bool
	}

	tmpl, err := Compile(`<%= for (_, item) in items { %><% if (item.Hidden) { %>hidden<%= touch(item.Name) %><% } %><% if (item.Stop) { %><%= break %><% } %><%= item.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	calls := []string{}
	ctx := plush.NewContextWith(map[string]interface{}{
		"items": []item{
			{Name: "A"},
			{Name: "B", Hidden: true},
			{Name: "C", Stop: true},
			{Name: "D"},
		},
		"touch": func(value string) string {
			calls = append(calls, value)
			return value
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "A;B;", out)
	require.Equal(t, []string{"B"}, calls)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
}
