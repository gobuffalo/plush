package vm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

func writeFastCallValue(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs, cacheSlot *object.InlineCacheSlot) error {
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
	return writeFastCallValueWithEntry(out, ctx, name, raw, args, entry)
}

func writeFastCallValueWithEntry(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs, entry *fastBuilderCallCacheEntry) error {
	if entry == nil {
		return fmt.Errorf("%+v (%T) is an invalid function", raw, raw)
	}
	if entry.invoker != nil {
		start := time.Now()
		if err := entry.invoker(out, ctx, name, raw, args); err != nil {
			if !errors.Is(err, errFastWriteUnsupported) {
				return err
			}
		} else {
			plush.AddRenderDiagnosticVMHelperTiming(ctx, name, time.Since(start))
			return nil
		}
	}
	value, err := fastCallValueWithEntry(name, raw, args, ctx, entry)
	if err != nil {
		return err
	}
	writeFastGoValue(out, ctx, value)
	return nil
}

func fastCallValue(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context, cacheSlot *object.InlineCacheSlot) (interface{}, error) {
	if obj, ok := raw.(object.Object); ok {
		raw = object.ToGo(obj)
	}
	rv := reflect.ValueOf(raw)
	if !rv.IsValid() {
		return nil, fmt.Errorf("%T is an invalid function", raw)
	}
	if rv.Kind() != reflect.Func {
		return nil, fmt.Errorf("%+v (%T) is an invalid function", raw, raw)
	}
	entry := cachedFastCallEntry(rv.Type(), raw, cacheSlot)
	return fastCallValueWithEntry(name, raw, args, ctx, entry)
}

func fastCallValueWithEntry(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context, entry *fastBuilderCallCacheEntry) (interface{}, error) {
	rv := reflect.ValueOf(raw)
	if entry == nil || entry.plan == nil {
		return nil, fmt.Errorf("%+v (%T) is an invalid function", raw, raw)
	}
	helperStart := time.Now()
	if value, handled, err := fastHelperContextCallValue(name, raw, args, ctx); handled || err != nil {
		if handled {
			plush.AddRenderDiagnosticVMHelperTiming(ctx, name, time.Since(helperStart))
		}
		return value, err
	}
	if entry.contextualValueInvoker != nil {
		start := time.Now()
		if value, err := entry.contextualValueInvoker(name, raw, args, ctx); err == nil {
			plush.AddRenderDiagnosticVMHelperTiming(ctx, name, time.Since(start))
			return value, nil
		} else if !errors.Is(err, errFastWriteUnsupported) {
			return nil, err
		}
	}
	if entry.valueInvoker != nil {
		start := time.Now()
		if value, err := entry.valueInvoker(name, raw, args); err == nil {
			plush.AddRenderDiagnosticVMHelperTiming(ctx, name, time.Since(start))
			return value, nil
		} else if !errors.Is(err, errFastWriteUnsupported) {
			return nil, err
		}
	}
	start := time.Now()
	var scratch [4]reflect.Value
	reflectArgs, err := fastReflectArgsInto(name, entry.plan, args, ctx, scratch[:0])
	if err != nil {
		return nil, err
	}
	res := rv.Call(reflectArgs)
	plush.AddRenderDiagnosticVMHelperTiming(ctx, name, time.Since(start))
	if len(res) == 0 {
		return nil, nil
	}
	if err := lastReturnError(res); err != nil {
		return nil, fmt.Errorf("could not call %s function: %w", name, err)
	}
	if isNilReflectValue(res[0]) {
		return nil, nil
	}
	return res[0].Interface(), nil
}

