package vm

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

type vmAccessProfile struct {
	Name string
}

type vmAccessUser struct {
	Profile *vmAccessProfile
	Friends []vmAccessProfile
	Labels  map[string]string
	Counts  map[string]int
}

type vmAccessWrapper struct {
	Profile vmAccessProfile
}

func vmAccessProfileNamePlan(nameIndex int) *compiler.FastValuePlan {
	return &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: nameIndex,
		Value:     "user",
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Profile", Receiver: "user", Full: "user.Profile", Line: 1},
			{Kind: compiler.FastPathStepProperty, Value: "Name", Receiver: "user.Profile", Full: "user.Profile.Name", Line: 1},
		},
		Line: 1,
	}
}

func Test_VM_Fast_Field_Chain_Value_And_Output_Branches(t *testing.T) {
	ctx := plush.NewContext()
	user := vmAccessUser{Profile: &vmAccessProfile{Name: "<mido>"}}
	plan := vmAccessProfileNamePlan(-1)

	value, handled, err := evalFastFieldChainValue(plan, user, ctx)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "<mido>", value)

	_, handled, err = evalFastFieldChainValue(plan, user, plush.NewContext().WithBudget(plush.NewBudget(0)))
	require.ErrorContains(t, err, "line 1")
	require.True(t, handled)

	var out strings.Builder
	handled, err = writeFastFieldChainValue(&out, ctx, plan, user)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "&lt;mido&gt;", out.String())

	out.Reset()
	handled, err = writeFastFieldChainValue(&out, ctx, plan, vmAccessUser{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, out.String())

	_, handled, err = evalFastFieldChainValue(plan, nil, ctx)
	require.NoError(t, err)
	require.True(t, handled)

	handled, err = writeFastFieldChainValue(&out, ctx, plan, nil)
	require.NoError(t, err)
	require.True(t, handled)

	_, handled, err = evalFastFieldChainValue(plan, object.NullObject, ctx)
	require.NoError(t, err)
	require.True(t, handled)

	value, handled, err = evalFastFieldChainValue(plan, &object.Native{Value: user}, ctx)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "<mido>", value)

	handled, err = writeFastFieldChainValue(&out, ctx, plan, 7)
	require.NoError(t, err)
	require.False(t, handled)

	chain, ok := fastFieldChainPlanFor(plan, reflect.TypeOf(user))
	require.True(t, ok)
	require.NoError(t, writeFastFieldChainPlanOutput(&out, ctx, nil, reflect.ValueOf(user)))
	require.NoError(t, writeFastFieldChainPlanOutput(&out, ctx, chain, reflect.Value{}))
	require.NoError(t, writeFastFieldChainPlanOutput(&out, ctx, &fastFieldChainPlan{}, reflect.ValueOf(user)))
	require.ErrorContains(t, writeFastFieldChainPlanOutput(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), chain, reflect.ValueOf(user)), "line 1")

	hiddenStep := fastFieldChainStep{
		name:      "hidden",
		receiver:  "user",
		full:      "user.hidden",
		line:      4,
		fieldType: stringType,
		lookup:    cachedPropertyLookup(reflect.TypeOf(vmFastPropertyUser{}), "hidden"),
	}
	hiddenChain := &fastFieldChainPlan{steps: []fastFieldChainStep{hiddenStep}}
	_, handled, err = evalFastFieldChainValue(&compiler.FastValuePlan{
		Kind: compiler.FastValuePath,
		Path: []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "hidden", Receiver: "user", Full: "user.hidden", Line: 4}},
	}, vmFastPropertyUser{hidden: "secret"}, ctx)
	require.ErrorContains(t, err, "line 4")
	require.True(t, handled)
	require.ErrorContains(t, writeFastFieldChainPlanOutput(&out, ctx, hiddenChain, reflect.ValueOf(vmFastPropertyUser{hidden: "secret"})), "line 4")
	require.ErrorContains(t, writeFastFieldChainPlanOutput(&out, ctx, &fastFieldChainPlan{steps: []fastFieldChainStep{hiddenStep, hiddenStep}}, reflect.ValueOf(vmFastPropertyUser{hidden: "secret"})), "line 4")

	wrapperPlan := &compiler.FastValuePlan{
		Kind: compiler.FastValuePath,
		Path: []compiler.FastPathStep{{Kind: compiler.FastPathStepProperty, Value: "Profile", Receiver: "wrapper", Full: "wrapper.Profile", Line: 5}},
	}
	wrapperChain, ok := fastFieldChainPlanFor(wrapperPlan, reflect.TypeOf(vmAccessWrapper{}))
	require.True(t, ok)
	out.Reset()
	require.NoError(t, writeFastFieldChainPlanOutput(&out, ctx, wrapperChain, reflect.ValueOf(vmAccessWrapper{Profile: vmAccessProfile{Name: "nested"}})))
	require.Empty(t, out.String())
}

