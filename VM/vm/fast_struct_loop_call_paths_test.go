package vm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

func Test_VM_Fast_Struct_Loop_Call_Part_Branches(t *testing.T) {
	item := reflect.ValueOf(vmStructLoopProduct{Name: "<bot>"})

	t.Run("nil plan", func(t *testing.T) {
		require.NoError(t, writeFastStructLoopCallPart(&strings.Builder{}, plush.NewContext(), fastRenderBindings{}, &fastStructLoopRenderState{}, nil, 0, item))
	})

	t.Run("registered helper wins", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value string) string { return "direct:" + value },
		})
		SetFastHelper(ctx, "label", func(w FastWriter, args FastArgs) error {
			value, ok := args.String(0)
			require.True(t, ok)
			w.WriteEscapedString("fast:" + value)
			return nil
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg())

		var out strings.Builder
		require.NoError(t, writeFastStructLoopCallPart(&out, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item))
		require.Equal(t, "fast:&lt;bot&gt;", out.String())
	})

	t.Run("registered helper no args", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func() string { return "slow" },
		})
		SetFastHelper(ctx, "label", func(w FastWriter, args FastArgs) error {
			require.Zero(t, args.Len())
			w.WriteEscapedString("fast")
			return nil
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := &fastStructLoopCallPlan{call: &compiler.FastCallPlan{Name: "label", NameIndex: 0, Line: 1}}

		var out strings.Builder
		require.NoError(t, writeFastStructLoopCallPart(&out, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item))
		require.Equal(t, "fast", out.String())
	})

	t.Run("registered helper error", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value string) string { return "direct:" + value },
		})
		SetFastHelper(ctx, "label", func(FastWriter, FastArgs) error {
			return errors.New("fast helper boom")
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg())

		err := writeFastStructLoopCallPart(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item)
		require.ErrorContains(t, err, "line 1")
		require.ErrorContains(t, err, "fast helper boom")
	})

	t.Run("registered helper unsupported falls through", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value string) string { return "direct:" + value },
		})
		SetFastHelper(ctx, "label", func(FastWriter, FastArgs) error {
			return ErrFastUnsupported
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg())

		var out strings.Builder
		require.NoError(t, writeFastStructLoopCallPart(&out, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item))
		require.Equal(t, "direct:&lt;bot&gt;", out.String())
	})

	t.Run("registered helper arg error", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value string) string { return "direct:" + value },
		})
		SetFastHelper(ctx, "label", func(FastWriter, FastArgs) error {
			return nil
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, compiler.FastValuePlan{
			Kind:      compiler.FastValueName,
			NameIndex: 99,
			Value:     "missing",
			Line:      4,
		})

		err := writeFastStructLoopCallPart(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item)
		require.ErrorContains(t, err, "line 4")
		require.ErrorContains(t, err, `"missing": unknown identifier`)
	})

	t.Run("direct writer", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value string) string { return "direct:" + value },
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg())

		var out strings.Builder
		state := &fastStructLoopRenderState{}
		require.NoError(t, writeFastStructLoopCallPart(&out, ctx, bindings, state, plan, 0, item))
		require.Equal(t, "direct:&lt;bot&gt;", out.String())
		require.Same(t, plan, state.singleCall)
		require.NotNil(t, state.singleResolvedCall)
	})

	t.Run("nil state resolves directly", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value string) string { return "direct:" + value },
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg())

		var out strings.Builder
		require.NoError(t, writeFastStructLoopCallPart(&out, ctx, bindings, nil, plan, 0, item))
		require.Equal(t, "direct:&lt;bot&gt;", out.String())
	})

	t.Run("direct writer error", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(string) (string, error) {
				return "", errors.New("direct writer boom")
			},
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg())

		err := writeFastStructLoopCallPart(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item)
		require.ErrorContains(t, err, "line 1")
		require.ErrorContains(t, err, "direct writer boom")
	})

	t.Run("generic call signature", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value string, count int) string {
				return fmt.Sprintf("%s:%d", value, count)
			},
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg(), compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 3, Line: 1})

		var out strings.Builder
		require.NoError(t, writeFastStructLoopCallPart(&out, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item))
		require.Equal(t, "&lt;bot&gt;:3", out.String())
	})

	t.Run("entry invoker error", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value int) (string, error) {
				return "", errors.New("entry invoker boom")
			},
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 3, Line: 1})

		err := writeFastStructLoopCallPart(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item)
		require.ErrorContains(t, err, "line 1")
		require.ErrorContains(t, err, "entry invoker boom")
	})

	t.Run("entry invoker arg error", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value int) string { return fmt.Sprintf("%d", value) },
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 99, Value: "missing", Line: 6})

		err := writeFastStructLoopCallPart(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item)
		require.ErrorContains(t, err, `line 6: "missing": unknown identifier`)
	})

	t.Run("reflect fallback", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value string, count int32) string {
				return fmt.Sprintf("%s:%d", value, count)
			},
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg(), compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 2, Line: 1})

		var out strings.Builder
		require.NoError(t, writeFastStructLoopCallPart(&out, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item))
		require.Equal(t, "&lt;bot&gt;:2", out.String())
	})

	t.Run("variadic fallback", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(values ...string) string {
				return strings.Join(values, ":")
			},
		})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "<x>", Line: 1})

		var out strings.Builder
		require.NoError(t, writeFastStructLoopCallPart(&out, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item))
		require.Equal(t, "&lt;x&gt;", out.String())
	})

	t.Run("missing helper", func(t *testing.T) {
		ctx := plush.NewContext()
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg())

		err := writeFastStructLoopCallPart(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item)
		require.ErrorContains(t, err, `"label": unknown identifier`)
	})

	t.Run("invalid helper", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{"label": 12})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg())

		err := writeFastStructLoopCallPart(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item)
		require.ErrorContains(t, err, "invalid function")
	})

	t.Run("nil helper value", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{"label": &object.Native{Value: nil}})
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg())

		err := writeFastStructLoopCallPart(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item)
		require.ErrorContains(t, err, "invalid function")
	})

	t.Run("budget error", func(t *testing.T) {
		ctx := plush.NewContextWith(map[string]interface{}{
			"label": func(value string) string { return value },
		}).WithBudget(plush.NewBudget(0))
		bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"label"}}, ctx)
		plan := structLoopCallPlan(t, structLoopNameArg())

		err := writeFastStructLoopCallPart(&strings.Builder{}, ctx, bindings, &fastStructLoopRenderState{}, plan, 0, item)
		require.ErrorContains(t, err, "line 1")
	})
}