func fastHelperContextCallValue(name string, raw interface{}, args *fastCallArgs, ctx hctx.Context) (interface{}, bool, error) {
	if args.Len() != 0 {
		return nil, false, nil
	}
	helperCtx := plush.NewHelperContext(ctx, nil)
	switch fn := raw.(type) {
	case func(plush.HelperContext) string:
		return fn(helperCtx), true, nil
	case func(hctx.HelperContext) string:
		return fn(helperCtx), true, nil
	case func(plush.HelperContext) (string, error):
		value, err := fn(helperCtx)
		if err != nil {
			return nil, true, fmt.Errorf("could not call %s function: %w", name, err)
		}
		return value, true, nil
	case func(hctx.HelperContext) (string, error):
		value, err := fn(helperCtx)
		if err != nil {
			return nil, true, fmt.Errorf("could not call %s function: %w", name, err)
		}
		return value, true, nil
	default:
		return nil, false, nil
	}
}

func cachedFastCallEntry(rt reflect.Type, raw interface{}, slot *object.InlineCacheSlot) *fastBuilderCallCacheEntry {
	if slot != nil {
		if cached, ok := slot.Load().(*fastBuilderCallCacheEntry); ok && cached != nil && cached.rt == rt {
			return cached
		}
	}
	entry := &fastBuilderCallCacheEntry{
		rt:                     rt,
		plan:                   cachedCallPlan(rt),
		invoker:                writeFastBuilderInvokerForRaw(raw),
		valueInvoker:           valueFastInvokerForRaw(raw),
		contextualValueInvoker: contextualValueFastInvokerForRaw(raw),
	}
	if slot != nil {
		slot.Store(entry)
	}
	return entry
}

func fastReflectArgsInto(name string, plan *callPlan, rawArgs *fastCallArgs, ctx hctx.Context, scratch []reflect.Value) ([]reflect.Value, error) {
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
			arg, ok := fastOptionalArg(plan.optionalArgs[len(args)], plan.argTypes[len(args)], ctx)
			if !ok {
				break
			}
			args = append(args, arg)
		}
	}
	if len(args) < plan.minArgs {
		return nil, fmt.Errorf("too few arguments (%d for %d)", len(args), plan.numIn)
	}
	return args, nil
}

func fastReflectArgForCall(name string, pos int, raw interface{}, expected reflect.Type) (reflect.Value, error) {
	if obj, ok := raw.(object.Object); ok {
		if object.IsNull(obj) {
			return reflect.New(expected).Elem(), nil
		}
		if value, ok := fastReflectArg(obj, expected); ok {
			return value, nil
		}
		raw = object.ToGo(obj)
	}
	if raw == nil {
		return reflect.New(expected).Elem(), nil
	}
	if value, ok := fastConvertibleValue(raw, expected); ok {
		return value, nil
	}
	return reflect.Value{}, fmt.Errorf("%+v (%T) is an invalid argument for %s at pos %d: expected (%s)", raw, raw, name, pos, expected)
}

func fastOptionalArg(kind optionalArgKind, expected reflect.Type, ctx hctx.Context) (reflect.Value, bool) {
	switch kind {
	case optionalArgHelperContext:
		hargs := plush.NewHelperContext(ctx, nil)
		value := reflect.ValueOf(hargs)
		if value.Type().AssignableTo(expected) {
			return value, true
		}
		if value.Type().ConvertibleTo(expected) {
			return value.Convert(expected), true
		}
	case optionalArgMap:
		value := reflect.ValueOf(map[string]interface{}{})
		if value.Type().AssignableTo(expected) {
			return value, true
		}
	}
	return reflect.Value{}, false
}

func isNilReflectValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (vm *VM) reflectArgs(name string, plan *callPlan, numArgs int, block *object.Closure, scratch []reflect.Value) ([]reflect.Value, error) {
	rawArgs := vm.stack[vm.sp-numArgs : vm.sp]
	needed := plan.numIn
	if plan.isVariadic && numArgs > needed {
		needed = numArgs
	}
	if cap(scratch) < needed {
		scratch = make([]reflect.Value, 0, needed)
	}
	args := scratch[:0]

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
		arg, err := vm.reflectArg(name, pos, rawArgs[pos], plan.argTypes[pos])
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}

	if plan.isVariadic {
		for pos := fixed; pos < numArgs; pos++ {
			arg, err := vm.reflectArg(name, pos, rawArgs[pos], plan.variadicElem)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
	} else if len(args) < plan.numIn {
		for len(args) < plan.numIn {
			arg, ok := vm.optionalArg(plan.optionalArgs[len(args)], plan.argTypes[len(args)], block)
			if !ok {
				break
			}
			args = append(args, arg)
		}
	}

	if len(args) < plan.minArgs {
		return nil, fmt.Errorf("too few arguments (%d for %d)", len(args), plan.numIn)
	}

	return args, nil
}