func Test_VM_Fast_Access_Chain_Root_Field_And_Intermediate_Branches(t *testing.T) {
	rv, nilValue, ok := fastAccessChainRootValue(object.NullObject)
	require.False(t, rv.IsValid())
	require.True(t, nilValue)
	require.True(t, ok)

	rv, nilValue, ok = fastAccessChainRootValue(&object.Native{Value: (*vmAccessUser)(nil)})
	require.False(t, rv.IsValid())
	require.True(t, nilValue)
	require.True(t, ok)

	rv, nilValue, ok = fastAccessChainRootValue(&object.String{Value: "name"})
	require.False(t, rv.IsValid())
	require.False(t, nilValue)
	require.False(t, ok)

	rv, nilValue, ok = fastAccessChainRootValue(vmAccessUser{})
	require.True(t, rv.IsValid())
	require.False(t, nilValue)
	require.True(t, ok)

	rv, nilValue, ok = fastAccessChainRootValue([]string{"a"})
	require.True(t, rv.IsValid())
	require.False(t, nilValue)
	require.True(t, ok)

	rv, nilValue, ok = fastAccessChainRootValue(map[string]string{"a": "b"})
	require.True(t, rv.IsValid())
	require.False(t, nilValue)
	require.True(t, ok)

	rv, nilValue, ok = fastAccessChainRootValue(7)
	require.False(t, rv.IsValid())
	require.False(t, nilValue)
	require.False(t, ok)

	step := &fastAccessChainStep{
		kind:      fastAccessStepField,
		name:      "Name",
		receiver:  "profile",
		full:      "profile.Name",
		line:      4,
		fieldType: stringType,
		lookup:    cachedPropertyLookup(reflect.TypeOf(vmAccessProfile{}), "Name"),
	}
	field, ok, err := fastAccessFieldValue(reflect.ValueOf(vmAccessProfile{Name: "Mido"}), step)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "Mido", field.Interface())

	field, ok, err = fastAccessFieldValue(reflect.Value{}, step)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, field.IsValid())

	field, ok, err = fastAccessFieldValue(reflect.ValueOf("not-struct"), step)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, field.IsValid())

	value, ok, err := fastAccessIntermediateValue(reflect.Value{}, step)
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, value.IsValid())

	hidden := reflect.ValueOf(vmFastPropertyUser{hidden: "secret"}).FieldByName("hidden")
	value, ok, err = fastAccessIntermediateValue(hidden, &fastAccessChainStep{name: "hidden", receiver: "user", full: "user.hidden", line: 9})
	require.ErrorContains(t, err, "line 9")
	require.False(t, ok)
	require.False(t, value.IsValid())
}

