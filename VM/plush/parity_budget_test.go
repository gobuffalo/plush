package plush_test

import (
	"errors"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	vmplush "github.com/gobuffalo/plush/v5/VM/plush"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/stretchr/testify/require"
)

func Test_Parity_Budget_Enough_Budget(t *testing.T) {
	compareRender(t, `<%= for (i,v) in items { %><%= v %><% } %>`, func() hctx.Context {
		ctx := rootplush.NewContextWith(map[string]interface{}{
			"items": []string{"a", "b"},
		})
		ctx.WithBudget(rootplush.NewBudget(100))
		return ctx
	})
}

type phase11BudgetGreeter struct{}

func (phase11BudgetGreeter) Greet() string {
	return "hi"
}

type phase11BudgetStats struct {
	Count int
}

type phase11BudgetRobot struct {
	Name  string
	Stats phase11BudgetStats
}

func Test_Parity_Phase_11_Budget_Work_Counters(t *testing.T) {
	costs := rootplush.ZeroCosts()
	costs.LoopIteration = 2
	costs.HelperCall = 3
	costs.ConditionCheck = 5
	costs.Assignment = 7

	result := compareBudgetRender(t, `<% let message = "start" %><% message = "next" %><%= if (ok) { %><%= for (i,v) in items { %><%= helper(v) %><% } %><% } %><%= message %>`, 100, costs, func() map[string]interface{} {
		return map[string]interface{}{
			"ok":    true,
			"items": []string{"a", "b"},
			"helper": func(value string) string {
				return value
			},
		}
	})

	require.Equal(t, "abnext", result.vmOut)
	require.Equal(t, int64(29), result.vmStats.TotalUsed)
	require.Equal(t, int64(4), result.vmStats.LoopIterations)
	require.Equal(t, int64(6), result.vmStats.FunctionCalls)
	require.Equal(t, int64(5), result.vmStats.ConditionChecks)
	require.Equal(t, int64(14), result.vmStats.Assignments)
	require.Equal(t, int64(6), result.vmStats.ByFunction["helper"])
}

func Test_Parity_Phase_11_Function_Cost_Overrides(t *testing.T) {
	costs := rootplush.ZeroCosts()
	costs.HelperCall = 1
	costs.FunctionCosts = map[string]int64{
		"expensive": 7,
		"add":       5,
		"Greet":     3,
	}

	result := compareBudgetRender(t, `<% let add = fn(x) { return x + 1 } %><%= expensive() %>|<%= add(2) %>|<%= greeter.Greet() %>`, 100, costs, func() map[string]interface{} {
		return map[string]interface{}{
			"expensive": func() string { return "x" },
			"greeter":   phase11BudgetGreeter{},
		}
	})

	require.Equal(t, "x|3|hi", result.vmOut)
	require.Equal(t, int64(15), result.vmStats.TotalUsed)
	require.Equal(t, int64(15), result.vmStats.FunctionCalls)
	require.Equal(t, int64(7), result.vmStats.ByFunction["expensive"])
	require.Equal(t, int64(5), result.vmStats.ByFunction["add"])
	require.Equal(t, int64(3), result.vmStats.ByFunction["Greet"])
}

func Test_Parity_Phase_11_Object_Traversal_Budget(t *testing.T) {
	costs := rootplush.ZeroCosts()
	costs.ObjectTraversal = 2

	result := compareBudgetRender(t, `<%= robot.Name %>|<%= robots[0].Stats.Count %>`, 100, costs, func() map[string]interface{} {
		return map[string]interface{}{
			"robot":  phase11BudgetRobot{Name: "Bender"},
			"robots": []phase11BudgetRobot{{Stats: phase11BudgetStats{Count: 3}}},
		}
	})

	require.Equal(t, "Bender|3", result.vmOut)
	require.Equal(t, int64(6), result.vmStats.TotalUsed)
	require.Equal(t, int64(6), result.vmStats.ObjectTraversals)
}

