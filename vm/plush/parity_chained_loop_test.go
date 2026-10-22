package plush_test

import (
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
