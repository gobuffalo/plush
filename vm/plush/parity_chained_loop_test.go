package plush_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	rootplush "github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/parser"
	"github.com/gobuffalo/plush/v5/vm/compiler"
	vmplush "github.com/gobuffalo/plush/v5/vm/plush"
	"github.com/gobuffalo/plush/v5/vm/vm"
	"github.com/stretchr/testify/require"
)

type chainedLoopContainer struct {
	items *chainedLoopItems
}

func (c *chainedLoopContainer) GetItems() *chainedLoopItems {
	return c.items
}

func (c *chainedLoopContainer) GetContainer() *chainedLoopContainer {
	return c
}

type chainedLoopItems struct {
	items []string
}

type chainedLoopFields struct {
	Containers *chainedLoopFieldItems
}

type chainedLoopFieldItems struct {
	Containers []string
}

func (i *chainedLoopItems) GetItems() []string {
	return i.items
}

type chainedLoopDynamicContainer struct {
	items interface{}
}

func (c *chainedLoopDynamicContainer) GetItems() interface{} { return c.items }

type chainedLoopDynamicFields struct {
	Containers interface{}
}

func Test_Parity_Loops_Chained_NonIterable(t *testing.T) {
	previous := rootplush.SetVMGenericFallback(false)
	defer rootplush.SetVMGenericFallback(previous)

	for _, tt := range []struct {
		name, expression string
		data             func(middle, leaf interface{}) map[string]interface{}
	}{
		{"methods", "container.GetItems().GetItems()", func(middle, leaf interface{}) map[string]interface{} {
			if middle == nil {
				middle = &chainedLoopDynamicContainer{items: leaf}
			}
			return map[string]interface{}{"container": &chainedLoopDynamicContainer{items: middle}}
		}},
		{"fields", "container.Containers.Containers", func(middle, leaf interface{}) map[string]interface{} {
			if middle == nil {
				middle = &chainedLoopDynamicFields{Containers: leaf}
			}
			return map[string]interface{}{"container": &chainedLoopDynamicFields{Containers: middle}}
		}},
		{"method then field", "container.GetItems().Containers", func(middle, leaf interface{}) map[string]interface{} {
			if middle == nil {
				middle = &chainedLoopDynamicFields{Containers: leaf}
			}
			return map[string]interface{}{"container": &chainedLoopDynamicContainer{items: middle}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := `<%= for (item) in ` + tt.expression + ` { %><%= item %>;<% } %>after`
			tmpl, err := vmplush.Compile(input)
			require.NoError(t, err)
			program, err := parser.Parse(input)
			require.NoError(t, err)
			comp := compiler.New()
			require.NoError(t, comp.Compile(program))
			bytecode := comp.Bytecode()
			bytecode.FastRenderPlan = nil
			// Reuse a successfully rendered plan across invalid receiver/result
			// shapes, then recover. Cached accessors must not retain old values.
			for _, tc := range []struct {
				name         string
				middle, leaf interface{}
				wantError    bool
			}{
				{"warm valid chain", nil, []string{"first"}, false},
				{"integer mid-chain", 42, nil, true},
				{"boolean mid-chain", false, nil, true},
				{"non-iterable final value", nil, 42, true},
				{"valid after errors", nil, []string{"second"}, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					factory := contextWith(tt.data(tc.middle, tc.leaf))
					want, wantErr := renderInterpreter(input, factory)
					ctx := factory()
					got, gotErr := tmpl.Render(ctx)
					if tc.wantError {
						require.Error(t, wantErr)
						require.Error(t, gotErr)
						require.Contains(t, wantErr.Error(), "line 1:")
						if tc.middle == nil {
							require.Contains(t, wantErr.Error(), "could not iterate over int")
						} else {
							require.Contains(t, wantErr.Error(), "container")
						}
					} else {
						require.NoError(t, wantErr)
						require.Equal(t, tc.leaf.([]string)[0]+";after", want)
					}
					require.Equal(t, errorString(wantErr), errorString(gotErr))
					require.Equal(t, want, got)
					requireNoInterpreterFallback(t, ctx, gotErr)
					// Also exercise the bytecode instructions without a fast plan.
					// Run returns the underlying error before Render adds its line.
					machine := vm.NewWithContext(bytecode, factory())
					defer machine.Release()
					bytecodeErr := machine.Run()
					require.Equal(t, strings.TrimPrefix(errorString(wantErr), "line 1: "), errorString(bytecodeErr))
					require.Equal(t, want, machine.Rendered())
				})
			}
		})
	}
}

