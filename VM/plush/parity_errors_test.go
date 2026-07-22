package plush_test

import (
	"fmt"
	"testing"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/stretchr/testify/require"
)

type phase13Robot struct {
	Name    string
	Friends []phase13Robot
	hidden  string
}

func (r phase13Robot) GetFriends() []phase13Robot {
	return r.Friends
}

func Test_Parity_Phase_13_Exact_Runtime_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		data  map[string]interface{}
	}{
		{name: "unknown identifier", input: `<%= missing %>`},
		{name: "unknown function", input: `<%= missing() %>`},
		{name: "unknown struct field", input: `<%= robot.Missing %>`, data: phase13Data()},
		{name: "unknown method", input: `<%= robot.Missing() %>`, data: phase13Data()},
		{name: "call field as method", input: `<%= robot.Name() %>`, data: phase13Data()},
		{name: "unknown chained method", input: `<%= robot.Name.Missing() %>`, data: phase13Data()},
		{name: "unexported field", input: `<%= robot.hidden %>`, data: phase13Data()},
		{name: "invalid helper argument", input: `<%= one("x") %>`, data: map[string]interface{}{
			"one": func(int) string { return "" },
		}},
		{name: "helper returned error", input: `<%= fail() %>`, data: map[string]interface{}{
			"fail": func() (string, error) { return "", fmt.Errorf("helper boom") },
		}},
		{name: "map key type", input: `<%= lookup["bad"] %>`, data: map[string]interface{}{
			"lookup": map[int]string{1: "one"},
		}},
		{name: "slice index type", input: `<%= items["bad"] %>`, data: map[string]interface{}{
			"items": []string{"a"},
		}},
		{name: "slice out of bounds", input: `<%= items[3] %>`, data: map[string]interface{}{
			"items": []string{"a"},
		}},
		{name: "invalid assignment", input: `<% missing = "x" %>`},
		{name: "division by zero", input: `<%= 10 / 0 %>`},
		{name: "break outside loop", input: `<% break %>`},
		{name: "continue outside loop", input: `<% continue %>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compareExactRenderError(t, tt.input, contextWith(tt.data))
		})
	}
}

func Test_Parity_Phase_13_Line_Numbers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		data  map[string]interface{}
		line  string
	}{
		{
			name: "unknown on line 2",
			input: `<p>
<%= f.Foo %>
</p>`,
			line: "line 2:",
		},
		{
			name: "for iterable on line 2",
			input: `
<%= for (n) in numbers.Foo { %>
<%= n %>
<% } %>`,
			line: "line 2:",
		},
		{
			name: "inside loop on line 3",
			input: `
<%= for (n) in numbers { %>
<%= n.Foo %>
<% } %>`,
			data: map[string]interface{}{"numbers": []int{1}},
			line: "line 3:",
		},
		{
			name: "missing keyword on line 6",
			input: `




<%=  (n) in numbers { %>
<%= n %>
<% } %>`,
			data: map[string]interface{}{"numbers": []int{1}},
			line: "line 6:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, interpreterErr := renderInterpreter(tt.input, contextWith(tt.data))
			_, vmErr := renderVM(tt.input, contextWith(tt.data))
			require.Error(t, interpreterErr)
			require.Error(t, vmErr)
			require.Equal(t, interpreterErr.Error(), vmErr.Error())
			require.Contains(t, vmErr.Error(), tt.line)
		})
	}
}

func Test_Parity_Phase_13_Parser_And_EOF_Syntax_Errors(t *testing.T) {
	compareExactRenderError(t, `<% let = %>`, emptyContext)
	compareBothRenderError(t, `<%= if true %>bad<% } %>`, emptyContext)
	compareBothRenderError(t, `<%= foo("unterminated) %>`, contextWith(map[string]interface{}{
		"foo": func(string) string { return "" },
	}))
}

func phase13Data() map[string]interface{} {
	return map[string]interface{}{
		"robot": phase13Robot{
			Name:    "bender",
			Friends: []phase13Robot{{Name: "fry"}},
			hidden:  "secret",
		},
	}
}

func compareExactRenderError(t *testing.T, input string, factory func() hctx.Context) {
	t.Helper()

	interpreterOut, interpreterErr := renderInterpreter(input, factory)
	vmOut, vmErr := renderVM(input, factory)

	require.Error(t, interpreterErr, "expected interpreter error, got output %q", interpreterOut)
	require.Error(t, vmErr, "expected VM error, got output %q", vmOut)
	require.Equal(t, interpreterOut, vmOut)
	require.Equal(t, interpreterErr.Error(), vmErr.Error())
}
