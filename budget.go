package plush

import (
	"errors"
	"sync"
	"sync/atomic"
)

// ErrBudgetExceeded is returned when a render exhausts its budget.
var ErrBudgetExceeded = errors.New("render budget exceeded")

// BudgetStats is a snapshot of work units consumed per operation category.
// Retrieve it after rendering via b.Stats().
type BudgetStats struct {
	// TotalUsed is the sum of all units spent (equals b.Used()).
	TotalUsed int64
	// LoopIterations is total units charged by loop iterations.
	LoopIterations int64
	// FunctionCalls is total units charged by all function/helper calls.
	FunctionCalls int64
	// FilterCalls is total units charged by filter calls.
	FilterCalls int64
	// SubRenders is total units charged by partial/snippet renders.
	SubRenders int64
	// ConditionChecks is total units charged by if/unless evaluations.
	ConditionChecks int64
	// Assignments is total units charged by variable assignments.
	Assignments int64
	// ObjectTraversals is total units charged by dot-notation traversal.
	ObjectTraversals int64
	// ByFunction breaks FunctionCalls down by name for calls made via
	// SpendFunctionCall. Functions without a FunctionCosts override appear
	// here using the generic HelperCall cost.
	ByFunction map[string]int64
}

// Budget tracks render work units during template evaluation.
// A nil Budget is always unlimited — zero breaking changes.
type Budget struct {
	limit   int64
	counter atomic.Int64
	costs   BudgetCosts

	// per-category stat counters — all lock-free
	statLoop      atomic.Int64
	statFunction  atomic.Int64 // total of all function/helper calls
	statFilter    atomic.Int64
	statSubRender atomic.Int64
	statCondition atomic.Int64
	statAssign    atomic.Int64
	statTraversal atomic.Int64

	// per-function breakdown — mutex-protected plain map
	statFuncsMu  sync.Mutex
	statFuncsMap map[string]int64
}

// NewBudget creates a Budget with a limit and default costs.
func NewBudget(limit int64) *Budget {
	return &Budget{
		limit:        limit,
		costs:        DefaultBudgetCosts(),
		statFuncsMap: make(map[string]int64),
	}
}

// NewBudgetWithCosts creates a Budget with fully custom per-operation costs.
func NewBudgetWithCosts(limit int64, costs BudgetCosts) *Budget {
	return &Budget{
		limit:        limit,
		costs:        costs,
		statFuncsMap: make(map[string]int64),
	}
}

// WithCosts replaces the cost configuration. Returns self for chaining.
func (b *Budget) WithCosts(costs BudgetCosts) *Budget {
	b.costs = costs
	return b
}

// Costs returns the active cost configuration.
func (b *Budget) Costs() BudgetCosts {
	return b.costs
}

// Used returns total units consumed so far.
func (b *Budget) Used() int64 {
	return b.counter.Load()
}

// Remaining returns units left before the limit is hit.
func (b *Budget) Remaining() int64 {
	r := b.limit - b.counter.Load()
	if r < 0 {
		return 0
	}
	return r
}

// Stats returns a snapshot of work units consumed per operation category.
// Safe to call at any point during or after rendering.
func (b *Budget) Stats() BudgetStats {
	if b == nil {
		return BudgetStats{}
	}
	s := BudgetStats{
		TotalUsed:        b.counter.Load(),
		LoopIterations:   b.statLoop.Load(),
		FunctionCalls:    b.statFunction.Load(),
		FilterCalls:      b.statFilter.Load(),
		SubRenders:       b.statSubRender.Load(),
		ConditionChecks:  b.statCondition.Load(),
		Assignments:      b.statAssign.Load(),
		ObjectTraversals: b.statTraversal.Load(),
		ByFunction:       make(map[string]int64),
	}
	b.statFuncsMu.Lock()
	for k, v := range b.statFuncsMap {
		s.ByFunction[k] = v
	}
	b.statFuncsMu.Unlock()
	return s
}

// SpendLoop spends the loop iteration cost.
func (b *Budget) SpendLoop() error {
	if b == nil {
		return nil
	}
	b.statLoop.Add(b.costs.LoopIteration)
	return b.spend(b.costs.LoopIteration)
}

// SpendHelperCall spends the helper call cost.
func (b *Budget) SpendHelperCall() error {
	if b == nil {
		return nil
	}
	b.statFunction.Add(b.costs.HelperCall)
	return b.spend(b.costs.HelperCall)
}

// SpendFilter spends the filter call cost.
func (b *Budget) SpendFilter() error {
	if b == nil {
		return nil
	}
	b.statFilter.Add(b.costs.FilterCall)
	return b.spend(b.costs.FilterCall)
}

// SpendSubRender spends the sub-render cost.
func (b *Budget) SpendSubRender() error {
	if b == nil {
		return nil
	}
	b.statSubRender.Add(b.costs.SubRender)
	return b.spend(b.costs.SubRender)
}

// SpendCondition spends the condition check cost.
func (b *Budget) SpendCondition() error {
	if b == nil {
		return nil
	}
	b.statCondition.Add(b.costs.ConditionCheck)
	return b.spend(b.costs.ConditionCheck)
}

// SpendAssignment spends the assignment cost.
func (b *Budget) SpendAssignment() error {
	if b == nil {
		return nil
	}
	b.statAssign.Add(b.costs.Assignment)
	return b.spend(b.costs.Assignment)
}

// SpendFunctionCall spends the cost for a named function call.
// Uses FunctionCosts[name] if set, otherwise falls back to HelperCall cost.
func (b *Budget) SpendFunctionCall(name string) error {
	if b == nil {
		return nil
	}
	cost := b.costs.HelperCall
	if c, ok := b.costs.FunctionCosts[name]; ok {
		cost = c
	}
	b.statFunction.Add(cost)
	b.statFuncsMu.Lock()
	b.statFuncsMap[name] += cost
	b.statFuncsMu.Unlock()
	return b.spend(cost)
}

// SpendObjectTraversal spends ObjectTraversal * segments units.
// e.g. product.variants.first = 3 segments → costs ObjectTraversal * 3
func (b *Budget) SpendObjectTraversal(segments int) error {
	if b == nil {
		return nil
	}
	units := b.costs.ObjectTraversal * int64(segments)
	b.statTraversal.Add(units)
	return b.spend(units)
}

// spend is the internal hot path. Uses atomic add with no locks.
func (b *Budget) spend(units int64) error {
	if b == nil || units == 0 {
		return nil
	}
	if b.counter.Add(units) > b.limit {
		return ErrBudgetExceeded
	}
	return nil
}
