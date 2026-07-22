package vm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

var (
	errFastLoopBreak    = errors.New("fast loop break")
	errFastLoopContinue = errors.New("fast loop continue")
)

func renderFastConditional(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, conditional *compiler.FastConditionalPlan) (bool, error) {
	if conditional == nil {
		return false, nil
	}
	if conditional.Silent {
		return renderFastConditionalSilently(ctx, bindings, conditional)
	}
	for i := range conditional.Branches {
		branch := &conditional.Branches[i]
		if err := spendFastCondition(ctx, branch.Line); err != nil {
			return true, err
		}
		value, ok, err := evalFastValue(&branch.Condition, ctx, bindings, nil)
		if err != nil {
			return true, err
		}
		if !ok {
			value = nil
		}
		if isTruthyFastValue(value) {
			branchCtx, branchBindings, cleanup := fastRenderSegmentScopeForLet(ctx, bindings, branch.Segments)
			defer cleanup()
			ctx = branchCtx
			bindings = branchBindings
			return renderFastSegments(out, ctx, bindings, branch.Segments)
		}
	}
	if len(conditional.ElseSegments) > 0 {
		branchCtx, branchBindings, cleanup := fastRenderSegmentScopeForLet(ctx, bindings, conditional.ElseSegments)
		defer cleanup()
		ctx = branchCtx
		bindings = branchBindings
		return renderFastSegments(out, ctx, bindings, conditional.ElseSegments)
	}
	return true, nil
}

func renderFastConditionalSilently(ctx hctx.Context, bindings fastRenderBindings, conditional *compiler.FastConditionalPlan) (bool, error) {
	if conditional == nil {
		return false, nil
	}
	for i := range conditional.Branches {
		branch := &conditional.Branches[i]
		if err := spendFastCondition(ctx, branch.Line); err != nil {
			return true, err
		}
		value, ok, err := evalFastValue(&branch.Condition, ctx, bindings, nil)
		if err != nil {
			return true, err
		}
		if !ok {
			value = nil
		}
		if isTruthyFastValue(value) {
			var discard strings.Builder
			return renderFastSegments(&discard, ctx, bindings, branch.Segments)
		}
	}
	if len(conditional.ElseSegments) > 0 {
		var discard strings.Builder
		return renderFastSegments(&discard, ctx, bindings, conditional.ElseSegments)
	}
	return true, nil
}

func renderFastPartialSegment(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, partial *compiler.FastPartialPlan) error {
	return renderFastPartialSegmentWithDataPlan(out, ctx, bindings, partial, nil)
}

