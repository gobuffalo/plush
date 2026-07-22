package vm

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

// tryRenderFastBytecode is the VM fast-render entry point. A false handled
// result means the caller should run the normal bytecode VM path.
func tryRenderFastBytecode(bytecode *compiler.Bytecode, ctx hctx.Context) (string, bool, error) {
	if bytecode == nil || bytecode.FastRenderPlan == nil {
		return "", false, nil
	}
	return renderFastPlanWithBindingPlan(bytecode.FastRenderPlan, ctx, topLevelFastBindingPlan(bytecode.FastRenderPlan, ctx))
}

// renderFastPlanWithBindingPlan tries the prepared fast plan variants in order:
// static-name, simple, mixed. Each variant must preserve Plush rendering
// semantics or decline so the normal VM can handle the template.
func renderFastPlanWithBindingPlan(plan *compiler.FastRenderPlan, ctx hctx.Context, bindingPlan *fastRenderBindingPlan) (string, bool, error) {
	if plan == nil {
		return "", false, nil
	}

	mixed := prepareFastMixedPlan(plan)
	bindings := newFastRenderBindingsWithPlan(plan, ctx, bindingPlan)
	var out strings.Builder
	if grow := fastOutputGrowSize(mixed, bindings); grow > 0 {
		out.Grow(grow)
	}

	if mixed.staticName != nil {
		ok, err := renderFastStaticNamePlan(&out, ctx, bindings, mixed.staticName)
		if !ok || err != nil {
			return "", ok, err
		}
		return out.String(), true, nil
	}

	if mixed.simple != nil {
		ok, err := renderFastSimplePlan(&out, ctx, bindings, mixed.simple)
		if err != nil {
			return "", true, err
		}
		if ok {
			return out.String(), true, nil
		}
		out.Reset()
		if grow := fastOutputGrowSize(mixed, bindings); grow > 0 {
			out.Grow(grow)
		}
	}

	ok, err := renderFastMixedPlan(&out, ctx, bindings, mixed)
	if !ok || err != nil {
		return "", ok, err
	}

	return out.String(), true, nil
}

func renderFastPlanInlineSafe(out *strings.Builder, plan *compiler.FastRenderPlan, ctx hctx.Context, bindingPlan *fastRenderBindingPlan) (bool, error) {
	if out == nil || plan == nil {
		return false, nil
	}
	bindings := newFastRenderBindingsWithPlan(plan, ctx, bindingPlan)
	return renderFastPlanInlineWithBindings(out, plan, ctx, bindings)
}

func renderFastPlanInlineWithBindings(out *strings.Builder, plan *compiler.FastRenderPlan, ctx hctx.Context, bindings fastRenderBindings) (bool, error) {
	if out == nil || plan == nil {
		return false, nil
	}
	mixed := prepareFastMixedPlan(plan)
	if mixed == nil || len(mixed.ops) == 0 {
		return false, nil
	}
	if mixed.staticName != nil {
		return renderFastStaticNamePlanInlineSafe(out, ctx, bindings, mixed.staticName)
	}
	if mixed.simple != nil {
		return renderFastSimplePlanInlineSafe(out, ctx, bindings, mixed.simple)
	}
	var scratch strings.Builder
	if grow := fastOutputGrowSize(mixed, bindings); grow > 0 {
		scratch.Grow(grow)
	}
	ok, err := renderFastMixedPlan(&scratch, ctx, bindings, mixed)
	if !ok || err != nil {
		return ok, err
	}
	out.WriteString(scratch.String())
	return true, nil
}

func topLevelFastBindingPlan(plan *compiler.FastRenderPlan, ctx hctx.Context) *fastRenderBindingPlan {
	if plan == nil || len(plan.Bindings) == 0 || !canCacheFastRenderBindingPlan(ctx) {
		return nil
	}
	if cached := plan.BindingPrepared.Load(); cached != nil {
		bindingPlan, _ := cached.(*fastRenderBindingPlan)
		if bindingPlan != nil && bindingPlan.matches(plan.Bindings) {
			return bindingPlan
		}
	}
	bindingPlan := newFastRenderBindingPlan(plan, ctx)
	plan.BindingPrepared.Store(bindingPlan)
	return bindingPlan
}

func canCacheFastRenderBindingPlan(ctx hctx.Context) bool {
	switch c := ctx.(type) {
	case nil:
		return false
	case *plush.Context:
		return true
	case *partialOverlayContext:
		return c.stableBindingIDs()
	default:
		return false
	}
}

func fastOutputGrowSize(plan *fastMixedPlan, bindings fastRenderBindings) int {
	if plan == nil {
		return 0
	}
	size := plan.staticSize + plan.nameCount*16
	for i := range plan.ops {
		op := &plan.ops[i]
		if op.loop == nil {
			continue
		}
		iter, ok, err := fastLoopIterableValue(op.loop, bindings.ctx, bindings)
		if err != nil || !ok {
			continue
		}
		length, ok := fastIterableLen(iter)
		if !ok || length <= 0 {
			continue
		}
		perItem := fastLoopGrowSize(op.loop)
		maxInt := int(^uint(0) >> 1)
		if perItem <= 0 || length > (maxInt-size)/perItem {
			continue
		}
		size += length * perItem
	}
	return size
}

func fastLoopGrowSize(loop *compiler.FastLoopPlan) int {
	if loop == nil {
		return 0
	}
	size := loop.StaticSize
	for i := range loop.Parts {
		switch loop.Parts[i].Kind {
		case compiler.FastLoopPartKey:
			size += 4
		case compiler.FastLoopPartValue, compiler.FastLoopPartValueProperty, compiler.FastLoopPartValuePath, compiler.FastLoopPartCall, compiler.FastLoopPartPartial:
			size += 16
		case compiler.FastLoopPartConditional:
			size += fastLoopConditionalGrowSize(loop.Parts[i].Conditional)
		case compiler.FastLoopPartLoop:
			size += fastLoopGrowSize(loop.Parts[i].Loop)
		}
	}
	return size
}

func fastLoopConditionalGrowSize(conditional *compiler.FastLoopConditionalPlan) int {
	if conditional == nil {
		return 0
	}
	size := 0
	for i := range conditional.Branches {
		if branchSize := fastLoopPartsGrowSize(conditional.Branches[i].Parts); branchSize > size {
			size = branchSize
		}
	}
	if elseSize := fastLoopPartsGrowSize(conditional.ElseParts); elseSize > size {
		size = elseSize
	}
	return size
}

