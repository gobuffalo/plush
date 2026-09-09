package plush

import (
	"fmt"
	"html/template"

	rootplush "github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/vm/vm"
)

type Template = vm.Template
type FastWriter = vm.FastWriter
type FastArgs = vm.FastArgs
type FastHelperFunc = vm.FastHelperFunc
type FastNoContextValueHelperFunc = vm.FastNoContextValueHelperFunc
type FastReadOnlyContext = vm.FastReadOnlyContext
type FastReadOnlyValueHelperFunc = vm.FastReadOnlyValueHelperFunc
type FastValueHelperFunc = vm.FastValueHelperFunc

var ErrFastUnsupported = vm.ErrFastUnsupported

func init() {
	rootplush.RegisterVMRenderer(Render)
	rootplush.RegisterTrustedVMCacheRenderer(RenderTrustedBytecodeCache)
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

// RenderTrustedBytecodeCache executes cached top-level bytecode by filename
// and reports a miss when source is required.
func RenderTrustedBytecodeCache(filename string, ctx hctx.Context) (string, bool, error) {
	return vm.RenderTrustedBytecodeCache(filename, ctx)
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

// SetFastValueHelper registers an optional custom fast value helper for a
// helper name on this context. It is used when a helper result is needed as a
// Go value, such as assignments, conditions, arguments, and loops.
func SetFastValueHelper(ctx hctx.Context, name string, helper FastValueHelperFunc) {
	vm.SetFastValueHelper(ctx, name, helper)
}

func ClearFastValueHelper(ctx hctx.Context, name string) {
	vm.ClearFastValueHelper(ctx, name)
}

// SetFastNoContextValueHelper registers a value helper that receives no render
// context. Use it when the helper only needs FastArgs. The VM skips scoped
// context creation and binding synchronization for these calls.
func SetFastNoContextValueHelper(ctx hctx.Context, name string, helper FastNoContextValueHelperFunc) {
	vm.SetFastNoContextValueHelper(ctx, name, helper)
}

func ClearFastNoContextValueHelper(ctx hctx.Context, name string) {
	vm.ClearFastNoContextValueHelper(ctx, name)
}

// SetFastReadOnlyValueHelper registers a helper that can read bindings through
// FastReadOnlyContext but cannot write bindings through that interface. Plush
// skips post-call binding synchronization.
//
// Read-only is shallow: maps, slices, pointers, arrays containing references,
// and objects obtained through Value may still expose mutable data. Helpers
// must treat those values as immutable or make defensive copies. Use
// SetFastValueHelper for intentional mutation.
func SetFastReadOnlyValueHelper(ctx hctx.Context, name string, helper FastReadOnlyValueHelperFunc) {
	vm.SetFastReadOnlyValueHelper(ctx, name, helper)
}

func ClearFastReadOnlyValueHelper(ctx hctx.Context, name string) {
	vm.ClearFastReadOnlyValueHelper(ctx, name)
}

// RenderSourcePartial renders runtime Plush source as a named partial value.
// Use FastWriter.WriteSourcePartial for direct output; this form supports
// assignments and nested helper arguments that require a value first.
func RenderSourcePartial(ctx hctx.Context, name, source string, data ...map[string]interface{}) (template.HTML, error) {
	return vm.RenderSourcePartial(ctx, name, source, data...)
}
