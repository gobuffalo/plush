package vm

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

func Test_VM_Property_Value_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())
	user := vmFastPropertyUser{Name: "<mido>", Count: 7}
	access := object.PropertyAccess{Receiver: "user", Full: "user.Name"}
	var nameSlot object.InlineCacheSlot
	var countSlot object.InlineCacheSlot
	var echoSlot object.InlineCacheSlot
	var pointerSlot object.InlineCacheSlot
	var missingSlot object.InlineCacheSlot

	hashKey := (&object.String{Value: "Name"}).HashKey()
	hash := &object.Hash{Pairs: map[object.HashKey]object.HashPair{
		hashKey: {Key: &object.String{Value: "Name"}, Value: &object.String{Value: "<hash>"}},
	}}
	value, err := machine.propertyValue(hash, "Name", access, &nameSlot)
	require.NoError(t, err)
	require.Equal(t, &object.String{Value: "<hash>"}, value)

	value, err = machine.propertyValue(hash, "Missing", access, &missingSlot)
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = machine.propertyValue(object.NullObject, "Name", access, &nameSlot)
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = machine.propertyValue(nil, "Name", access, &nameSlot)
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = machine.propertyValue((*vmFastPropertyUser)(nil), "Name", access, &nameSlot)
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = machine.propertyValue(user, "Name", access, &nameSlot)
	require.NoError(t, err)
	require.Equal(t, "<mido>", value)

	value, err = machine.propertyValue(&user, "Count", object.PropertyAccess{Receiver: "user", Full: "user.Count"}, &countSlot)
	require.NoError(t, err)
	require.Equal(t, int32(7), value)

	value, err = machine.propertyValue(user, "Echo", object.PropertyAccess{Receiver: "user", Full: "user.Echo()", Method: true}, &echoSlot)
	require.NoError(t, err)
	require.IsType(t, func() string { return "" }, value)

	value, err = machine.propertyValue(user, "PointerEcho", object.PropertyAccess{Receiver: "user", Full: "user.PointerEcho()", Method: true}, &pointerSlot)
	require.NoError(t, err)
	require.IsType(t, func() string { return "" }, value)

	_, err = machine.propertyValue(user, "Name", object.PropertyAccess{Receiver: "user", Full: "user.Name()", Method: true}, &nameSlot)
	require.ErrorContains(t, err, "does not have a method")

	_, err = machine.propertyValue(user, "Missing", object.PropertyAccess{Receiver: "user", Full: "user.Missing"}, &missingSlot)
	require.ErrorContains(t, err, "does not have a field or method")

	_, err = machine.propertyValue(3, "Missing", object.PropertyAccess{Receiver: "value", Full: "value.Missing"}, &missingSlot)
	require.ErrorContains(t, err, "does not have a field or method")
}