func renderFastPartialSegmentWithDataPlan(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, partial *compiler.FastPartialPlan, dataPlan *fastPartialDataBindingPlan) error {
	if partial == nil {
		return nil
	}
	if err := spendFastFunctionCall(ctx, "partial", partial.Line); err != nil {
		return err
	}
	if len(partial.Data) > 0 {
		if ok, err := renderFastDataPartialInto(out, partial, ctx, bindings, dataPlan); ok || err != nil {
			if err != nil {
				return err
			}
			return nil
		}
	}
	if ok, err := renderFastNoDataPartialInto(out, partial.Name, ctx, partial.Line); ok || err != nil {
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func renderFastLoop(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan) (bool, error) {
	if loop == nil {
		return false, nil
	}
	if fastLoopPartsHaveLet(loop.Parts) {
		scopedCtx, scopedBindings, cleanup := fastRenderScopedBindings(ctx, bindings)
		defer cleanup()
		ctx = scopedCtx
		bindings = scopedBindings
		bindings = fastRenderBindingsWithLocalCopy(bindings)
		bindings.ensureLocalCapacity()
	}

	iter, ok, err := fastLoopIterableValue(loop, ctx, bindings)
	if err != nil {
		return true, err
	}
	if !ok {
		return true, fastLineError(loop.Line, fmt.Errorf("%q: unknown identifier", loop.IterableName))
	}
	if obj, ok := iter.(object.Object); ok {
		iter = object.ToGo(obj)
	}
	if iter == nil {
		return true, nil
	}

	switch iter := iter.(type) {
	case []string:
		if handled, err := renderFastStringKeyValueLoop(out, ctx, loop, iter); handled || err != nil {
			return true, err
		}
		for i, value := range iter {
			stop, err := renderFastLoopIterationOrControl(out, ctx, bindings, loop, i, value)
			if err != nil {
				return true, err
			}
			if stop {
				break
			}
		}
		return true, nil
	case []interface{}:
		for i, value := range iter {
			stop, err := renderFastLoopIterationOrControl(out, ctx, bindings, loop, i, value)
			if err != nil {
				return true, err
			}
			if stop {
				break
			}
		}
		return true, nil
	case []object.Object:
		for i, value := range iter {
			stop, err := renderFastLoopIterationOrControl(out, ctx, bindings, loop, i, value)
			if err != nil {
				return true, err
			}
			if stop {
				break
			}
		}
		return true, nil
	}

	rv := reflect.ValueOf(iter)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return true, nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		if handled, err := renderFastStructFieldLoop(out, ctx, bindings, loop, rv); handled || err != nil {
			return true, err
		}
		for i := 0; i < rv.Len(); i++ {
			stop, err := renderFastLoopIterationOrControl(out, ctx, bindings, loop, i, rv.Index(i).Interface())
			if err != nil {
				return true, err
			}
			if stop {
				break
			}
		}
		return true, nil
	case reflect.Map:
		for _, key := range rv.MapKeys() {
			stop, err := renderFastLoopIterationOrControl(out, ctx, bindings, loop, key.Interface(), rv.MapIndex(key).Interface())
			if err != nil {
				return true, err
			}
			if stop {
				break
			}
		}
		return true, nil
	default:
		return false, nil
	}
}

func fastLoopIterableValue(loop *compiler.FastLoopPlan, ctx hctx.Context, bindings fastRenderBindings) (interface{}, bool, error) {
	if loop == nil {
		return nil, false, nil
	}
	if loop.Iterable.Kind != compiler.FastValueInvalid {
		return evalFastValue(&loop.Iterable, ctx, bindings, nil)
	}
	value, ok := bindings.value(loop.IterableNameIndex)
	return value, ok, nil
}

func renderFastLoopIterationOrControl(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, key, value interface{}) (bool, error) {
	err := renderFastLoopIteration(out, ctx, bindings, loop, key, value)
	switch err {
	case nil:
		return false, nil
	case errFastLoopBreak:
		return true, nil
	case errFastLoopContinue:
		return false, nil
	default:
		return false, err
	}
}

func renderFastLoopIteration(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, key, value interface{}) error {
	if err := spendFastLoop(ctx, loop.Line); err != nil {
		return err
	}

	return renderFastLoopParts(out, ctx, bindings, loop, loop.Parts, key, value)
}

func renderFastLoopParts(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, parts []compiler.FastLoopPart, key, value interface{}) error {
	for i := range parts {
		part := &parts[i]
		switch part.Kind {
		case compiler.FastLoopPartStatic:
			out.WriteString(part.Value)
		case compiler.FastLoopPartKey:
			writeFastGoValue(out, ctx, key)
		case compiler.FastLoopPartValue:
			writeFastGoValue(out, ctx, value)
		case compiler.FastLoopPartValueProperty:
			if err := spendFastTraversal(ctx, part.Line); err != nil {
				return err
			}
			if err := writeFastPropertyOutput(out, ctx, value, part.Value, object.PropertyAccess{
				Receiver: part.Receiver,
				Full:     part.Full,
			}, &part.PropertyCache); err != nil {
				return fastLineError(part.Line, err)
			}
		case compiler.FastLoopPartValuePath:
			property, ok, err := evalFastLoopValue(&part.ValuePlan, ctx, bindings, key, value)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			writeFastGoValue(out, ctx, property)
		case compiler.FastLoopPartCall:
			if err := writeFastLoopCallPart(out, ctx, bindings, part.Call, key, value); err != nil {
				return err
			}
		case compiler.FastLoopPartBlockCall:
			if err := writeFastLoopBlockCallPart(out, ctx, bindings, loop, part.BlockCall, key, value); err != nil {
				return err
			}
		case compiler.FastLoopPartPartial:
			if err := renderFastLoopPartialPart(out, ctx, bindings, loop, part.Partial, key, value); err != nil {
				return err
			}
		case compiler.FastLoopPartLet:
			local, ok, err := evalFastLoopValue(&part.ValuePlan, ctx, bindings, key, value)
			if err != nil {
				return err
			}
			if !ok {
				return fastLineError(part.Line, fmt.Errorf("%q: unknown identifier", fastValueMissingName(&part.ValuePlan)))
			}
			if err := spendFastAssignment(ctx, part.Line); err != nil {
				return err
			}
			bindings.setLocalAndContext(part.NameIndex, local)
		case compiler.FastLoopPartConditional:
			if err := renderFastLoopConditional(out, ctx, bindings, loop, part.Conditional, key, value); err != nil {
				return err
			}
		case compiler.FastLoopPartLoop:
			nestedBindings := fastLoopBindingsWithCurrentLocals(bindings, loop, key, value)
			ok, err := renderFastLoop(out, ctx, nestedBindings, part.Loop)
			if err != nil {
				return err
			}
			if !ok {
				return fastLineError(part.Line, fmt.Errorf("unsupported nested fast loop"))
			}
		case compiler.FastLoopPartBreak:
			return errFastLoopBreak
		case compiler.FastLoopPartContinue:
			return errFastLoopContinue
		default:
			return nil
		}
	}
	return nil
}

func renderFastLoopPartialPart(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, partial *compiler.FastPartialPlan, key, value interface{}) error {
	if partial == nil {
		return nil
	}
	scopedCtx, scopedBindings, cleanup := fastRenderScopedBindings(ctx, fastLoopBindingsWithCurrentLocals(bindings, loop, key, value))
	defer cleanup()
	if loop != nil && scopedCtx != nil {
		if fastLoopBindingName(loop.KeyName) {
			scopedCtx.Set(loop.KeyName, key)
		}
		if fastLoopBindingName(loop.ValueName) {
			scopedCtx.Set(loop.ValueName, value)
		}
	}
	return renderFastPartialSegment(out, scopedCtx, scopedBindings, partial)
}

func renderFastLoopConditional(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, conditional *compiler.FastLoopConditionalPlan, key, value interface{}) error {
	if conditional == nil {
		return nil
	}
	if conditional.Silent {
		return renderFastLoopConditionalSilently(ctx, bindings, loop, conditional, key, value)
	}
	for i := range conditional.Branches {
		branch := &conditional.Branches[i]
		if err := spendFastCondition(ctx, branch.Line); err != nil {
			return err
		}
		result, ok, err := evalFastLoopValue(&branch.Condition, ctx, bindings, key, value)
		if err != nil {
			return err
		}
		if !ok {
			result = nil
		}
		if isTruthyFastValue(result) {
			branchCtx, branchBindings, cleanup := fastRenderLoopPartScopeForLet(ctx, bindings, branch.Parts)
			defer cleanup()
			ctx = branchCtx
			bindings = branchBindings
			return renderFastLoopParts(out, ctx, bindings, loop, branch.Parts, key, value)
		}
	}
	if len(conditional.ElseParts) > 0 {
		branchCtx, branchBindings, cleanup := fastRenderLoopPartScopeForLet(ctx, bindings, conditional.ElseParts)
		defer cleanup()
		ctx = branchCtx
		bindings = branchBindings
		return renderFastLoopParts(out, ctx, bindings, loop, conditional.ElseParts, key, value)
	}
	return nil
}

func renderFastLoopConditionalSilently(ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, conditional *compiler.FastLoopConditionalPlan, key, value interface{}) error {
	if conditional == nil {
		return nil
	}
	for i := range conditional.Branches {
		branch := &conditional.Branches[i]
		if err := spendFastCondition(ctx, branch.Line); err != nil {
			return err
		}
		result, ok, err := evalFastLoopValue(&branch.Condition, ctx, bindings, key, value)
		if err != nil {
			return err
		}
		if !ok {
			result = nil
		}
		if isTruthyFastValue(result) {
			var discard strings.Builder
			return renderFastLoopParts(&discard, ctx, bindings, loop, branch.Parts, key, value)
		}
	}
	if len(conditional.ElseParts) > 0 {
		var discard strings.Builder
		return renderFastLoopParts(&discard, ctx, bindings, loop, conditional.ElseParts, key, value)
	}
	return nil
}

func fastLoopBindingsWithCurrentLocals(bindings fastRenderBindings, loop *compiler.FastLoopPlan, key, value interface{}) fastRenderBindings {
	if loop == nil || len(bindings.names) == 0 {
		return bindings
	}
	scoped := bindings
	if fastLoopBindingName(loop.KeyName) || fastLoopBindingName(loop.ValueName) {
		scoped = fastRenderBindingsWithLocalCopy(scoped)
	}
	if fastLoopBindingName(loop.KeyName) {
		scoped.setLocalByName(loop.KeyName, key)
	}
	if fastLoopBindingName(loop.ValueName) {
		scoped.setLocalByName(loop.ValueName, value)
	}
	return scoped
}

func fastRenderBindingsWithLocalCopy(bindings fastRenderBindings) fastRenderBindings {
	if len(bindings.localOK) == 0 {
		return bindings
	}
	localOK := make([]bool, len(bindings.localOK))
	localVals := make([]interface{}, len(bindings.localVals))
	copy(localOK, bindings.localOK)
	copy(localVals, bindings.localVals)
	bindings.localOK = localOK
	bindings.localVals = localVals
	return bindings
}

func fastLoopPartsHaveLet(parts []compiler.FastLoopPart) bool {
	for i := range parts {
		part := &parts[i]
		switch part.Kind {
		case compiler.FastLoopPartLet:
			return true
		case compiler.FastLoopPartConditional:
			if part.Conditional == nil {
				continue
			}
			for branchIndex := range part.Conditional.Branches {
				if fastLoopPartsHaveLet(part.Conditional.Branches[branchIndex].Parts) {
					return true
				}
			}
			if fastLoopPartsHaveLet(part.Conditional.ElseParts) {
				return true
			}
		}
	}
	return false
}

func fastRenderSegmentsHaveLet(segments []compiler.FastRenderSegment) bool {
	for i := range segments {
		segment := &segments[i]
		switch segment.Kind {
		case compiler.FastRenderSegmentLet:
			return true
		case compiler.FastRenderSegmentConditional:
			if segment.Conditional == nil {
				continue
			}
			for branchIndex := range segment.Conditional.Branches {
				if fastRenderSegmentsHaveLet(segment.Conditional.Branches[branchIndex].Segments) {
					return true
				}
			}
			if fastRenderSegmentsHaveLet(segment.Conditional.ElseSegments) {
				return true
			}
		}
	}
	return false
}

func fastRenderSegmentScopeForLet(ctx hctx.Context, bindings fastRenderBindings, segments []compiler.FastRenderSegment) (hctx.Context, fastRenderBindings, func()) {
	if !fastRenderSegmentsHaveLet(segments) {
		return ctx, bindings, func() {}
	}
	childCtx, scopedBindings, cleanup := fastRenderScopedBindings(ctx, bindings)
	scopedBindings = fastRenderBindingsWithLocalCopy(scopedBindings)
	scopedBindings.ensureLocalCapacity()
	return childCtx, scopedBindings, cleanup
}

func fastRenderLoopPartScopeForLet(ctx hctx.Context, bindings fastRenderBindings, parts []compiler.FastLoopPart) (hctx.Context, fastRenderBindings, func()) {
	if !fastLoopPartsHaveLet(parts) {
		return ctx, bindings, func() {}
	}
	childCtx, scopedBindings, cleanup := fastRenderScopedBindings(ctx, bindings)
	scopedBindings = fastRenderBindingsWithLocalCopy(scopedBindings)
	scopedBindings.ensureLocalCapacity()
	return childCtx, scopedBindings, cleanup
}

func (b *fastRenderBindings) setLocalByName(name string, value interface{}) bool {
	if b == nil || !fastLoopBindingName(name) {
		return false
	}
	for i := range b.names {
		if b.names[i] == name {
			b.setLocal(i, value)
			return true
		}
	}
	return false
}

func fastLoopBindingName(name string) bool {
	return name != "" && name != "_"
}

func writeFastLoopCallPart(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, call *compiler.FastCallPlan, loopKey, loopValue interface{}) error {
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
	var args *fastCallArgs
	var err error
	if helper, ok := fastHelperForContext(ctx, call.Name); ok {
		args, err = evalFastLoopCallArgsInto(call.Args, ctx, bindings, loopKey, loopValue, &argStore)
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
	if args == nil {
		args, err = evalFastLoopCallArgsInto(call.Args, ctx, bindings, loopKey, loopValue, &argStore)
		if err != nil {
			return err
		}
	}
	if err := writeFastCallValue(out, ctx, call.Name, raw, args, &call.Cache); err != nil {
		return fastLineError(call.Line, err)
	}
	return nil
}

func evalFastLoopCallArgsInto(plans []compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue interface{}, args *fastCallArgs) (*fastCallArgs, error) {
	if len(plans) == 0 {
		return nil, nil
	}
	if args == nil {
		args = &fastCallArgs{}
	}
	args.Reset()
	for i := range plans {
		value, ok, err := evalFastLoopValue(&plans[i], ctx, bindings, loopKey, loopValue)
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
