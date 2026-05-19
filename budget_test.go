package plush

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// --- Core budget enforcement ---

func TestBudget_LoopExceedsLimit(t *testing.T) {
	r := require.New(t)
	// 5 iterations, each costs 1, limit is 3 → should exceed
	tmpl := `<% for (i,v) in items { } %>`
	ctx := NewContext()
	ctx.Set("items", []int{1, 2, 3, 4, 5})

	_, err := RenderWithBudget(tmpl, 3, ctx)
	r.True(errors.Is(err, ErrBudgetExceeded), "expected ErrBudgetExceeded, got %v", err)
}

func TestBudget_LoopWithinLimit(t *testing.T) {
	r := require.New(t)
	tmpl := `<% for (i,v) in items { } %>`
	ctx := NewContext()
	ctx.Set("items", []int{1, 2, 3})

	_, err := RenderWithBudget(tmpl, 100, ctx)
	r.NoError(err)
}

// --- Nil budget = unlimited (backwards compat) ---

func TestBudget_NilIsUnlimited(t *testing.T) {
	r := require.New(t)
	tmpl := `<% for (i,v) in items { } %>`
	ctx := NewContext()
	ctx.Set("items", []int{1, 2, 3, 4, 5})

	_, err := Render(tmpl, ctx) // no budget attached
	r.NoError(err, "unlimited render should not fail")
}

// --- Zero cost fields are skipped ---

func TestBudget_ZeroCostNeverExceeds(t *testing.T) {
	r := require.New(t)
	costs := ZeroCosts()
	costs.LoopIteration = 0 // free

	tmpl := `<% for (i,v) in items { } %>`
	ctx := NewContext()
	ctx.Set("items", make([]int, 10_000))

	_, err := RenderWithBudgetConfig(tmpl, 1, costs, ctx)
	r.NoError(err, "zero cost loop should never exceed")
}

// --- Custom helper call costs ---

func TestBudget_CustomHelperCost(t *testing.T) {
	r := require.New(t)
	costs := ZeroCosts()
	costs.HelperCall = 100 // each call costs 100

	// 2 calls = 200 units, limit is 150 → should exceed
	tmpl := `<%= myHelper() %><%= myHelper() %>`
	ctx := NewContext()
	ctx.Set("myHelper", func() string { return "ok" })

	_, err := RenderWithBudgetConfig(tmpl, 150, costs, ctx)
	r.True(errors.Is(err, ErrBudgetExceeded), "expected ErrBudgetExceeded, got %v", err)
}

// --- Remaining / Used ---

func TestBudget_UsedAndRemaining(t *testing.T) {
	r := require.New(t)
	b := NewBudget(100)
	ctx := NewContext()
	ctx.Set("items", []int{1, 2, 3}) // 3 loop iterations = 3 units
	ctx.WithBudget(b)

	tmpl := `<% for (i,v) in items { } %>`
	Render(tmpl, ctx)

	r.Greater(b.Used(), int64(0), "expected some units to be used")
	r.Less(b.Remaining(), int64(100), "remaining should be less than limit after render")
}

// --- Condition check cost ---

func TestBudget_ConditionExceedsLimit(t *testing.T) {
	r := require.New(t)
	costs := ZeroCosts()
	costs.ConditionCheck = 5

	// Two if-checks = 10 units, limit is 7 → second exceeds
	tmpl := `<% if (true) { %>a<% } %><% if (true) { %>b<% } %>`
	ctx := NewContext()

	_, err := RenderWithBudgetConfig(tmpl, 7, costs, ctx)
	r.True(errors.Is(err, ErrBudgetExceeded), "expected ErrBudgetExceeded, got %v", err)
}

// --- Sub-render shares parent budget (unit test on Budget directly) ---

func TestBudget_SubRenderSharesParentBudget(t *testing.T) {
	r := require.New(t)
	costs := ZeroCosts()
	costs.SubRender = 50 // each snippet costs 50

	b := NewBudgetWithCosts(75, costs) // limit 75 — second snippet exceeds
	ctx := NewContext()
	ctx.WithBudget(b)

	err1 := b.SpendSubRender() // 50 — ok
	err2 := b.SpendSubRender() // 100 — exceeds 75

	r.NoError(err1, "first snippet should succeed")
	r.True(errors.Is(err2, ErrBudgetExceeded), "second snippet should exceed budget, got %v", err2)
}

// --- NewBudget / WithCosts / Costs ---

func TestBudget_WithCosts(t *testing.T) {
	r := require.New(t)
	b := NewBudget(1000)
	custom := ZeroCosts()
	custom.HelperCall = 42
	b.WithCosts(custom)

	r.Equal(int64(42), b.Costs().HelperCall)
}

func TestBudget_NewBudgetWithCosts(t *testing.T) {
	r := require.New(t)
	costs := DefaultBudgetCosts()
	b := NewBudgetWithCosts(500, costs)

	r.Equal(int64(500), b.Remaining())
	r.Equal(int64(0), b.Used())
}

// --- Per-function cost override ---