func fastLoopPartsGrowSize(parts []compiler.FastLoopPart) int {
	size := 0
	for i := range parts {
		switch parts[i].Kind {
		case compiler.FastLoopPartStatic:
			continue
		case compiler.FastLoopPartKey:
			size += 4
		case compiler.FastLoopPartValue, compiler.FastLoopPartValueProperty, compiler.FastLoopPartValuePath, compiler.FastLoopPartCall, compiler.FastLoopPartPartial:
			size += 16
		case compiler.FastLoopPartConditional:
			size += fastLoopConditionalGrowSize(parts[i].Conditional)
		}
	}
	return size
}

func fastIterableLen(iter interface{}) (int, bool) {
	if obj, ok := iter.(object.Object); ok {
		switch obj := obj.(type) {
		case *object.Array:
			return len(obj.Elements), true
		case *object.Hash:
			return len(obj.Pairs), true
		default:
			iter = object.ToGo(obj)
		}
	}
	if iter == nil {
		return 0, true
	}
	switch iter := iter.(type) {
	case []string:
		return len(iter), true
	case []interface{}:
		return len(iter), true
	case []object.Object:
		return len(iter), true
	}
	rv := reflect.ValueOf(iter)
	rv = unwrapFastFieldChainValue(rv)
	if !rv.IsValid() {
		return 0, true
	}
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		return rv.Len(), true
	default:
		return 0, false
	}
}

func prepareFastMixedPlan(plan *compiler.FastRenderPlan) *fastMixedPlan {
	if plan == nil {
		return nil
	}
	if cached := plan.Prepared.Load(); cached != nil {
		return cached.(*fastMixedPlan)
	}
	prepared := buildFastMixedPlan(plan)
	plan.Prepared.Store(prepared)
	return prepared
}

func buildFastMixedPlan(plan *compiler.FastRenderPlan) *fastMixedPlan {
	mixed := &fastMixedPlan{
		staticSize: plan.StaticSize,
		nameCount:  plan.NameCount,
	}
	prefix := ""
	for i := range plan.Segments {
		segment := &plan.Segments[i]
		if segment.Kind == compiler.FastRenderSegmentStatic {
			prefix += segment.Value
			continue
		}
		op := fastMixedOp{
			prefix:        prefix,
			value:         segment.Value,
			nameIndex:     segment.NameIndex,
			nullOnMissing: segment.NullOnMissing,
			property:      segment.Property,
			receiver:      segment.Receiver,
			full:          segment.Full,
			line:          segment.Line,
			loop:          segment.Loop,
			valuePlan:     segment.ValuePlan,
			call:          segment.Call,
			blockCall:     segment.BlockCall,
			conditional:   segment.Conditional,
			partial:       segment.Partial,
			propertyCache: segment.PropertyCache,
			outputCache:   segment.OutputCache,
		}
		prefix = ""
		switch segment.Kind {
		case compiler.FastRenderSegmentName:
			op.kind = fastMixedOpName
		case compiler.FastRenderSegmentProperty:
			op.kind = fastMixedOpProperty
		case compiler.FastRenderSegmentValue:
			if canUseFastTopLevelAccessChain(&segment.ValuePlan) {
				op.kind = fastMixedOpAccessChain
			} else {
				op.kind = fastMixedOpValue
			}
		case compiler.FastRenderSegmentCall:
			op.kind = fastMixedOpCall
		case compiler.FastRenderSegmentBlockCall:
			op.kind = fastMixedOpBlockCall
		case compiler.FastRenderSegmentConditional:
			op.kind = fastMixedOpConditional
			op.simpleCond = buildFastSimpleConditionalPlan(segment.Conditional)
		case compiler.FastRenderSegmentPartial:
			op.kind = fastMixedOpPartial
			op.partialData = buildFastPartialDataBindingPlan(segment.Partial)
		case compiler.FastRenderSegmentLoop:
			op.kind = fastMixedOpLoop
		case compiler.FastRenderSegmentLet:
			op.kind = fastMixedOpLet
		case compiler.FastRenderSegmentAssign:
			op.kind = fastMixedOpAssign
		default:
			op.kind = fastMixedOpStatic
		}
		mixed.ops = append(mixed.ops, op)
	}
	if prefix != "" {
		mixed.ops = append(mixed.ops, fastMixedOp{kind: fastMixedOpStatic, prefix: prefix})
	}
	mixed.staticName = buildFastStaticNamePlan(mixed)
	if mixed.staticName == nil {
		mixed.simple = buildFastSimplePlan(mixed)
	}
	return mixed
}

func buildFastStaticNamePlan(mixed *fastMixedPlan) *fastStaticNamePlan {
	if mixed == nil || len(mixed.ops) == 0 {
		return nil
	}
	plan := &fastStaticNamePlan{ops: make([]fastStaticNameOp, 0, len(mixed.ops))}
	for i := range mixed.ops {
		op := &mixed.ops[i]
		switch op.kind {
		case fastMixedOpStatic:
			if op.prefix != "" {
				plan.ops = append(plan.ops, fastStaticNameOp{
					prefix:      op.prefix,
					nameIndex:   -1,
					lookupIndex: -1,
				})
			}
		case fastMixedOpName:
			lookupIndex := plan.bindNameIndex(op.nameIndex)
			plan.ops = append(plan.ops, fastStaticNameOp{
				prefix:        op.prefix,
				value:         op.value,
				nameIndex:     op.nameIndex,
				lookupIndex:   lookupIndex,
				nullOnMissing: op.nullOnMissing,
				line:          op.line,
				outputCache:   &op.outputCache,
			})
		default:
			return nil
		}
	}
	if len(plan.ops) == 0 {
		return nil
	}
	return plan
}

func (p *fastStaticNamePlan) bindNameIndex(nameIndex int) int {
	if p == nil || nameIndex < 0 {
		return -1
	}
	for i, existing := range p.nameIndexes {
		if existing == nameIndex {
			return i
		}
	}
	p.nameIndexes = append(p.nameIndexes, nameIndex)
	return len(p.nameIndexes) - 1
}

