package parser_test

import (
	"testing"

	"github.com/gobuffalo/plush/v5/parser"
	"github.com/stretchr/testify/require"
)

func Test_Unterminated_Comment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		err   string
	}{
		{"empty", "<%#", "line 1: unterminated comment"},
		{"text", "<%# comment", "line 1: unterminated comment"},
		{"partial closing tag", "<%# comment %", "line 1: unterminated comment"},
		{"multiline", "<%# first line\nsecond line", "line 2: unterminated comment"},
		{"trailing newline", "<%#\n", "line 2: unterminated comment"},
		{"after HTML", "Hello\n<%# comment", "line 2: unterminated comment"},
		{"inside code", "<% <%# comment", "line 1: unterminated comment"},
		{"after closed comment", "<%# closed %><%#", "line 1: unterminated comment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.input)
			require.EqualError(t, err, tt.err)
		})
	}
}
