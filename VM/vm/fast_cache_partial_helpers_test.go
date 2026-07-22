package vm

import (
	"context"
	"html/template"
	"reflect"
	"testing"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/stretchr/testify/require"
)

func Test_VM_Clone_Fast_Top_Level_Access_Cache_Branches(t *testing.T) {
	require.Nil(t, cloneFastTopLevelAccessCache(nil, 2))
	require.Nil(t, cloneFastTopLevelAccessCache(&fastTopLevelAccessCacheEntry{}, 0))

	methodPlan := &fastLoopMethodCallPlan{}
	fieldPlan := &fastFieldChainPlan{}
	chainPlan := &fastAccessChainPlan{}
	head := &fastTopLevelAccessCacheEntry{
		typ:        reflect.TypeOf(""),
		kind:       fastTopLevelAccessMethodCall,
		method:     methodPlan,
		fieldChain: fieldPlan,
		chain:      chainPlan,
		next: &fastTopLevelAccessCacheEntry{
			typ:  reflect.TypeOf(1),
			kind: fastTopLevelAccessFieldChain,
			next: &fastTopLevelAccessCacheEntry{
				typ:  reflect.TypeOf(uint32(1)),
				kind: fastTopLevelAccessChain,
			},
		},
	}

	clone := cloneFastTopLevelAccessCache(head, 2)
	require.NotNil(t, clone)
	require.NotSame(t, head, clone)
	require.Equal(t, head.typ, clone.typ)
	require.Equal(t, fastTopLevelAccessMethodCall, clone.kind)
	require.Same(t, methodPlan, clone.method)
	require.Same(t, fieldPlan, clone.fieldChain)
	require.Same(t, chainPlan, clone.chain)

	require.NotNil(t, clone.next)
	require.NotSame(t, head.next, clone.next)
	require.Equal(t, reflect.TypeOf(1), clone.next.typ)
	require.Nil(t, clone.next.next)
}

func Test_VM_Fast_Top_Level_Access_Cache_Entry_For_Branches(t *testing.T) {
	value := &compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "name"}

	entry := fastTopLevelAccessCacheEntryFor(nil, value, reflect.TypeOf(""))
	require.NotNil(t, entry)
	require.Equal(t, reflect.TypeOf(""), entry.typ)
	require.Equal(t, fastTopLevelAccessUnsupported, entry.kind)

	var slot object.InlineCacheSlot
	cachedString := fastTopLevelAccessCacheEntryFor(&slot, value, reflect.TypeOf(""))
	require.NotNil(t, cachedString)
	require.Equal(t, reflect.TypeOf(""), cachedString.typ)

	cachedInt := fastTopLevelAccessCacheEntryFor(&slot, value, reflect.TypeOf(1))
	require.NotNil(t, cachedInt)
	require.Equal(t, reflect.TypeOf(1), cachedInt.typ)
	require.NotNil(t, cachedInt.next)
	require.Equal(t, reflect.TypeOf(""), cachedInt.next.typ)
}