func cachedCallPlanForSlot(rt reflect.Type, slot *object.InlineCacheSlot) *callPlan {
	if slot != nil {
		switch cached := slot.Load().(type) {
		case *callCacheEntry:
			if cached != nil && cached.rt == rt {
				return cached.plan
			}
		case *callPlan:
			if cached != nil && cached.rt == rt {
				return cached
			}
		}
	}
	plan := cachedCallPlan(rt)
	if slot != nil {
		slot.Store(&callCacheEntry{rt: rt, plan: plan})
	}
	return plan
}

func cachedCallEntryForSlot(rt reflect.Type, raw interface{}, slot *object.InlineCacheSlot) *callCacheEntry {
	if slot != nil {
		switch cached := slot.Load().(type) {
		case *callCacheEntry:
			if cached != nil && cached.rt == rt {
				if raw != nil && cached.invoker == nil && !cached.noFast {
					entry := newCallCacheEntry(cached.plan, raw)
					slot.Store(entry)
					return entry
				}
				return cached
			}
		case *callPlan:
			if cached != nil && cached.rt == rt {
				return newCallCacheEntry(cached, raw)
			}
		}
	}

	entry := newCallCacheEntry(cachedCallPlan(rt), raw)
	if slot != nil {
		slot.Store(entry)
	}
	return entry
}

func newCallCacheEntry(plan *callPlan, raw interface{}) *callCacheEntry {
	entry := &callCacheEntry{rt: plan.rt, plan: plan}
	if raw != nil {
		entry.invoker = writeFastInvokerForRaw(raw)
		entry.noFast = entry.invoker == nil
	}
	return entry
}

func cachedCallPlan(rt reflect.Type) *callPlan {
	if plan, ok := callPlanCache.Load(rt); ok {
		return plan.(*callPlan)
	}

	plan := &callPlan{
		rt:           rt,
		numIn:        rt.NumIn(),
		isVariadic:   rt.IsVariadic(),
		argTypes:     make([]reflect.Type, rt.NumIn()),
		optionalArgs: make([]optionalArgKind, rt.NumIn()),
	}
	plan.minArgs = plan.numIn
	if plan.isVariadic {
		plan.minArgs = plan.numIn - 1
		plan.variadicElem = rt.In(plan.numIn - 1).Elem()
	}
	for i := 0; i < plan.numIn; i++ {
		argType := rt.In(i)
		plan.argTypes[i] = argType
		plan.optionalArgs[i] = optionalArgKindFor(argType)
	}
	plan.returnKind = callReturnKindFor(rt)

	actual, _ := callPlanCache.LoadOrStore(rt, plan)
	return actual.(*callPlan)
}

func callReturnKindFor(rt reflect.Type) callReturnKind {
	if rt.NumOut() == 0 {
		return callReturnNone
	}
	out := rt.Out(0)
	switch out {
	case stringType:
		return callReturnString
	case templateHTMLType:
		return callReturnHTML
	}
	if out.Implements(objectInterfaceType) {
		return callReturnObject
	}
	if out.PkgPath() != "" {
		return callReturnGeneric
	}
	switch out.Kind() {
	case reflect.Bool:
		return callReturnBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return callReturnInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return callReturnUint
	case reflect.Float32, reflect.Float64:
		return callReturnFloat
	default:
		return callReturnGeneric
	}
}

