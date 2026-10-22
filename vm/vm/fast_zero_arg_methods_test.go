package vm

import (
	"errors"
	"strconv"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/vm/compiler"
	"github.com/stretchr/testify/require"
)

type zeroArgMethodFixture struct {
	next  *zeroArgMethodFixture
	items []string
	calls int
}

func (f *zeroArgMethodFixture) Next() *zeroArgMethodFixture       { f.calls++; return f.next }
func (f *zeroArgMethodFixture) Items() []string                   { f.calls++; return f.items }
func (f zeroArgMethodFixture) Value() int                         { return len(f.items) }
func (f zeroArgMethodFixture) WithArgs(value string) []string     { return []string{value} }
func (f zeroArgMethodFixture) Variadic(values ...string) []string { return values }
func (f *zeroArgMethodFixture) Failing() ([]string, error) {
	f.calls++
	return nil, errors.New("getter failed")
}

type zeroArgOtherFixture struct{}

func (zeroArgOtherFixture) Items() []string { return []string{"other"} }

func zeroArgMethodSteps(name string) []compiler.FastPathStep {
	return []compiler.FastPathStep{
		{Kind: compiler.FastPathStepProperty, Value: name, Method: true, Line: 1},
		{Kind: compiler.FastPathStepCall, Value: name, Line: 1},
	}
}

func Test_Fast_Zero_Arg_Method_Chain_Uses_Current_Receiver(t *testing.T) {
	ctx := plush.NewContext()
	steps := append(zeroArgMethodSteps("Next"), zeroArgMethodSteps("Items")...)
	bindings := fastRenderBindings{ctx: ctx}
	for _, label := range []string{"first", "second"} {
		child := &zeroArgMethodFixture{items: []string{label}}
		root := &zeroArgMethodFixture{next: child}
		for call := 1; call <= 2; call++ {
			out, err := evalFastPathSteps(root, steps, ctx, bindings, nil, nil, false)
			require.NoError(t, err)
			require.Equal(t, []string{label}, out)
			require.Equal(t, call, root.calls)
			require.Equal(t, call, child.calls)
		}
	}
	require.IsType(t, &fastZeroArgMethodPlan{}, steps[1].PropertyCache.Load())
	require.IsType(t, &fastZeroArgMethodPlan{}, steps[3].PropertyCache.Load())

	// Reuse the same call site with a different receiver type.
	out, err := evalFastPathSteps(zeroArgOtherFixture{}, steps[2:], ctx, bindings, nil, nil, false)
	require.NoError(t, err)
	require.Equal(t, []string{"other"}, out)
}

func Test_Fast_Zero_Arg_Method_Values_And_Fallbacks(t *testing.T) {
	ctx := plush.NewContext()
	bindings := fastRenderBindings{ctx: ctx}
	for _, tt := range []struct {
		name     string
		receiver interface{}
		method   string
		expected interface{}
		error    string
	}{
		{name: "value receiver", receiver: zeroArgMethodFixture{items: []string{"a"}}, method: "Value", expected: 1},
		{name: "pointer method on value copy", receiver: zeroArgMethodFixture{items: []string{"a"}}, method: "Items", expected: []string{"a"}},
		{name: "nil result", receiver: &zeroArgMethodFixture{}, method: "Items"},
		{name: "nil receiver", receiver: (*zeroArgMethodFixture)(nil), method: "Items", error: "invalid function"},
		{name: "missing method", receiver: &zeroArgMethodFixture{}, method: "Missing", error: "does not have"},
		{name: "error result", receiver: &zeroArgMethodFixture{}, method: "Failing", error: "getter failed"},
		{name: "required argument", receiver: &zeroArgMethodFixture{}, method: "WithArgs", error: "too few arguments"},
		{name: "variadic", receiver: &zeroArgMethodFixture{}, method: "Variadic", expected: []string{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := evalFastPathSteps(tt.receiver, zeroArgMethodSteps(tt.method), ctx, bindings, nil, nil, false)
			if tt.error != "" {
				require.ErrorContains(t, err, tt.error)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, out)
			}
		})
	}
}

func Test_Fast_Zero_Arg_Method_Preserves_Helper_Override(t *testing.T) {
	ctx := plush.NewContext()
	fixture := &zeroArgMethodFixture{items: []string{"original"}}
	steps := zeroArgMethodSteps("Items")
	bindings := fastRenderBindings{ctx: ctx}
	out, err := evalFastPathSteps(fixture, steps, ctx, bindings, nil, nil, false)
	require.NoError(t, err)
	require.Equal(t, []string{"original"}, out)
	SetFastNoContextValueHelper(ctx, "Items", func(FastArgs) (interface{}, error) {
		return []string{"override"}, nil
	})
	out, err = evalFastPathSteps(fixture, steps, ctx, bindings, nil, nil, false)
	require.NoError(t, err)
	require.Equal(t, []string{"override"}, out)
	require.Equal(t, 1, fixture.calls)
}

func Test_Fast_Zero_Arg_Method_Budget(t *testing.T) {
	costs := plush.ZeroCosts()
	costs.HelperCall = 3
	costs.ObjectTraversal = 2
	budget := plush.NewBudgetWithCosts(9, costs)
	ctx := plush.NewContext().WithBudget(budget)
	child := &zeroArgMethodFixture{items: []string{"a"}}
	root := &zeroArgMethodFixture{next: child}
	steps := append(zeroArgMethodSteps("Next"), zeroArgMethodSteps("Items")...)
	_, err := evalFastPathSteps(root, steps, ctx, fastRenderBindings{ctx: ctx}, nil, nil, false)
	require.Error(t, err)
	require.Equal(t, 1, root.calls)
	require.Zero(t, child.calls, "budget exhaustion must prevent the second call")
}

func Test_Fast_Zero_Arg_Method_Shared_Plan(t *testing.T) {
	steps := zeroArgMethodSteps("Items")
	for i := 0; i < 16; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			ctx := plush.NewContext()
			fixture := &zeroArgMethodFixture{items: []string{t.Name()}}
			bindings := fastRenderBindings{ctx: ctx}
			for call := 0; call < 10; call++ {
				out, err := evalFastPathSteps(fixture, steps, ctx, bindings, nil, nil, false)
				require.NoError(t, err)
				require.Equal(t, fixture.items, out)
			}
			require.Equal(t, 10, fixture.calls)
		})
	}
}
