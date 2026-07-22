package vm

import (
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

func (a *fastCallArgs) Append(value interface{}) {
	if a.n < len(a.inline) {
		a.inline[a.n] = value
	} else {
		a.extra = append(a.extra, value)
	}
	a.n++
}

func (a *fastCallArgs) Reset() {
	if a == nil {
		return
	}
	for i := 0; i < a.n && i < len(a.inline); i++ {
		a.inline[i] = nil
	}
	for i := range a.extra {
		a.extra[i] = nil
	}
	a.n = 0
	a.extra = a.extra[:0]
	a.objects = nil
}

func (a *fastCallArgs) Len() int {
	if a == nil {
		return 0
	}
	return a.n
}

func (a *fastCallArgs) Raw(index int) interface{} {
	if a == nil || index < 0 || index >= a.n {
		return nil
	}
	if index < len(a.inline) {
		return a.inline[index]
	}
	return a.extra[index-len(a.inline)]
}

func (a *fastCallArgs) Objects() []object.Object {
	if a == nil || a.n == 0 {
		return nil
	}
	if len(a.objects) == a.n {
		return a.objects
	}
	objects := make([]object.Object, a.n)
	for i := 0; i < a.n; i++ {
		objects[i] = object.Wrap(a.Raw(i))
	}
	a.objects = objects
	return objects
}

func writeFastCallSegment(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, call *compiler.FastCallPlan) error {
	if call == nil {
		return nil
	}
	raw, ok := bindings.value(call.NameIndex)
	if !ok {
		return fastLineError(call.Line, fmt.Errorf("%q: unknown identifier", call.Name))
	}
	if err := spendFastFunctionCall(ctx, call.Name, call.Line); err != nil {
		return err
	}
	if helper, ok := fastHelperForContext(ctx, call.Name); ok {
		var argStore fastCallArgs
		args, err := evalFastCallArgsInto(call.Args, ctx, bindings, &argStore)
		if err != nil {
			return err
		}
		if handled, err := writeRegisteredFastHelperNamed(out, ctx, call.Name, helper, args); handled || err != nil {
			if err != nil {
				return fastLineError(call.Line, err)
			}
			return nil
		}
	}
	if handled, err := writeFastDirectStringCallSegment(out, ctx, bindings, call, raw); handled || err != nil {
		return err
	}
	var argStore fastCallArgs
	args, err := evalFastCallArgsInto(call.Args, ctx, bindings, &argStore)
	if err != nil {
		return err
	}
	if err := writeFastCallValue(out, ctx, call.Name, raw, args, &call.Cache); err != nil {
		return fastLineError(call.Line, err)
	}
	return nil
}

func writeFastBlockCallSegment(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, call *compiler.FastBlockCallPlan) error {
	if call == nil {
		return nil
	}
	raw, ok := bindings.value(call.NameIndex)
	if !ok {
		return fastLineError(call.Line, fmt.Errorf("%q: unknown identifier", call.Name))
	}
	if err := spendFastFunctionCall(ctx, call.Name, call.Line); err != nil {
		return err
	}
	var argStore fastCallArgs
	args, err := evalFastCallArgsInto(call.Args, ctx, bindings, &argStore)
	if err != nil {
		return err
	}
	helperCtx := plush.NewHelperContext(ctx, func(blockCtx hctx.Context) (string, error) {
		return renderFastBlockCallBytecode(call, fastBlockContext(blockCtx, bindings))
	})
	if err := writeFastBlockCallValue(out, ctx, call.Name, raw, args, helperCtx, &call.Cache); err != nil {
		return fastLineError(call.Line, err)
	}
	return nil
}

func writeFastLoopBlockCallPart(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, call *compiler.FastBlockCallPlan, loopKey, loopValue interface{}) error {
	if call == nil {
		return nil
	}
	raw, ok := bindings.value(call.NameIndex)
	if !ok {
		return fastLineError(call.Line, fmt.Errorf("%q: unknown identifier", call.Name))
	}
	if err := spendFastFunctionCall(ctx, call.Name, call.Line); err != nil {
		return err
	}
	var argStore fastCallArgs
	args, err := evalFastLoopCallArgsInto(call.Args, ctx, bindings, loopKey, loopValue, &argStore)
	if err != nil {
		return err
	}
	helperCtx := plush.NewHelperContext(ctx, func(blockCtx hctx.Context) (string, error) {
		return renderFastBlockCallBytecode(call, fastLoopBlockContext(blockCtx, bindings, loop, loopKey, loopValue))
	})
	if err := writeFastBlockCallValue(out, ctx, call.Name, raw, args, helperCtx, &call.Cache); err != nil {
		return fastLineError(call.Line, err)
	}
	return nil
}

func renderFastBlockCallBytecode(call *compiler.FastBlockCallPlan, ctx hctx.Context) (string, error) {
	if call == nil || call.BlockBytecode == nil {
		return "", nil
	}
	bytecode := call.BlockBytecode
	if bytecode.Static {
		return bytecode.StaticOutput, nil
	}
	if bytecode.FastRenderPlan != nil {
		if rendered, ok, err := renderFastPlanWithBindingPlan(bytecode.FastRenderPlan, ctx, topLevelFastBindingPlan(bytecode.FastRenderPlan, ctx)); ok || err != nil {
			return rendered, err
		}
	}
	if restorePartial := installVMPartialHelperForBytecode(bytecode, ctx); restorePartial != nil {
		defer restorePartial()
	}
	return renderBytecodeVMWithState(bytecode, ctx, "", false, "")
}

func fastBlockContext(ctx hctx.Context, bindings fastRenderBindings) hctx.Context {
	if ctx == nil {
		ctx = plush.NewContext()
	}
	scoped := ctx.New()
	for i, ok := range bindings.localOK {
		if !ok || i >= len(bindings.names) || i >= len(bindings.localVals) {
			continue
		}
		scoped.Set(bindings.names[i], bindings.localVals[i])
	}
	return scoped
}

func fastLoopBlockContext(ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, key, value interface{}) hctx.Context {
	scoped := fastBlockContext(ctx, bindings)
	if loop == nil || (!fastLoopBindingName(loop.KeyName) && !fastLoopBindingName(loop.ValueName)) {
		return scoped
	}
	scoped = scoped.New()
	if fastLoopBindingName(loop.KeyName) {
		scoped.Set(loop.KeyName, key)
	}
	if fastLoopBindingName(loop.ValueName) {
		scoped.Set(loop.ValueName, value)
	}
	return scoped
}

func writeFastBlockCallValue(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs, helperCtx plush.HelperContext, cacheSlot *object.InlineCacheSlot) error {
	if obj, ok := raw.(object.Object); ok {
		raw = object.ToGo(obj)
	}
	rv := reflect.ValueOf(raw)
	if !rv.IsValid() {
		return fmt.Errorf("%T is an invalid function", raw)
	}
	if rv.Kind() != reflect.Func {
		return fmt.Errorf("%+v (%T) is an invalid function", raw, raw)
	}
	entry := cachedFastCallEntry(rv.Type(), raw, cacheSlot)
	if entry == nil || entry.plan == nil {
		return fmt.Errorf("%+v (%T) is an invalid function", raw, raw)
	}
	var scratch [4]reflect.Value
	reflectArgs, err := fastReflectArgsIntoWithHelperContext(name, entry.plan, args, ctx, helperCtx, scratch[:0])
	if err != nil {
		return err
	}
	start := time.Now()
	res := rv.Call(reflectArgs)
	plush.AddRenderDiagnosticVMHelperTiming(ctx, name, time.Since(start))
	return writeFastReflectCallResults(out, ctx, name, res)
}

func fastReflectArgsIntoWithHelperContext(name string, plan *callPlan, rawArgs *fastCallArgs, ctx hctx.Context, helperCtx plush.HelperContext, scratch []reflect.Value) ([]reflect.Value, error) {
	numArgs := rawArgs.Len()
	needed := plan.numIn
	if plan.isVariadic && numArgs > needed {
		needed = numArgs
	}
	args := scratch
	if cap(args) < needed {
		args = make([]reflect.Value, 0, needed)
	} else {
		args = args[:0]
	}

	if !plan.isVariadic && numArgs > plan.numIn {
		return nil, fmt.Errorf("too many arguments (%d for %d)", numArgs, plan.numIn)
	}
	if plan.isVariadic && numArgs < plan.minArgs {
		return nil, fmt.Errorf("too few arguments (%d for %d)", numArgs, plan.numIn)
	}

	fixed := numArgs
	if plan.isVariadic {
		fixed = plan.minArgs
	}
	for pos := 0; pos < fixed; pos++ {
		arg, err := fastReflectArgForCall(name, pos, rawArgs.Raw(pos), plan.argTypes[pos])
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	if plan.isVariadic {
		for pos := fixed; pos < numArgs; pos++ {
			arg, err := fastReflectArgForCall(name, pos, rawArgs.Raw(pos), plan.variadicElem)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
	} else if len(args) < plan.numIn {
		for len(args) < plan.numIn {
			arg, ok := fastOptionalArgWithHelperContext(plan.optionalArgs[len(args)], plan.argTypes[len(args)], ctx, helperCtx)
			if !ok {
				break
			}
			args = append(args, arg)
		}
	}
	if len(args) < plan.minArgs {
		return nil, errFastWriteUnsupported
	}
	return args, nil
}

func fastOptionalArgWithHelperContext(kind optionalArgKind, expected reflect.Type, ctx hctx.Context, helperCtx plush.HelperContext) (reflect.Value, bool) {
	if kind == optionalArgHelperContext {
		value := reflect.ValueOf(helperCtx)
		if value.Type().AssignableTo(expected) {
			return value, true
		}
		if value.Type().ConvertibleTo(expected) {
			return value.Convert(expected), true
		}
		if expected.Kind() == reflect.Interface && value.Type().Implements(expected) {
			return value, true
		}
		return reflect.Value{}, false
	}
	return fastOptionalArg(kind, expected, ctx)
}

func writeFastDirectStringCallSegment(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, call *compiler.FastCallPlan, raw interface{}) (bool, error) {
	if out == nil || call == nil || len(call.Args) != 1 {
		return false, nil
	}
	if obj, ok := raw.(object.Object); ok {
		raw = object.ToGo(obj)
	}
	arg, ok, err := evalFastCallStringArg(&call.Args[0], ctx, bindings)
	if err != nil {
		return true, err
	}
	if !ok {
		return false, nil
	}
	switch fn := raw.(type) {
	case func(string) string:
		writeFastEscapedString(out, fn(arg))
		return true, nil
	case func(string) (string, error):
		value, err := fn(arg)
		if err != nil {
			return true, fastLineError(call.Line, fmt.Errorf("could not call %s function: %w", call.Name, err))
		}
		writeFastEscapedString(out, value)
		return true, nil
	case func(string) template.HTML:
		out.WriteString(string(fn(arg)))
		return true, nil
	case func(string) (template.HTML, error):
		value, err := fn(arg)
		if err != nil {
			return true, fastLineError(call.Line, fmt.Errorf("could not call %s function: %w", call.Name, err))
		}
		out.WriteString(string(value))
		return true, nil
	case func(string) object.Object:
		writeFastObject(out, ctx, fn(arg))
		return true, nil
	}
	return false, nil
}

func evalFastCallStringArg(plan *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings) (string, bool, error) {
	value, ok, err := evalFastValue(plan, ctx, bindings, nil)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, fastLineError(plan.Line, fmt.Errorf("%q: unknown identifier", plan.Value))
	}
	arg, ok := fastWriteRawStringArg(value)
	return arg, ok, nil
}

func evalFastCallArgsInto(plans []compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, args *fastCallArgs) (*fastCallArgs, error) {
	if len(plans) == 0 {
		return nil, nil
	}
	if args == nil {
		args = &fastCallArgs{}
	}
	args.Reset()
	for i := range plans {
		value, ok, err := evalFastValue(&plans[i], ctx, bindings, nil)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fastLineError(plans[i].Line, fmt.Errorf("%q: unknown identifier", plans[i].Value))
		}
		args.Append(value)
	}
	return args, nil
}