func Test_VM_Get_Property_And_Field_Value_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())
	user := vmFastPropertyUser{Name: "<mido>", Count: 7, hidden: "secret"}
	access := object.PropertyAccess{Receiver: "user", Full: "user.Name"}
	var nameSlot object.InlineCacheSlot
	var countSlot object.InlineCacheSlot
	var echoSlot object.InlineCacheSlot
	var pointerSlot object.InlineCacheSlot
	var missingSlot object.InlineCacheSlot

	hashKey := (&object.String{Value: "Name"}).HashKey()
	hash := &object.Hash{Pairs: map[object.HashKey]object.HashPair{
		hashKey: {Key: &object.String{Value: "Name"}, Value: &object.String{Value: "<hash>"}},
	}}
	require.NoError(t, machine.getProperty(hash, "Name", access, &nameSlot))
	require.Equal(t, &object.String{Value: "<hash>"}, machine.pop())

	require.NoError(t, machine.getProperty(hash, "Missing", access, &missingSlot))
	require.Same(t, Null, machine.pop())

	require.NoError(t, machine.getProperty(object.NullObject, "Name", access, &nameSlot))
	require.Same(t, Null, machine.pop())

	require.NoError(t, machine.getProperty(&object.Native{Value: (*vmFastPropertyUser)(nil)}, "Name", access, &nameSlot))
	require.Same(t, Null, machine.pop())

	require.NoError(t, machine.getProperty(&object.Native{Value: user}, "Name", access, &nameSlot))
	require.Equal(t, &object.String{Value: "<mido>"}, machine.pop())

	require.NoError(t, machine.getProperty(&object.Native{Value: &user}, "Count", object.PropertyAccess{Receiver: "user", Full: "user.Count"}, &countSlot))
	require.Equal(t, &object.Integer{Value: 7}, machine.pop())

	require.NoError(t, machine.getProperty(&object.Native{Value: user}, "Echo", object.PropertyAccess{Receiver: "user", Full: "user.Echo()", Method: true}, &echoSlot))
	require.IsType(t, &object.Native{}, machine.pop())

	require.NoError(t, machine.getProperty(&object.Native{Value: user}, "PointerEcho", object.PropertyAccess{Receiver: "user", Full: "user.PointerEcho()", Method: true}, &pointerSlot))
	require.IsType(t, &object.Native{}, machine.pop())

	err := machine.getProperty(&object.Native{Value: user}, "Name", object.PropertyAccess{Receiver: "user", Full: "user.Name()", Method: true}, &nameSlot)
	require.ErrorContains(t, err, "does not have a method")

	err = machine.getProperty(&object.Native{Value: user}, "Missing", object.PropertyAccess{}, &missingSlot)
	require.ErrorContains(t, err, "does not have a field or method")

	budgeted := newRuntimeHelperTestVM(plush.NewContext().WithBudget(plush.NewBudget(0)))
	err = budgeted.getProperty(&object.Native{Value: user}, "Name", access, &nameSlot)
	require.ErrorContains(t, err, "render budget exceeded")

	value, err := machine.fieldValue(reflect.ValueOf((*vmFastPropertyUser)(nil)), access, "Name")
	require.NoError(t, err)
	require.Nil(t, value)

	hidden := reflect.ValueOf(user).FieldByName("hidden")
	_, err = machine.fieldValue(hidden, object.PropertyAccess{}, "hidden")
	require.ErrorContains(t, err, "cannot return value")

	_, err = machine.fieldValue(hidden, object.PropertyAccess{Receiver: "user", Full: "user.hidden"}, "hidden")
	require.ErrorContains(t, err, "user")
}

func Test_VM_Write_Fast_Call_Value_With_Entry_Branches(t *testing.T) {
	ctx := plush.NewContext()
	var out strings.Builder

	err := writeFastCallValueWithEntry(&out, ctx, "missing", nil, nil, nil)
	require.ErrorContains(t, err, "invalid function")

	err = writeFastCallValueWithEntry(&out, ctx, "label", func() string { return "<ok>" }, nil, &fastBuilderCallCacheEntry{
		plan: cachedCallPlan(reflect.TypeOf(func() string { return "" })),
	})
	require.NoError(t, err)
	require.Equal(t, "&lt;ok&gt;", out.String())

	out.Reset()
	rawStruct := func() vmSegmentUser { return vmSegmentUser{Name: "ignored"} }
	entry := &fastBuilderCallCacheEntry{plan: cachedCallPlan(reflect.TypeOf(rawStruct))}
	err = writeFastCallValueWithEntry(&out, ctx, "rawStruct", rawStruct, nil, entry)
	require.NoError(t, err)
	require.Empty(t, out.String())

	out.Reset()
	rawErr := func() (string, error) { return "", errors.New("boom") }
	err = writeFastCallValueWithEntry(&out, ctx, "rawErr", rawErr, nil, cachedFastCallEntry(reflect.TypeOf(rawErr), rawErr, nil))
	require.ErrorContains(t, err, "could not call rawErr function")
}