func buildFastSimplePlan(mixed *fastMixedPlan) *fastSimplePlan {
	if mixed == nil || len(mixed.ops) == 0 {
		return nil
	}
	plan := &fastSimplePlan{ops: make([]fastSimpleOp, 0, len(mixed.ops))}
	for i := range mixed.ops {
		op := &mixed.ops[i]
		simpleOp := fastSimpleOp{op: op, lookupIndex: -1}
		switch op.kind {
		case fastMixedOpStatic:
			if op.prefix == "" {
				continue
			}
		case fastMixedOpName, fastMixedOpProperty:
			simpleOp.lookupIndex = plan.bindNameIndex(op.nameIndex)
		case fastMixedOpAccessChain:
			if op.valuePlan.NameIndex < 0 {
				return nil
			}
			simpleOp.lookupIndex = plan.bindNameIndex(op.valuePlan.NameIndex)
		case fastMixedOpValue:
			if op.valuePlan.Kind != compiler.FastValueInfix {
				return nil
			}
			simpleOp.value = buildFastSimpleValuePlan(plan, &op.valuePlan)
			if simpleOp.value == nil {
				return nil
			}
		default:
			return nil
		}
		plan.ops = append(plan.ops, simpleOp)
	}
	if len(plan.ops) == 0 {
		return nil
	}
	return plan
}

func buildFastSimpleConditionalPlan(conditional *compiler.FastConditionalPlan) *fastSimpleConditionalPlan {
	if conditional == nil || conditional.Silent || len(conditional.Branches) == 0 {
		return nil
	}
	plan := &fastSimpleConditionalPlan{
		branches: make([]fastSimpleConditionalBranch, 0, len(conditional.Branches)),
	}
	for i := range conditional.Branches {
		branch := &conditional.Branches[i]
		condition := buildFastSimpleValuePlan(plan, &branch.Condition)
		if condition == nil {
			return nil
		}
		segments := buildFastSimplePlanFromSegments(branch.Segments)
		if len(branch.Segments) > 0 && segments == nil {
			return nil
		}
		plan.branches = append(plan.branches, fastSimpleConditionalBranch{
			condition: condition,
			segments:  segments,
			line:      branch.Line,
		})
	}
	if len(conditional.ElseSegments) > 0 {
		plan.elseSegments = buildFastSimplePlanFromSegments(conditional.ElseSegments)
		if plan.elseSegments == nil {
			return nil
		}
	}
	return plan
}

func buildFastPartialDataBindingPlan(partial *compiler.FastPartialPlan) *fastPartialDataBindingPlan {
	if partial == nil || len(partial.Data) == 0 {
		return nil
	}
	plan := &fastPartialDataBindingPlan{
		pairs: make([]fastPartialDataBindingPair, 0, len(partial.Data)),
		keys:  make([]string, 0, len(partial.Data)),
	}
	for i := range partial.Data {
		pair := &partial.Data[i]
		value := buildFastSimpleValuePlan(plan, &pair.Value)
		if value == nil {
			return nil
		}
		plan.pairs = append(plan.pairs, fastPartialDataBindingPair{
			key:   pair.Key,
			value: value,
			line:  pair.Line,
		})
		plan.keys = append(plan.keys, pair.Key)
	}
	return plan
}

func buildFastSimplePlanFromSegments(segments []compiler.FastRenderSegment) *fastSimplePlan {
	if len(segments) == 0 {
		return nil
	}
	mixed := &fastMixedPlan{ops: fastMixedOpsFromSegments(segments)}
	return buildFastSimplePlan(mixed)
}

func fastMixedOpsFromSegments(segments []compiler.FastRenderSegment) []fastMixedOp {
	ops := make([]fastMixedOp, 0, len(segments))
	prefix := ""
	for i := range segments {
		segment := &segments[i]
		if segment.Kind == compiler.FastRenderSegmentStatic {
			prefix += segment.Value
			continue
		}
		op := fastMixedOp{
			prefix:        prefix,
			value:         segment.Value,
			nameIndex:     segment.NameIndex,
			nullOnMissing: segment.NullOnMissing,
			property:      segment.Property,
			receiver:      segment.Receiver,
			full:          segment.Full,
			line:          segment.Line,
			loop:          segment.Loop,
			valuePlan:     segment.ValuePlan,
			call:          segment.Call,
			blockCall:     segment.BlockCall,
			conditional:   segment.Conditional,
			partial:       segment.Partial,
			propertyCache: segment.PropertyCache,
			outputCache:   segment.OutputCache,
		}
		prefix = ""
		switch segment.Kind {
		case compiler.FastRenderSegmentName:
			op.kind = fastMixedOpName
		case compiler.FastRenderSegmentProperty:
			op.kind = fastMixedOpProperty
		case compiler.FastRenderSegmentValue:
			if canUseFastTopLevelAccessChain(&segment.ValuePlan) {
				op.kind = fastMixedOpAccessChain
			} else {
				op.kind = fastMixedOpValue
			}
		case compiler.FastRenderSegmentCall:
			op.kind = fastMixedOpCall
		case compiler.FastRenderSegmentBlockCall:
			op.kind = fastMixedOpBlockCall
		case compiler.FastRenderSegmentConditional:
			op.kind = fastMixedOpConditional
			op.simpleCond = buildFastSimpleConditionalPlan(segment.Conditional)
		case compiler.FastRenderSegmentPartial:
			op.kind = fastMixedOpPartial
			op.partialData = buildFastPartialDataBindingPlan(segment.Partial)
		case compiler.FastRenderSegmentLoop:
			op.kind = fastMixedOpLoop
		case compiler.FastRenderSegmentLet:
			op.kind = fastMixedOpLet
		case compiler.FastRenderSegmentAssign:
			op.kind = fastMixedOpAssign
		default:
			op.kind = fastMixedOpStatic
		}
		ops = append(ops, op)
	}
	if prefix != "" {
		ops = append(ops, fastMixedOp{kind: fastMixedOpStatic, prefix: prefix})
	}
	return ops
}