func Test_VM_Fast_Struct_Loop_Reflect_Call_Eligibility_Branches(t *testing.T) {
	entry := &fastBuilderCallCacheEntry{plan: &callPlan{numIn: 1, argTypes: []reflect.Type{stringType}}}
	plan := structLoopCallPlan(t, structLoopNameArg())

	require.False(t, canUseFastStructLoopReflectCall(nil, entry))
	require.False(t, canUseFastStructLoopReflectCall(plan, nil))
	require.False(t, canUseFastStructLoopReflectCall(plan, &fastBuilderCallCacheEntry{}))
	require.False(t, canUseFastStructLoopReflectCall(plan, &fastBuilderCallCacheEntry{plan: &callPlan{numIn: 2}}))
	require.False(t, canUseFastStructLoopReflectCall(plan, &fastBuilderCallCacheEntry{plan: &callPlan{numIn: 1, isVariadic: true}}))

	genericPlan := &fastStructLoopCallPlan{
		call: plan.call,
		args: []fastStructLoopCallArgPlan{{kind: fastStructLoopCallArgGeneric}},
	}
	require.False(t, canUseFastStructLoopReflectCall(genericPlan, entry))
	require.True(t, canUseFastStructLoopReflectCall(plan, entry))
}

func Test_VM_Fast_Struct_Loop_Call_Arg_Reflect_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"prefix": "pre"})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"prefix"}}, ctx)
	item := reflect.ValueOf(vmStructLoopProduct{Name: "bot"})
	accessPlan := structLoopCallPlan(t, structLoopNameArg()).args[0].accessPlan

	value, err := evalFastStructLoopCallArgReflect(nil, ctx, bindings, 0, item, stringType, "helper", 0)
	require.NoError(t, err)
	require.Equal(t, "", value.Interface())

	value, err = evalFastStructLoopCallArgReflect(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: accessPlan}, ctx, bindings, 0, item, stringType, "helper", 0)
	require.NoError(t, err)
	require.Equal(t, "bot", value.Interface())

	value, err = evalFastStructLoopCallArgReflect(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgAccessChain, accessPlan: accessPlan}, ctx, bindings, 0, reflect.Value{}, stringType, "helper", 0)
	require.NoError(t, err)
	require.Equal(t, "", value.Interface())

	value, err = evalFastStructLoopCallArgReflect(&fastStructLoopCallArgPlan{kind: fastStructLoopCallArgBinding, nameIndex: 0}, ctx, bindings, 0, item, stringType, "helper", 1)
	require.NoError(t, err)
	require.Equal(t, "pre", value.Interface())

	_, err = evalFastStructLoopCallArgReflect(&fastStructLoopCallArgPlan{
		kind:      fastStructLoopCallArgBinding,
		value:     compiler.FastValuePlan{Value: "missing"},
		line:      9,
		nameIndex: 99,
	}, ctx, bindings, 0, item, stringType, "helper", 1)
	require.ErrorContains(t, err, "line 9")

	_, err = evalFastStructLoopCallArgReflect(&fastStructLoopCallArgPlan{
		kind:       fastStructLoopCallArgAccessChain,
		accessPlan: accessPlan,
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, 0, item, stringType, "helper", 2)
	require.ErrorContains(t, err, "line 1")

	value, err = evalFastStructLoopCallArgReflect(&fastStructLoopCallArgPlan{
		kind: fastStructLoopCallArgAccessChain,
		accessPlan: &fastAccessChainPlan{steps: []fastAccessChainStep{{
			kind: fastAccessStepField,
		}}},
	}, ctx, bindings, 0, reflect.ValueOf(12), stringType, "helper", 3)
	require.NoError(t, err)
	require.Equal(t, "", value.Interface())

	_, err = evalFastStructLoopCallArgReflect(&fastStructLoopCallArgPlan{
		kind: fastStructLoopCallArgGeneric,
		value: compiler.FastValuePlan{
			Kind:      compiler.FastValuePath,
			NameIndex: -1,
			Path:      structLoopNameArg().Path,
		},
	}, plush.NewContext().WithBudget(plush.NewBudget(0)), bindings, 0, item, stringType, "helper", 4)
	require.ErrorContains(t, err, "line 1")
}