func Test_VM_Fast_Access_Chain_Plan_Value_And_Output_Edges(t *testing.T) {
	ctx := plush.NewContext()
	user := vmAccessUser{
		Profile: &vmAccessProfile{Name: "<mido>"},
		Friends: []vmAccessProfile{{Name: "<amy>"}},
		Labels:  map[string]string{"status": "<ready>"},
	}
	fieldChain, ok := fastAccessChainPlanFor(vmAccessProfileNamePlan(-1), reflect.TypeOf(user))
	require.True(t, ok)

	value, handled, err := evalFastAccessChainPlanValue(fieldChain, reflect.ValueOf(user), ctx)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "<mido>", value)

	value, handled, err = evalFastAccessChainPlanValue(&fastAccessChainPlan{}, reflect.ValueOf("root"), ctx)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "root", value)

	value, handled, err = evalFastAccessChainPlanValue(fieldChain, reflect.ValueOf("bad-root"), ctx)
	require.NoError(t, err)
	require.True(t, handled)
	require.Nil(t, value)

	_, handled, err = evalFastAccessChainPlanValue(fieldChain, reflect.ValueOf(user), plush.NewContext().WithBudget(plush.NewBudget(0)))
	require.ErrorContains(t, err, "line 1")
	require.True(t, handled)

	hiddenChain := &fastAccessChainPlan{steps: []fastAccessChainStep{{
		kind:      fastAccessStepField,
		name:      "hidden",
		receiver:  "user",
		full:      "user.hidden",
		line:      4,
		fieldType: stringType,
		lookup:    cachedPropertyLookup(reflect.TypeOf(vmFastPropertyUser{}), "hidden"),
	}}}
	_, handled, err = evalFastAccessChainPlanValue(hiddenChain, reflect.ValueOf(vmFastPropertyUser{hidden: "secret"}), ctx)
	require.ErrorContains(t, err, "line 4")
	require.True(t, handled)

	friendsPlan := &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Friends", Receiver: "user", Full: "user.Friends", Line: 2},
			{Kind: compiler.FastPathStepIndexInteger, Index: 9, Line: 2},
		},
	}
	indexChain, ok := fastAccessChainPlanFor(friendsPlan, reflect.TypeOf(user))
	require.True(t, ok)
	_, handled, err = evalFastAccessChainPlanValue(indexChain, reflect.ValueOf(user), ctx)
	require.ErrorContains(t, err, "line 2")
	require.True(t, handled)

	missingMapPlan := &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Labels", Receiver: "user", Full: "user.Labels", Line: 3},
			{Kind: compiler.FastPathStepIndexString, Value: "missing", Line: 3},
		},
	}
	mapChain, ok := fastAccessChainPlanFor(missingMapPlan, reflect.TypeOf(user))
	require.True(t, ok)
	value, handled, err = evalFastAccessChainPlanValue(mapChain, reflect.ValueOf(user), ctx)
	require.NoError(t, err)
	require.True(t, handled)
	require.Nil(t, value)

	var out strings.Builder
	handled, err = writeFastAccessChainValue(&out, ctx, vmAccessProfileNamePlan(-1), nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, out.String())

	handled, err = writeFastAccessChainValue(&out, ctx, vmAccessProfileNamePlan(-1), 7)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = writeFastAccessChainValue(&out, ctx, &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "bad"}, user)
	require.NoError(t, err)
	require.False(t, handled)

	out.Reset()
	handled, err = writeFastAccessChainValue(&out, ctx, vmAccessProfileNamePlan(-1), user)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "&lt;mido&gt;", out.String())

	out.Reset()
	handled, err = writeFastAccessChainPlanOutput(&out, ctx, &fastAccessChainPlan{}, reflect.ValueOf(user))
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, out.String())

	handled, err = writeFastAccessChainPlanOutput(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), fieldChain, reflect.ValueOf(user))
	require.ErrorContains(t, err, "line 1")
	require.True(t, handled)

	handled, err = writeFastAccessChainPlanOutput(&out, ctx, fieldChain, reflect.ValueOf("bad-root"))
	require.NoError(t, err)
	require.True(t, handled)

	handled, err = writeFastAccessChainPlanOutput(&out, ctx, hiddenChain, reflect.ValueOf(vmFastPropertyUser{hidden: "secret"}))
	require.ErrorContains(t, err, "line 4")
	require.True(t, handled)

	handled, err = writeFastAccessChainPlanOutput(&out, ctx, indexChain, reflect.ValueOf(user))
	require.ErrorContains(t, err, "line 2")
	require.True(t, handled)

	out.Reset()
	handled, err = writeFastAccessChainPlanOutput(&out, ctx, mapChain, reflect.ValueOf(user))
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, out.String())

	structFieldPlan := &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Profile", Receiver: "wrapper", Full: "wrapper.Profile", Line: 5},
		},
	}
	structFieldChain, ok := fastAccessChainPlanFor(structFieldPlan, reflect.TypeOf(vmAccessWrapper{}))
	require.True(t, ok)
	handled, err = writeFastAccessChainPlanOutput(&out, ctx, structFieldChain, reflect.ValueOf(vmAccessWrapper{Profile: vmAccessProfile{Name: "nested"}}))
	require.NoError(t, err)
	require.True(t, handled)
	require.Empty(t, out.String())
}