func Test_VM_Partial_Bytecode_Link_Cache_Edges(t *testing.T) {
	var nilCache *partialBytecodeLinkCache
	bytecode, ok := nilCache.Get("row", 1)
	require.False(t, ok)
	require.Nil(t, bytecode)

	link, ok := nilCache.GetLink("row", 1)
	require.False(t, ok)
	require.Nil(t, link)

	require.Nil(t, nilCache.Set("row", 1, &compiler.Bytecode{}))
	require.Zero(t, nilCache.Len())
	require.Equal(t, -1, nilCache.partialFeederID(nil))

	cache := newPartialBytecodeLinkCache()
	require.Nil(t, cache.Set("", 1, &compiler.Bytecode{}))
	require.Nil(t, cache.Set("row", 1, nil))
	require.Zero(t, cache.Len())

	stored := cache.Set("row", 1, &compiler.Bytecode{FastRenderPlan: &compiler.FastRenderPlan{Bindings: []string{"name"}}})
	require.NotNil(t, stored)
	require.Equal(t, 1, cache.Len())
	bytecode, ok = cache.Get("row", 2)
	require.False(t, ok)
	require.Nil(t, bytecode)
	bytecode, ok = cache.Get("row", 1)
	require.True(t, ok)
	require.Same(t, stored.bytecode, bytecode)

	_, ok = cache.partialFeeder(nil)
	require.False(t, ok)
	metaIDs, ok := cache.partialMetaIDs(newVMFallbackContext(map[string]interface{}{}))
	require.False(t, ok)
	require.Equal(t, partialMetaIDs{}, metaIDs)

	partialCtx := borrowPartialOverlayContext(plush.NewContext())
	defer releasePartialOverlayContext(partialCtx)
	feeder := func(name string) (string, error) { return name, nil }
	partialCtx.Set(vmPartialFeederName, feeder)
	gotFeeder, ok := cache.partialFeeder(partialCtx)
	require.True(t, ok)
	require.NotNil(t, gotFeeder)

	id := cache.partialFeederID(partialCtx)
	require.Equal(t, id, cache.partialFeederID(partialCtx))

	firstPlan := stored.fastBindingPlan(partialCtx)
	require.NotNil(t, firstPlan)
	require.Same(t, firstPlan, stored.fastBindingPlan(partialCtx))
	require.Nil(t, (*partialBytecodeLink)(nil).fastBindingPlan(partialCtx))
}

