package plush

// BudgetCosts defines the work-unit cost for each operation type.
type BudgetCosts struct {
	// LoopIteration is spent once per for-loop iteration.
	// Default: 1
	LoopIteration int64

	// HelperCall is spent each time a registered helper is invoked.
	// Default: 5
	HelperCall int64

	// FilterCall is spent per filter applied (sort, map, where).
	// Default: 3
	FilterCall int64

	// SubRender is spent each time a partial/snippet is rendered.
	// Default: 10
	SubRender int64

	// ConditionCheck is spent per if/unless/case evaluation.
	// Default: 1
	ConditionCheck int64

	// Assignment is spent per variable assignment.
	// Default: 0 (free — rarely the bottleneck)
	Assignment int64

	// ObjectTraversal is spent per dot-notation segment accessed.
	// e.g. product.variants.first = 3 segments = 3 units
	// Default: 1
	ObjectTraversal int64

	// FunctionCosts overrides the default HelperCall cost for specific named
	// functions. The key is the function name as registered in the context.
	// If a name is present here, its cost is used instead of HelperCall.
	// e.g. costs.FunctionCosts = map[string]int64{"expensiveQuery": 50}
	FunctionCosts map[string]int64
}

// DefaultBudgetCosts returns recommended production defaults.
func DefaultBudgetCosts() BudgetCosts {
	return BudgetCosts{
		LoopIteration:   1,
		HelperCall:      5,
		FilterCall:      3,
		SubRender:       10,
		ConditionCheck:  1,
		Assignment:      0,
		ObjectTraversal: 1,
	}
}

// ZeroCosts returns all-zero costs.
// Useful for isolating one operation type in tests.
func ZeroCosts() BudgetCosts {
	return BudgetCosts{}
}