func Test_VM_Fast_Top_Level_Access_Chain_Output_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"user": vmAccessUser{
			Profile: &vmAccessProfile{Name: "<mido>"},
			Friends: []vmAccessProfile{{Name: "<amy>"}},
			Labels:  map[string]string{"status": "<ready>"},
			Counts:  map[string]int{"count": 3},
		},
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"user"}}, ctx)

	fieldPlan := vmAccessProfileNamePlan(0)
	var out strings.Builder
	handled, ok, err := writeFastTopLevelAccessChainOutput(&out, ctx, bindings, fieldPlan, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, ok)
	require.Equal(t, "&lt;mido&gt;", out.String())

	out.Reset()
	slicePlan := &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: 0,
		Value:     "user",
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Friends", Receiver: "user", Full: "user.Friends", Line: 1},
			{Kind: compiler.FastPathStepIndexInteger, Index: 0, Line: 1},
			{Kind: compiler.FastPathStepProperty, Value: "Name", Receiver: "user.Friends[0]", Full: "user.Friends[0].Name", Line: 1},
		},
		Line: 1,
	}
	handled, ok, err = writeFastTopLevelAccessChainOutput(&out, ctx, bindings, slicePlan, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, ok)
	require.Equal(t, "&lt;amy&gt;", out.String())

	out.Reset()
	mapPlan := &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: 0,
		Value:     "user",
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Labels", Receiver: "user", Full: "user.Labels", Line: 1},
			{Kind: compiler.FastPathStepIndexString, Value: "status", Line: 1},
		},
		Line: 1,
	}
	handled, ok, err = writeFastTopLevelAccessChainOutput(&out, ctx, bindings, mapPlan, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, ok)
	require.Equal(t, "&lt;ready&gt;", out.String())

	out.Reset()
	countPlan := &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: 0,
		Value:     "user",
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Counts", Receiver: "user", Full: "user.Counts", Line: 1},
			{Kind: compiler.FastPathStepIndexString, Value: "count", Line: 1},
		},
		Line: 1,
	}
	handled, ok, err = writeFastTopLevelAccessChainOutput(&out, ctx, bindings, countPlan, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, ok)
	require.Equal(t, "3", out.String())

	handled, ok, err = writeFastTopLevelAccessChainOutput(&out, ctx, bindings, &compiler.FastValuePlan{Kind: compiler.FastValueString}, nil)
	require.NoError(t, err)
	require.False(t, handled)
	require.False(t, ok)

	require.False(t, canUseFastTopLevelAccessChain(&compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: 0,
		Path:      []compiler.FastPathStep{{Kind: compiler.FastPathStepKind(255)}},
	}))

	handled, err = writeFastTopLevelAccessChainRaw(&out, ctx, &compiler.FastValuePlan{Kind: compiler.FastValueString}, ctx.Value("user"), nil)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = writeFastTopLevelAccessChainRaw(&out, ctx, fieldPlan, nil, nil)
	require.NoError(t, err)
	require.True(t, handled)

	var unknownKindSlot object.InlineCacheSlot
	unknownKindSlot.Store(&fastTopLevelAccessCacheEntry{
		typ:  reflect.TypeOf(vmAccessUser{}),
		kind: fastTopLevelAccessKind(255),
	})
	handled, err = writeFastTopLevelAccessChainRaw(&out, ctx, fieldPlan, ctx.Value("user"), &unknownKindSlot)
	require.NoError(t, err)
	require.False(t, handled)

	handled, ok, err = writeFastTopLevelAccessChainOutput(&out, ctx, bindings, &compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: 99, NullOnMissing: true, Path: fieldPlan.Path}, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, ok)

	handled, ok, err = writeFastTopLevelAccessChainOutput(&out, ctx, bindings, &compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: 99, Value: "missing", Path: fieldPlan.Path}, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.False(t, ok)
}

func Test_VM_Fast_Access_Index_Plan_Branches(t *testing.T) {
	_, _, ok := buildFastIndexAccessStep(reflect.TypeOf([2]string{}), &compiler.FastPathStep{Kind: compiler.FastPathStepIndexString, Value: "bad"})
	require.False(t, ok)

	_, _, ok = buildFastIndexAccessStep(reflect.TypeOf([2]string{}), &compiler.FastPathStep{Kind: compiler.FastPathStepIndexInteger, Index: 3})
	require.False(t, ok)

	_, _, ok = buildFastIndexAccessStep(reflect.TypeOf([]string{}), &compiler.FastPathStep{Kind: compiler.FastPathStepIndexInteger, Index: -1})
	require.False(t, ok)

	step, next, ok := buildFastIndexAccessStep(reflect.TypeOf(map[int]string{}), &compiler.FastPathStep{Kind: compiler.FastPathStepIndexInteger, Index: 7})
	require.True(t, ok)
	require.Equal(t, stringType, next)
	require.True(t, step.mapKey.IsValid())

	_, _, ok = buildFastIndexAccessStep(reflect.TypeOf(map[int]string{}), &compiler.FastPathStep{Kind: compiler.FastPathStepIndexString, Value: "bad"})
	require.False(t, ok)

	_, _, ok = buildFastIndexAccessStep(reflect.TypeOf(struct{}{}), &compiler.FastPathStep{Kind: compiler.FastPathStepIndexInteger, Index: 0})
	require.False(t, ok)
}

func Test_VM_Fast_Access_Index_Value_Edge_Branches(t *testing.T) {
	_, ok, err := fastAccessIndexValue(reflect.Value{}, &fastAccessChainStep{index: 0})
	require.NoError(t, err)
	require.False(t, ok)

	_, ok, err = fastAccessIndexValue(reflect.ValueOf(map[bool]string{}), &fastAccessChainStep{index: 1})
	require.NoError(t, err)
	require.False(t, ok)

	_, ok, err = fastAccessIndexValue(reflect.ValueOf(12), &fastAccessChainStep{index: 0})
	require.NoError(t, err)
	require.False(t, ok)
}