func (vm *VM) reflectArg(name string, pos int, obj object.Object, expected reflect.Type) (reflect.Value, error) {
	if object.IsNull(obj) {
		return reflect.New(expected).Elem(), nil
	}
	if value, ok := fastReflectArg(obj, expected); ok {
		return value, nil
	}

	value := object.ToGo(obj)
	arg := reflect.ValueOf(value)
	if arg.Type().AssignableTo(expected) {
		return arg, nil
	}
	if arg.Type().ConvertibleTo(expected) {
		return arg.Convert(expected), nil
	}
	return reflect.Value{}, fmt.Errorf("%+v (%T) is an invalid argument for %s at pos %d: expected (%s)", value, value, name, pos, expected)
}

func fastReflectArg(obj object.Object, expected reflect.Type) (reflect.Value, bool) {
	switch obj := obj.(type) {
	case *object.String:
		return fastConvertibleValue(obj.Value, expected)
	case *object.Boolean:
		return fastConvertibleValue(obj.Value, expected)
	case *object.Integer:
		return fastConvertibleValue(int(obj.Value), expected)
	case *object.Float:
		return fastConvertibleValue(obj.Value, expected)
	case *object.Native:
		if obj.Value == nil {
			return reflect.New(expected).Elem(), true
		}
		return fastConvertibleValue(obj.Value, expected)
	}
	return reflect.Value{}, false
}

func fastConvertibleValue(value interface{}, expected reflect.Type) (reflect.Value, bool) {
	arg := reflect.ValueOf(value)
	if !arg.IsValid() {
		return reflect.New(expected).Elem(), true
	}
	if arg.Type().AssignableTo(expected) {
		return arg, true
	}
	if arg.Type().ConvertibleTo(expected) {
		return arg.Convert(expected), true
	}
	if expected.Kind() == reflect.Ptr {
		elem := expected.Elem()
		if arg.Type().AssignableTo(elem) {
			ptr := reflect.New(elem)
			ptr.Elem().Set(arg)
			return ptr, true
		}
		if arg.Type().ConvertibleTo(elem) {
			ptr := reflect.New(elem)
			ptr.Elem().Set(arg.Convert(elem))
			return ptr, true
		}
	}
	return reflect.Value{}, false
}

func optionalArgKindFor(expected reflect.Type) optionalArgKind {
	if plushHelperContextType.AssignableTo(expected) ||
		plushHelperContextType.ConvertibleTo(expected) ||
		expected.Implements(hctxHelperContextInterface) {
		return optionalArgHelperContext
	}
	if emptyMapType.AssignableTo(expected) || emptyMapType.ConvertibleTo(expected) {
		return optionalArgMap
	}
	return optionalArgNone
}

func (vm *VM) optionalArg(kind optionalArgKind, expected reflect.Type, block *object.Closure) (reflect.Value, bool) {
	switch kind {
	case optionalArgHelperContext:
		hargs := vm.helperContext(block)
		value := reflect.ValueOf(hargs)
		if value.Type().AssignableTo(expected) {
			return value, true
		}
		if value.Type().ConvertibleTo(expected) {
			return value.Convert(expected), true
		}
	case optionalArgMap:
		value := reflect.ValueOf(map[string]interface{}{})
		if value.Type().AssignableTo(expected) {
			return value, true
		}
	}

	return reflect.Value{}, false
}

func (vm *VM) helperContext(block *object.Closure) plush.HelperContext {
	var runner func(hctx.Context) (string, error)
	if block != nil {
		runner = func(ctx hctx.Context) (string, error) {
			return vm.runBlock(block, ctx)
		}
	}
	return plush.NewHelperContext(vm.contextWithFrameLocals(), runner)
}

func (vm *VM) contextWithFrameLocals() hctx.Context {
	ctx := vm.ctx
	frame := vm.currentFrame()
	if ctx == nil || frame == nil || frame.cl == nil || frame.cl.Fn == nil || len(frame.cl.Fn.LocalNames) == 0 {
		return ctx
	}

	scoped := ctx.New()
	for index, name := range frame.cl.Fn.LocalNames {
		stackIndex := frame.basePointer + index
		if stackIndex < 0 || stackIndex >= len(vm.stack) {
			continue
		}
		value := vm.stack[stackIndex]
		if value == nil {
			continue
		}
		scoped.Set(name, object.ToGo(value))
	}
	return scoped
}