func buildFastSimpleValuePlan(plan fastSimpleNameBinder, value *compiler.FastValuePlan) *fastSimpleValuePlan {
	if plan == nil || value == nil {
		return nil
	}
	prepared := &fastSimpleValuePlan{
		value:       value,
		lookupIndex: -1,
	}
	switch value.Kind {
	case compiler.FastValueName:
		if value.Value != "nil" {
			prepared.lookupIndex = plan.bindNameIndex(value.NameIndex)
		}
	case compiler.FastValuePath:
		prepared.lookupIndex = plan.bindNameIndex(value.NameIndex)
	case compiler.FastValueInfix:
		if value.Left == nil || value.Right == nil {
			return nil
		}
		prepared.left = buildFastSimpleValuePlan(plan, value.Left)
		prepared.right = buildFastSimpleValuePlan(plan, value.Right)
		if prepared.left == nil || prepared.right == nil {
			return nil
		}
	case compiler.FastValuePrefix:
		if value.Right == nil {
			return nil
		}
		prepared.right = buildFastSimpleValuePlan(plan, value.Right)
		if prepared.right == nil {
			return nil
		}
	case compiler.FastValueConcat:
		if value.Left == nil || value.Right == nil {
			return nil
		}
		prepared.left = buildFastSimpleValuePlan(plan, value.Left)
		prepared.right = buildFastSimpleValuePlan(plan, value.Right)
		if prepared.left == nil || prepared.right == nil {
			return nil
		}
	case compiler.FastValueCall:
		if value.Call == nil {
			return nil
		}
		prepared.lookupIndex = plan.bindNameIndex(value.Call.NameIndex)
		if len(value.Call.Args) > 0 {
			prepared.args = make([]*fastSimpleValuePlan, 0, len(value.Call.Args))
			for i := range value.Call.Args {
				arg := buildFastSimpleValuePlan(plan, &value.Call.Args[i])
				if arg == nil {
					return nil
				}
				prepared.args = append(prepared.args, arg)
			}
		}
	case compiler.FastValueArray:
		if len(value.Elements) > 0 {
			prepared.args = make([]*fastSimpleValuePlan, 0, len(value.Elements))
			for i := range value.Elements {
				element := buildFastSimpleValuePlan(plan, &value.Elements[i])
				if element == nil {
					return nil
				}
				prepared.args = append(prepared.args, element)
			}
		}
	case compiler.FastValueHash:
		if len(value.Pairs) > 0 {
			prepared.args = make([]*fastSimpleValuePlan, 0, len(value.Pairs))
			for i := range value.Pairs {
				pairValue := buildFastSimpleValuePlan(plan, &value.Pairs[i].Value)
				if pairValue == nil {
					return nil
				}
				prepared.args = append(prepared.args, pairValue)
			}
		}
	case compiler.FastValueString, compiler.FastValueInteger, compiler.FastValueFloat, compiler.FastValueBool:
		// Literal operands do not need binding slots.
	default:
		return nil
	}
	return prepared
}

func (p *fastSimplePlan) bindNameIndex(nameIndex int) int {
	if p == nil || nameIndex < 0 {
		return -1
	}
	for i, existing := range p.nameIndexes {
		if existing == nameIndex {
			return i
		}
	}
	p.nameIndexes = append(p.nameIndexes, nameIndex)
	return len(p.nameIndexes) - 1
}

func (p *fastSimpleConditionalPlan) bindNameIndex(nameIndex int) int {
	if p == nil || nameIndex < 0 {
		return -1
	}
	for i, existing := range p.nameIndexes {
		if existing == nameIndex {
			return i
		}
	}
	p.nameIndexes = append(p.nameIndexes, nameIndex)
	return len(p.nameIndexes) - 1
}

func (p *fastPartialDataBindingPlan) bindNameIndex(nameIndex int) int {
	if p == nil || nameIndex < 0 {
		return -1
	}
	for i, existing := range p.nameIndexes {
		if existing == nameIndex {
			return i
		}
	}
	p.nameIndexes = append(p.nameIndexes, nameIndex)
	return len(p.nameIndexes) - 1
}

func renderFastStaticNamePlan(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStaticNamePlan) (bool, error) {
	if plan == nil {
		return false, nil
	}
	var cachedValues [8]interface{}
	var cachedOK [8]bool
	cacheValues := len(plan.nameIndexes) > 0 && len(plan.nameIndexes) <= len(cachedValues)
	if cacheValues {
		for i, nameIndex := range plan.nameIndexes {
			cachedValues[i], cachedOK[i] = bindings.value(nameIndex)
		}
	}
	for i := range plan.ops {
		op := &plan.ops[i]
		if op.prefix != "" {
			out.WriteString(op.prefix)
		}
		if op.nameIndex < 0 {
			continue
		}
		var value interface{}
		var ok bool
		if cacheValues && op.lookupIndex >= 0 {
			value, ok = cachedValues[op.lookupIndex], cachedOK[op.lookupIndex]
		} else {
			value, ok = bindings.value(op.nameIndex)
		}
		if !ok {
			if op.nullOnMissing {
				continue
			}
			return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.value))
		}
		if !writeFastBindingOutput(out, ctx, value, op.outputCache) {
			return false, nil
		}
	}
	return true, nil
}

func renderFastStaticNamePlanInlineSafe(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastStaticNamePlan) (bool, error) {
	if out == nil || plan == nil {
		return false, nil
	}
	var inlineValues [8]interface{}
	var inlineOK [8]bool
	values := inlineValues[:]
	oks := inlineOK[:]
	if len(plan.nameIndexes) > len(inlineValues) {
		values = make([]interface{}, len(plan.nameIndexes))
		oks = make([]bool, len(plan.nameIndexes))
	} else {
		values = values[:len(plan.nameIndexes)]
		oks = oks[:len(plan.nameIndexes)]
	}
	for i, nameIndex := range plan.nameIndexes {
		values[i], oks[i] = bindings.value(nameIndex)
	}
	for i := range plan.ops {
		op := &plan.ops[i]
		if op.nameIndex < 0 {
			continue
		}
		value, ok := fastStaticNameCachedValue(op, bindings, values, oks)
		if !ok {
			if op.nullOnMissing {
				continue
			}
			return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.value))
		}
		if !canWriteFastBindingOutput(value) {
			return false, nil
		}
	}
	for i := range plan.ops {
		op := &plan.ops[i]
		if op.prefix != "" {
			out.WriteString(op.prefix)
		}
		if op.nameIndex < 0 {
			continue
		}
		value, ok := fastStaticNameCachedValue(op, bindings, values, oks)
		if !ok {
			continue
		}
		writeFastBindingOutput(out, ctx, value, op.outputCache)
	}
	return true, nil
}

