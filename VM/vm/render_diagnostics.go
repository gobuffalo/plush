package vm

import (
	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

func updateBytecodeDiagnostics(ctx hctx.Context, bytecode *compiler.Bytecode) {
	if ctx == nil || bytecode == nil {
		return
	}
	stats := cachedFastRenderPlanDiagnostics(bytecode)
	filename := plush.PunchHoleTemplateFilename(ctx)
	plush.UpdateRenderDiagnosticsForTemplate(ctx, filename, func(d *plush.RenderDiagnostics) {
		d.FastPlan = stats
		d.FastRejectLine = bytecode.FastRejectLine
		d.FastReject = bytecode.FastReject
	})
}

func cachedFastRenderPlanDiagnostics(bytecode *compiler.Bytecode) plush.RenderFastPlanDiagnostics {
	if bytecode == nil {
		return plush.RenderFastPlanDiagnostics{}
	}
	if cached := bytecode.FastDiagnostics.Load(); cached != nil {
		if stats, ok := cached.(*plush.RenderFastPlanDiagnostics); ok && stats != nil {
			return *stats
		}
	}
	stats := fastRenderPlanDiagnostics(bytecode.FastRenderPlan)
	bytecode.FastDiagnostics.Store(&stats)
	return stats
}

type fastPlanDiagnosticBuilder struct {
	stats        plush.RenderFastPlanDiagnostics
	helpersSeen  map[string]struct{}
	partialsSeen map[string]struct{}
}

func fastRenderPlanDiagnostics(plan *compiler.FastRenderPlan) plush.RenderFastPlanDiagnostics {
	if plan == nil {
		return plush.RenderFastPlanDiagnostics{}
	}
	builder := fastPlanDiagnosticBuilder{
		stats: plush.RenderFastPlanDiagnostics{
			Bindings: len(plan.Bindings),
			Segments: len(plan.Segments),
		},
		helpersSeen:  map[string]struct{}{},
		partialsSeen: map[string]struct{}{},
	}
	builder.segments(plan.Segments, 1)
	return builder.stats
}

func (b *fastPlanDiagnosticBuilder) depth(depth int) {
	if depth > b.stats.MaxDepth {
		b.stats.MaxDepth = depth
	}
}

func (b *fastPlanDiagnosticBuilder) helper(name string) {
	if name == "" {
		return
	}
	if _, ok := b.helpersSeen[name]; ok {
		return
	}
	b.helpersSeen[name] = struct{}{}
	b.stats.HelperNames = append(b.stats.HelperNames, name)
}

func (b *fastPlanDiagnosticBuilder) partial(name string) {
	if name == "" {
		return
	}
	if _, ok := b.partialsSeen[name]; ok {
		return
	}
	b.partialsSeen[name] = struct{}{}
	b.stats.PartialNames = append(b.stats.PartialNames, name)
}

func (b *fastPlanDiagnosticBuilder) segments(segments []compiler.FastRenderSegment, depth int) {
	b.depth(depth)
	for i := range segments {
		b.segment(&segments[i], depth)
	}
}

func (b *fastPlanDiagnosticBuilder) segment(segment *compiler.FastRenderSegment, depth int) {
	if segment == nil {
		return
	}
	switch segment.Kind {
	case compiler.FastRenderSegmentStatic:
		b.stats.StaticSegments++
	case compiler.FastRenderSegmentName:
		b.stats.NameSegments++
	case compiler.FastRenderSegmentProperty:
		b.stats.PropertyReads++
	case compiler.FastRenderSegmentValue:
		b.stats.ValueWrites++
		b.value(segment.ValuePlan, depth+1)
	case compiler.FastRenderSegmentCall:
		b.call(segment.Call, depth+1)
	case compiler.FastRenderSegmentBlockCall:
		b.blockCall(segment.BlockCall)
	case compiler.FastRenderSegmentConditional:
		b.conditional(segment.Conditional, depth+1)
	case compiler.FastRenderSegmentLoop:
		b.loop(segment.Loop, depth+1)
	case compiler.FastRenderSegmentPartial:
		b.partialPlan(segment.Partial, depth+1)
	case compiler.FastRenderSegmentLet:
		b.value(segment.ValuePlan, depth+1)
	case compiler.FastRenderSegmentAssign:
		b.value(segment.ValuePlan, depth+1)
	}
}

func (b *fastPlanDiagnosticBuilder) blockCall(call *compiler.FastBlockCallPlan) {
	if call == nil {
		return
	}
	b.stats.HelperCalls++
	b.helper(call.Name)
}

func (b *fastPlanDiagnosticBuilder) conditional(plan *compiler.FastConditionalPlan, depth int) {
	if plan == nil {
		return
	}
	b.depth(depth)
	b.stats.Conditionals++
	for i := range plan.Branches {
		b.value(plan.Branches[i].Condition, depth+1)
		b.segments(plan.Branches[i].Segments, depth+1)
	}
	b.segments(plan.ElseSegments, depth+1)
}

func (b *fastPlanDiagnosticBuilder) loop(loop *compiler.FastLoopPlan, depth int) {
	if loop == nil {
		return
	}
	b.depth(depth)
	b.stats.Loops++
	b.stats.LoopParts += len(loop.Parts)
	for i := range loop.Parts {
		b.loopPart(&loop.Parts[i], depth+1)
	}
}

func (b *fastPlanDiagnosticBuilder) loopPart(part *compiler.FastLoopPart, depth int) {
	if part == nil {
		return
	}
	b.depth(depth)
	switch part.Kind {
	case compiler.FastLoopPartStatic:
		b.stats.StaticSegments++
	case compiler.FastLoopPartKey, compiler.FastLoopPartValue:
		b.stats.ValueWrites++
	case compiler.FastLoopPartValueProperty:
		b.stats.ValueWrites++
		b.stats.PropertyReads++
	case compiler.FastLoopPartValuePath:
		b.stats.ValueWrites++
		b.value(part.ValuePlan, depth+1)
	case compiler.FastLoopPartCall:
		b.call(part.Call, depth+1)
	case compiler.FastLoopPartBlockCall:
		b.blockCall(part.BlockCall)
	case compiler.FastLoopPartPartial:
		b.partialPlan(part.Partial, depth+1)
	case compiler.FastLoopPartLet:
		b.value(part.ValuePlan, depth+1)
	case compiler.FastLoopPartConditional:
		b.loopConditional(part.Conditional, depth+1)
	case compiler.FastLoopPartLoop:
		b.loop(part.Loop, depth+1)
	}
}

func (b *fastPlanDiagnosticBuilder) loopConditional(plan *compiler.FastLoopConditionalPlan, depth int) {
	if plan == nil {
		return
	}
	b.depth(depth)
	b.stats.Conditionals++
	for i := range plan.Branches {
		b.loopCondition(plan.Branches[i].Condition, depth+1)
		b.stats.LoopParts += len(plan.Branches[i].Parts)
		for j := range plan.Branches[i].Parts {
			b.loopPart(&plan.Branches[i].Parts[j], depth+1)
		}
	}
	b.stats.LoopParts += len(plan.ElseParts)
	for i := range plan.ElseParts {
		b.loopPart(&plan.ElseParts[i], depth+1)
	}
}

func (b *fastPlanDiagnosticBuilder) loopCondition(plan compiler.FastValuePlan, depth int) {
	b.value(plan, depth)
}

func (b *fastPlanDiagnosticBuilder) partialPlan(plan *compiler.FastPartialPlan, depth int) {
	if plan == nil {
		return
	}
	b.depth(depth)
	b.stats.Partials++
	b.partial(plan.Name)
	for i := range plan.Data {
		b.value(plan.Data[i].Value, depth+1)
	}
}

func (b *fastPlanDiagnosticBuilder) call(call *compiler.FastCallPlan, depth int) {
	if call == nil {
		return
	}
	b.depth(depth)
	b.stats.HelperCalls++
	b.helper(call.Name)
	for i := range call.Args {
		b.value(call.Args[i], depth+1)
	}
}

func (b *fastPlanDiagnosticBuilder) value(plan compiler.FastValuePlan, depth int) {
	b.depth(depth)
	switch plan.Kind {
	case compiler.FastValueName:
		b.stats.NameSegments++
	case compiler.FastValuePath:
		b.stats.PropertyReads += len(plan.Path)
	case compiler.FastValueInfix, compiler.FastValueConcat:
		if plan.Left != nil {
			b.value(*plan.Left, depth+1)
		}
		if plan.Right != nil {
			b.value(*plan.Right, depth+1)
		}
	case compiler.FastValuePrefix:
		if plan.Right != nil {
			b.value(*plan.Right, depth+1)
		}
	case compiler.FastValueCall:
		b.call(plan.Call, depth+1)
	case compiler.FastValueArray:
		for i := range plan.Elements {
			b.value(plan.Elements[i], depth+1)
		}
	case compiler.FastValueHash:
		for i := range plan.Pairs {
			b.value(plan.Pairs[i].Value, depth+1)
		}
	}
}
