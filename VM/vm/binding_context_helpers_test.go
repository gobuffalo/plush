package vm

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/stretchr/testify/require"
)

type vmBindingIDContext struct {
	*idLookupTestContext
	internIDs int
}

func (c *vmBindingIDContext) InternIDs(keys []string, ids []int) {
	c.internIDs++
	for i, key := range keys {
		ids[i] = c.InternID(key)
	}
}

type vmFallbackContext struct {
	context.Context
	values map[string]interface{}
	has    int
	value  int
}

func newVMFallbackContext(values map[string]interface{}) *vmFallbackContext {
	return &vmFallbackContext{Context: context.Background(), values: values}
}

func (c *vmFallbackContext) New() hctx.Context {
	return newVMFallbackContext(c.values)
}

func (c *vmFallbackContext) Has(key string) bool {
	c.has++
	_, ok := c.values[key]
	return ok
}

func (c *vmFallbackContext) Value(key interface{}) interface{} {
	c.value++
	if key, ok := key.(string); ok {
		return c.values[key]
	}
	return nil
}

func (c *vmFallbackContext) Set(key string, value interface{}) {
	c.values[key] = value
}

func (c *vmFallbackContext) Update(key string, value interface{}) bool {
	if _, ok := c.values[key]; !ok {
		return false
	}
	c.values[key] = value
	return true
}

func Test_VM_Fill_Fast_Render_Binding_I_Ds_Branches(t *testing.T) {
	var inline [8]int
	var ids []int
	fillFastRenderBindingIDs(nil, nil, &inline, &ids)
	require.Nil(t, ids)

	lookup := newIDLookupTestContext(map[string]interface{}{})
	fillFastRenderBindingIDs([]string{"a", "b"}, lookup, &inline, &ids)
	require.Nil(t, ids)
	require.Equal(t, lookup.stringToID["a"], inline[0])
	require.Equal(t, lookup.stringToID["b"], inline[1])

	manyNames := make([]string, 9)
	for i := range manyNames {
		manyNames[i] = fmt.Sprintf("n%d", i)
	}
	fillFastRenderBindingIDs(manyNames, lookup, &inline, &ids)
	require.Len(t, ids, len(manyNames))
	require.Equal(t, lookup.stringToID["n8"], ids[8])

	binder := &vmBindingIDContext{idLookupTestContext: newIDLookupTestContext(map[string]interface{}{})}
	inline = [8]int{}
	ids = nil
	fillFastRenderBindingIDs([]string{"x", "y"}, binder, &inline, &ids)
	require.Equal(t, 1, binder.internIDs)
	require.Equal(t, binder.stringToID["x"], inline[0])

	fillFastRenderBindingIDs(manyNames, binder, &inline, &ids)
	require.Equal(t, 2, binder.internIDs)
	require.Len(t, ids, len(manyNames))
}

func Test_VM_Fast_Render_Bindings_Value_Branches(t *testing.T) {
	require.Nil(t, newFastRenderBindingPlan(nil, plush.NewContext()))
	require.Nil(t, newFastRenderBindingPlan(&compiler.FastRenderPlan{}, plush.NewContext()))
	require.Nil(t, newFastRenderBindingPlan(&compiler.FastRenderPlan{Bindings: []string{"name"}}, newVMFallbackContext(map[string]interface{}{"name": "fallback"})))

	ctx := newIDLookupTestContext(map[string]interface{}{"a": "A", "b": "B"})
	plan := &compiler.FastRenderPlan{Bindings: []string{"a", "b"}}
	bindings := newFastRenderBindings(plan, ctx)

	value, ok := bindings.value(-1)
	require.False(t, ok)
	require.Nil(t, value)

	value, ok = bindings.value(99)
	require.False(t, ok)
	require.Nil(t, value)

	value, ok = bindings.value(0)
	require.True(t, ok)
	require.Equal(t, "A", value)

	bindings.localOK = []bool{true}
	bindings.localVals = []interface{}{"local"}
	value, ok = bindings.value(0)
	require.True(t, ok)
	require.Equal(t, "local", value)

	fallback := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name"}}, plush.NewContextWith(map[string]interface{}{"name": "fallback"}))
	value, ok = fallback.value(0)
	require.True(t, ok)
	require.Equal(t, "fallback", value)

	bigPlan := &compiler.FastRenderPlan{Bindings: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}}
	bigCtx := plush.NewContextWith(map[string]interface{}{"i": "I"})
	prepared := newFastRenderBindingPlan(bigPlan, bigCtx)
	require.NotNil(t, prepared)
	bigBindings := newFastRenderBindingsWithPlan(bigPlan, bigCtx, prepared)
	require.NotNil(t, bigBindings.ids)
	value, ok = bigBindings.value(8)
	require.True(t, ok)
	require.Equal(t, "I", value)

	mismatch := newFastRenderBindingsWithPlan(&compiler.FastRenderPlan{Bindings: []string{"i"}}, bigCtx, prepared)
	value, ok = mismatch.value(0)
	require.True(t, ok)
	require.Equal(t, "I", value)
}