func Test_VM_Fast_Struct_Loop_Access_Chain_Reflect_Value_Edges(t *testing.T) {
	ctx := plush.NewContext()

	_, ok, err := evalFastAccessChainReflectValue(&fastAccessChainPlan{steps: []fastAccessChainStep{{
		kind: fastAccessStepField,
	}}}, reflect.ValueOf(12), ctx)
	require.NoError(t, err)
	require.False(t, ok)

	_, ok, err = evalFastAccessChainReflectValue(&fastAccessChainPlan{steps: []fastAccessChainStep{{
		kind:  fastAccessStepIndex,
		index: 1,
		line:  2,
	}}}, reflect.ValueOf([]string{}), ctx)
	require.ErrorContains(t, err, "line 2")
	require.True(t, ok)

	_, ok, err = evalFastAccessChainReflectValue(&fastAccessChainPlan{steps: []fastAccessChainStep{{
		kind:   fastAccessStepIndex,
		mapKey: reflect.ValueOf("missing"),
		line:   3,
	}}}, reflect.ValueOf(map[string]string{}), ctx)
	require.NoError(t, err)
	require.False(t, ok)

	_, ok, err = evalFastAccessChainReflectValue(&fastAccessChainPlan{steps: []fastAccessChainStep{
		{kind: fastAccessStepIndex, index: 0, line: 4},
		{kind: fastAccessStepField, line: 4},
	}}, reflect.ValueOf([]*vmAccessProfile{nil}), ctx)
	require.NoError(t, err)
	require.False(t, ok)

	_, ok, err = evalFastAccessChainReflectValue(&fastAccessChainPlan{steps: []fastAccessChainStep{
		{
			kind:     fastAccessStepField,
			line:     5,
			name:     "hidden",
			receiver: "private",
			full:     "private.hidden",
			lookup: propertyLookup{
				kind:       propertyLookupField,
				fieldIndex: []int{0},
			},
		},
		{kind: fastAccessStepField, line: 5},
	}}, reflect.ValueOf(vmReflectPrivate{hidden: "secret"}), ctx)
	require.ErrorContains(t, err, "line 5")
	require.False(t, ok)

	_, ok, err = evalFastAccessChainReflectValue(&fastAccessChainPlan{steps: []fastAccessChainStep{{
		kind: fastAccessStepKind(255),
	}}}, reflect.ValueOf(vmStructLoopProduct{}), ctx)
	require.NoError(t, err)
	require.False(t, ok)
}

func Test_VM_Fast_Struct_Loop_Resolved_Call_Cache_Branches(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"one": func(value string) string { return value },
		"two": func(value string) string { return value },
	})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"one", "two"}}, ctx)
	first := structLoopCallPlan(t, structLoopNameArg())
	first.call.Name = "one"
	first.call.NameIndex = 0
	second := structLoopCallPlan(t, structLoopNameArg())
	second.call.Name = "two"
	second.call.NameIndex = 1

	state := &fastStructLoopRenderState{}
	resolved, err := state.resolvedCall(first, bindings)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Same(t, first, state.singleCall)

	again, err := state.resolvedCall(first, bindings)
	require.NoError(t, err)
	require.Same(t, resolved, again)

	other, err := state.resolvedCall(second, bindings)
	require.NoError(t, err)
	require.NotNil(t, other)
	require.NotNil(t, state.calls)
	require.Same(t, resolved, state.calls[first])
	require.Same(t, other, state.calls[second])

	cachedOther, err := state.resolvedCall(second, bindings)
	require.NoError(t, err)
	require.Same(t, other, cachedOther)

	nilResolved, err := state.resolvedCall(nil, bindings)
	require.NoError(t, err)
	require.Nil(t, nilResolved)
}