func fastStaticNameCachedValue(op *fastStaticNameOp, bindings fastRenderBindings, values []interface{}, oks []bool) (interface{}, bool) {
	if op != nil && op.lookupIndex >= 0 && op.lookupIndex < len(values) && op.lookupIndex < len(oks) {
		return values[op.lookupIndex], oks[op.lookupIndex]
	}
	if op == nil {
		return nil, false
	}
	return bindings.value(op.nameIndex)
}

func renderFastSimplePlan(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastSimplePlan) (bool, error) {
	if out == nil || plan == nil {
		return false, nil
	}
	var inlineValues [8]interface{}
	var inlineOK [8]bool
	values := inlineValues[:]
	oks := inlineOK[:]
	if len(plan.nameIndexes) > len(inlineValues) {
		values = make([]interface{}, len(plan.nameIndexes))
		oks = make([]bool, len(plan.nameIndexes))
	} else {
		values = values[:len(plan.nameIndexes)]
		oks = oks[:len(plan.nameIndexes)]
	}
	for i, nameIndex := range plan.nameIndexes {
		values[i], oks[i] = bindings.value(nameIndex)
	}
	for i := range plan.ops {
		simpleOp := &plan.ops[i]
		op := simpleOp.op
		if op == nil {
			return false, nil
		}
		if op.prefix != "" {
			out.WriteString(op.prefix)
		}
		switch op.kind {
		case fastMixedOpStatic:
			continue
		case fastMixedOpName:
			value, ok := fastSimpleCachedOpValue(simpleOp, bindings, values, oks)
			if !ok {
				if op.nullOnMissing {
					continue
				}
				return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.value))
			}
			if !writeFastBindingOutput(out, ctx, value, &op.outputCache) {
				return false, nil
			}
		case fastMixedOpProperty:
			value, ok := fastSimpleCachedOpValue(simpleOp, bindings, values, oks)
			if !ok {
				return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.value))
			}
			if err := spendFastTraversal(ctx, op.line); err != nil {
				return true, err
			}
			if err := writeFastPropertyOutput(out, ctx, value, op.property, object.PropertyAccess{
				Receiver: op.receiver,
				Full:     op.full,
			}, &op.propertyCache); err != nil {
				return true, fastLineError(op.line, err)
			}
		case fastMixedOpAccessChain:
			value, ok := fastSimpleCachedOpValue(simpleOp, bindings, values, oks)
			if !ok {
				if op.valuePlan.NullOnMissing {
					continue
				}
				return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.valuePlan.Value))
			}
			handled, err := writeFastTopLevelAccessChainRaw(out, ctx, &op.valuePlan, value, &op.accessCache)
			if err != nil {
				return true, err
			}
			if !handled {
				return false, nil
			}
		case fastMixedOpValue:
			if !fastMixedValueCanUseSimple(op.valuePlan.Kind) || simpleOp.value == nil {
				return false, nil
			}
			result, ok, err := evalFastSimpleValue(simpleOp.value, ctx, bindings, values, oks, nil)
			if err != nil {
				return true, err
			}
			if !ok {
				return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.valuePlan.Value))
			}
			if truth, ok := result.(bool); ok {
				out.WriteString(strconv.FormatBool(truth))
				continue
			}
			writeFastGoValue(out, ctx, result)
		default:
			return false, nil
		}
	}
	return true, nil
}

func fastMixedValueCanUseSimple(kind compiler.FastValueKind) bool {
	switch kind {
	case compiler.FastValueInfix, compiler.FastValuePrefix, compiler.FastValueConcat:
		return true
	default:
		return false
	}
}

func renderFastSimplePlanInlineSafe(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastSimplePlan) (bool, error) {
	if out == nil || plan == nil {
		return false, nil
	}
	var scratch strings.Builder
	if grow := fastSimplePlanGrowSize(plan); grow > 0 {
		scratch.Grow(grow)
	}
	ok, err := renderFastSimplePlan(&scratch, ctx, bindings, plan)
	if !ok || err != nil {
		return ok, err
	}
	out.WriteString(scratch.String())
	return true, nil
}

func fastSimplePlanGrowSize(plan *fastSimplePlan) int {
	if plan == nil {
		return 0
	}
	size := 0
	for i := range plan.ops {
		op := plan.ops[i].op
		if op == nil {
			continue
		}
		size += len(op.prefix)
		if op.kind != fastMixedOpStatic {
			size += 16
		}
	}
	return size
}

func renderFastSimpleConditionalPlan(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastSimpleConditionalPlan) (bool, error) {
	if out == nil || plan == nil {
		return false, nil
	}
	var inlineValues [8]interface{}
	var inlineOK [8]bool
	values := inlineValues[:]
	oks := inlineOK[:]
	if len(plan.nameIndexes) > len(inlineValues) {
		values = make([]interface{}, len(plan.nameIndexes))
		oks = make([]bool, len(plan.nameIndexes))
	} else {
		values = values[:len(plan.nameIndexes)]
		oks = oks[:len(plan.nameIndexes)]
	}
	for i, nameIndex := range plan.nameIndexes {
		values[i], oks[i] = bindings.value(nameIndex)
	}
	for i := range plan.branches {
		branch := &plan.branches[i]
		if err := spendFastCondition(ctx, branch.line); err != nil {
			return true, err
		}
		value, ok, err := evalFastSimpleValue(branch.condition, ctx, bindings, values, oks, nil)
		if err != nil {
			return true, err
		}
		if !ok {
			value = nil
		}
		if isTruthyFastValue(value) {
			if branch.segments == nil {
				return true, nil
			}
			return renderFastSimplePlan(out, ctx, bindings, branch.segments)
		}
	}
	if plan.elseSegments != nil {
		return renderFastSimplePlan(out, ctx, bindings, plan.elseSegments)
	}
	return true, nil
}

func fastSimpleCachedOpValue(op *fastSimpleOp, bindings fastRenderBindings, values []interface{}, oks []bool) (interface{}, bool) {
	if op == nil || op.op == nil {
		return nil, false
	}
	return fastSimpleCachedLookupValue(op.lookupIndex, bindings, values, oks, op.op.nameIndex)
}