func Test_Concurrent_Compiled_Chained_Loop(t *testing.T) {
	previous := rootplush.SetVMGenericFallback(false)
	defer rootplush.SetVMGenericFallback(previous)

	for _, expression := range []string{"container.GetItems().GetItems()", "container.Containers.Containers"} {
		for _, warm := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/warm=%t", expression, warm), func(t *testing.T) {
				tmpl, err := vmplush.Compile(`<%= for (item) in ` + expression + ` { %><%= item %>;<% } %><%= marker %>`)
				require.NoError(t, err)
				render := func(worker, iteration int) error {
					marker := fmt.Sprintf("request-%d-%d", worker, iteration)
					values := []string{marker + "-a", marker + "-b"}
					var items interface{} = values
					want := values[0] + ";" + values[1] + ";" + marker
					switch iteration % 3 {
					case 1:
						items = [2]string{values[0], values[1]}
					case 2:
						items = []string(nil)
						want = marker
					}
					var container interface{} = &chainedLoopDynamicContainer{items: &chainedLoopDynamicContainer{items: items}}
					if expression == "container.Containers.Containers" {
						container = &chainedLoopDynamicFields{Containers: &chainedLoopDynamicFields{Containers: items}}
					}
					ctx := rootplush.NewContextWith(map[string]interface{}{"container": container, "marker": marker})
					got, err := tmpl.Render(ctx)
					if err != nil {
						return err
					}
					if got != want {
						return fmt.Errorf("%s: want %q, got %q", marker, want, got)
					}
					diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
					if !ok || diagnostics.FastPath != rootplush.RenderFastPathFast {
						return fmt.Errorf("%s: expected fast render, got %+v", marker, diagnostics)
					}
					return nil
				}
				if warm {
					require.NoError(t, render(-1, 0))
				}
				const workers = 32
				var ready sync.WaitGroup
				ready.Add(workers)
				runConcurrentChecks(t, workers, func(worker int) error {
					ready.Done()
					ready.Wait()
					for iteration := 0; iteration < 32; iteration++ {
						if err := render(worker, iteration); err != nil {
							return err
						}
					}
					return nil
				})
			})
		}
	}
}

func Test_Parity_Loops_Chained_Method_Calls(t *testing.T) {
	factory := contextWith(map[string]interface{}{
		"container": &chainedLoopContainer{items: &chainedLoopItems{items: []string{"first", "second"}}},
	})
	for _, tt := range []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "repeated method name",
			input:    `<%= for (item) in container.GetItems().GetItems() { %><%= item %>;<% } %>`,
			expected: "first;second;",
		},
		{
			name:     "three calls",
			input:    `<%= for (item) in container.GetContainer().GetItems().GetItems() { %><%= item %>;<% } %>`,
			expected: "first;second;",
		},
		{
			name:     "silent loop",
			input:    `<% let result = "" %><% for (item) in container.GetItems().GetItems() { %><% result = result + item + ";" %><% } %><%= result %>`,
			expected: "first;second;",
		},
		{
			name:     "continue and break",
			input:    `<%= for (item) in container.GetItems().GetItems() { if (item == "first") { continue } %><%= item %><% break } %>`,
			expected: "second",
		},
		{
			name:     "repeated method outside loop",
			input:    `<%= container.GetItems().GetItems()[0] %>`,
			expected: "first",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requireBothRender(t, tt.input, tt.expected, factory)
		})
	}
}

