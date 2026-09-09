package ast_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/token"
	"github.com/stretchr/testify/require"
)

func Test_Infix_Expression_Regex_Cache_Is_Local_And_Bounded(t *testing.T) {
	expression := ast.NewInfixExpression(ast.TokenAble{}, nil, "~=")

	first, err := expression.CachedRegex(`^Mi`)
	require.NoError(t, err)
	second, err := expression.CachedRegex(`^Mi`)
	require.NoError(t, err)
	require.Same(t, first, second)

	replacement, err := expression.CachedRegex(`^Ad`)
	require.NoError(t, err)
	require.NotSame(t, first, replacement)

	otherExpression := ast.NewInfixExpression(ast.TokenAble{}, nil, "~=")
	other, err := otherExpression.CachedRegex(`^Ad`)
	require.NoError(t, err)
	require.NotSame(t, replacement, other)
}

func Test_Infix_Expression_Regex_Cache_Reuses_Compile_Error(t *testing.T) {
	expression := ast.NewInfixExpression(ast.TokenAble{}, nil, "~=")

	_, first := expression.CachedRegex(`[`)
	require.Error(t, first)
	_, second := expression.CachedRegex(`[`)
	require.Error(t, second)
	require.Same(t, first, second)
}

func Test_Infix_Expression_Regex_Cache_Is_Concurrent(t *testing.T) {
	expression := ast.NewInfixExpression(ast.TokenAble{}, nil, "~=")
	errs := make(chan error, 32)
	var wg sync.WaitGroup

	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			value := fmt.Sprintf("category-%d", i)
			pattern := fmt.Sprintf(`^category-%d$`, i)
			for iteration := 0; iteration < 100; iteration++ {
				re, err := expression.CachedRegex(pattern)
				if err != nil {
					errs <- err
					return
				}
				if !re.MatchString(value) {
					errs <- fmt.Errorf("pattern %q did not match %q", pattern, value)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func Test_Program_String(t *testing.T) {
	r := require.New(t)
	program := &ast.Program{
		Statements: []ast.Statement{
			&ast.LetStatement{
				TokenAble: ast.TokenAble{token.Token{Type: token.LET, Literal: "let"}},
				Name: &ast.Identifier{
					TokenAble: ast.TokenAble{token.Token{Type: token.IDENT, Literal: "myVar"}},
					Value:     "myVar",
				},
				Value: &ast.Identifier{
					TokenAble: ast.TokenAble{token.Token{Type: token.IDENT, Literal: "anotherVar"}},
					Value:     "anotherVar",
				},
			},
		},
	}

	r.Equal("let myVar = anotherVar;", program.String())
}