func Test_VM_Fast_Call_Value_Wrapper_Branches(t *testing.T) {
	ctx := plush.NewContext()
	var out strings.Builder

	err := writeFastCallValue(&out, ctx, "nil", nil, nil, nil)
	require.ErrorContains(t, err, "invalid function")

	err = writeFastCallValue(&out, ctx, "bad", 7, nil, nil)
	require.ErrorContains(t, err, "invalid function")

	raw := func() string { return "<wrapped>" }
	err = writeFastCallValue(&out, ctx, "wrapped", &object.Native{Value: raw}, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "&lt;wrapped&gt;", out.String())

	_, err = fastCallValue("nil", nil, nil, ctx, nil)
	require.ErrorContains(t, err, "invalid function")

	_, err = fastCallValue("bad", 7, nil, ctx, nil)
	require.ErrorContains(t, err, "invalid function")

	value, err := fastCallValue("none", func() {}, nil, ctx, nil)
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = fastCallValue("nilReturn", func() *string { return nil }, nil, ctx, nil)
	require.NoError(t, err)
	require.Nil(t, value)

	_, err = fastCallValue("errReturn", func() (string, error) { return "", errors.New("boom") }, nil, ctx, nil)
	require.ErrorContains(t, err, "could not call errReturn function")

	value, err = fastCallValue("native", &object.Native{Value: raw}, nil, ctx, nil)
	require.NoError(t, err)
	require.Equal(t, "<wrapped>", value)

	value, err = fastCallValueWithEntry("fallback", raw, nil, ctx, &fastBuilderCallCacheEntry{
		plan: cachedCallPlan(reflect.TypeOf(raw)),
		valueInvoker: func(string, interface{}, *fastCallArgs) (interface{}, error) {
			return nil, errFastWriteUnsupported
		},
	})
	require.NoError(t, err)
	require.Equal(t, "<wrapped>", value)

	_, err = fastCallValueWithEntry("badValueInvoker", raw, nil, ctx, &fastBuilderCallCacheEntry{
		plan: cachedCallPlan(reflect.TypeOf(raw)),
		valueInvoker: func(string, interface{}, *fastCallArgs) (interface{}, error) {
			return nil, errors.New("bad fast value")
		},
	})
	require.ErrorContains(t, err, "bad fast value")

	_, err = fastCallValueWithEntry("missingPlan", raw, nil, ctx, &fastBuilderCallCacheEntry{})
	require.ErrorContains(t, err, "invalid function")

	_, err = fastCallValueWithEntry("badArg", func(value int) string { return "" }, fastArgs("bad"), ctx, &fastBuilderCallCacheEntry{
		plan: cachedCallPlan(reflect.TypeOf(func(value int) string { return "" })),
	})
	require.ErrorContains(t, err, "invalid argument")
}

func Test_VM_Cached_Call_Entry_For_Slot_Branches(t *testing.T) {
	raw := func() string { return "ok" }
	rt := reflect.TypeOf(raw)
	plan := cachedCallPlan(rt)
	var slot object.InlineCacheSlot

	require.Same(t, plan, cachedCallPlanForSlot(rt, nil))

	slot.Store(&callCacheEntry{rt: rt, plan: plan})
	require.Same(t, plan, cachedCallPlanForSlot(rt, &slot))

	var planSlot object.InlineCacheSlot
	planSlot.Store(plan)
	require.Same(t, plan, cachedCallPlanForSlot(rt, &planSlot))

	var staleEntrySlot object.InlineCacheSlot
	staleEntrySlot.Store(&callCacheEntry{rt: reflect.TypeOf(func(int) string { return "" }), plan: cachedCallPlan(reflect.TypeOf(func(int) string { return "" }))})
	require.Same(t, plan, cachedCallPlanForSlot(rt, &staleEntrySlot))

	var entrySlot object.InlineCacheSlot
	entrySlot.Store(plan)
	entry := cachedCallEntryForSlot(rt, raw, &entrySlot)
	require.NotNil(t, entry)
	require.Same(t, plan, entry.plan)

	slot = object.InlineCacheSlot{}
	slot.Store(&callCacheEntry{rt: rt, plan: plan})
	entry = cachedCallEntryForSlot(rt, raw, &slot)
	require.NotNil(t, entry)
	require.NotNil(t, entry.invoker)

	entry = cachedCallEntryForSlot(rt, nil, &slot)
	require.NotNil(t, entry)
	require.Equal(t, rt, entry.rt)

	other := func(value string) string { return value }
	entry = cachedCallEntryForSlot(reflect.TypeOf(other), other, &slot)
	require.NotNil(t, entry)
	require.Equal(t, reflect.TypeOf(other), entry.rt)
}