func Test_VM_Context_Value_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(nil)
	value, ok := machine.contextValue("missing")
	require.False(t, ok)
	require.Nil(t, value)

	lookup := newLookupTestContext(map[string]interface{}{"name": "lookup"})
	machine.ctx = lookup
	value, ok = machine.contextValue("name")
	require.True(t, ok)
	require.Equal(t, "lookup", value)
	require.Equal(t, 1, lookup.lookup)

	machine.ctx = plush.NewContextWith(map[string]interface{}{"name": "ctx"})
	value, ok = machine.contextValue("name")
	require.True(t, ok)
	require.Equal(t, "ctx", value)

	value, ok = machine.contextValue("missing")
	require.False(t, ok)
	require.Nil(t, value)

	fallback := newVMFallbackContext(map[string]interface{}{"name": "fallback"})
	machine.ctx = fallback
	value, ok = machine.contextValue("name")
	require.True(t, ok)
	require.Equal(t, "fallback", value)
	require.Equal(t, 1, fallback.has)
	require.Equal(t, 1, fallback.value)

	value, ok = machine.contextValue("missing")
	require.False(t, ok)
	require.Nil(t, value)
	require.Equal(t, 2, fallback.has)

	machine.ctx = nil
	value, ok = machine.contextValueByNameIndex(0)
	require.False(t, ok)
	require.Nil(t, value)

	idLookup := newIDLookupTestContext(map[string]interface{}{"name": "id"})
	machine.ctx = idLookup
	value, ok = machine.contextValueByNameIndex(0)
	require.True(t, ok)
	require.Equal(t, "id", value)
	require.Equal(t, 1, idLookup.lookupID)

	value, ok = machine.contextValueByNameIndex(0)
	require.True(t, ok)
	require.Equal(t, "id", value)
	require.Equal(t, 2, idLookup.lookupID)

	machine.clearNameIDCache()
	machine.ctx = plush.NewContextWith(map[string]interface{}{"name": "fallback"})
	value, ok = machine.contextValueByNameIndex(0)
	require.True(t, ok)
	require.Equal(t, "fallback", value)

	value, ok = fastContextValue(nil, "name")
	require.False(t, ok)
	require.Nil(t, value)

	value, ok = fastContextValue(fallback, "name")
	require.True(t, ok)
	require.Equal(t, "fallback", value)
	value, ok = fastContextValue(fallback, "missing")
	require.False(t, ok)
	require.Nil(t, value)
}

func Test_VM_Push_Name_And_Context_Name_I_D_Cache_Branches(t *testing.T) {
	ctx := newIDLookupTestContext(map[string]interface{}{"name": "Mido"})
	machine := newRuntimeHelperTestVM(ctx)
	machine.constants = append(machine.constants, &object.String{Value: "nil"})

	require.NoError(t, machine.pushName(0))
	require.Equal(t, &object.String{Value: "Mido"}, machine.pop())

	require.NoError(t, machine.pushName(5))
	require.Same(t, Null, machine.pop())

	require.ErrorContains(t, machine.pushName(3), "unknown identifier")

	require.NoError(t, machine.pushNameOrNull(0))
	require.Equal(t, &object.String{Value: "Mido"}, machine.pop())

	require.NoError(t, machine.pushNameOrNull(3))
	require.Same(t, Null, machine.pop())

	require.NoError(t, machine.pushNameOrNull(5))
	require.Same(t, Null, machine.pop())

	nameConstants := make([]object.Object, 9)
	for i := range nameConstants {
		nameConstants[i] = &object.String{Value: fmt.Sprintf("cache_%d", i)}
	}
	machine.constants = nameConstants
	machine.clearNameIDCache()
	for i := 0; i < 8; i++ {
		require.Equal(t, ctx.InternID(fmt.Sprintf("cache_%d", i)), machine.contextNameID(ctx, i))
	}
	require.Equal(t, 8, machine.nameIDCacheLen)

	overflowID := machine.contextNameID(ctx, 8)
	require.Len(t, machine.nameIDOverflow, 1)
	require.Equal(t, overflowID, machine.contextNameID(ctx, 8))
}

func Test_VM_Fast_Line_And_Budget_Helper_Branches(t *testing.T) {
	require.NoError(t, fastLineError(7, nil))
	require.EqualError(t, fastLineError(0, errors.New("boom")), "line 1: boom")

	require.NoError(t, spendFastTraversal(nil, 2))
	require.NoError(t, spendFastLoop(nil, 2))
	require.NoError(t, spendFastCondition(nil, 2))
	require.NoError(t, spendFastFunctionCall(nil, "helper", 2))
	require.NoError(t, spendFastSubRender(nil, 2))

	require.ErrorContains(t, spendFastTraversal(plush.NewContext().WithBudget(plush.NewBudget(0)), 3), "line 3")
	require.ErrorContains(t, spendFastLoop(plush.NewContext().WithBudget(plush.NewBudget(0)), 4), "line 4")
	require.ErrorContains(t, spendFastCondition(plush.NewContext().WithBudget(plush.NewBudget(0)), 5), "line 5")
	require.ErrorContains(t, spendFastFunctionCall(plush.NewContext().WithBudget(plush.NewBudget(0)), "helper", 6), "line 6")
	require.ErrorContains(t, spendFastSubRender(plush.NewContext().WithBudget(plush.NewBudget(0)), 7), "line 7")

	noBudget := newLookupTestContext(map[string]interface{}{})
	require.Nil(t, fastBudget(noBudget))
}
