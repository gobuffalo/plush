package vm

import (
	"reflect"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/vm/compiler"
)

// A plan holds an unbound method, never a receiver or a method result.
type fastZeroArgMethodPlan struct {
	typ     reflect.Type
	method  reflect.Value
	pointer bool
}

func evalFastZeroArgMethod(base interface{}, property, call *compiler.FastPathStep, ctx hctx.Context, bindings fastRenderBindings) (interface{}, bool, error) {
	// Preserve registered helper overrides and their diagnostics through the
	// general call path. The specialization handles only one-result methods.
	if bindings.vmHotspots.Enabled() {
		return nil, false, nil
	}
	if _, registered := fastValueHelperRegistrationForContext(ctx, call.Value); registered {
		return nil, false, nil
	}
	rv, _, ok, err := fastPropertyReflectValue(base)
	if err != nil || !ok {
		return nil, false, nil
	}
	// PropertyCache is otherwise unused on call steps. Keep this separate
	// from CallCache, which belongs to the general function-call machinery.
	plan, cached := call.PropertyCache.Load().(*fastZeroArgMethodPlan)
	if !cached || plan.typ != rv.Type() {
		plan = &fastZeroArgMethodPlan{typ: rv.Type()}
		lookup := cachedPropertyLookup(rv.Type(), property.Value)
		methodType := rv.Type()
		if lookup.kind == propertyLookupPointerMethod {
			plan.pointer = true
			methodType = reflect.PointerTo(methodType)
		}
		if lookup.kind == propertyLookupValueMethod || lookup.kind == propertyLookupPointerMethod {
			method := methodType.Method(lookup.index)
			if !method.Type.IsVariadic() && method.Type.NumIn() == 1 && method.Type.NumOut() == 1 &&
				!method.Type.Out(0).Implements(errorType) {
				plan.method = method.Func
			}
		}
		call.PropertyCache.Store(plan)
	}
	if !plan.method.IsValid() {
		return nil, false, nil
	}
	if err := spendFastTraversal(ctx, property.Line); err != nil {
		return nil, true, err
	}
	if plan.pointer {
		if rv.CanAddr() {
			rv = rv.Addr()
		} else {
			ptr := reflect.New(rv.Type())
			ptr.Elem().Set(rv)
			rv = ptr
		}
	}
	if err := spendFastFunctionCall(ctx, call.Value, call.Line); err != nil {
		return nil, true, err
	}
	args := [1]reflect.Value{rv}
	result := plan.method.Call(args[:])[0]
	if isNilReflectValue(result) {
		return nil, true, nil
	}
	return result.Interface(), true, nil
}
