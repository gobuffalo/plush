package plush_test

import "testing"

func Test_Parity_Syntax_Current_Parenthesized_If(t *testing.T) {
	compareRender(t, `<%= if (enabled) { %>yes<% } else { %>no<% } %>`, contextWith(map[string]interface{}{
		"enabled": true,
	}))
}

func Test_Parity_Syntax_Current_Else_If(t *testing.T) {
	compareRender(t, `<%= if (first) { %>first<% } else if (second) { %>second<% } else { %>none<% } %>`, contextWith(map[string]interface{}{
		"first":  false,
		"second": true,
	}))
}

func Test_Parity_Syntax_Context_Shadows_VM_Builtins(t *testing.T) {
	compareRender(t, `<%= first %>|<%= if (first) { %>yes<% } else { %>no<% } %>`, contextWith(map[string]interface{}{
		"first": false,
	}))
}

func Test_Parity_Syntax_Comments(t *testing.T) {
	inputs := []string{
		`
		<%# this is a comment %>
		Hi
		`,
		`
		<% <%# this is a comment %> %>
		Hi
		`,
		`
		<%# this is
		a block comment %>
		Hi
		`,
		`
		<% <%# this is
		a block comment %> %>
		Hi`,
	}

	for _, input := range inputs {
		compareRender(t, input, emptyContext)
	}
}

func Test_Parity_Syntax_Regex_Match(t *testing.T) {
	compareRender(t, `<%= if ("foo" ~= "^fo") { %>good<% } else { %>bad<% } %>`, emptyContext)
}