func TestBudget_FunctionCostOverride_Exceeds(t *testing.T) {
	r := require.New(t)
	costs := ZeroCosts()
	costs.FunctionCosts = map[string]int64{
		"expensive": 60, // overrides generic HelperCall
	}

	// 2 calls × 60 = 120, limit 100 → second call exceeds
	tmpl := `<%= expensive() %><%= expensive() %>`
	ctx := NewContext()
	ctx.Set("expensive", func() string { return "x" })

	_, err := RenderWithBudgetConfig(tmpl, 100, costs, ctx)
	r.True(errors.Is(err, ErrBudgetExceeded), "expected ErrBudgetExceeded, got %v", err)
}

func TestBudget_FunctionCostOverride_FallsBackToHelperCall(t *testing.T) {
	r := require.New(t)
	costs := ZeroCosts()
	costs.HelperCall = 10
	costs.FunctionCosts = map[string]int64{
		"cheap": 1, // cheap function — does NOT affect "other"
	}

	// "other" has no override, falls back to HelperCall=10; 2 calls = 20 > limit 15
	tmpl := `<%= other() %><%= other() %>`
	ctx := NewContext()
	ctx.Set("other", func() string { return "y" })

	_, err := RenderWithBudgetConfig(tmpl, 15, costs, ctx)
	r.True(errors.Is(err, ErrBudgetExceeded), "expected ErrBudgetExceeded, got %v", err)
}

func TestBudget_FunctionCostOverride_CheapFunctionDoesNotExceed(t *testing.T) {
	r := require.New(t)
	costs := ZeroCosts()
	costs.HelperCall = 100 // default is huge
	costs.FunctionCosts = map[string]int64{
		"cheap": 1, // this function is cheap
	}

	// 5 cheap calls = 5, limit 50 → fine
	tmpl := `<%= cheap() %><%= cheap() %><%= cheap() %><%= cheap() %><%= cheap() %>`
	ctx := NewContext()
	ctx.Set("cheap", func() string { return "ok" })

	_, err := RenderWithBudgetConfig(tmpl, 50, costs, ctx)
	r.NoError(err)
}

// --- Stats report ---

func TestBudget_Stats_LoopIterations(t *testing.T) {
	r := require.New(t)
	b := NewBudget(1_000)
	ctx := NewContext()
	ctx.Set("items", []int{1, 2, 3})
	ctx.WithBudget(b)

	_, err := Render(`<% for (i,v) in items { } %>`, ctx)
	r.NoError(err)

	s := b.Stats()
	r.Equal(int64(3), s.LoopIterations, "3 iterations × cost 1 = 3")
	r.Equal(int64(3), s.TotalUsed)
	r.Equal(int64(0), s.FunctionCalls)
	r.Equal(int64(0), s.ConditionChecks)
}

func TestBudget_Stats_FunctionCalls(t *testing.T) {
	r := require.New(t)
	b := NewBudget(1_000)
	ctx := NewContext()
	ctx.Set("greet", func() string { return "hi" })
	ctx.WithBudget(b)

	_, err := Render(`<%= greet() %><%= greet() %><%= greet() %>`, ctx)
	r.NoError(err)

	s := b.Stats()
	r.Equal(int64(15), s.FunctionCalls, "3 calls × default HelperCall cost 5 = 15")
	r.Equal(int64(15), s.TotalUsed)
	r.Equal(int64(15), s.ByFunction["greet"])
}

func TestBudget_Stats_ByFunctionPerFunctionCost(t *testing.T) {
	r := require.New(t)
	costs := ZeroCosts()
	costs.FunctionCosts = map[string]int64{
		"heavy": 20,
		"light": 2,
	}
	b := NewBudgetWithCosts(1_000, costs)
	ctx := NewContext()
	ctx.Set("heavy", func() string { return "h" })
	ctx.Set("light", func() string { return "l" })
	ctx.WithBudget(b)

	_, err := Render(`<%= heavy() %><%= light() %><%= light() %>`, ctx)
	r.NoError(err)

	s := b.Stats()
	r.Equal(int64(20), s.ByFunction["heavy"], "1 call × 20")
	r.Equal(int64(4), s.ByFunction["light"], "2 calls × 2")
	r.Equal(int64(24), s.FunctionCalls)
	r.Equal(int64(24), s.TotalUsed)
}

func TestBudget_Stats_MixedOperations(t *testing.T) {
	r := require.New(t)
	costs := BudgetCosts{
		LoopIteration:  2,
		HelperCall:     10,
		ConditionCheck: 3,
	}
	b := NewBudgetWithCosts(1_000, costs)
	ctx := NewContext()
	ctx.Set("calc", func() string { return "x" })
	ctx.WithBudget(b)

	// 2 calc calls × 10 = 20, 1 if × 3 = 3 → total 23
	tmpl := `<%= calc() %><%= calc() %><% if (true) { %>ok<% } %>`
	_, err := Render(tmpl, ctx)
	r.NoError(err)

	s := b.Stats()
	r.Equal(int64(20), s.FunctionCalls)
	r.Equal(int64(3), s.ConditionChecks)
	r.Equal(int64(0), s.LoopIterations)
	r.Equal(int64(23), s.TotalUsed)
}

func TestBudget_Stats_NilBudgetReturnsZero(t *testing.T) {
	r := require.New(t)
	var b *Budget
	s := b.Stats()
	r.Equal(BudgetStats{}, s)
}