func fastSimpleCachedLookupValue(lookupIndex int, bindings fastRenderBindings, values []interface{}, oks []bool, nameIndex int) (interface{}, bool) {
	if lookupIndex >= 0 && lookupIndex < len(values) && lookupIndex < len(oks) {
		return values[lookupIndex], oks[lookupIndex]
	}
	return bindings.value(nameIndex)
}

func evalFastSimpleValue(plan *fastSimpleValuePlan, ctx hctx.Context, bindings fastRenderBindings, values []interface{}, oks []bool, base interface{}) (interface{}, bool, error) {
	if plan == nil || plan.value == nil {
		return nil, true, nil
	}
	value := plan.value
	switch value.Kind {
	case compiler.FastValueName:
		if value.Value == "nil" {
			return nil, true, nil
		}
		raw, ok := fastSimpleCachedLookupValue(plan.lookupIndex, bindings, values, oks, value.NameIndex)
		if !ok {
			if value.NullOnMissing {
				return nil, true, nil
			}
			return nil, false, nil
		}
		return raw, true, nil
	case compiler.FastValueString:
		return value.Value, true, nil
	case compiler.FastValueInteger:
		return int(value.IntValue), true, nil
	case compiler.FastValueFloat:
		return value.FloatValue, true, nil
	case compiler.FastValueBool:
		return value.BoolValue, true, nil
	case compiler.FastValueInfix:
		return evalFastSimpleInfixValue(plan, ctx, bindings, values, oks, base)
	case compiler.FastValuePrefix:
		return evalFastSimplePrefixValue(plan, ctx, bindings, values, oks, base)
	case compiler.FastValueConcat:
		return evalFastSimpleConcatValue(plan, ctx, bindings, values, oks, base)
	case compiler.FastValueCall:
		return evalFastSimpleCallValue(plan, ctx, bindings, values, oks)
	case compiler.FastValueArray:
		return evalFastSimpleArrayValue(plan, ctx, bindings, values, oks, base)
	case compiler.FastValueHash:
		return evalFastSimpleHashValue(plan, ctx, bindings, values, oks, base)
	case compiler.FastValuePath:
		raw := base
		if value.NameIndex >= 0 {
			var ok bool
			raw, ok = fastSimpleCachedLookupValue(plan.lookupIndex, bindings, values, oks, value.NameIndex)
			if !ok {
				if value.NullOnMissing {
					return nil, true, nil
				}
				return nil, false, nil
			}
		}
		if result, handled, err := evalFastFieldChainValue(value, raw, ctx); handled || err != nil {
			return result, true, err
		}
		if result, handled, err := evalFastAccessChainValue(value, raw, ctx); handled || err != nil {
			return result, true, err
		}
		for i := range value.Path {
			step := &value.Path[i]
			var err error
			raw, err = evalFastPathStep(raw, step, ctx)
			if err != nil {
				return nil, true, err
			}
		}
		return raw, true, nil
	default:
		return evalFastValue(value, ctx, bindings, base)
	}
}

func evalFastSimpleCallValue(plan *fastSimpleValuePlan, ctx hctx.Context, bindings fastRenderBindings, values []interface{}, oks []bool) (interface{}, bool, error) {
	if plan == nil || plan.value == nil || plan.value.Call == nil {
		return nil, false, nil
	}
	return evalFastCallValuePlan(plan.value.Call, plan.args, ctx, bindings, values, oks, plan)
}

func evalFastSimpleArrayValue(plan *fastSimpleValuePlan, ctx hctx.Context, bindings fastRenderBindings, values []interface{}, oks []bool, base interface{}) ([]interface{}, bool, error) {
	if plan == nil || plan.value == nil || plan.value.Kind != compiler.FastValueArray || len(plan.args) != len(plan.value.Elements) {
		return nil, false, nil
	}
	out := make([]interface{}, 0, len(plan.args))
	for i := range plan.args {
		value, ok, err := evalFastSimpleValue(plan.args[i], ctx, bindings, values, oks, base)
		if err != nil || !ok {
			return nil, ok, err
		}
		out = append(out, value)
	}
	return out, true, nil
}

func evalFastSimpleHashValue(plan *fastSimpleValuePlan, ctx hctx.Context, bindings fastRenderBindings, values []interface{}, oks []bool, base interface{}) (map[string]interface{}, bool, error) {
	if plan == nil || plan.value == nil || plan.value.Kind != compiler.FastValueHash || len(plan.args) != len(plan.value.Pairs) {
		return nil, false, nil
	}
	out := make(map[string]interface{}, len(plan.args))
	for i := range plan.args {
		value, ok, err := evalFastSimpleValue(plan.args[i], ctx, bindings, values, oks, base)
		if err != nil || !ok {
			return nil, ok, err
		}
		out[plan.value.Pairs[i].Key] = value
	}
	return out, true, nil
}

func evalFastCallValuePlan(call *compiler.FastCallPlan, simpleArgs []*fastSimpleValuePlan, ctx hctx.Context, bindings fastRenderBindings, values []interface{}, oks []bool, simpleCall *fastSimpleValuePlan) (interface{}, bool, error) {
	if call == nil {
		return nil, true, nil
	}
	raw, ok := bindings.value(call.NameIndex)
	if simpleCall != nil {
		raw, ok = fastSimpleCachedLookupValue(simpleCall.lookupIndex, bindings, values, oks, call.NameIndex)
	}
	if !ok {
		return nil, false, nil
	}
	if err := spendFastFunctionCall(ctx, call.Name, call.Line); err != nil {
		return nil, true, err
	}

	var argStore fastCallArgs
	args := fastCallArgsOrNil(&argStore, len(call.Args))
	if len(call.Args) > 0 {
		argStore.Reset()
		for i := range call.Args {
			var value interface{}
			var argOK bool
			var err error
			if i < len(simpleArgs) && simpleArgs[i] != nil {
				value, argOK, err = evalFastSimpleValue(simpleArgs[i], ctx, bindings, values, oks, nil)
			} else {
				value, argOK, err = evalFastValue(&call.Args[i], ctx, bindings, nil)
			}
			if err != nil {
				return nil, true, err
			}
			if !argOK {
				return nil, false, nil
			}
			argStore.Append(value)
		}
	}
	result, err := fastCallValue(call.Name, raw, args, ctx, &call.Cache)
	if err != nil {
		return nil, true, fastLineError(call.Line, err)
	}
	return result, true, nil
}