func Test_Parity_Phase_11_Sub_Render_Budget(t *testing.T) {
	costs := rootplush.ZeroCosts()
	costs.SubRender = 11

	result := compareBudgetRender(t, `<%= partial("child.plush") %>`, 100, costs, func() map[string]interface{} {
		return map[string]interface{}{
			"name": "mark",
			"partialFeeder": func(name string) (string, error) {
				require.Equal(t, "child.plush", name)
				return `<%= name %>`, nil
			},
		}
	})

	require.Equal(t, "mark", result.vmOut)
	require.Equal(t, int64(11), result.vmStats.TotalUsed)
	require.Equal(t, int64(11), result.vmStats.SubRenders)
}

func Test_Parity_Phase_11_Budget_Exceeded(t *testing.T) {
	costs := rootplush.ZeroCosts()
	costs.FunctionCosts = map[string]int64{"expensive": 7}

	result := compareBudgetRender(t, `<%= expensive() %>`, 6, costs, func() map[string]interface{} {
		return map[string]interface{}{
			"expensive": func() string { return "x" },
		}
	})

	require.ErrorIs(t, result.vmErr, rootplush.ErrBudgetExceeded)
	require.Equal(t, int64(7), result.vmStats.TotalUsed)
	require.Equal(t, int64(7), result.vmStats.FunctionCalls)
	require.Equal(t, int64(7), result.vmStats.ByFunction["expensive"])
}

type budgetRenderResult struct {
	interpreterOut   string
	interpreterErr   error
	interpreterStats rootplush.BudgetStats
	vmOut            string
	vmErr            error
	vmStats          rootplush.BudgetStats
}

func compareBudgetRender(t *testing.T, input string, limit int64, costs rootplush.BudgetCosts, data func() map[string]interface{}) budgetRenderResult {
	t.Helper()

	interpreterOut, interpreterErr, interpreterStats := renderInterpreterWithBudget(input, limit, costs, data)
	vmOut, vmErr, vmStats := renderVMWithBudget(input, limit, costs, data)

	if errors.Is(interpreterErr, rootplush.ErrBudgetExceeded) || errors.Is(vmErr, rootplush.ErrBudgetExceeded) {
		require.ErrorIs(t, interpreterErr, rootplush.ErrBudgetExceeded)
		require.ErrorIs(t, vmErr, rootplush.ErrBudgetExceeded)
	} else {
		require.Equalf(t, errorString(interpreterErr), errorString(vmErr), "error mismatch\ninterpreter: %q\nvm:          %q", errorString(interpreterErr), errorString(vmErr))
	}
	require.Equalf(t, interpreterOut, vmOut, "output mismatch\ninterpreter: %q\nvm:          %q", interpreterOut, vmOut)
	require.Equal(t, interpreterStats, vmStats)

	return budgetRenderResult{
		interpreterOut:   interpreterOut,
		interpreterErr:   interpreterErr,
		interpreterStats: interpreterStats,
		vmOut:            vmOut,
		vmErr:            vmErr,
		vmStats:          vmStats,
	}
}

func renderInterpreterWithBudget(input string, limit int64, costs rootplush.BudgetCosts, data func() map[string]interface{}) (string, error, rootplush.BudgetStats) {
	budget := rootplush.NewBudgetWithCosts(limit, costs)
	ctx := rootplush.NewContextWith(data())
	ctx.WithBudget(budget)
	out, err := rootplush.Render(input, ctx)
	return out, err, budget.Stats()
}

func renderVMWithBudget(input string, limit int64, costs rootplush.BudgetCosts, data func() map[string]interface{}) (string, error, rootplush.BudgetStats) {
	budget := rootplush.NewBudgetWithCosts(limit, costs)
	ctx := rootplush.NewContextWith(data())
	ctx.WithBudget(budget)
	out, err := vmplush.Render(input, ctx)
	return out, err, budget.Stats()
}