func Test_Parity_Loops_Chained_Method_Calls_Empty(t *testing.T) {
	for _, tt := range []struct {
		name  string
		items []string
	}{
		{name: "empty slice", items: []string{}},
		{name: "nil slice", items: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			factory := contextWith(map[string]interface{}{
				"container": &chainedLoopContainer{items: &chainedLoopItems{items: tt.items}},
			})
			input := `before|<%= for (item) in container.GetItems().GetItems() { %>unexpected:<%= item %>;<% } %>|<%= "after" %>`
			requireBothRender(t, input, "before||after", factory)
		})
	}
}

func Test_Parity_Loops_Chained_Method_Calls_FastPath(t *testing.T) {
	input := `<%= for (item) in container.GetItems().GetItems() { %><%= item %>;<% } %>`
	ctx := contextWith(map[string]interface{}{
		"container": &chainedLoopContainer{items: &chainedLoopItems{items: []string{"first", "second"}}},
	})()

	tmpl, err := vmplush.Compile(input)
	require.NoError(t, err)
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "first;second;", out)
	diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, rootplush.RenderFastPathFast, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
}

func Test_Parity_Loops_Chained_Method_Calls_Bytecode(t *testing.T) {
	input := `<%= for (item) in container.GetItems().GetItems() { %><%= item %>;<% } %><%= "after" %>`
	factory := contextWith(map[string]interface{}{
		"container": &chainedLoopContainer{items: &chainedLoopItems{items: []string{"first", "second"}}},
	})

	interpreterOut, err := renderInterpreter(input, factory)
	require.NoError(t, err)
	require.Equal(t, "first;second;after", interpreterOut)

	program, err := parser.Parse(input)
	require.NoError(t, err)
	comp := compiler.New()
	require.NoError(t, comp.Compile(program))
	bytecode := comp.Bytecode()
	bytecode.FastRenderPlan = nil

	// Execute the instructions directly so this regression cannot be hidden
	// by a fast render plan or interpreter fallback.
	machine := vm.NewWithContext(bytecode, factory())
	require.NoError(t, machine.Run())
	require.Equal(t, interpreterOut, machine.Rendered())
}

func Test_Parity_Loops_Repeated_Field_Names(t *testing.T) {
	input := `<%= for (item) in b.Containers.Containers { %><%= item %>;<% } %>`
	for _, tt := range []struct {
		name     string
		items    []string
		expected string
	}{
		{name: "populated", items: []string{"first", "second"}, expected: "first;second;"},
		{name: "empty", items: []string{}, expected: ""},
		{name: "nil slice", items: nil, expected: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			factory := contextWith(map[string]interface{}{
				"b": &chainedLoopFields{Containers: &chainedLoopFieldItems{Containers: tt.items}},
			})
			interpreterOut, err := renderInterpreter(input, factory)
			require.NoError(t, err)
			require.Equal(t, tt.expected, interpreterOut)

			ctx := factory()
			out, err := renderVMContext(t, input, ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, out)
			// Check the cached render: the uncached Render path currently
			// leaves FastPath diagnostics at their default "generic" value.
			ctx = factory()
			out, err = renderVMContext(t, input, ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, out)
			diagnostics, ok := rootplush.RenderDiagnosticsFromContext(ctx)
			require.True(t, ok)
			require.Equal(t, rootplush.RenderFastPathFast, diagnostics.FastPath)
			require.Empty(t, diagnostics.FastReject)

			program, err := parser.Parse(input)
			require.NoError(t, err)
			comp := compiler.New()
			require.NoError(t, comp.Compile(program))
			bytecode := comp.Bytecode()
			bytecode.FastRenderPlan = nil
			machine := vm.NewWithContext(bytecode, factory())
			require.NoError(t, machine.Run())
			require.Equal(t, tt.expected, machine.Rendered())

			// Also verify that following output is preserved on the fast path.
			requireBothRender(t, input+`<%= "after" %>`, tt.expected+"after", factory)
			ctx = factory()
			out, err = renderVMContext(t, input+`<%= "after" %>`, ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected+"after", out)
			diagnostics, ok = rootplush.RenderDiagnosticsFromContext(ctx)
			require.True(t, ok)
			require.Equal(t, rootplush.RenderFastPathFast, diagnostics.FastPath)
		})
	}
}