func fastSimpleValueMissingName(plan *fastSimpleValuePlan) string {
	if plan == nil {
		return ""
	}
	return fastValueMissingName(plan.value)
}

func fastValueMissingName(value *compiler.FastValuePlan) string {
	if value == nil {
		return ""
	}
	if value.Kind == compiler.FastValueCall && value.Call != nil {
		return value.Call.Name
	}
	if value.Value != "" {
		return value.Value
	}
	if value.Left != nil {
		if name := fastValueMissingName(value.Left); name != "" {
			return name
		}
	}
	if value.Right != nil {
		return fastValueMissingName(value.Right)
	}
	return ""
}

func evalFastSimpleInfixValue(plan *fastSimpleValuePlan, ctx hctx.Context, bindings fastRenderBindings, values []interface{}, oks []bool, base interface{}) (interface{}, bool, error) {
	if plan == nil || plan.value == nil || plan.value.Kind != compiler.FastValueInfix || plan.left == nil || plan.right == nil {
		return nil, false, nil
	}
	value := plan.value
	if value.Operator == "&&" || value.Operator == "||" {
		return evalFastSimpleLogicalInfixValue(plan, ctx, bindings, values, oks, base)
	}
	left, ok, err := evalFastSimpleValue(plan.left, ctx, bindings, values, oks, base)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		left = nil
	}
	right, ok, err := evalFastSimpleValue(plan.right, ctx, bindings, values, oks, base)
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

func evalFastSimplePrefixValue(plan *fastSimpleValuePlan, ctx hctx.Context, bindings fastRenderBindings, values []interface{}, oks []bool, base interface{}) (interface{}, bool, error) {
	if plan == nil || plan.value == nil || plan.value.Kind != compiler.FastValuePrefix || plan.right == nil {
		return nil, false, nil
	}
	if plan.value.Operator != "!" {
		return nil, true, fastLineError(plan.value.Line, fmt.Errorf("unknown fast prefix operator: %s", plan.value.Operator))
	}
	right, ok, err := evalFastSimpleValue(plan.right, ctx, bindings, values, oks, base)
	if err != nil {
		return nil, true, err
	}
	return !(ok && isTruthyFastValue(right)), true, nil
}

func evalFastSimpleConcatValue(plan *fastSimpleValuePlan, ctx hctx.Context, bindings fastRenderBindings, values []interface{}, oks []bool, base interface{}) (interface{}, bool, error) {
	if plan == nil || plan.value == nil || plan.value.Kind != compiler.FastValueConcat || plan.left == nil || plan.right == nil {
		return nil, false, nil
	}
	left, ok, err := evalFastSimpleValue(plan.left, ctx, bindings, values, oks, base)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, false, nil
	}
	right, ok, err := evalFastSimpleValue(plan.right, ctx, bindings, values, oks, base)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, false, nil
	}
	result, err := evalFastAddOperator(left, right)
	if err != nil {
		return nil, true, fastLineError(plan.value.Line, err)
	}
	return result, true, nil
}

func evalFastSimpleLogicalInfixValue(plan *fastSimpleValuePlan, ctx hctx.Context, bindings fastRenderBindings, values []interface{}, oks []bool, base interface{}) (interface{}, bool, error) {
	if plan == nil || plan.value == nil || plan.left == nil || plan.right == nil {
		return nil, false, nil
	}
	value := plan.value
	left, ok, err := evalFastSimpleValue(plan.left, ctx, bindings, values, oks, base)
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

	right, ok, err := evalFastSimpleValue(plan.right, ctx, bindings, values, oks, base)
	if err != nil {
		return nil, true, err
	}
	return ok && isTruthyFastValue(right), true, nil
}

func renderFastMixedPlan(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, plan *fastMixedPlan) (bool, error) {
	if plan == nil {
		return false, nil
	}
	for i := range plan.ops {
		op := &plan.ops[i]
		if op.prefix != "" {
			out.WriteString(op.prefix)
		}
		switch op.kind {
		case fastMixedOpStatic:
			continue
		case fastMixedOpName:
			value, ok := bindings.value(op.nameIndex)
			if !ok {
				if op.nullOnMissing {
					continue
				}
				return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.value))
			}
			if !writeFastBindingOutput(out, ctx, value, &op.outputCache) {
				return false, nil
			}
		case fastMixedOpProperty:
			value, ok := bindings.value(op.nameIndex)
			if !ok {
				return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.value))
			}
			if err := spendFastTraversal(ctx, op.line); err != nil {
				return true, err
			}
			if err := writeFastPropertyOutput(out, ctx, value, op.property, object.PropertyAccess{
				Receiver: op.receiver,
				Full:     op.full,
			}, &op.propertyCache); err != nil {
				return true, fastLineError(op.line, err)
			}
		case fastMixedOpValue:
			if handled, ok, err := writeFastValuePlanOutput(out, ctx, bindings, &op.valuePlan); handled || err != nil {
				if err != nil {
					return true, err
				}
				if !ok {
					return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.valuePlan.Value))
				}
				continue
			}
			value, ok, err := evalFastValue(&op.valuePlan, ctx, bindings, nil)
			if err != nil {
				return true, err
			}
			if !ok {
				return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.valuePlan.Value))
			}
			writeFastGoValue(out, ctx, value)
		case fastMixedOpAccessChain:
			if handled, ok, err := writeFastTopLevelAccessChainOutput(out, ctx, bindings, &op.valuePlan, &op.accessCache); handled || err != nil {
				if err != nil {
					return true, err
				}
				if !ok {
					return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.valuePlan.Value))
				}
				continue
			}
			if handled, ok, err := writeFastValuePlanOutput(out, ctx, bindings, &op.valuePlan); handled || err != nil {
				if err != nil {
					return true, err
				}
				if !ok {
					return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.valuePlan.Value))
				}
				continue
			}
			value, ok, err := evalFastValue(&op.valuePlan, ctx, bindings, nil)
			if err != nil {
				return true, err
			}
			if !ok {
				return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", op.valuePlan.Value))
			}
			writeFastGoValue(out, ctx, value)
		case fastMixedOpCall:
			if err := writeFastCallSegment(out, ctx, bindings, op.call); err != nil {
				return true, err
			}
		case fastMixedOpBlockCall:
			if err := writeFastBlockCallSegment(out, ctx, bindings, op.blockCall); err != nil {
				if errors.Is(err, errFastWriteUnsupported) {
					return false, nil
				}
				return true, err
			}
		case fastMixedOpConditional:
			if op.simpleCond != nil {
				ok, err := renderFastSimpleConditionalPlan(out, ctx, bindings, op.simpleCond)
				if !ok || err != nil {
					if errors.Is(err, errFastWriteUnsupported) {
						return false, nil
					}
					return ok, err
				}
				continue
			}
			ok, err := renderFastConditional(out, ctx, bindings, op.conditional)
			if !ok || err != nil {
				if errors.Is(err, errFastWriteUnsupported) {
					return false, nil
				}
				return ok, err
			}
		case fastMixedOpPartial:
			if err := renderFastPartialSegmentWithDataPlan(out, ctx, bindings, op.partial, op.partialData); err != nil {
				return true, err
			}
		case fastMixedOpLoop:
			ok, err := renderFastLoop(out, ctx, bindings, op.loop)
			if !ok || err != nil {
				if errors.Is(err, errFastWriteUnsupported) {
					return false, nil
				}
				return ok, err
			}
		case fastMixedOpLet:
			value, ok, err := evalFastValue(&op.valuePlan, ctx, bindings, nil)
			if err != nil {
				return true, err
			}
			if !ok {
				return true, fastLineError(op.line, fmt.Errorf("%q: unknown identifier", fastValueMissingName(&op.valuePlan)))
			}
			if err := spendFastAssignment(ctx, op.line); err != nil {
				return true, err
			}
			bindings.setLocalAndContext(op.nameIndex, value)
		case fastMixedOpAssign:
			if err := assignFastValue(ctx, &bindings, op.value, op.nameIndex, &op.valuePlan, op.line); err != nil {
				return true, err
			}
		default:
			return false, nil
		}
	}
	return true, nil
}

