package vm

import (
	"fmt"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

type fastRenderBindings struct {
	ctx       hctx.Context
	names     []string
	lookup    contextIDLookup
	ids       []int
	inlineIDs [8]int
	localOK   []bool
	localVals []interface{}
}

type fastRenderBindingPlan struct {
	names     []string
	ids       []int
	inlineIDs [8]int
}

func newFastRenderBindingsWithPlan(plan *compiler.FastRenderPlan, ctx hctx.Context, bindingPlan *fastRenderBindingPlan) fastRenderBindings {
	bindings := fastRenderBindings{ctx: ctx}
	if plan != nil {
		bindings.names = plan.Bindings
	}
	lookup, ok := ctx.(contextIDLookup)
	if !ok || len(bindings.names) == 0 {
		return bindings
	}
	bindings.lookup = lookup
	if bindingPlan != nil && bindingPlan.matches(bindings.names) {
		if len(bindings.names) > len(bindings.inlineIDs) {
			bindings.ids = bindingPlan.ids
		} else {
			bindings.inlineIDs = bindingPlan.inlineIDs
		}
		return bindings
	}
	fillFastRenderBindingIDs(bindings.names, lookup, &bindings.inlineIDs, &bindings.ids)
	return bindings
}

func newFastRenderBindingPlan(plan *compiler.FastRenderPlan, ctx hctx.Context) *fastRenderBindingPlan {
	if plan == nil || len(plan.Bindings) == 0 {
		return nil
	}
	if !canCacheFastRenderBindingPlan(ctx) {
		return nil
	}
	lookup := ctx.(contextIDLookup)
	bindingPlan := &fastRenderBindingPlan{names: plan.Bindings}
	fillFastRenderBindingIDs(plan.Bindings, lookup, &bindingPlan.inlineIDs, &bindingPlan.ids)
	return bindingPlan
}

func fillFastRenderBindingIDs(names []string, lookup contextIDLookup, inlineIDs *[8]int, ids *[]int) {
	if lookup == nil || len(names) == 0 {
		return
	}
	if len(names) > len(*inlineIDs) {
		if *ids == nil || len(*ids) != len(names) {
			*ids = make([]int, len(names))
		}
		if binder, ok := lookup.(contextIDBinder); ok {
			binder.InternIDs(names, *ids)
			return
		}
		for i, name := range names {
			(*ids)[i] = lookup.InternID(name)
		}
		return
	}
	if binder, ok := lookup.(contextIDBinder); ok {
		tmp := inlineIDs[:len(names)]
		binder.InternIDs(names, tmp)
		return
	}
	for i, name := range names {
		(*inlineIDs)[i] = lookup.InternID(name)
	}
}

func (p *fastRenderBindingPlan) matches(names []string) bool {
	if p == nil || len(p.names) != len(names) {
		return false
	}
	for i := range names {
		if p.names[i] != names[i] {
			return false
		}
	}
	return true
}

func (b fastRenderBindings) value(index int) (interface{}, bool) {
	if index < 0 || index >= len(b.names) {
		return nil, false
	}
	if index < len(b.localOK) && b.localOK[index] {
		return b.localVals[index], true
	}
	if b.lookup != nil {
		var id int
		if b.ids != nil {
			id = b.ids[index]
		} else {
			id = b.inlineIDs[index]
		}
		return b.lookup.LookupID(id)
	}
	return fastContextValue(b.ctx, b.names[index])
}

func (b *fastRenderBindings) setLocal(index int, value interface{}) {
	if b == nil || index < 0 || index >= len(b.names) {
		return
	}
	b.ensureLocalCapacity()
	b.localOK[index] = true
	b.localVals[index] = value
}

func (b *fastRenderBindings) setLocalAndContext(index int, value interface{}) {
	if b == nil || index < 0 || index >= len(b.names) {
		return
	}
	b.setLocal(index, value)
	if b.ctx == nil {
		return
	}
	if id, ok := b.bindingID(index); ok {
		if setter, ok := b.ctx.(interface{ SetID(int, interface{}) }); ok {
			setter.SetID(id, value)
			return
		}
	}
	b.ctx.Set(b.names[index], value)
}

func (b fastRenderBindings) bindingID(index int) (int, bool) {
	if index < 0 || index >= len(b.names) {
		return 0, false
	}
	if b.ids != nil {
		if index >= len(b.ids) {
			return 0, false
		}
		return b.ids[index], true
	}
	if index >= len(b.inlineIDs) {
		return 0, false
	}
	return b.inlineIDs[index], true
}

func (b *fastRenderBindings) ensureLocalCapacity() {
	if b == nil || len(b.localOK) >= len(b.names) {
		return
	}
	ok := make([]bool, len(b.names))
	vals := make([]interface{}, len(b.names))
	copy(ok, b.localOK)
	copy(vals, b.localVals)
	b.localOK = ok
	b.localVals = vals
}

func fastRenderScopedBindings(ctx hctx.Context, bindings fastRenderBindings) (hctx.Context, fastRenderBindings, func()) {
	child, cleanup := partialHelperChildContext(ctx)
	if child == nil {
		return ctx, bindings, func() {}
	}
	scoped := bindings
	scoped.ctx = child
	if lookup, ok := child.(contextIDLookup); ok {
		scoped.lookup = lookup
	} else {
		scoped.lookup = nil
		scoped.ids = nil
		scoped.inlineIDs = [8]int{}
	}
	if cleanup == nil {
		cleanup = func() {}
	}
	return child, scoped, cleanup
}

type fastPartialLocalStorage struct {
	ok   [8]bool
	vals [8]interface{}
}

func fastContextValue(ctx hctx.Context, name string) (interface{}, bool) {
	if ctx == nil {
		return nil, false
	}
	if lookup, ok := ctx.(contextLookup); ok {
		return lookup.Lookup(name)
	}
	if ctx.Has(name) {
		return ctx.Value(name), true
	}
	return nil, false
}

func fastLineError(line int, err error) error {
	if err == nil {
		return nil
	}
	if line <= 0 {
		line = 1
	}
	return fmt.Errorf("line %d: %w", line, err)
}

func fastBudget(ctx hctx.Context) *plush.Budget {
	if provider, ok := ctx.(interface{ Budget() *plush.Budget }); ok {
		return provider.Budget()
	}
	return nil
}

func spendFastTraversal(ctx hctx.Context, line int) error {
	if err := fastBudget(ctx).SpendObjectTraversal(1); err != nil {
		return fastLineError(line, err)
	}
	return nil
}

func spendFastLoop(ctx hctx.Context, line int) error {
	if err := fastBudget(ctx).SpendLoop(); err != nil {
		return fastLineError(line, err)
	}
	return nil
}

func spendFastCondition(ctx hctx.Context, line int) error {
	if err := fastBudget(ctx).SpendCondition(); err != nil {
		return fastLineError(line, err)
	}
	return nil
}

func spendFastAssignment(ctx hctx.Context, line int) error {
	if err := fastBudget(ctx).SpendAssignment(); err != nil {
		return fastLineError(line, err)
	}
	return nil
}

func spendFastFunctionCall(ctx hctx.Context, name string, line int) error {
	if err := fastBudget(ctx).SpendFunctionCall(name); err != nil {
		return fastLineError(line, err)
	}
	return nil
}

func spendFastSubRender(ctx hctx.Context, line int) error {
	if err := fastBudget(ctx).SpendSubRender(); err != nil {
		return fastLineError(line, err)
	}
	return nil
}
