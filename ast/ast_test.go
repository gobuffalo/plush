package ast

import (
	"testing"

	"github.com/gobuffalo/plush/v4/token"
	"github.com/stretchr/testify/require"
)

func Test_Program_String(t *testing.T) {
	r := require.New(t)
	program := &Program{
		Statements: []Statement{
			&LetStatement{
				TokenAble: TokenAble{token.Token{Type: token.LET, Literal: "let"}},
				Name: &Identifier{
					TokenAble: TokenAble{token.Token{Type: token.IDENT, Literal: "myVar"}},
					Value:     "myVar",
				},
				Value: &Identifier{
					TokenAble: TokenAble{token.Token{Type: token.IDENT, Literal: "anotherVar"}},
					Value:     "anotherVar",
				},
			},
		},
	}

	r.Equal("let myVar = anotherVar;", program.String())
}
