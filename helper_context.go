package plush

import (
	"fmt"
	"strings"

	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

var _ hctx.HelperContext = &HelperContext{}

// HelperContext is an optional last argument to helpers
// that provides the current context of the call, and access
// to an optional "block" of code that can be executed from
// within the helper.
type HelperContext struct {
	hctx.Context
	compiler *compiler
	block    *ast.BlockStatement
}

const helperContextKind = "HelperContext"

// Render a string with the current context
func (h HelperContext) Render(s string) (string, error) {
	return Render(s, h.Context)
}

// HasBlock returns true if a block is associated with the helper function
func (h HelperContext) HasBlock() bool {
	return h.block != nil
}

// Block executes the block of template associated with
// the helper, think the block inside of an "if" or "each"
// statement.
func (h HelperContext) Block() (string, error) {
	return h.BlockWith(h.Context)
}

// BlockWith executes the block of template associated with
// the helper, think the block inside of an "if" or "each"
// statement, but with it's own context.
func (h HelperContext) BlockWith(hc hctx.Context) (string, error) {
	ctx, ok := hc.(*Context)
	if !ok {
		return "", fmt.Errorf("expected *Context, got %T", hc)
	}

	octx := h.compiler.ctx
	defer func() { h.compiler.ctx = octx }()

	h.compiler.ctx = ctx.New()

	if h.block == nil {
		return "", fmt.Errorf("no block defined")
	}

	i, err := h.compiler.evalBlockStatement(h.block)
	if err != nil {
		return "", err
	}

	bb := &strings.Builder{}
	h.compiler.write(bb, i)

	return bb.String(), nil
}