func Test_VM_Fast_Partial_Metadata_Helpers(t *testing.T) {
	var nilCache *partialBytecodeLinkCache
	_, ok := nilCache.partialMetaIDs(plush.NewContext())
	require.False(t, ok)

	cache := newPartialBytecodeLinkCache()
	_, ok = cache.partialMetaIDs(newIDLookupTestContext(map[string]interface{}{}))
	require.False(t, ok)

	metaCtx := borrowPartialOverlayContext(plush.NewContext())
	defer releasePartialOverlayContext(metaCtx)
	idsFromCache, ok := cache.partialMetaIDs(metaCtx)
	require.True(t, ok)
	require.NotEqual(t, 0, idsFromCache.templateBaseFileID)
	idsFromCacheAgain, ok := cache.partialMetaIDs(metaCtx)
	require.True(t, ok)
	require.Equal(t, idsFromCache, idsFromCacheAgain)

	filename, err := fastPartialTemplateFilename(nil, "row.plush")
	require.NoError(t, err)
	require.Equal(t, "row.plush", filename)

	ctx := plush.NewContextWith(map[string]interface{}{
		meta.TemplateBaseFileNameKey: "index",
		meta.TemplateExtensionKey:    "plush",
		meta.TemplateFileKey:         "templates/index.plush",
	})
	filename, err = fastPartialTemplateFilename(ctx, "row.plush")
	require.NoError(t, err)
	require.Equal(t, "templates/row.plush", filename)

	ctx.Set(vmAlreadyInPartial, "parent.plush")
	ctx.Set(meta.TemplateFileKey, "templates/index.plushparent.plush")
	filename, err = fastPartialTemplateFilename(ctx, "row.plush")
	require.NoError(t, err)
	require.Equal(t, "templates/row.plush", filename)

	ctx.Set(meta.TemplateFileKey, 99)
	_, err = fastPartialTemplateFilename(ctx, "row.plush")
	require.ErrorContains(t, err, "expected fileKey to be a string")

	require.True(t, partialNeedsJSEscape(plush.NewContextWith(map[string]interface{}{"contentType": "application/javascript"}), "row.plush"))
	require.False(t, partialNeedsJSEscape(plush.NewContextWith(map[string]interface{}{"contentType": "application/javascript"}), "row.js"))
	require.False(t, partialNeedsJSEscape(plush.NewContextWith(map[string]interface{}{"contentType": "text/html"}), "row.plush"))
	require.False(t, partialNeedsJSEscape(nil, "row.plush"))

	partialCtx := borrowPartialOverlayContext(plush.NewContext())
	defer releasePartialOverlayContext(partialCtx)
	ids := partialMetaIDs{
		templateFileID:     partialCtx.InternID(meta.TemplateFileKey),
		templateBaseFileID: partialCtx.InternID(meta.TemplateBaseFileNameKey),
		templateExtID:      partialCtx.InternID(meta.TemplateExtensionKey),
		alreadyPartialID:   partialCtx.InternID(vmAlreadyInPartial),
		contentTypeID:      partialCtx.InternID("contentType"),
	}
	partialCtx.SetID(ids.contentTypeID, "application/javascript")
	require.True(t, partialNeedsJSEscapeFast(partialCtx, "row.plush", ids))
	require.False(t, partialNeedsJSEscapeFast(partialCtx, "row.js", ids))
	partialCtx.SetID(ids.contentTypeID, 12)
	require.False(t, partialNeedsJSEscapeFast(partialCtx, "row.plush", ids))
	require.False(t, partialNeedsJSEscapeFast(nil, "row.plush", ids))
	require.NoError(t, setupFastPartialTemplateFile(nil, "row.plush", ids))

	partialCtx.SetID(ids.templateBaseFileID, "index")
	partialCtx.SetID(ids.templateExtID, "plush")
	partialCtx.SetID(ids.templateFileID, "templates/index.plush")
	require.NoError(t, setupFastPartialTemplateFile(partialCtx, "row.plush", ids))
	value, ok := partialCtx.LookupID(ids.templateFileID)
	require.True(t, ok)
	require.Equal(t, "templates/row.plush", value)

	partialCtx.SetID(ids.templateFileID, "templates/parent.plush")
	partialCtx.SetID(ids.alreadyPartialID, "parent.plush")
	require.NoError(t, setupFastPartialTemplateFile(partialCtx, "child.plush", ids))
	value, ok = partialCtx.LookupID(ids.templateFileID)
	require.True(t, ok)
	require.Equal(t, "templates/child.plush", value)

	fallbackCtx := borrowPartialOverlayContext(plush.NewContext())
	defer releasePartialOverlayContext(fallbackCtx)
	fallbackIDs := partialMetaIDs{templateFileID: fallbackCtx.InternID(meta.TemplateFileKey)}
	require.NoError(t, setupFastPartialTemplateFile(fallbackCtx, "plain.plush", fallbackIDs))
	value, ok = fallbackCtx.LookupID(fallbackIDs.templateFileID)
	require.True(t, ok)
	require.Equal(t, "plain.plush", value)

	errorCtx := borrowPartialOverlayContext(plush.NewContext())
	defer releasePartialOverlayContext(errorCtx)
	errorIDs := partialMetaIDs{
		templateFileID:     errorCtx.InternID(meta.TemplateFileKey),
		templateBaseFileID: errorCtx.InternID(meta.TemplateBaseFileNameKey),
		templateExtID:      errorCtx.InternID(meta.TemplateExtensionKey),
	}
	errorCtx.SetID(errorIDs.templateBaseFileID, "index")
	errorCtx.SetID(errorIDs.templateExtID, "plush")
	errorCtx.SetID(errorIDs.templateFileID, 99)
	require.ErrorContains(t, setupFastPartialTemplateFile(errorCtx, "row.plush", errorIDs), "expected fileKey to be a string")
}

func Test_VM_Partial_Helper_Install_And_Function_Identity_Branches(t *testing.T) {
	require.Nil(t, installVMPartialHelper(nil))
	require.False(t, sameFunction(nil, plush.PartialHelper))
	require.False(t, sameFunction("partial", plush.PartialHelper))
	require.False(t, sameFunction(plush.PartialHelper, "partial"))

	ctx := newLookupTestContext(map[string]interface{}{})
	require.Nil(t, installVMPartialHelper(ctx))

	plushCtx := plush.NewContext()
	plushCtx.Set("partial", func(string, interface{}, plush.HelperContext) (template.HTML, error) {
		return template.HTML("custom"), nil
	})
	require.Nil(t, installVMPartialHelper(plushCtx))

	plushCtx.Set("partial", plush.PartialHelper)
	restore := installVMPartialHelper(plushCtx)
	require.NotNil(t, restore)
	require.True(t, sameFunction(plushCtx.Value("partial"), vmPartialHelper))
	restore()
	require.True(t, sameFunction(plushCtx.Value("partial"), plush.PartialHelper))
}

