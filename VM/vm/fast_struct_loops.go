package vm

import (
	"errors"
	"fmt"
	"html/template"
	"reflect"
	"strconv"
	"strings"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

func renderFastStringKeyValueLoop(out *strings.Builder, ctx hctx.Context, loop *compiler.FastLoopPlan, iter []string) (bool, error) {
	separator, suffix, ok := fastStringKeyValueLoopParts(loop)
	if !ok {
		return false, nil
	}
	for i, value := range iter {
		if err := spendFastLoop(ctx, loop.Line); err != nil {
			return true, err
		}
		writeBuilderFastInt(out, int64(i))
		out.WriteString(separator)
		writeFastEscapedString(out, value)
		out.WriteString(suffix)
	}
	return true, nil
}

func fastStringKeyValueLoopParts(loop *compiler.FastLoopPlan) (string, string, bool) {
	if loop == nil || len(loop.Parts) != 4 {
		return "", "", false
	}
	if loop.Parts[0].Kind != compiler.FastLoopPartKey ||
		loop.Parts[1].Kind != compiler.FastLoopPartStatic ||
		loop.Parts[2].Kind != compiler.FastLoopPartValue ||
		loop.Parts[3].Kind != compiler.FastLoopPartStatic {
		return "", "", false
	}
	return loop.Parts[1].Value, loop.Parts[3].Value, true
}

func renderFastStructFieldLoop(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, iter reflect.Value) (bool, error) {
	elemType, ok := fastStructLoopElementType(iter.Type())
	if !ok {
		return false, nil
	}
	plan, ok := fastStructLoopWriterPlanFor(loop, elemType)
	if !ok {
		return false, nil
	}

	state := &fastStructLoopRenderState{}
	for i := 0; i < iter.Len(); i++ {
		if err := spendFastLoop(ctx, loop.Line); err != nil {
			return true, err
		}

		if err := renderFastStructLoopWriterOps(out, ctx, bindings, state, loop, plan.ops, i, iter.Index(i)); err != nil {
			return true, err
		}
	}
	return true, nil
}

func renderFastStructLoopWriterOps(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, state *fastStructLoopRenderState, loop *compiler.FastLoopPlan, ops []fastStructLoopWriterOp, key interface{}, item reflect.Value) error {
	for opIndex := range ops {
		op := &ops[opIndex]
		switch op.kind {
		case fastStructLoopWriterStatic:
			out.WriteString(op.value)
		case fastStructLoopWriterKey:
			writeFastGoValue(out, ctx, key)
		case fastStructLoopWriterField:
			if err := spendFastTraversal(ctx, op.line); err != nil {
				return err
			}
			rv := unwrapFastFieldChainValue(item)
			if !rv.IsValid() {
				continue
			}
			field := rv.FieldByIndex(op.fieldIndex)
			access := object.PropertyAccess{
				Receiver: op.receiver,
				Full:     op.full,
			}
			written, err := writeFastField(out, ctx, field, access, op.name, op.fieldType)
			if err != nil {
				return fastLineError(op.line, err)
			}
			if written {
				continue
			}
			value, _ := fastFieldValue(field, access, op.name)
			writeFastGoValue(out, ctx, value)
		case fastStructLoopWriterAccessChain:
			handled, err := writeFastAccessChainPlanOutput(out, ctx, op.accessPlan, item)
			if err != nil {
				return err
			}
			_ = handled
		case fastStructLoopWriterMethodCall:
			if err := writeFastLoopMethodCall(out, ctx, op.methodPlan, item); err != nil {
				return err
			}
		case fastStructLoopWriterCall:
			if err := writeFastStructLoopCallPart(out, ctx, bindings, state, op.call, key, item); err != nil {
				return err
			}
		case fastStructLoopWriterConditional:
			if err := renderFastStructLoopConditional(out, ctx, bindings, state, loop, op.conditional, key, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderFastStructLoopConditional(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, state *fastStructLoopRenderState, loop *compiler.FastLoopPlan, conditional *fastStructLoopConditionalWriterPlan, key interface{}, item reflect.Value) error {
	if conditional == nil {
		return nil
	}
	for i := range conditional.branches {
		branch := &conditional.branches[i]
		if err := spendFastCondition(ctx, branch.line); err != nil {
			return err
		}
		truthy, ok, err := isTruthyFastStructLoopCondition(branch, ctx, bindings, key, item)
		if err != nil {
			return err
		}
		if ok && truthy {
			return renderFastStructLoopWriterOps(out, ctx, bindings, state, loop, branch.ops, key, item)
		}
	}
	if len(conditional.elseOps) > 0 {
		return renderFastStructLoopWriterOps(out, ctx, bindings, state, loop, conditional.elseOps, key, item)
	}
	return nil
}

func isTruthyFastStructLoopCondition(branch *fastStructLoopConditionalWriterBranch, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (bool, bool, error) {
	if branch == nil {
		return false, false, nil
	}
	if branch.conditionPlan != nil {
		return evalFastStructLoopConditionPlan(branch.conditionPlan, ctx, bindings, loopKey, item)
	}
	return isTruthyFastStructLoopValue(&branch.condition, ctx, bindings, loopKey, item)
}

func writeFastStructLoopCallPart(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, state *fastStructLoopRenderState, plan *fastStructLoopCallPlan, loopKey interface{}, item reflect.Value) error {
	if plan == nil || plan.call == nil {
		return nil
	}
	call := plan.call
	if err := spendFastFunctionCall(ctx, call.Name, call.Line); err != nil {
		return err
	}

	resolved, err := state.resolvedCall(plan, bindings)
	if err != nil {
		return fastLineError(call.Line, err)
	}

	var args fastCallArgs
	argsReady := false
	if helper, ok := fastHelperForContext(ctx, call.Name); ok {
		if len(plan.args) > 0 {
			if err := evalFastStructLoopCallPlanArgs(plan, ctx, bindings, loopKey, item, &args); err != nil {
				return err
			}
			argsReady = true
		}
		if handled, err := writeRegisteredFastHelperNamed(out, ctx, call.Name, helper, fastCallArgsOrNil(&args, len(plan.args))); handled || err != nil {
			if err != nil {
				return fastLineError(call.Line, err)
			}
			return nil
		}
	}
	if resolved.directWriter != nil {
		if handled, err := resolved.directWriter(out, ctx, bindings, plan, loopKey, item); err != nil {
			return fastLineError(call.Line, err)
		} else if handled {
			return nil
		}
	}
	if resolved.entry != nil && resolved.entry.invoker != nil {
		if len(plan.args) > 0 && !argsReady {
			if err := evalFastStructLoopCallPlanArgs(plan, ctx, bindings, loopKey, item, &args); err != nil {
				return err
			}
			argsReady = true
		}
		if err := resolved.entry.invoker(out, ctx, call.Name, resolved.raw, fastCallArgsOrNil(&args, len(plan.args))); err != nil {
			if !errors.Is(err, errFastWriteUnsupported) {
				return fastLineError(call.Line, err)
			}
		} else {
			return nil
		}
	}

	if resolved.canReflect {
		if err := writeFastStructLoopReflectCall(out, ctx, bindings, plan, resolved, loopKey, item); err != nil {
			return fastLineError(call.Line, err)
		}
		return nil
	}

	if len(plan.args) > 0 && !argsReady {
		if err := evalFastStructLoopCallPlanArgs(plan, ctx, bindings, loopKey, item, &args); err != nil {
			return err
		}
		argsReady = true
	}
	if err := writeFastCallValueWithEntry(out, ctx, call.Name, resolved.raw, fastCallArgsOrNil(&args, len(plan.args)), resolved.entry); err != nil {
		return fastLineError(call.Line, err)
	}
	return nil
}

func fastCallArgsOrNil(args *fastCallArgs, count int) *fastCallArgs {
	if count == 0 {
		return nil
	}
	return args
}

func (s *fastStructLoopRenderState) resolvedCall(plan *fastStructLoopCallPlan, bindings fastRenderBindings) (*fastStructLoopResolvedCall, error) {
	if plan == nil || plan.call == nil {
		return nil, nil
	}
	if s == nil {
		return resolveFastStructLoopCall(plan, bindings)
	}
	if s.singleCall == plan {
		return s.singleResolvedCall, nil
	}
	if s.calls != nil {
		if resolved := s.calls[plan]; resolved != nil {
			return resolved, nil
		}
	}
	resolved, err := resolveFastStructLoopCall(plan, bindings)
	if err != nil {
		return nil, err
	}
	if s.singleCall == nil {
		s.singleCall = plan
		s.singleResolvedCall = resolved
		return resolved, nil
	}
	if s.calls == nil {
		s.calls = map[*fastStructLoopCallPlan]*fastStructLoopResolvedCall{
			s.singleCall: s.singleResolvedCall,
		}
	}
	s.calls[plan] = resolved
	return resolved, nil
}

func resolveFastStructLoopCall(plan *fastStructLoopCallPlan, bindings fastRenderBindings) (*fastStructLoopResolvedCall, error) {
	call := plan.call
	raw, ok := bindings.value(call.NameIndex)
	if !ok {
		return nil, fmt.Errorf("%q: unknown identifier", call.Name)
	}
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
	entry := cachedFastCallEntry(rv.Type(), raw, &call.Cache)
	resolved := &fastStructLoopResolvedCall{
		raw:          raw,
		fn:           rv,
		entry:        entry,
		directWriter: fastStructLoopDirectCallWriterForRaw(raw, plan),
	}
	if canUseFastStructLoopReflectCall(plan, entry) {
		resolved.canReflect = true
		resolved.reflectArgs = make([]reflect.Value, 0, len(plan.args))
		if err := resolved.prepareStaticReflectArgs(plan, bindings); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

func canUseFastStructLoopReflectCall(plan *fastStructLoopCallPlan, entry *fastBuilderCallCacheEntry) bool {
	if plan == nil || entry == nil || entry.plan == nil || entry.plan.isVariadic || len(plan.args) != entry.plan.numIn {
		return false
	}
	for i := range plan.args {
		if plan.args[i].kind == fastStructLoopCallArgGeneric {
			return false
		}
	}
	return true
}

func writeFastStructLoopReflectCall(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStructLoopCallPlan, resolved *fastStructLoopResolvedCall, loopKey interface{}, item reflect.Value) error {
	if plan == nil || plan.call == nil || resolved == nil || resolved.entry == nil || resolved.entry.plan == nil {
		return errFastWriteUnsupported
	}
	callPlan := resolved.entry.plan
	args := resolved.reflectArgs[:0]
	for i := range plan.args {
		var arg reflect.Value
		if i < len(resolved.staticReflectArgOK) && resolved.staticReflectArgOK[i] {
			arg = resolved.staticReflectArgs[i]
		} else {
			var err error
			arg, err = evalFastStructLoopCallArgReflect(&plan.args[i], ctx, bindings, loopKey, item, callPlan.argTypes[i], plan.call.Name, i)
			if err != nil {
				return err
			}
		}
		args = append(args, arg)
	}
	resolved.reflectArgs = args[:0]
	results := resolved.fn.Call(args)
	return writeFastReflectCallResults(out, ctx, plan.call.Name, results)
}

func (r *fastStructLoopResolvedCall) prepareStaticReflectArgs(plan *fastStructLoopCallPlan, bindings fastRenderBindings) error {
	if r == nil || r.entry == nil || r.entry.plan == nil || plan == nil || len(plan.args) == 0 {
		return nil
	}
	r.staticReflectArgs = make([]reflect.Value, len(plan.args))
	r.staticReflectArgOK = make([]bool, len(plan.args))
	for i := range plan.args {
		if !fastStructLoopCallArgStatic(plan.args[i].kind) {
			continue
		}
		arg, err := evalFastStructLoopStaticCallArgReflect(&plan.args[i], bindings, r.entry.plan.argTypes[i], plan.call.Name, i)
		if err != nil {
			return err
		}
		r.staticReflectArgs[i] = arg
		r.staticReflectArgOK[i] = true
	}
	return nil
}

func fastStructLoopCallArgStatic(kind fastStructLoopCallArgKind) bool {
	switch kind {
	case fastStructLoopCallArgBinding,
		fastStructLoopCallArgNil,
		fastStructLoopCallArgString,
		fastStructLoopCallArgInt,
		fastStructLoopCallArgFloat,
		fastStructLoopCallArgBool:
		return true
	default:
		return false
	}
}

func evalFastStructLoopStaticCallArgReflect(plan *fastStructLoopCallArgPlan, bindings fastRenderBindings, expected reflect.Type, name string, pos int) (reflect.Value, error) {
	value, ok := evalFastStructLoopStaticCallArgValue(plan, bindings)
	if !ok {
		return reflect.Value{}, fastLineError(plan.line, fmt.Errorf("%q: unknown identifier", plan.value.Value))
	}
	return fastReflectArgForCall(name, pos, value, expected)
}

func evalFastStructLoopStaticCallArgValue(plan *fastStructLoopCallArgPlan, bindings fastRenderBindings) (interface{}, bool) {
	if plan == nil {
		return nil, true
	}
	switch plan.kind {
	case fastStructLoopCallArgBinding:
		raw, ok := bindings.value(plan.nameIndex)
		return raw, ok
	case fastStructLoopCallArgNil:
		return nil, true
	case fastStructLoopCallArgString:
		return plan.stringVal, true
	case fastStructLoopCallArgInt:
		return int(plan.intVal), true
	case fastStructLoopCallArgFloat:
		return plan.floatVal, true
	case fastStructLoopCallArgBool:
		return plan.boolVal, true
	default:
		return nil, false
	}
}

func buildFastStructLoopCallPlan(call *compiler.FastCallPlan, elemType reflect.Type) (*fastStructLoopCallPlan, bool) {
	if call == nil {
		return nil, false
	}
	plan := &fastStructLoopCallPlan{
		call: call,
		args: make([]fastStructLoopCallArgPlan, 0, len(call.Args)),
	}
	for i := range call.Args {
		plan.args = append(plan.args, buildFastStructLoopCallArgPlan(&call.Args[i], elemType))
	}
	return plan, true
}

func buildFastStructLoopCallArgPlan(value *compiler.FastValuePlan, elemType reflect.Type) fastStructLoopCallArgPlan {
	if value == nil {
		return fastStructLoopCallArgPlan{kind: fastStructLoopCallArgNil}
	}
	plan := fastStructLoopCallArgPlan{
		kind:  fastStructLoopCallArgGeneric,
		value: *value,
		line:  value.Line,
	}
	switch value.Kind {
	case compiler.FastValueLoopKey:
		plan.kind = fastStructLoopCallArgKey
	case compiler.FastValueName:
		if value.Value == "nil" {
			plan.kind = fastStructLoopCallArgNil
			break
		}
		plan.kind = fastStructLoopCallArgBinding
		plan.nameIndex = value.NameIndex
	case compiler.FastValueString:
		plan.kind = fastStructLoopCallArgString
		plan.stringVal = value.Value
	case compiler.FastValueInteger:
		plan.kind = fastStructLoopCallArgInt
		plan.intVal = value.IntValue
	case compiler.FastValueFloat:
		plan.kind = fastStructLoopCallArgFloat
		plan.floatVal = value.FloatValue
	case compiler.FastValueBool:
		plan.kind = fastStructLoopCallArgBool
		plan.boolVal = value.BoolValue
	case compiler.FastValuePath:
		if value.NameIndex < 0 {
			if accessPlan, ok := fastAccessChainPlanFor(value, elemType); ok {
				plan.kind = fastStructLoopCallArgAccessChain
				plan.accessPlan = accessPlan
			}
		}
	}
	return plan
}

func evalFastStructLoopCallPlanArgs(plan *fastStructLoopCallPlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value, args *fastCallArgs) error {
	if plan == nil || len(plan.args) == 0 {
		return nil
	}
	args.Reset()
	for i := range plan.args {
		value, ok, err := evalFastStructLoopCallArgValue(&plan.args[i], ctx, bindings, loopKey, item)
		if err != nil {
			return err
		}
		if !ok {
			return fastLineError(plan.args[i].line, fmt.Errorf("%q: unknown identifier", plan.args[i].value.Value))
		}
		args.Append(value)
	}
	return nil
}

func evalFastStructLoopCallArgValue(plan *fastStructLoopCallArgPlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (interface{}, bool, error) {
	if plan == nil {
		return nil, true, nil
	}
	switch plan.kind {
	case fastStructLoopCallArgKey:
		return loopKey, true, nil
	case fastStructLoopCallArgBinding:
		raw, ok := bindings.value(plan.nameIndex)
		return raw, ok, nil
	case fastStructLoopCallArgNil:
		return nil, true, nil
	case fastStructLoopCallArgString:
		return plan.stringVal, true, nil
	case fastStructLoopCallArgInt:
		return int(plan.intVal), true, nil
	case fastStructLoopCallArgFloat:
		return plan.floatVal, true, nil
	case fastStructLoopCallArgBool:
		return plan.boolVal, true, nil
	case fastStructLoopCallArgAccessChain:
		rv := unwrapFastFieldChainValue(item)
		if !rv.IsValid() {
			return nil, true, nil
		}
		if plan.accessPlan == nil {
			return nil, false, nil
		}
		return evalFastAccessChainPlanValue(plan.accessPlan, rv, ctx)
	default:
		return evalFastStructLoopValue(&plan.value, ctx, bindings, loopKey, item)
	}
}

func fastStructLoopDirectCallWriterForRaw(raw interface{}, plan *fastStructLoopCallPlan) fastStructLoopDirectCallWriter {
	if plan == nil {
		return nil
	}
	switch raw := raw.(type) {
	case func(string) string:
		if len(plan.args) != 1 {
			return nil
		}
		return func(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStructLoopCallPlan, loopKey interface{}, item reflect.Value) (bool, error) {
			arg, ok, err := evalFastStructLoopCallArgString(&plan.args[0], ctx, bindings, loopKey, item)
			if err != nil || !ok {
				return ok, err
			}
			writeFastEscapedString(out, raw(arg))
			return true, nil
		}
	case func(string, string) string:
		if len(plan.args) != 2 {
			return nil
		}
		return func(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStructLoopCallPlan, loopKey interface{}, item reflect.Value) (bool, error) {
			first, ok, err := evalFastStructLoopCallArgString(&plan.args[0], ctx, bindings, loopKey, item)
			if err != nil || !ok {
				return ok, err
			}
			second, ok, err := evalFastStructLoopCallArgString(&plan.args[1], ctx, bindings, loopKey, item)
			if err != nil || !ok {
				return ok, err
			}
			writeFastEscapedString(out, raw(first, second))
			return true, nil
		}
	case func(string) (string, error):
		if len(plan.args) != 1 {
			return nil
		}
		return func(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStructLoopCallPlan, loopKey interface{}, item reflect.Value) (bool, error) {
			arg, ok, err := evalFastStructLoopCallArgString(&plan.args[0], ctx, bindings, loopKey, item)
			if err != nil || !ok {
				return ok, err
			}
			value, err := raw(arg)
			if err != nil {
				return true, fmt.Errorf("could not call %s function: %w", plan.call.Name, err)
			}
			writeFastEscapedString(out, value)
			return true, nil
		}
	case func(string, string) (string, error):
		if len(plan.args) != 2 {
			return nil
		}
		return func(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStructLoopCallPlan, loopKey interface{}, item reflect.Value) (bool, error) {
			first, ok, err := evalFastStructLoopCallArgString(&plan.args[0], ctx, bindings, loopKey, item)
			if err != nil || !ok {
				return ok, err
			}
			second, ok, err := evalFastStructLoopCallArgString(&plan.args[1], ctx, bindings, loopKey, item)
			if err != nil || !ok {
				return ok, err
			}
			value, err := raw(first, second)
			if err != nil {
				return true, fmt.Errorf("could not call %s function: %w", plan.call.Name, err)
			}
			writeFastEscapedString(out, value)
			return true, nil
		}
	case func(string) template.HTML:
		if len(plan.args) != 1 {
			return nil
		}
		return func(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStructLoopCallPlan, loopKey interface{}, item reflect.Value) (bool, error) {
			arg, ok, err := evalFastStructLoopCallArgString(&plan.args[0], ctx, bindings, loopKey, item)
			if err != nil || !ok {
				return ok, err
			}
			out.WriteString(string(raw(arg)))
			return true, nil
		}
	case func(string, string) template.HTML:
		if len(plan.args) != 2 {
			return nil
		}
		return func(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStructLoopCallPlan, loopKey interface{}, item reflect.Value) (bool, error) {
			first, ok, err := evalFastStructLoopCallArgString(&plan.args[0], ctx, bindings, loopKey, item)
			if err != nil || !ok {
				return ok, err
			}
			second, ok, err := evalFastStructLoopCallArgString(&plan.args[1], ctx, bindings, loopKey, item)
			if err != nil || !ok {
				return ok, err
			}
			out.WriteString(string(raw(first, second)))
			return true, nil
		}
	}
	return nil
}

func evalFastStructLoopCallArgString(plan *fastStructLoopCallArgPlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (string, bool, error) {
	if plan == nil {
		return "", true, nil
	}
	if plan.kind == fastStructLoopCallArgAccessChain {
		rv := unwrapFastFieldChainValue(item)
		if !rv.IsValid() || plan.accessPlan == nil {
			return "", false, nil
		}
		value, ok, err := evalFastAccessChainReflectValue(plan.accessPlan, rv, ctx)
		if err != nil || !ok {
			return "", ok, err
		}
		value = unwrapFastFieldChainValue(value)
		if !value.IsValid() || isNilReflectValue(value) {
			return "", false, nil
		}
		if value.Kind() == reflect.String {
			return value.String(), true, nil
		}
		if value.CanInterface() {
			arg, ok := fastWriteRawStringArg(value.Interface())
			return arg, ok, nil
		}
		return "", false, nil
	}
	value, ok, err := evalFastStructLoopCallArgValue(plan, ctx, bindings, loopKey, item)
	if err != nil || !ok {
		return "", ok, err
	}
	arg, ok := fastWriteRawStringArg(value)
	return arg, ok, nil
}

func evalFastStructLoopCallArgReflect(plan *fastStructLoopCallArgPlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value, expected reflect.Type, name string, pos int) (reflect.Value, error) {
	if plan == nil {
		return reflect.Zero(expected), nil
	}
	switch plan.kind {
	case fastStructLoopCallArgAccessChain:
		rv := unwrapFastFieldChainValue(item)
		if !rv.IsValid() {
			return reflect.Zero(expected), nil
		}
		value, ok, err := evalFastAccessChainReflectValue(plan.accessPlan, rv, ctx)
		if err != nil {
			return reflect.Value{}, err
		}
		if !ok {
			return reflect.Zero(expected), nil
		}
		return fastReflectArgValueForCall(name, pos, value, expected)
	default:
		value, ok, err := evalFastStructLoopCallArgValue(plan, ctx, bindings, loopKey, item)
		if err != nil {
			return reflect.Value{}, err
		}
		if !ok {
			return reflect.Value{}, fastLineError(plan.line, fmt.Errorf("%q: unknown identifier", plan.value.Value))
		}
		return fastReflectArgForCall(name, pos, value, expected)
	}
}

func fastReflectArgValueForCall(name string, pos int, value reflect.Value, expected reflect.Type) (reflect.Value, error) {
	if !value.IsValid() || isNilReflectValue(value) {
		return reflect.Zero(expected), nil
	}
	if value.Kind() == reflect.Interface && !value.Type().AssignableTo(expected) {
		value = value.Elem()
	}
	if value.Type().AssignableTo(expected) {
		return value, nil
	}
	if value.Type().ConvertibleTo(expected) {
		return value.Convert(expected), nil
	}
	if value.CanInterface() {
		return fastReflectArgForCall(name, pos, value.Interface(), expected)
	}
	return reflect.Value{}, fmt.Errorf("%+v (%s) is an invalid argument for %s at pos %d: expected (%s)", value, value.Type(), name, pos, expected)
}

func evalFastStructLoopValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (interface{}, bool, error) {
	if value == nil {
		return nil, true, nil
	}
	switch value.Kind {
	case compiler.FastValueLoopKey:
		return loopKey, true, nil
	case compiler.FastValueInfix:
		return evalFastStructLoopInfixValue(value, ctx, bindings, loopKey, item)
	case compiler.FastValuePrefix:
		return evalFastStructLoopPrefixValue(value, ctx, bindings, loopKey, item)
	case compiler.FastValueConcat:
		return evalFastStructLoopConcatValue(value, ctx, bindings, loopKey, item)
	case compiler.FastValuePath:
		if value.NameIndex >= 0 {
			return evalFastValue(value, ctx, bindings, nil)
		}
		return evalFastStructLoopPathValue(value, ctx, bindings, item)
	default:
		return evalFastValue(value, ctx, bindings, nil)
	}
}

func isTruthyFastStructLoopValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (bool, bool, error) {
	if value != nil && value.Kind == compiler.FastValuePath && value.NameIndex < 0 {
		rv := unwrapFastFieldChainValue(item)
		if !rv.IsValid() {
			return false, true, nil
		}
		if len(value.Path) == 0 {
			return isTruthyFastReflectValue(rv), true, nil
		}
		if chain, ok := fastAccessChainPlanFor(value, rv.Type()); ok {
			field, ok, err := evalFastAccessChainReflectValue(chain, rv, ctx)
			if err != nil || !ok {
				return false, ok, err
			}
			return isTruthyFastReflectValue(field), true, nil
		}
	}
	result, ok, err := evalFastStructLoopValue(value, ctx, bindings, loopKey, item)
	if err != nil || !ok {
		return false, ok, err
	}
	return isTruthyFastValue(result), true, nil
}

func isTruthyFastReflectValue(value reflect.Value) bool {
	if !value.IsValid() {
		return false
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return false
		}
		if value.CanInterface() {
			if obj, ok := value.Interface().(object.Object); ok {
				return isTruthy(obj)
			}
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Bool:
		return value.Bool()
	case reflect.String:
		return value.Len() != 0
	case reflect.Ptr:
		return !value.IsNil()
	default:
		return true
	}
}

func evalFastStructLoopPathValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, item reflect.Value) (interface{}, bool, error) {
	rv := unwrapFastFieldChainValue(item)
	if !rv.IsValid() {
		return nil, true, nil
	}
	if len(value.Path) == 0 {
		return fastReflectInterface(rv), true, nil
	}
	if chain, ok := fastAccessChainPlanFor(value, rv.Type()); ok {
		return evalFastAccessChainPlanValue(chain, rv, ctx)
	}
	return evalFastValue(value, ctx, bindings, fastReflectInterface(rv))
}

func evalFastStructLoopInfixValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (interface{}, bool, error) {
	if value == nil || value.Kind != compiler.FastValueInfix || value.Left == nil || value.Right == nil {
		return nil, false, nil
	}
	if value.Operator == "&&" || value.Operator == "||" {
		return evalFastStructLoopLogicalInfixValue(value, ctx, bindings, loopKey, item)
	}
	left, ok, err := evalFastStructLoopValue(value.Left, ctx, bindings, loopKey, item)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		left = nil
	}
	right, ok, err := evalFastStructLoopValue(value.Right, ctx, bindings, loopKey, item)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		right = nil
	}
	result, err := evalFastInfixOperator(value.Operator, left, right)
	if err != nil {
		return nil, true, fastLineError(value.Line, err)
	}
	return result, true, nil
}

func evalFastStructLoopPrefixValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (interface{}, bool, error) {
	if value == nil || value.Kind != compiler.FastValuePrefix || value.Right == nil {
		return nil, false, nil
	}
	if value.Operator != "!" {
		return nil, true, fastLineError(value.Line, fmt.Errorf("unknown fast prefix operator: %s", value.Operator))
	}
	right, ok, err := evalFastStructLoopValue(value.Right, ctx, bindings, loopKey, item)
	if err != nil {
		return nil, true, err
	}
	return !(ok && isTruthyFastValue(right)), true, nil
}

func evalFastStructLoopConcatValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (interface{}, bool, error) {
	if value == nil || value.Kind != compiler.FastValueConcat || value.Left == nil || value.Right == nil {
		return nil, false, nil
	}
	left, ok, err := evalFastStructLoopValue(value.Left, ctx, bindings, loopKey, item)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, false, nil
	}
	right, ok, err := evalFastStructLoopValue(value.Right, ctx, bindings, loopKey, item)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, false, nil
	}
	result, err := evalFastAddOperator(left, right)
	if err != nil {
		return nil, true, fastLineError(value.Line, err)
	}
	return result, true, nil
}

func evalFastStructLoopLogicalInfixValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (interface{}, bool, error) {
	left, ok, err := evalFastStructLoopValue(value.Left, ctx, bindings, loopKey, item)
	if err != nil {
		return nil, true, err
	}
	leftTruthy := ok && isTruthyFastValue(left)
	switch value.Operator {
	case "&&":
		if !leftTruthy {
			return false, true, nil
		}
	case "||":
		if leftTruthy {
			return true, true, nil
		}
	default:
		return nil, false, nil
	}
	right, ok, err := evalFastStructLoopValue(value.Right, ctx, bindings, loopKey, item)
	if err != nil {
		return nil, true, err
	}
	return ok && isTruthyFastValue(right), true, nil
}

func evalFastStructLoopConditionPlan(plan *fastStructLoopConditionPlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (bool, bool, error) {
	if plan == nil {
		return false, false, nil
	}
	switch plan.kind {
	case fastStructLoopConditionTruthy:
		value, ok, err := evalFastStructLoopConditionOperand(&plan.value, ctx, bindings, loopKey, item)
		if err != nil || !ok {
			return false, ok, err
		}
		return isTruthyFastConditionOperand(value), true, nil
	case fastStructLoopConditionLogical:
		leftTruthy, leftOK, err := evalFastStructLoopConditionPlan(plan.left, ctx, bindings, loopKey, item)
		if err != nil {
			return false, true, err
		}
		switch plan.operator {
		case "&&":
			if !leftOK || !leftTruthy {
				return false, true, nil
			}
		case "||":
			if leftOK && leftTruthy {
				return true, true, nil
			}
		default:
			return false, false, nil
		}
		rightTruthy, rightOK, err := evalFastStructLoopConditionPlan(plan.right, ctx, bindings, loopKey, item)
		if err != nil {
			return false, true, err
		}
		return rightOK && rightTruthy, true, nil
	case fastStructLoopConditionInfix:
		left, leftOK, err := evalFastStructLoopConditionOperand(&plan.leftValue, ctx, bindings, loopKey, item)
		if err != nil {
			return false, true, err
		}
		right, rightOK, err := evalFastStructLoopConditionOperand(&plan.rightValue, ctx, bindings, loopKey, item)
		if err != nil {
			return false, true, err
		}
		if !leftOK {
			left = fastConditionOperandValue{}
		}
		if !rightOK {
			right = fastConditionOperandValue{}
		}
		result, err := evalFastConditionInfixOperator(plan.operator, left, right)
		if err != nil {
			return false, true, fastLineError(plan.line, err)
		}
		return result, true, nil
	default:
		return false, false, nil
	}
}

func evalFastStructLoopConditionOperand(plan *fastStructLoopCallArgPlan, ctx hctx.Context, bindings fastRenderBindings, loopKey interface{}, item reflect.Value) (fastConditionOperandValue, bool, error) {
	if plan == nil {
		return fastConditionOperandValue{}, true, nil
	}
	if plan.kind == fastStructLoopCallArgAccessChain {
		rv := unwrapFastFieldChainValue(item)
		if !rv.IsValid() {
			return fastConditionOperandValue{}, true, nil
		}
		if plan.accessPlan == nil {
			return fastConditionOperandValue{}, false, nil
		}
		value, ok, err := evalFastAccessChainReflectValue(plan.accessPlan, rv, ctx)
		if err != nil || !ok {
			return fastConditionOperandValue{}, ok, err
		}
		return fastConditionOperandValue{
			raw:        fastReflectInterface(value),
			reflect:    value,
			hasReflect: true,
		}, true, nil
	}
	value, ok, err := evalFastStructLoopCallArgValue(plan, ctx, bindings, loopKey, item)
	if err != nil || !ok {
		return fastConditionOperandValue{}, ok, err
	}
	return fastConditionOperandValue{raw: value}, true, nil
}

func evalFastConditionInfixOperator(operator string, left, right fastConditionOperandValue) (bool, error) {
	switch operator {
	case "==":
		return compareFastConditionEquality(operator, left, right)
	case "!=":
		result, err := compareFastConditionEquality(operator, left, right)
		return !result, err
	case ">":
		return compareFastConditionOrdered(code.OpGreaterThan, left, right)
	case ">=":
		return compareFastConditionOrdered(code.OpGreaterEqual, left, right)
	case "<":
		return compareFastConditionOrdered(code.OpGreaterThan, right, left)
	case "<=":
		return compareFastConditionOrdered(code.OpGreaterEqual, right, left)
	default:
		return false, fmt.Errorf("unknown fast infix operator: %s", operator)
	}
}

func compareFastConditionEquality(operator string, left, right fastConditionOperandValue) (bool, error) {
	if left.isNil() || right.isNil() {
		return left.isNil() && right.isNil(), nil
	}
	if value, ok := left.boolValue(); ok {
		return value == isTruthyFastConditionOperand(right), nil
	}
	if value, ok := left.stringValue(); ok {
		return value == fmt.Sprint(right.goValue()), nil
	}
	l, lok := left.numericValue()
	r, rok := right.numericValue()
	if lok && rok {
		return compareNumericEquality(l, r), nil
	}
	if lok || rok {
		return false, fmt.Errorf("unable to operate (%s) on %s and %s ", operator, fastConditionTypeName(left), fastConditionTypeName(right))
	}
	return reflect.DeepEqual(left.goValue(), right.goValue()), nil
}

func compareFastConditionOrdered(op code.Opcode, left, right fastConditionOperandValue) (bool, error) {
	l, lok := left.numericValue()
	r, rok := right.numericValue()
	if lok && rok {
		return compareNumericOrdered(op, l, r)
	}
	if lok || rok {
		return false, fmt.Errorf("unable to operate (%s) on %s and %s ", orderedOperatorString(op), fastConditionTypeName(left), fastConditionTypeName(right))
	}
	lString := fmt.Sprint(left.goValue())
	rString := fmt.Sprint(right.goValue())
	switch op {
	case code.OpGreaterThan:
		return lString > rString, nil
	case code.OpGreaterEqual:
		return lString >= rString, nil
	default:
		return false, fmt.Errorf("unknown ordered comparison: %d", op)
	}
}

func isTruthyFastConditionOperand(value fastConditionOperandValue) bool {
	if value.hasReflect {
		return isTruthyFastReflectValue(value.reflect)
	}
	return isTruthyFastValue(value.raw)
}

func (v fastConditionOperandValue) isNil() bool {
	if v.hasReflect {
		return !v.reflect.IsValid() || isNilReflectValue(v.reflect)
	}
	if obj, ok := v.raw.(object.Object); ok {
		return object.IsNull(obj)
	}
	return v.raw == nil
}

func (v fastConditionOperandValue) boolValue() (bool, bool) {
	if v.hasReflect {
		value := v.reflect
		if !value.IsValid() || isNilReflectValue(value) {
			return false, false
		}
		if value.Kind() == reflect.Interface {
			value = value.Elem()
		}
		if value.Type() == boolType {
			return value.Bool(), true
		}
		return false, false
	}
	switch value := v.raw.(type) {
	case bool:
		return value, true
	case *object.Boolean:
		return value.Value, true
	default:
		return false, false
	}
}

func (v fastConditionOperandValue) stringValue() (string, bool) {
	if v.hasReflect {
		value := v.reflect
		if !value.IsValid() || isNilReflectValue(value) {
			return "", false
		}
		if value.Kind() == reflect.Interface {
			value = value.Elem()
		}
		if value.Type() == stringType {
			return value.String(), true
		}
		return "", false
	}
	switch value := v.raw.(type) {
	case string:
		return value, true
	case *object.String:
		return value.Value, true
	default:
		return "", false
	}
}

func (v fastConditionOperandValue) numericValue() (numericValue, bool) {
	if v.hasReflect {
		return numericValueFromReflectValue(v.reflect)
	}
	if obj, ok := v.raw.(object.Object); ok {
		return numericValueFromObject(obj)
	}
	return numericValueFromGo(v.raw)
}

func (v fastConditionOperandValue) goValue() interface{} {
	if v.hasReflect {
		return fastReflectInterface(v.reflect)
	}
	if obj, ok := v.raw.(object.Object); ok {
		return object.ToGo(obj)
	}
	return v.raw
}

func fastConditionTypeName(value fastConditionOperandValue) string {
	if value.hasReflect && value.reflect.IsValid() {
		return value.reflect.Type().String()
	}
	return plushTypeName(object.Wrap(value.raw))
}

func fastStructLoopElementType(iterType reflect.Type) (reflect.Type, bool) {
	if iterType.Kind() != reflect.Array && iterType.Kind() != reflect.Slice {
		return nil, false
	}
	elemType := iterType.Elem()
	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}
	if elemType.Kind() != reflect.Struct {
		return nil, false
	}
	return elemType, true
}

func fastStructLoopWriterPlanFor(loop *compiler.FastLoopPlan, elemType reflect.Type) (*fastStructLoopWriterPlan, bool) {
	if loop == nil {
		return nil, false
	}
	key := fastStructLoopWriterPlanKey{loop: loop, typ: elemType}
	if cached, ok := fastStructLoopWriterPlanCache.Load(key); ok {
		plan, _ := cached.(*fastStructLoopWriterPlan)
		return plan, plan != nil
	}
	plan, ok := buildFastStructLoopWriterPlan(loop, elemType)
	if !ok {
		fastStructLoopWriterPlanCache.Store(key, (*fastStructLoopWriterPlan)(nil))
		return nil, false
	}
	actual, _ := fastStructLoopWriterPlanCache.LoadOrStore(key, plan)
	plan, _ = actual.(*fastStructLoopWriterPlan)
	return plan, plan != nil
}

func buildFastStructLoopWriterPlan(loop *compiler.FastLoopPlan, elemType reflect.Type) (*fastStructLoopWriterPlan, bool) {
	if loop == nil {
		return nil, false
	}
	ops, ok := buildFastStructLoopWriterOps(loop.Parts, elemType)
	if !ok {
		return nil, false
	}
	return &fastStructLoopWriterPlan{ops: ops}, true
}

func buildFastStructLoopWriterOps(parts []compiler.FastLoopPart, elemType reflect.Type) ([]fastStructLoopWriterOp, bool) {
	ops := make([]fastStructLoopWriterOp, 0, len(parts))
	for i := range parts {
		part := &parts[i]
		switch part.Kind {
		case compiler.FastLoopPartStatic:
			ops = append(ops, fastStructLoopWriterOp{
				kind:  fastStructLoopWriterStatic,
				value: part.Value,
			})
		case compiler.FastLoopPartKey:
			ops = append(ops, fastStructLoopWriterOp{kind: fastStructLoopWriterKey})
		case compiler.FastLoopPartValueProperty:
			if part.Value == "" {
				return nil, false
			}
			entry := inlinePropertyEntry(&part.PropertyCache, elemType, part.Value)
			if entry == nil || entry.lookup.kind != propertyLookupField {
				return nil, false
			}
			field, ok := fieldByIndex(elemType, entry.lookup.fieldIndex)
			if !ok {
				return nil, false
			}
			ops = append(ops, fastStructLoopWriterOp{
				kind:       fastStructLoopWriterField,
				name:       part.Value,
				receiver:   part.Receiver,
				full:       part.Full,
				line:       part.Line,
				fieldIndex: entry.lookup.fieldIndex,
				fieldType:  field.Type,
			})
		case compiler.FastLoopPartValuePath:
			if methodPlan, ok := buildFastLoopMethodCallPlan(&part.ValuePlan, elemType); ok {
				ops = append(ops, fastStructLoopWriterOp{
					kind:       fastStructLoopWriterMethodCall,
					methodPlan: methodPlan,
					line:       part.Line,
				})
				continue
			}
			accessPlan, ok := fastAccessChainPlanFor(&part.ValuePlan, elemType)
			if !ok {
				return nil, false
			}
			ops = append(ops, fastStructLoopWriterOp{
				kind:       fastStructLoopWriterAccessChain,
				accessPlan: accessPlan,
				line:       part.Line,
			})
		case compiler.FastLoopPartCall:
			if part.Call == nil {
				return nil, false
			}
			call, _ := buildFastStructLoopCallPlan(part.Call, elemType)
			ops = append(ops, fastStructLoopWriterOp{
				kind: fastStructLoopWriterCall,
				call: call,
				line: part.Line,
			})
		case compiler.FastLoopPartConditional:
			conditional, ok := buildFastStructLoopConditionalWriterPlan(part.Conditional, elemType)
			if !ok {
				return nil, false
			}
			ops = append(ops, fastStructLoopWriterOp{
				kind:        fastStructLoopWriterConditional,
				conditional: conditional,
				line:        part.Line,
			})
		default:
			return nil, false
		}
	}
	return ops, true
}

func buildFastStructLoopConditionalWriterPlan(conditional *compiler.FastLoopConditionalPlan, elemType reflect.Type) (*fastStructLoopConditionalWriterPlan, bool) {
	if conditional == nil {
		return nil, false
	}
	plan := &fastStructLoopConditionalWriterPlan{
		branches: make([]fastStructLoopConditionalWriterBranch, 0, len(conditional.Branches)),
	}
	for i := range conditional.Branches {
		branch := &conditional.Branches[i]
		ops, ok := buildFastStructLoopWriterOps(branch.Parts, elemType)
		if !ok {
			return nil, false
		}
		plan.branches = append(plan.branches, fastStructLoopConditionalWriterBranch{
			condition:     branch.Condition,
			conditionPlan: buildFastStructLoopConditionPlan(&branch.Condition, elemType),
			ops:           ops,
			line:          branch.Line,
		})
	}
	if len(conditional.ElseParts) > 0 {
		ops, ok := buildFastStructLoopWriterOps(conditional.ElseParts, elemType)
		if !ok {
			return nil, false
		}
		plan.elseOps = ops
	}
	return plan, true
}

func buildFastStructLoopConditionPlan(value *compiler.FastValuePlan, elemType reflect.Type) *fastStructLoopConditionPlan {
	if value == nil {
		return nil
	}
	if value.Kind == compiler.FastValueInfix {
		if value.Left == nil || value.Right == nil {
			return nil
		}
		if value.Operator == "&&" || value.Operator == "||" {
			left := buildFastStructLoopConditionPlan(value.Left, elemType)
			right := buildFastStructLoopConditionPlan(value.Right, elemType)
			if left == nil || right == nil {
				return nil
			}
			return &fastStructLoopConditionPlan{
				kind:     fastStructLoopConditionLogical,
				operator: value.Operator,
				left:     left,
				right:    right,
				line:     value.Line,
			}
		}
		left, ok := buildFastStructLoopConditionOperand(value.Left, elemType)
		if !ok {
			return nil
		}
		right, ok := buildFastStructLoopConditionOperand(value.Right, elemType)
		if !ok {
			return nil
		}
		return &fastStructLoopConditionPlan{
			kind:       fastStructLoopConditionInfix,
			operator:   value.Operator,
			leftValue:  left,
			rightValue: right,
			line:       value.Line,
		}
	}
	operand, ok := buildFastStructLoopConditionOperand(value, elemType)
	if !ok {
		return nil
	}
	return &fastStructLoopConditionPlan{
		kind:  fastStructLoopConditionTruthy,
		value: operand,
		line:  value.Line,
	}
}

func buildFastStructLoopConditionOperand(value *compiler.FastValuePlan, elemType reflect.Type) (fastStructLoopCallArgPlan, bool) {
	operand := buildFastStructLoopCallArgPlan(value, elemType)
	return operand, operand.kind != fastStructLoopCallArgGeneric
}

func buildFastLoopMethodCallPlan(value *compiler.FastValuePlan, elemType reflect.Type) (*fastLoopMethodCallPlan, bool) {
	if value == nil || value.Kind != compiler.FastValuePath || len(value.Path) < 2 {
		return nil, false
	}
	call := value.Path[len(value.Path)-1]
	method := value.Path[len(value.Path)-2]
	if call.Kind != compiler.FastPathStepCall ||
		method.Kind != compiler.FastPathStepProperty ||
		!method.Method {
		return nil, false
	}
	receiver, receiverType, ok := buildFastAccessChainPlanForSteps(value.Path[:len(value.Path)-2], elemType)
	if !ok {
		return nil, false
	}
	receiverType = unwrapReflectType(receiverType)
	lookup := cachedPropertyLookup(receiverType, method.Value)
	if lookup.kind != propertyLookupValueMethod && lookup.kind != propertyLookupPointerMethod {
		return nil, false
	}
	return &fastLoopMethodCallPlan{
		receiver: receiver,
		method:   method,
		call:     call,
		lookup:   lookup,
	}, true
}

func writeFastLoopMethodCall(out *strings.Builder, ctx hctx.Context, plan *fastLoopMethodCallPlan, item reflect.Value) error {
	if plan == nil {
		return nil
	}
	receiver, ok, err := fastLoopMethodReceiver(plan.receiver, item, ctx)
	if err != nil || !ok {
		return err
	}
	if err := spendFastTraversal(ctx, plan.method.Line); err != nil {
		return err
	}
	method, ok := fastBoundMethodValue(receiver, plan.lookup)
	if !ok {
		return fastLineError(plan.method.Line, propertyMissingError(object.PropertyAccess{
			Receiver: plan.method.Receiver,
			Full:     plan.method.Full,
			Method:   true,
		}, fastReflectInterface(receiver), plan.method.Value))
	}
	if err := spendFastFunctionCall(ctx, plan.call.Value, plan.call.Line); err != nil {
		return err
	}
	results := method.Call(nil)
	return writeFastReflectCallResults(out, ctx, plan.call.Value, results)
}

func fastLoopMethodReceiver(plan *fastAccessChainPlan, item reflect.Value, ctx hctx.Context) (reflect.Value, bool, error) {
	if plan == nil || len(plan.steps) == 0 {
		rv := unwrapFastFieldChainValue(item)
		if !rv.IsValid() {
			return reflect.Value{}, false, nil
		}
		return rv, true, nil
	}
	value, ok, err := evalFastAccessChainReflectValue(plan, item, ctx)
	if err != nil || !ok {
		return reflect.Value{}, ok, err
	}
	return value, true, nil
}

func evalFastAccessChainReflectValue(chain *fastAccessChainPlan, rv reflect.Value, ctx hctx.Context) (reflect.Value, bool, error) {
	current := rv
	for i := range chain.steps {
		step := &chain.steps[i]
		switch step.kind {
		case fastAccessStepField:
			if err := spendFastTraversal(ctx, step.line); err != nil {
				return reflect.Value{}, true, err
			}
			field, ok, err := fastAccessFieldValue(current, step)
			if !ok || err != nil {
				return reflect.Value{}, ok, err
			}
			current, ok, err = fastAccessIntermediateValue(field, step)
			if !ok || err != nil {
				return reflect.Value{}, ok, err
			}
		case fastAccessStepIndex:
			indexed, ok, err := fastAccessIndexValue(current, step)
			if err != nil {
				return reflect.Value{}, true, fastLineError(step.line, err)
			}
			if !ok {
				return reflect.Value{}, false, nil
			}
			current, ok, err = fastAccessIntermediateValue(indexed, step)
			if !ok || err != nil {
				return reflect.Value{}, ok, err
			}
		default:
			return reflect.Value{}, false, nil
		}
	}
	return current, true, nil
}

func fastBoundMethodValue(receiver reflect.Value, lookup propertyLookup) (reflect.Value, bool) {
	receiver = unwrapFastFieldChainValue(receiver)
	if !receiver.IsValid() {
		return reflect.Value{}, false
	}
	switch lookup.kind {
	case propertyLookupValueMethod:
		method := receiver.Method(lookup.index)
		return method, method.IsValid()
	case propertyLookupPointerMethod:
		if receiver.CanAddr() {
			method := receiver.Addr().Method(lookup.index)
			return method, method.IsValid()
		}
		ptr := reflect.New(receiver.Type())
		ptr.Elem().Set(receiver)
		method := ptr.Method(lookup.index)
		return method, method.IsValid()
	default:
		return reflect.Value{}, false
	}
}

func writeFastReflectCallResults(out *strings.Builder, ctx hctx.Context, name string, results []reflect.Value) error {
	if len(results) == 0 {
		return nil
	}
	if err := lastReturnError(results); err != nil {
		return fmt.Errorf("could not call %s function: %w", name, err)
	}
	if isNilReflectValue(results[0]) {
		return nil
	}
	value := results[0]
	if !value.IsValid() {
		return nil
	}
	if value.Type() == templateHTMLType {
		out.WriteString(value.String())
		return nil
	}
	if value.Type() == stringType {
		writeFastEscapedString(out, value.String())
		return nil
	}
	if value.Type().Implements(objectInterfaceType) && value.CanInterface() {
		if obj, ok := value.Interface().(object.Object); ok {
			writeFastObject(out, ctx, obj)
			return nil
		}
	}
	if value.Type().PkgPath() == "" {
		switch value.Kind() {
		case reflect.Bool:
			out.WriteString(strconv.FormatBool(value.Bool()))
			return nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			writeBuilderFastInt(out, value.Int())
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			writeBuilderFastUint(out, value.Uint())
			return nil
		case reflect.Float32, reflect.Float64:
			writeBuilderFastFloat(out, value.Float(), int(value.Type().Bits()))
			return nil
		}
	}
	return writeFastReflectValue(out, ctx, value)
}
