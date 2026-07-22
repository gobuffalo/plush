package plush

import (
	"fmt"

	rootplush "github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/vm"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

type Template = vm.Template
type FastWriter = vm.FastWriter
type FastArgs = vm.FastArgs
type FastHelperFunc = vm.FastHelperFunc

var ErrFastUnsupported = vm.ErrFastUnsupported

func init() {
	rootplush.RegisterVMRenderer(Render)
}

func Compile(input string) (*Template, error) {
	return vm.Compile(input)
}

// Render renders a Plush template through the compiled VM path.
//
// The root github.com/gobuffalo/plush/v5.Render function remains
// interpreter-backed by default. This package is the opt-in compiled renderer.
func Render(input string, ctx hctx.Context) (string, error) {
	return vm.Render(input, ctx)
}

// RunScript executes a pure Plush script through the compiled VM path.
func RunScript(input string, ctx hctx.Context) error {
	ctx = ctx.New()
	ctx.Set("print", func(i interface{}) {
		fmt.Print(i)
	})
	ctx.Set("println", func(i interface{}) {
		fmt.Println(i)
	})

	_, err := Render("<% "+input+" %>", ctx)
	return err
}

// SetFastHelper registers an optional custom fast writer for a helper name on
// this context. The normal helper should still be present in the context for
// correctness and fallback; returning ErrFastUnsupported from the fast helper
// tells the VM to use the normal helper call path.
func SetFastHelper(ctx hctx.Context, name string, helper FastHelperFunc) {
	vm.SetFastHelper(ctx, name, helper)
}

func ClearFastHelper(ctx hctx.Context, name string) {
	vm.ClearFastHelper(ctx, name)
}
