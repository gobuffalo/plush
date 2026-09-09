package plush_test

import (
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

func Test_Interpreter_Regex_Cache_Follows_Cached_AST_Lifetime(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	t.Cleanup(func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	})

	previousMode := plush.SetRenderMode(plush.RenderModeInterpreter)
	t.Cleanup(func() {
		plush.SetRenderMode(previousMode)
	})

	const filename = "interpreter-regex-cache.plush"
	const firstSource = `<%= path ~= pattern %>`
	const categoryPattern = `^/categories/[^/]+/?$`
	ctx := plush.NewContextWith(map[string]interface{}{
		"path":    "/categories/books/",
		"pattern": categoryPattern,
	})
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := plush.Render(firstSource, ctx)
	require.NoError(t, err)
	require.Equal(t, "true", out)

	firstCached, ok := cache.Get(plush.GenerateASTKey(filename))
	require.True(t, ok)
	firstExpression := cachedTemplateRegexExpression(t, firstCached)
	firstRegex, err := firstExpression.CachedRegex(categoryPattern)
	require.NoError(t, err)

	out, err = plush.Render(firstSource, ctx)
	require.NoError(t, err)
	require.Equal(t, "true", out)
	unchangedCached, ok := cache.Get(plush.GenerateASTKey(filename))
	require.True(t, ok)
	unchangedExpression := cachedTemplateRegexExpression(t, unchangedCached)
	require.Same(t, firstExpression, unchangedExpression)
	unchangedRegex, err := unchangedExpression.CachedRegex(categoryPattern)
	require.NoError(t, err)
	require.Same(t, firstRegex, unchangedRegex)

	const productPattern = `^/products/[^/]+/?$`
	ctx.Set("path", "/products/desk")
	ctx.Set("pattern", productPattern)
	out, err = plush.Render(firstSource, ctx)
	require.NoError(t, err)
	require.Equal(t, "true", out)
	replacementRegex, err := unchangedExpression.CachedRegex(productPattern)
	require.NoError(t, err)
	require.NotSame(t, firstRegex, replacementRegex)

	const changedSource = `<%= path ~= pattern %> `
	out, err = plush.Render(changedSource, ctx)
	require.NoError(t, err)
	require.Equal(t, "true ", out)
	changedCached, ok := cache.Get(plush.GenerateASTKey(filename))
	require.True(t, ok)
	changedExpression := cachedTemplateRegexExpression(t, changedCached)
	require.NotSame(t, unchangedExpression, changedExpression)
	changedRegex, err := changedExpression.CachedRegex(productPattern)
	require.NoError(t, err)
	require.NotSame(t, replacementRegex, changedRegex)
}

func cachedTemplateRegexExpression(t *testing.T, template *plush.Template) *ast.InfixExpression {
	t.Helper()
	require.NotNil(t, template)
	require.NotNil(t, template.Program)
	require.NotEmpty(t, template.Program.Statements)
	statement, ok := template.Program.Statements[0].(*ast.ReturnStatement)
	require.Truef(t, ok, "expected return statement, got %T", template.Program.Statements[0])
	expression, ok := statement.ReturnValue.(*ast.InfixExpression)
	require.Truef(t, ok, "expected infix expression, got %T", statement.ReturnValue)
	return expression
}