func renderFastSegments(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, segments []compiler.FastRenderSegment) (bool, error) {
	for i := range segments {
		segment := &segments[i]
		switch segment.Kind {
		case compiler.FastRenderSegmentStatic:
			out.WriteString(segment.Value)
		case compiler.FastRenderSegmentName:
			value, ok := bindings.value(segment.NameIndex)
			if !ok {
				if segment.NullOnMissing {
					continue
				}
				return true, fastLineError(segment.Line, fmt.Errorf("%q: unknown identifier", segment.Value))
			}
			if !writeFastBindingOutput(out, ctx, value, &segment.OutputCache) {
				return false, nil
			}
		case compiler.FastRenderSegmentProperty:
			value, ok := bindings.value(segment.NameIndex)
			if !ok {
				return true, fastLineError(segment.Line, fmt.Errorf("%q: unknown identifier", segment.Value))
			}
			if err := spendFastTraversal(ctx, segment.Line); err != nil {
				return true, err
			}
			if err := writeFastPropertyOutput(out, ctx, value, segment.Property, object.PropertyAccess{
				Receiver: segment.Receiver,
				Full:     segment.Full,
			}, &segment.PropertyCache); err != nil {
				return true, fastLineError(segment.Line, err)
			}
		case compiler.FastRenderSegmentValue:
			if handled, ok, err := writeFastValuePlanOutput(out, ctx, bindings, &segment.ValuePlan); handled || err != nil {
				if err != nil {
					return true, err
				}
				if !ok {
					return true, fastLineError(segment.Line, fmt.Errorf("%q: unknown identifier", segment.ValuePlan.Value))
				}
				continue
			}
			value, ok, err := evalFastValue(&segment.ValuePlan, ctx, bindings, nil)
			if err != nil {
				return true, err
			}
			if !ok {
				return true, fastLineError(segment.Line, fmt.Errorf("%q: unknown identifier", segment.ValuePlan.Value))
			}
			writeFastGoValue(out, ctx, value)
		case compiler.FastRenderSegmentCall:
			if err := writeFastCallSegment(out, ctx, bindings, segment.Call); err != nil {
				return true, err
			}
		case compiler.FastRenderSegmentConditional:
			ok, err := renderFastConditional(out, ctx, bindings, segment.Conditional)
			if !ok || err != nil {
				if errors.Is(err, errFastWriteUnsupported) {
					return false, nil
				}
				return ok, err
			}
		case compiler.FastRenderSegmentPartial:
			if err := renderFastPartialSegment(out, ctx, bindings, segment.Partial); err != nil {
				return true, err
			}
		case compiler.FastRenderSegmentLoop:
			ok, err := renderFastLoop(out, ctx, bindings, segment.Loop)
			if !ok || err != nil {
				if errors.Is(err, errFastWriteUnsupported) {
					return false, nil
				}
				return ok, err
			}
		case compiler.FastRenderSegmentLet:
			value, ok, err := evalFastValue(&segment.ValuePlan, ctx, bindings, nil)
			if err != nil {
				return true, err
			}
			if !ok {
				return true, fastLineError(segment.Line, fmt.Errorf("%q: unknown identifier", fastValueMissingName(&segment.ValuePlan)))
			}
			if err := spendFastAssignment(ctx, segment.Line); err != nil {
				return true, err
			}
			bindings.setLocalAndContext(segment.NameIndex, value)
		case compiler.FastRenderSegmentAssign:
			if err := assignFastValue(ctx, &bindings, segment.Value, segment.NameIndex, &segment.ValuePlan, segment.Line); err != nil {
				return true, err
			}
		default:
			return false, nil
		}
	}

	return true, nil
}

func assignFastValue(ctx hctx.Context, bindings *fastRenderBindings, name string, nameIndex int, valuePlan *compiler.FastValuePlan, line int) error {
	if bindings == nil {
		return fastLineError(line, fmt.Errorf("%q: unknown identifier", name))
	}
	if _, ok := bindings.value(nameIndex); !ok {
		return fastLineError(line, fmt.Errorf("%q: unknown identifier", name))
	}
	value, ok, err := evalFastValue(valuePlan, ctx, *bindings, nil)
	if err != nil {
		return err
	}
	if !ok {
		return fastLineError(line, fmt.Errorf("%q: unknown identifier", fastValueMissingName(valuePlan)))
	}
	if err := spendFastAssignment(ctx, line); err != nil {
		return err
	}
	bindings.setLocalAndContext(nameIndex, value)
	return nil
}