func Test_VM_Partial_Overlay_Update_ID_Branches(t *testing.T) {
	require.False(t, (*partialOverlayContext)(nil).UpdateID(1, "x"))
	require.False(t, (*partialOverlayContext)(nil).UpdateID(1, nil))

	parent := newIDLookupTestContext(map[string]interface{}{"parent": "old"})
	parentID := parent.InternID("parent")
	local := borrowPartialOverlayContext(parent)
	defer releasePartialOverlayContext(local)

	localID := local.InternID("local")
	local.SetID(localID, "first")
	require.True(t, local.UpdateID(localID, "second"))
	value, ok := local.LookupID(localID)
	require.True(t, ok)
	require.Equal(t, "second", value)

	require.True(t, local.UpdateID(parentID, "new"))
	require.Equal(t, "new", parent.values["parent"])

	missingID := local.InternID("missing")
	require.False(t, local.UpdateID(missingID, "value"))
	require.False(t, local.UpdateID(missingID, nil))
}

func Test_VM_Partial_Overlay_Lookup_And_Context_Branches(t *testing.T) {
	var nilOverlay *partialOverlayContext
	nilOverlay.reset(nil)
	require.False(t, nilOverlay.stableBindingIDs())
	deadline, ok := nilOverlay.Deadline()
	require.False(t, ok)
	require.True(t, deadline.IsZero())
	require.Nil(t, nilOverlay.Done())
	require.NoError(t, nilOverlay.Err())
	require.Nil(t, nilOverlay.Value("missing"))
	require.False(t, nilOverlay.Has("missing"))
	value, ok := nilOverlay.Lookup("missing")
	require.False(t, ok)
	require.Nil(t, value)
	require.Equal(t, -1, nilOverlay.InternID("missing"))
	nilOverlay.InternIDs([]string{"a"}, []int{0})
	value, ok = nilOverlay.LookupID(1)
	require.False(t, ok)
	require.Nil(t, value)
	nilOverlay.SetID(1, "x")
	require.False(t, nilOverlay.Update("x", "y"))
	nilOverlay.Set("x", "y")
	require.Nil(t, nilOverlay.Budget())

	childParent := newLookupTestContext(map[string]interface{}{"fallback": "F"})
	fallbackChild, fallbackRelease := partialHelperChildContext(childParent)
	require.Nil(t, fallbackRelease)
	require.NotSame(t, childParent, fallbackChild)
	require.Equal(t, "F", fallbackChild.Value("fallback"))

	orphan := borrowPartialOverlayContext(nil)
	defer releasePartialOverlayContext(orphan)
	require.Nil(t, orphan.Value("missing"))
	value, ok = orphan.Lookup("missing")
	require.False(t, ok)
	require.Nil(t, value)
	require.False(t, orphan.Update("missing", "M"))
	require.False(t, orphan.UpdateID(12, "M"))
	require.Nil(t, orphan.Budget())
	fallbackIDs := make([]int, 2)
	orphan.InternIDs([]string{"one", "two"}, fallbackIDs)
	require.NotEqual(t, fallbackIDs[0], fallbackIDs[1])
	name, ok := orphan.nameForID(fallbackIDs[0])
	require.True(t, ok)
	require.Equal(t, "one", name)
	clear(orphan.idNames)
	name, ok = orphan.nameForID(fallbackIDs[0])
	require.True(t, ok)
	require.Equal(t, "one", name)
	orphan.setLocalWithID("", fallbackIDs[0], "first")
	orphan.setLocalWithID("", fallbackIDs[0], "second")
	value, ok = orphan.LookupID(fallbackIDs[0])
	require.True(t, ok)
	require.Equal(t, "second", value)

	noBudgetParent := newLookupTestContext(map[string]interface{}{"parent": "P"})
	noBudgetOverlay := borrowPartialOverlayContext(noBudgetParent)
	defer releasePartialOverlayContext(noBudgetOverlay)
	require.Nil(t, noBudgetOverlay.Budget())

	parent := plush.NewContextWith(map[string]interface{}{"parent": "P"})
	budget := plush.NewBudget(10)
	parent.WithBudget(budget)
	overlay := borrowPartialOverlayContext(parent)
	defer releasePartialOverlayContext(overlay)
	require.True(t, overlay.stableBindingIDs())

	overlay.Set("local", "L")
	require.Equal(t, "L", overlay.Value("local"))
	require.Equal(t, "P", overlay.Value("parent"))
	require.True(t, overlay.Has("local"))
	require.True(t, overlay.Has("parent"))
	require.False(t, overlay.Has("missing"))

	value, ok = overlay.Lookup("local")
	require.True(t, ok)
	require.Equal(t, "L", value)
	value, ok = overlay.Lookup("parent")
	require.True(t, ok)
	require.Equal(t, "P", value)
	value, ok = overlay.Lookup("missing")
	require.False(t, ok)
	require.Nil(t, value)

	child := overlay.New()
	require.NotSame(t, overlay, child)
	require.Equal(t, "L", child.Value("local"))
	require.Same(t, budget, overlay.Budget())

	ids := make([]int, 2)
	overlay.InternIDs(nil, ids)
	overlay.InternIDs([]string{"too", "short"}, ids[:1])
	overlay.InternIDs([]string{"local", "parent"}, ids)
	require.NotEqual(t, ids[0], ids[1])
	value, ok = overlay.LookupID(ids[0])
	require.True(t, ok)
	require.Equal(t, "L", value)
	value, ok = overlay.LookupID(ids[1])
	require.True(t, ok)
	require.Equal(t, "P", value)

	overlay.SetID(ids[0], "L2")
	value, ok = overlay.LookupID(ids[0])
	require.True(t, ok)
	require.Equal(t, "L2", value)
	require.True(t, overlay.Update("local", "L3"))
	value, ok = overlay.Lookup("local")
	require.True(t, ok)
	require.Equal(t, "L3", value)
	require.True(t, overlay.Update("parent", "P2"))
	require.Equal(t, "P2", parent.Value("parent"))
	require.False(t, overlay.Update("missing", "M"))

	extra := borrowPartialOverlayContext(nil)
	defer releasePartialOverlayContext(extra)
	for i := 0; i < 10; i++ {
		extra.Set(string(rune('a'+i)), i)
	}
	value, ok = extra.Lookup("j")
	require.True(t, ok)
	require.Equal(t, 9, value)
	extra.Set("j", "updated")
	value, ok = extra.Lookup("j")
	require.True(t, ok)
	require.Equal(t, "updated", value)

	extraID := extra.InternID("extra")
	extra.SetID(extraID, "first")
	require.True(t, extra.UpdateID(extraID, "second"))
	value, ok = extra.LookupID(extraID)
	require.True(t, ok)
	require.Equal(t, "second", value)

	deadlineAt := time.Now().Add(time.Hour)
	deadlineCtx, cancel := context.WithDeadline(context.Background(), deadlineAt)
	parentWithDeadline := plush.NewContextWithContext(deadlineCtx)
	overlayWithDeadline := borrowPartialOverlayContext(parentWithDeadline)
	defer releasePartialOverlayContext(overlayWithDeadline)

	deadline, ok = overlayWithDeadline.Deadline()
	require.True(t, ok)
	require.Equal(t, deadlineAt, deadline)
	require.NotNil(t, overlayWithDeadline.Done())
	require.NoError(t, overlayWithDeadline.Err())
	cancel()
	require.ErrorIs(t, overlayWithDeadline.Err(), context.Canceled)

	childOverlay := borrowPartialOverlayContext(overlay)
	defer releasePartialOverlayContext(childOverlay)
	require.True(t, childOverlay.stableBindingIDs())

	binderParent := &vmBindingIDContext{idLookupTestContext: newIDLookupTestContext(map[string]interface{}{})}
	binderOverlay := borrowPartialOverlayContext(binderParent)
	defer releasePartialOverlayContext(binderOverlay)
	binderIDs := make([]int, 2)
	binderOverlay.InternIDs([]string{"boundA", "boundB"}, binderIDs)
	require.Equal(t, 1, binderParent.internIDs)
	require.NotEqual(t, binderIDs[0], binderIDs[1])
	_, ok = binderOverlay.nameForID(binderIDs[0])
	require.True(t, ok)

	fallbackParent := newVMFallbackContext(map[string]interface{}{"fallback": "F"})
	fallbackOverlay := borrowPartialOverlayContext(fallbackParent)
	defer releasePartialOverlayContext(fallbackOverlay)
	value, ok = fallbackOverlay.Lookup("fallback")
	require.True(t, ok)
	require.Equal(t, "F", value)
	require.Equal(t, 1, fallbackParent.has)
	require.Equal(t, 1, fallbackParent.value)

	releasePartialOverlayContext(nil)
}

