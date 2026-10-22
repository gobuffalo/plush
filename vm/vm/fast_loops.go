package vm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/vm/compiler"
	"github.com/gobuffalo/plush/v5/vm/object"
)

var (
	errFastLoopBreak    = errors.New("fast loop break")
	errFastLoopContinue = errors.New("fast loop continue")
	errFastLoopReturn   = errors.New("fast loop return")
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
			return renderFastScopedSegments(out, ctx, bindings, branch.Segments, branch.BindingSync)
		}
	}
	if len(conditional.ElseSegments) > 0 {
		return renderFastScopedSegments(out, ctx, bindings, conditional.ElseSegments, conditional.ElseBindingSync)
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
			return renderFastScopedSegments(&discard, ctx, bindings, branch.Segments, branch.BindingSync)
		}
	}
	if len(conditional.ElseSegments) > 0 {
		var discard strings.Builder
		return renderFastScopedSegments(&discard, ctx, bindings, conditional.ElseSegments, conditional.ElseBindingSync)
	}
	return true, nil
}

func renderFastScopedSegments(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, segments []compiler.FastRenderSegment, plan compiler.FastBindingSyncPlan) (bool, error) {
	outerBindings := bindings
	branchCtx, branchBindings, bindingUndo, scope := fastRenderSegmentScopeForLet(ctx, bindings, segments, plan)
	defer scope.release()

	handled, err := renderFastSegments(out, branchCtx, branchBindings, segments)
	syncFastSegmentAssignmentBindingsPlan(branchCtx, &outerBindings, segments, plan)
	bindingUndo.restorePlan(&branchBindings, &outerBindings, plan)
	return handled, err
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
	defer bindings.syncLocalValuesFromContext()
	if len(partial.Data) > 0 {
		if ok, err := renderFastDataPartialInto(out, partial, ctx, bindings, dataPlan); ok || err != nil {
			if err != nil {
				return err
			}
			return nil
		}
	}
	if ok, err := renderFastNoDataPartialIntoWithDiagnostics(out, partial.Name, ctx, partial.Line, bindings.vmHotspots); ok || err != nil {
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func renderFastLoop(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan) (handled bool, renderErr error) {
	if loop == nil {
		return false, nil
	}
	if loop.Silent {
		var discard strings.Builder
		out = &discard
	}
	hasLet, hasAssign := fastLoopPartFlags(loop)
	if hasLet {
		outerBindings := bindings
		scopedCtx, scopedBindings, scope := fastRenderScopedBindings(ctx, bindings)
		defer scope.release()
		var bindingUndo fastBindingUndo
		if loop.BindingSync.Prepared {
			bindingUndo.capturePlan(&scopedBindings, loop.BindingSync)
			defer bindingUndo.restorePlan(&scopedBindings, &outerBindings, loop.BindingSync)
		} else {
			scopedBindings = fastRenderBindingsWithLocalCopy(scopedBindings)
			scopedBindings.ensureLocalCapacity()
		}
		if hasAssign {
			defer syncFastLoopAssignmentBindingsPlan(scopedCtx, &outerBindings, loop.Parts, loop.BindingSync)
		}
		ctx = scopedCtx
		bindings = scopedBindings
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
	startLen := out.Len()
	renderedItems := 0
	itemCount, itemCountKnown := fastIterableLen(iter)
	outputSizeObservation := beginFastLoopSizeObservation(out, loop, itemCount, itemCountKnown)
	defer func() {
		if handled && renderErr == nil {
			observeFastLoopOutput(ctx, loop, out.Len()-startLen, renderedItems, outputSizeObservation)
		}
	}()

	if handled, err := renderFastPrimitiveValueLoop(out, ctx, loop, iter); handled || err != nil {
		if handled && err == nil {
			renderedItems = itemCount
		}
		return true, err
	}

	switch iter := iter.(type) {
	case []string:
		if handled, err := renderFastStringKeyValueLoop(out, ctx, loop, iter); handled || err != nil {
			if handled && err == nil {
				renderedItems = len(iter)
			}
			return true, err
		}
		for i, value := range iter {
			stop, err := renderFastLoopIterationOrControl(out, ctx, bindings, loop, i, value)
			if err != nil {
				return true, err
			}
			renderedItems++
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
			renderedItems++
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
			renderedItems++
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
		if rv.Kind() == reflect.Array {
			if handled, err := renderFastPrimitiveArrayLoop(out, ctx, loop, rv); handled || err != nil {
				if handled && err == nil {
					renderedItems = rv.Len()
				}
				return true, err
			}
		}
		if handled, err := renderFastStructFieldLoop(out, ctx, bindings, loop, rv); handled || err != nil {
			if handled && err == nil {
				renderedItems = rv.Len()
			}
			return true, err
		}
		for i := 0; i < rv.Len(); i++ {
			stop, err := renderFastLoopIterationOrControl(out, ctx, bindings, loop, i, rv.Index(i).Interface())
			if err != nil {
				return true, err
			}
			renderedItems++
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
			renderedItems++
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
	case errFastLoopReturn:
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
	currentKey := key
	currentValue := value
	for i := range parts {
		part := &parts[i]
		switch part.Kind {
		case compiler.FastLoopPartStatic:
			out.WriteString(part.Value)
		case compiler.FastLoopPartKey:
			writeFastGoValue(out, ctx, currentKey)
		case compiler.FastLoopPartValue:
			writeFastGoValue(out, ctx, currentValue)
		case compiler.FastLoopPartValueProperty:
			if err := spendFastTraversal(ctx, part.Line); err != nil {
				return err
			}
			if err := writeFastPropertyOutput(out, ctx, currentValue, part.Value, object.PropertyAccess{
				Receiver: part.Receiver,
				Full:     part.Full,
			}, &part.PropertyCache); err != nil {
				return fastLineError(part.Line, err)
			}
		case compiler.FastLoopPartValuePath:
			property, ok, err := evalFastLoopValue(&part.ValuePlan, ctx, bindings, currentKey, currentValue)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			writeFastGoValue(out, ctx, property)
		case compiler.FastLoopPartCall:
			if err := writeFastLoopCallPart(out, ctx, bindings, loop, part.Call, currentKey, currentValue); err != nil {
				return err
			}
		case compiler.FastLoopPartBlockCall:
			if err := writeFastLoopBlockCallPart(out, ctx, bindings, loop, part.BlockCall, currentKey, currentValue); err != nil {
				return err
			}
		case compiler.FastLoopPartPartial:
			err := renderFastLoopPartialPart(out, ctx, bindings, loop, part.Partial, currentKey, currentValue)
			bindings.syncLocalValuesFromContext()
			if err != nil {
				return err
			}
		case compiler.FastLoopPartLet:
			local, ok, err := evalFastLoopValue(&part.ValuePlan, ctx, bindings, currentKey, currentValue)
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
		case compiler.FastLoopPartAssign:
			assigned, err := assignFastLoopIteratorPartValue(ctx, bindings, loop, part, &currentKey, &currentValue)
			if err != nil {
				return err
			}
			if assigned {
				continue
			}
			if err := assignFastLoopPartValue(ctx, &bindings, part, currentKey, currentValue); err != nil {
				return err
			}
		case compiler.FastLoopPartReturn:
			value, ok, err := evalFastLoopValue(&part.ValuePlan, ctx, bindings, currentKey, currentValue)
			if err != nil {
				return err
			}
			if ok {
				writeFastGoValue(out, ctx, value)
			}
			return errFastLoopReturn
		case compiler.FastLoopPartConditional:
			if err := renderFastLoopConditional(out, ctx, bindings, loop, part.Conditional, currentKey, currentValue); err != nil {
				return err
			}
		case compiler.FastLoopPartLoop:
			ok, err := renderFastNestedLoop(out, ctx, bindings, loop, part.Loop, currentKey, currentValue)
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

func assignFastLoopIteratorPartValue(ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, part *compiler.FastLoopPart, key, value *interface{}) (bool, error) {
	if loop == nil || part == nil || part.AssignTarget == nil || part.AssignTarget.Kind != compiler.FastAssignTargetName {
		return false, nil
	}
	target := part.AssignTarget.Name
	if target != loop.KeyName && target != loop.ValueName {
		return false, nil
	}
	next, ok, err := evalFastLoopValue(&part.ValuePlan, ctx, bindings, *key, *value)
	if err != nil {
		return true, err
	}
	if !ok {
		return true, fastLineError(part.Line, fmt.Errorf("%q: unknown identifier", fastValueMissingName(&part.ValuePlan)))
	}
	if err := spendFastAssignment(ctx, part.Line); err != nil {
		return true, err
	}
	if target == loop.KeyName {
		*key = next
		return true, nil
	}
	*value = next
	return true, nil
}

func assignFastLoopPartValue(ctx hctx.Context, bindings *fastRenderBindings, part *compiler.FastLoopPart, key, loopValue interface{}) error {
	if part == nil {
		return nil
	}
	if part.AssignTarget == nil || part.AssignTarget.Kind == compiler.FastAssignTargetName {
		return assignFastLoopValue(ctx, bindings, part.Value, part.NameIndex, &part.ValuePlan, part.Line, key, loopValue)
	}
	return assignFastLoopIndexValue(ctx, bindings, part.AssignTarget, &part.ValuePlan, part.Line, key, loopValue)
}

func assignFastLoopValue(ctx hctx.Context, bindings *fastRenderBindings, name string, nameIndex int, valuePlan *compiler.FastValuePlan, line int, key, loopValue interface{}) error {
	if bindings == nil {
		return fastLineError(line, fmt.Errorf("%q: unknown identifier", name))
	}
	if _, ok := bindings.value(nameIndex); !ok {
		return fastLineError(line, fmt.Errorf("%q: unknown identifier", name))
	}
	value, ok, err := evalFastLoopValue(valuePlan, ctx, *bindings, key, loopValue)
	if err != nil {
		return err
	}
	if !ok {
		return fastLineError(line, fmt.Errorf("%q: unknown identifier", fastValueMissingName(valuePlan)))
	}
	if err := spendFastAssignment(ctx, line); err != nil {
		return err
	}
	if !bindings.assignExistingLocalAndContext(nameIndex, value) {
		return fastLineError(line, fmt.Errorf("%q: unknown identifier", name))
	}
	return nil
}

func assignFastLoopIndexValue(ctx hctx.Context, bindings *fastRenderBindings, target *compiler.FastAssignTarget, valuePlan *compiler.FastValuePlan, line int, key, loopValue interface{}) error {
	if bindings == nil || target == nil || target.Kind != compiler.FastAssignTargetIndex {
		return fastLineError(line, fmt.Errorf("unsupported assignment target"))
	}
	container, ok, err := evalFastLoopValue(&target.Container, ctx, *bindings, key, loopValue)
	if err != nil {
		return err
	}
	if !ok {
		return fastLineError(target.Line, fmt.Errorf("%q: unknown identifier", fastValueMissingName(&target.Container)))
	}
	index, ok, err := evalFastLoopValue(&target.Index, ctx, *bindings, key, loopValue)
	if err != nil {
		return err
	}
	if !ok {
		return fastLineError(target.Line, fmt.Errorf("%q: unknown identifier", fastValueMissingName(&target.Index)))
	}
	value, ok, err := evalFastLoopValue(valuePlan, ctx, *bindings, key, loopValue)
	if err != nil {
		return err
	}
	if !ok {
		return fastLineError(line, fmt.Errorf("%q: unknown identifier", fastValueMissingName(valuePlan)))
	}
	if err := spendFastAssignment(ctx, line); err != nil {
		return err
	}
	if err := setFastIndexGoValue(container, index, value); err != nil {
		return fastLineError(line, err)
	}
	return nil
}

func renderFastLoopPartialPart(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, partial *compiler.FastPartialPlan, key, value interface{}) error {
	if partial == nil {
		return nil
	}
	currentBindings, bindingUndo := fastLoopBindingsWithCurrentLocals(bindings, loop, key, value)
	defer bindingUndo.restore(&currentBindings)
	scopedCtx, scopedBindings, scope := fastRenderScopedBindings(ctx, currentBindings)
	defer scope.release()
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
		return renderFastLoopConditionalSilently(out, ctx, bindings, loop, conditional, key, value)
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
			return renderFastScopedLoopParts(out, ctx, bindings, loop, branch.Parts, branch.BindingSync, key, value)
		}
	}
	if len(conditional.ElseParts) > 0 {
		return renderFastScopedLoopParts(out, ctx, bindings, loop, conditional.ElseParts, conditional.ElseBindingSync, key, value)
	}
	return nil
}

func renderFastScopedLoopParts(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, parts []compiler.FastLoopPart, plan compiler.FastBindingSyncPlan, key, value interface{}) error {
	outerBindings := bindings
	branchCtx, branchBindings, bindingUndo, scope := fastRenderLoopPartScopeForLet(ctx, bindings, parts, plan)
	defer scope.release()

	err := renderFastLoopParts(out, branchCtx, branchBindings, loop, parts, key, value)
	syncFastLoopAssignmentBindingsPlan(branchCtx, &outerBindings, parts, plan)
	bindingUndo.restorePlan(&branchBindings, &outerBindings, plan)
	return err
}

func renderFastLoopConditionalSilently(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, conditional *compiler.FastLoopConditionalPlan, key, value interface{}) error {
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
			var rendered strings.Builder
			err := renderFastScopedLoopParts(&rendered, ctx, bindings, loop, branch.Parts, branch.BindingSync, key, value)
			if err == errFastLoopBreak || err == errFastLoopContinue || err == errFastLoopReturn {
				out.WriteString(rendered.String())
			}
			return err
		}
	}
	if len(conditional.ElseParts) > 0 {
		var rendered strings.Builder
		err := renderFastScopedLoopParts(&rendered, ctx, bindings, loop, conditional.ElseParts, conditional.ElseBindingSync, key, value)
		if err == errFastLoopBreak || err == errFastLoopContinue || err == errFastLoopReturn {
			out.WriteString(rendered.String())
		}
		return err
	}
	return nil
}

func syncFastLoopAssignmentBindings(ctx hctx.Context, bindings *fastRenderBindings, parts []compiler.FastLoopPart) {
	syncFastLoopAssignmentBindingsWithLets(ctx, bindings, parts, nil)
}

func syncFastLoopAssignmentBindingsPlan(ctx hctx.Context, bindings *fastRenderBindings, parts []compiler.FastLoopPart, plan compiler.FastBindingSyncPlan) {
	if !plan.Prepared {
		syncFastLoopAssignmentBindings(ctx, bindings, parts)
		return
	}
	syncPreparedFastAssignmentBindings(ctx, bindings, plan.NameIndexes)
}

func syncFastSegmentAssignmentBindings(ctx hctx.Context, bindings *fastRenderBindings, segments []compiler.FastRenderSegment) {
	syncFastSegmentAssignmentBindingsWithLets(ctx, bindings, segments, nil)
}

func syncFastSegmentAssignmentBindingsPlan(ctx hctx.Context, bindings *fastRenderBindings, segments []compiler.FastRenderSegment, plan compiler.FastBindingSyncPlan) {
	if !plan.Prepared {
		syncFastSegmentAssignmentBindings(ctx, bindings, segments)
		return
	}
	syncPreparedFastAssignmentBindings(ctx, bindings, plan.NameIndexes)
}

func syncPreparedFastAssignmentBindings(ctx hctx.Context, bindings *fastRenderBindings, nameIndexes []int) {
	if ctx == nil || bindings == nil {
		return
	}
	for _, index := range nameIndexes {
		if index < 0 || index >= len(bindings.names) {
			continue
		}
		if value, ok := fastContextValue(ctx, bindings.names[index]); ok {
			bindings.setLocal(index, value)
		}
	}
}

func syncFastSegmentAssignmentBindingsWithLets(ctx hctx.Context, bindings *fastRenderBindings, segments []compiler.FastRenderSegment, localLets map[string]struct{}) {
	if ctx == nil || bindings == nil {
		return
	}
	letNames := cloneFastLoopSyncNames(localLets)
	for i := range segments {
		segment := &segments[i]
		switch segment.Kind {
		case compiler.FastRenderSegmentLet:
			if segment.Value != "" {
				letNames[segment.Value] = struct{}{}
			}
		case compiler.FastRenderSegmentAssign:
			if segment.AssignTarget != nil && segment.AssignTarget.Kind != compiler.FastAssignTargetName {
				continue
			}
			if _, local := letNames[segment.Value]; local {
				continue
			}
			if value, ok := fastContextValue(ctx, segment.Value); ok {
				bindings.setLocal(segment.NameIndex, value)
			}
		case compiler.FastRenderSegmentConditional:
			if segment.Conditional == nil {
				continue
			}
			for branchIndex := range segment.Conditional.Branches {
				syncFastSegmentAssignmentBindingsWithLets(ctx, bindings, segment.Conditional.Branches[branchIndex].Segments, letNames)
			}
			syncFastSegmentAssignmentBindingsWithLets(ctx, bindings, segment.Conditional.ElseSegments, letNames)
		case compiler.FastRenderSegmentLoop:
			if segment.Loop != nil {
				syncFastLoopAssignmentBindingsWithLets(ctx, bindings, segment.Loop.Parts, letNames)
			}
		}
	}
}

func syncFastLoopAssignmentBindingsWithLets(ctx hctx.Context, bindings *fastRenderBindings, parts []compiler.FastLoopPart, localLets map[string]struct{}) {
	if ctx == nil || bindings == nil {
		return
	}
	letNames := cloneFastLoopSyncNames(localLets)
	for i := range parts {
		part := &parts[i]
		switch part.Kind {
		case compiler.FastLoopPartLet:
			if part.Value != "" {
				letNames[part.Value] = struct{}{}
			}
		case compiler.FastLoopPartAssign:
			if part.AssignTarget != nil && part.AssignTarget.Kind != compiler.FastAssignTargetName {
				continue
			}
			if _, local := letNames[part.Value]; local {
				continue
			}
			if value, ok := fastContextValue(ctx, part.Value); ok {
				bindings.setLocal(part.NameIndex, value)
			}
		case compiler.FastLoopPartConditional:
			if part.Conditional == nil {
				continue
			}
			for branchIndex := range part.Conditional.Branches {
				syncFastLoopAssignmentBindingsWithLets(ctx, bindings, part.Conditional.Branches[branchIndex].Parts, letNames)
			}
			syncFastLoopAssignmentBindingsWithLets(ctx, bindings, part.Conditional.ElseParts, letNames)
		case compiler.FastLoopPartLoop:
			if part.Loop != nil {
				syncFastLoopAssignmentBindingsWithLets(ctx, bindings, part.Loop.Parts, letNames)
			}
		}
	}
}

func cloneFastLoopSyncNames(names map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(names)+1)
	for name := range names {
		clone[name] = struct{}{}
	}
	return clone
}

func fastLoopBindingsWithCurrentLocals(bindings fastRenderBindings, loop *compiler.FastLoopPlan, key, value interface{}) (fastRenderBindings, fastBindingUndo) {
	var undo fastBindingUndo
	if loop == nil || len(bindings.names) == 0 {
		return bindings, undo
	}
	scoped := bindings
	if fastLoopBindingName(loop.KeyName) {
		setFastTemporaryLocal(&scoped, &undo, loop.KeyName, key)
	}
	if fastLoopBindingName(loop.ValueName) {
		setFastTemporaryLocal(&scoped, &undo, loop.ValueName, value)
	}
	return scoped, undo
}

func setFastTemporaryLocal(bindings *fastRenderBindings, undo *fastBindingUndo, name string, value interface{}) {
	if bindings == nil {
		return
	}
	index := fastBindingNameIndex(bindings.names, name)
	if index < 0 {
		return
	}
	if len(bindings.localOK) > 0 {
		undo.capture(bindings, index)
	}
	bindings.setLocal(index, value)
}

func renderFastNestedLoop(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, parent, nested *compiler.FastLoopPlan, key, value interface{}) (bool, error) {
	nestedBindings, bindingUndo := fastLoopBindingsWithCurrentLocals(bindings, parent, key, value)
	defer bindingUndo.restore(&nestedBindings)
	handled, err := renderFastLoop(out, ctx, nestedBindings, nested)
	if nested != nil {
		syncFastLoopAssignmentBindingsPlan(ctx, &bindings, nested.Parts, nested.BindingSync)
	}
	return handled, err
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

func fastLoopPartsHaveAssign(parts []compiler.FastLoopPart) bool {
	for i := range parts {
		part := &parts[i]
		switch part.Kind {
		case compiler.FastLoopPartAssign:
			return true
		case compiler.FastLoopPartConditional:
			if part.Conditional == nil {
				continue
			}
			for branchIndex := range part.Conditional.Branches {
				if fastLoopPartsHaveAssign(part.Conditional.Branches[branchIndex].Parts) {
					return true
				}
			}
			if fastLoopPartsHaveAssign(part.Conditional.ElseParts) {
				return true
			}
		case compiler.FastLoopPartLoop:
			if part.Loop != nil && fastLoopPartsHaveAssign(part.Loop.Parts) {
				return true
			}
		}
	}
	return false
}

func fastLoopPartFlags(loop *compiler.FastLoopPlan) (bool, bool) {
	if loop == nil {
		return false, false
	}
	if loop.PartFlagsSet {
		return loop.HasLet, loop.HasAssign
	}
	return fastLoopPartsHaveLet(loop.Parts), fastLoopPartsHaveAssign(loop.Parts)
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

func fastRenderSegmentScopeForLet(ctx hctx.Context, bindings fastRenderBindings, segments []compiler.FastRenderSegment, plan compiler.FastBindingSyncPlan) (hctx.Context, fastRenderBindings, fastBindingUndo, partialChildContextScope) {
	var undo fastBindingUndo
	if !fastRenderSegmentsHaveLet(segments) {
		return ctx, bindings, undo, partialChildContextScope{}
	}
	childCtx, scopedBindings, scope := fastRenderScopedBindings(ctx, bindings)
	if plan.Prepared {
		undo.capturePlan(&scopedBindings, plan)
	} else {
		scopedBindings = fastRenderBindingsWithLocalCopy(scopedBindings)
		scopedBindings.ensureLocalCapacity()
	}
	return childCtx, scopedBindings, undo, scope
}

func fastRenderLoopPartScopeForLet(ctx hctx.Context, bindings fastRenderBindings, parts []compiler.FastLoopPart, plan compiler.FastBindingSyncPlan) (hctx.Context, fastRenderBindings, fastBindingUndo, partialChildContextScope) {
	var undo fastBindingUndo
	if !fastLoopPartsHaveLet(parts) {
		return ctx, bindings, undo, partialChildContextScope{}
	}
	childCtx, scopedBindings, scope := fastRenderScopedBindings(ctx, bindings)
	if plan.Prepared {
		undo.capturePlan(&scopedBindings, plan)
	} else {
		scopedBindings = fastRenderBindingsWithLocalCopy(scopedBindings)
		scopedBindings.ensureLocalCapacity()
	}
	return childCtx, scopedBindings, undo, scope
}

func (b *fastRenderBindings) setLocalByName(name string, value interface{}) bool {
	if b == nil || !fastLoopBindingName(name) {
		return false
	}
	index := fastBindingNameIndex(b.names, name)
	if index < 0 {
		return false
	}
	b.setLocal(index, value)
	return true
}

func fastBindingNameIndex(names []string, name string) int {
	for i := range names {
		if names[i] == name {
			return i
		}
	}
	return -1
}

func fastLoopBindingName(name string) bool {
	return name != "" && name != "_"
}

func writeFastLoopCallPart(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, loop *compiler.FastLoopPlan, call *compiler.FastCallPlan, loopKey, loopValue interface{}) error {
	if call == nil {
		return nil
	}
	writeOut := out
	if call.Silent {
		var discard strings.Builder
		writeOut = &discard
	}
	raw, ok := bindings.value(call.NameIndex)
	if !ok {
		return fastLineError(call.Line, fmt.Errorf("%q: unknown identifier", call.Name))
	}
	if err := spendFastFunctionCall(ctx, call.Name, call.Line); err != nil {
		return err
	}
	callCtx := fastLoopBlockContext(ctx, bindings, loop, loopKey, loopValue)
	var argStore fastCallArgs
	var args *fastCallArgs
	var err error
	if helper, ok := fastHelperForContext(ctx, call.Name); ok {
		args, err = evalFastLoopCallArgsInto(call.Args, ctx, bindings, loopKey, loopValue, &argStore)
		if err != nil {
			return err
		}
		if handled, err := writeRegisteredFastHelperNamed(writeOut, callCtx, call.Name, helper, args, bindings.vmHotspots); handled || err != nil {
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
	if err := writeFastCallValueWithDiagnostics(writeOut, callCtx, call.Name, raw, args, &call.Cache, bindings.vmHotspots); err != nil {
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
			if plans[i].NullOnMissing {
				args.Append(nil)
				continue
			}
			return nil, fastLineError(plans[i].Line, fmt.Errorf("%q: unknown identifier", plans[i].Value))
		}
		args.Append(value)
	}
	return args, nil
}