func (vm *VM) runBlock(block *object.Closure, ctx hctx.Context) (string, error) {
	if ctx == nil {
		ctx = vm.ctx
	}
	blockCtx := ctx.New()
	child := vm.childVM(block, nil, blockCtx)
	defer child.Release()
	child.deferHolePositions = true
	if err := child.Run(); err != nil {
		return "", child.wrapRuntimeError(err)
	}
	if !object.IsNull(child.lastPopped) && child.lastPopped != nil {
		return child.renderObject(child.lastPopped), nil
	}
	return child.Rendered(), nil
}

func (vm *VM) childVM(cl *object.Closure, args []object.Object, ctx hctx.Context) *VM {
	frame := newFrame(cl, 0, true)
	frames := borrowFrames(true)
	frames[0] = frame
	child := &VM{
		constants:   vm.constants,
		stack:       borrowStack(true),
		globals:     vm.globals,
		globalNames: vm.globalNames,
		frames:      frames,
		framesIndex: 1,
		ctx:         ctx,
		holes:       vm.holes,
		pooled:      true,
	}
	child.resetChild(cl, args, ctx)
	return child
}

func (vm *VM) resetChild(cl *object.Closure, args []object.Object, ctx hctx.Context) {
	if vm.stackMax > len(vm.stack) {
		vm.stackMax = len(vm.stack)
	}
	if vm.stackMax > 0 {
		clearObjectSlice(vm.stack[:vm.stackMax])
	}
	vm.stackMax = 0
	for i := 1; i < vm.framesIndex && i < len(vm.frames); i++ {
		releaseFrame(vm.frames[i])
		vm.frames[i] = nil
	}
	vm.framesIndex = 1
	if vm.frames[0] == nil {
		vm.frames[0] = newFrame(cl, 0, true)
	} else {
		vm.frames[0].reset(cl, 0)
	}
	if vm.ctx != ctx {
		vm.clearNameIDCache()
	}
	vm.ctx = ctx
	vm.lastPopped = nil
	vm.lastIP = 0
	vm.halted = false
	copy(vm.stack, args)
	vm.sp = cl.Fn.NumLocals
	if vm.sp < len(args) {
		vm.sp = len(args)
	}
	vm.markStack()
}

func lastReturnError(res []reflect.Value) error {
	last := res[len(res)-1]
	if !last.IsValid() {
		return nil
	}
	if !last.Type().Implements(errorType) {
		return nil
	}
	switch last.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if last.IsNil() {
			return nil
		}
	}
	return last.Interface().(error)
}

func (vm *VM) pushClosure(constIndex, numFree int) error {
	cl, err := vm.closureFromConstant(constIndex)
	if err != nil {
		return err
	}

	free := make([]object.Object, numFree)
	for i := 0; i < numFree; i++ {
		free[i] = vm.stack[vm.sp-numFree+i]
	}
	vm.sp = vm.sp - numFree
	cl.Free = free
	return vm.push(cl)
}

func (vm *VM) closureFromConstant(constIndex int) (*object.Closure, error) {
	constant := vm.constants[constIndex]
	function, ok := constant.(*object.CompiledFunction)
	if !ok {
		return nil, fmt.Errorf("not a function: %+v", constant)
	}
	return &object.Closure{Fn: function}, nil
}

func (vm *VM) closureFromStack(constIndex, numFree int) (*object.Closure, error) {
	block, err := vm.closureFromConstant(constIndex)
	if err != nil {
		return nil, err
	}

	free := make([]object.Object, numFree)
	for i := 0; i < numFree; i++ {
		free[i] = vm.stack[vm.sp-numFree+i]
	}
	vm.sp -= numFree
	block.Free = free
	return block, nil
}
