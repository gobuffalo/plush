package plush_test

import "testing"

type phase9Robot struct {
	Name string
}

func Test_Parity_Phase_9_Optional_If_Parentheses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		factory  contextFactory
	}{
		{
			name:     "literal true",
			input:    `<%= if true { %>yes<% } %>`,
			expected: "yes",
			factory:  emptyContext,
		},
		{
			name:     "identifier truthy",
			input:    `<%= if name { %><%= name %><% } %>`,
			expected: "mark",
			factory: contextWith(map[string]interface{}{
				"name": "mark",
			}),
		},
		{
			name:     "bang identifier",
			input:    `<%= if !name { %>empty<% } %>`,
			expected: "empty",
			factory: contextWith(map[string]interface{}{
				"name": "",
			}),
		},
		{
			name:     "equality",
			input:    `<%= if name == "mark" { %>yes<% } %>`,
			expected: "yes",
			factory: contextWith(map[string]interface{}{
				"name": "mark",
			}),
		},
		{
			name:     "or expression",
			input:    `<%= if name == "mark" || admin { %>yes<% } %>`,
			expected: "yes",
			factory: contextWith(map[string]interface{}{
				"name":  "ringo",
				"admin": true,
			}),
		},
		{
			name:     "grouped expression with or",
			input:    `<%= if (name == "mark") || admin { %>yes<% } %>`,
			expected: "yes",
			factory: contextWith(map[string]interface{}{
				"name":  "ringo",
				"admin": true,
			}),
		},
		{
			name:     "else if literal",
			input:    `<%= if false { %>no<% } else if true { %>yes<% } %>`,
			expected: "yes",
			factory:  emptyContext,
		},
		{
			name:     "else if expression",
			input:    `<%= if false { %>no<% } else if name == "mark" { %>yes<% } %>`,
			expected: "yes",
			factory: contextWith(map[string]interface{}{
				"name": "mark",
			}),
		},
		{
			name:     "indexed struct field",
			input:    `<%= if robots[0].Name == "mark" { %>yes<% } %>`,
			expected: "yes",
			factory: contextWith(map[string]interface{}{
				"robots": []phase9Robot{{Name: "mark"}},
			}),
		},
		{
			name:     "indexed array compare identifier",
			input:    `<%= if robots[0] == mark { %>yes<% } %>`,
			expected: "yes",
			factory: contextWith(map[string]interface{}{
				"robots": []string{"mark"},
				"mark":   "mark",
			}),
		},
		{
			name:     "indexed map compare string",
			input:    `<%= if robots["TESTING"] == "mark" { %>yes<% } %>`,
			expected: "yes",
			factory: contextWith(map[string]interface{}{
				"robots": map[string]string{"TESTING": "mark"},
			}),
		},
		{
			name:     "old if syntax still works",
			input:    `<%= if (true) { %>yes<% } %>`,
			expected: "yes",
			factory:  emptyContext,
		},
		{
			name:     "old else if syntax still works",
			input:    `<%= if (false) { %>no<% } else if (true) { %>yes<% } %>`,
			expected: "yes",
			factory:  emptyContext,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireBothRender(t, tt.input, tt.expected, tt.factory)
		})
	}
}

func Test_Parity_Phase_9_Optional_If_Parentheses_Errors(t *testing.T) {
	compareBothRenderError(t, `<%= if { %>bad<% } %>`, emptyContext)
	compareBothRenderError(t, `<%= if true %>bad<% } %>`, emptyContext)
}