func Test_VM_Fast_Partial_Data_Local_Storage_And_Pair_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"name": "Mido"})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name"}}, ctx)

	require.NoError(t, evalFastPartialDataLocalValues(&fastPartialDataBindingPlan{}, ctx, bindings))

	value, err := evalFastPartialDataPairValue(nil, ctx, bindings)
	require.NoError(t, err)
	require.Nil(t, value)

	stringPair := &fastPartialDataBindingPair{
		key:   "label",
		value: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "static", Line: 1}},
		line:  1,
	}
	value, err = evalFastPartialDataPairValue(stringPair, ctx, bindings)
	require.NoError(t, err)
	require.Equal(t, "static", value)

	missingPair := &fastPartialDataBindingPair{
		key:   "missing",
		value: &fastSimpleValuePlan{value: &compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 5}},
		line:  5,
	}
	_, err = evalFastPartialDataPairValue(missingPair, ctx, bindings)
	require.ErrorContains(t, err, "line 5")

	require.Error(t, evalFastPartialDataLocalValues(&fastPartialDataBindingPlan{pairs: []fastPartialDataBindingPair{*missingPair}}, ctx, bindings))

	var storage fastPartialLocalStorage
	require.NotPanics(t, func() {
		prepareFastPartialLocalStorage(nil, &storage)
	})

	emptyBindings := newFastRenderBindings(&compiler.FastRenderPlan{}, ctx)
	prepareFastPartialLocalStorage(&emptyBindings, &storage)
	require.Nil(t, emptyBindings.localOK)
	require.Nil(t, emptyBindings.localVals)

	localBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"a", "b"}}, ctx)
	prepareFastPartialLocalStorage(&localBindings, &storage)
	require.Len(t, localBindings.localOK, 2)
	require.Len(t, localBindings.localVals, 2)
	localBindings.localOK[0] = true
	localBindings.localVals[1] = "stored"
	require.True(t, storage.ok[0])
	require.Equal(t, "stored", storage.vals[1])

	manyBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}}, ctx)
	var manyStorage fastPartialLocalStorage
	prepareFastPartialLocalStorage(&manyBindings, &manyStorage)
	require.Len(t, manyBindings.localOK, 9)
	require.Len(t, manyBindings.localVals, 9)
	manyBindings.localOK[0] = true
	manyBindings.localVals[0] = "heap"
	require.False(t, manyStorage.ok[0])
	require.Nil(t, manyStorage.vals[0])

	partialBindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label", "other"}}, ctx)
	plan := &fastPartialDataBindingPlan{pairs: []fastPartialDataBindingPair{*stringPair}}
	require.NoError(t, attachFastPartialDataLocalsFromPlan(&partialBindings, plan, ctx, bindings, &storage))
	require.True(t, partialBindings.localOK[0])
	require.Equal(t, "static", partialBindings.localVals[0])

	require.NoError(t, attachFastPartialDataLocalsFromPlan(nil, plan, ctx, bindings, &storage))
	require.NoError(t, attachFastPartialDataLocalsFromPlan(&partialBindings, nil, ctx, bindings, &storage))
}
