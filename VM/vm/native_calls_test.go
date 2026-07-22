package vm

import (
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/stretchr/testify/require"
)

type vmOptionalHelperContext plush.HelperContext
type vmOptionalMap map[string]interface{}
type vmNativeIntObject int
type vmPointerArgSection struct {
	Name string
}

func (v vmNativeIntObject) Type() object.ObjectType { return object.NATIVE_OBJ }
func (v vmNativeIntObject) Inspect() string         { return fmt.Sprintf("%d", v) }

func Test_VM_Fast_Reflect_Args_Into_Branches(t *testing.T) {
	ctx := plush.NewContext()
	plan := cachedCallPlan(reflect.TypeOf(func(string, int64) {}))
	args, err := fastReflectArgsInto("helper", plan, fastArgs(&object.String{Value: "name"}, &object.Integer{Value: 7}), ctx, nil)
	require.NoError(t, err)
	require.Len(t, args, 2)
	require.Equal(t, "name", args[0].Interface())
	require.Equal(t, int64(7), args[1].Interface())

	_, err = fastReflectArgsInto("helper", plan, fastArgs("a", int64(1), "extra"), ctx, nil)
	require.ErrorContains(t, err, "too many arguments")

	_, err = fastReflectArgsInto("helper", plan, fastArgs("a"), ctx, nil)
	require.ErrorContains(t, err, "too few arguments")

	variadic := cachedCallPlan(reflect.TypeOf(func(string, ...int) {}))
	args, err = fastReflectArgsInto("helper", variadic, fastArgs("n", 1, 2), ctx, nil)
	require.NoError(t, err)
	require.Len(t, args, 3)
	require.Equal(t, 2, args[2].Interface())

	_, err = fastReflectArgsInto("helper", variadic, fastArgs("n", "bad"), ctx, nil)
	require.ErrorContains(t, err, "invalid argument")

	_, err = fastReflectArgsInto("helper", variadic, fastArgs(), ctx, nil)
	require.ErrorContains(t, err, "too few arguments")

	withMap := cachedCallPlan(reflect.TypeOf(func(string, map[string]interface{}) {}))
	args, err = fastReflectArgsInto("helper", withMap, fastArgs("n"), ctx, nil)
	require.NoError(t, err)
	require.Len(t, args, 2)
	require.Equal(t, map[string]interface{}{}, args[1].Interface())

	withHelperContext := cachedCallPlan(reflect.TypeOf(func(string, plush.HelperContext) {}))
	args, err = fastReflectArgsInto("helper", withHelperContext, fastArgs("n"), ctx, nil)
	require.NoError(t, err)
	require.Len(t, args, 2)
	require.IsType(t, plush.HelperContext{}, args[1].Interface())

	_, err = fastReflectArgsInto("helper", plan, fastArgs("a", struct{}{}), ctx, nil)
	require.ErrorContains(t, err, "invalid argument")
}

func Test_VM_Reflect_Args_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())

	plan := cachedCallPlan(reflect.TypeOf(func(string, map[string]interface{}) {}))
	machine.stack[0] = &object.String{Value: "name"}
	machine.sp = 1
	args, err := machine.reflectArgs("helper", plan, 1, nil, nil)
	require.NoError(t, err)
	require.Len(t, args, 2)
	require.Equal(t, "name", args[0].Interface())
	require.Equal(t, map[string]interface{}{}, args[1].Interface())

	variadic := cachedCallPlan(reflect.TypeOf(func(string, ...int) {}))
	machine.stack[0] = &object.String{Value: "name"}
	machine.stack[1] = &object.Integer{Value: 3}
	machine.sp = 2
	args, err = machine.reflectArgs("helper", variadic, 2, nil, nil)
	require.NoError(t, err)
	require.Len(t, args, 2)
	require.Equal(t, 3, args[1].Interface())

	tooMany := cachedCallPlan(reflect.TypeOf(func(string) {}))
	machine.stack[0] = &object.String{Value: "a"}
	machine.stack[1] = &object.String{Value: "b"}
	machine.sp = 2
	_, err = machine.reflectArgs("helper", tooMany, 2, nil, nil)
	require.ErrorContains(t, err, "too many arguments")

	machine.sp = 0
	_, err = machine.reflectArgs("helper", variadic, 0, nil, nil)
	require.ErrorContains(t, err, "too few arguments")

	fixedBad := cachedCallPlan(reflect.TypeOf(func(int) {}))
	machine.stack[0] = &object.String{Value: "bad"}
	machine.sp = 1
	_, err = machine.reflectArgs("helper", fixedBad, 1, nil, nil)
	require.ErrorContains(t, err, "invalid argument")

	machine.stack[0] = &object.String{Value: "name"}
	machine.stack[1] = &object.String{Value: "bad"}
	machine.sp = 2
	_, err = machine.reflectArgs("helper", variadic, 2, nil, nil)
	require.ErrorContains(t, err, "invalid argument")
}

func Test_VM_Fast_Call_Value_With_Entry_Branches(t *testing.T) {
	ctx := plush.NewContext()
	raw := func(value string) string { return value }
	entry := cachedFastCallEntry(reflect.TypeOf(raw), raw, nil)

	var out strings.Builder
	require.NoError(t, writeFastCallValueWithEntry(&out, ctx, "helper", raw, fastArgs("<ok>"), entry))
	require.Equal(t, "&lt;ok&gt;", out.String())

	_, err := fastCallValueWithEntry("bad", raw, fastArgs(struct{}{}), ctx, entry)
	require.ErrorContains(t, err, "invalid argument")

	out.Reset()
	require.ErrorContains(t, writeFastCallValueWithEntry(&out, ctx, "bad", raw, fastArgs(struct{}{}), entry), "invalid argument")

	require.ErrorContains(t, writeFastCallValueWithEntry(&out, ctx, "nil", raw, fastArgs(), nil), "invalid function")

	called := false
	voidRaw := func() { called = true }
	voidEntry := &fastBuilderCallCacheEntry{
		rt:   reflect.TypeOf(voidRaw),
		plan: cachedCallPlan(reflect.TypeOf(voidRaw)),
	}
	value, err := fastCallValueWithEntry("void", voidRaw, fastArgs(), ctx, voidEntry)
	require.NoError(t, err)
	require.Nil(t, value)
	require.True(t, called)

	failRaw := func() (string, error) { return "", fmt.Errorf("boom") }
	failEntry := &fastBuilderCallCacheEntry{
		rt:   reflect.TypeOf(failRaw),
		plan: cachedCallPlan(reflect.TypeOf(failRaw)),
	}
	_, err = fastCallValueWithEntry("fail", failRaw, fastArgs(), ctx, failEntry)
	require.ErrorContains(t, err, "could not call fail function")

	valueErrEntry := &fastBuilderCallCacheEntry{
		rt:   reflect.TypeOf(raw),
		plan: cachedCallPlan(reflect.TypeOf(raw)),
		valueInvoker: func(name string, raw interface{}, args *fastCallArgs) (interface{}, error) {
			return nil, fmt.Errorf("value boom")
		},
	}
	_, err = fastCallValueWithEntry("valueErr", raw, fastArgs("x"), ctx, valueErrEntry)
	require.ErrorContains(t, err, "value boom")
}

func Test_VM_Fast_Call_Value_With_Helper_Context_Values(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"name": "Mido"})

	plushHelper := func(help plush.HelperContext) string {
		return "plush:" + help.Value("name").(string)
	}
	value, err := fastCallValue("plushHelper", plushHelper, fastArgs(), ctx, nil)
	require.NoError(t, err)
	require.Equal(t, "plush:Mido", value)

	hctxHelper := func(help hctx.HelperContext) string {
		return "hctx:" + help.Value("name").(string)
	}
	value, err = fastCallValue("hctxHelper", hctxHelper, fastArgs(), ctx, nil)
	require.NoError(t, err)
	require.Equal(t, "hctx:Mido", value)

	plushHelperErr := func(plush.HelperContext) (string, error) {
		return "ignored", fmt.Errorf("boom")
	}
	_, err = fastCallValue("plushHelperErr", plushHelperErr, fastArgs(), ctx, nil)
	require.ErrorContains(t, err, "could not call plushHelperErr function")
}

func Test_VM_Contextual_Value_Fast_Invoker_For_String_Helper_Context(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"suffix": "-ctx"})

	raw := func(value string, help plush.HelperContext) string {
		return value + help.Value("suffix").(string)
	}
	entry := cachedFastCallEntry(reflect.TypeOf(raw), raw, nil)
	require.NotNil(t, entry.contextualValueInvoker)

	value, err := fastCallValueWithEntry("label", raw, fastArgs("name"), ctx, entry)
	require.NoError(t, err)
	require.Equal(t, "name-ctx", value)
}

func Test_VM_Contextual_Value_Fast_Invoker_For_Arbitrary_Return(t *testing.T) {
	type menuLike struct {
		Name string
	}
	ctx := plush.NewContextWith(map[string]interface{}{"suffix": "-ctx"})

	raw := func(value string, help plush.HelperContext) *menuLike {
		return &menuLike{Name: value + help.Value("suffix").(string)}
	}
	entry := cachedFastCallEntry(reflect.TypeOf(raw), raw, nil)
	require.NotNil(t, entry.contextualValueInvoker)

	value, err := fastCallValueWithEntry("menu_items", raw, fastArgs("main"), ctx, entry)
	require.NoError(t, err)
	require.Equal(t, &menuLike{Name: "main-ctx"}, value)
}

func Test_VM_Contextual_Value_Fast_Invoker_For_Hctx_Helper_Context_Error(t *testing.T) {
	ctx := plush.NewContext()

	raw := func(value string, help hctx.HelperContext) (template.HTML, error) {
		return "", fmt.Errorf("boom %s", value)
	}
	entry := cachedFastCallEntry(reflect.TypeOf(raw), raw, nil)
	require.NotNil(t, entry.contextualValueInvoker)

	_, err := fastCallValueWithEntry("datetime", raw, fastArgs("date"), ctx, entry)
	require.ErrorContains(t, err, "could not call datetime function: boom date")
}

func Test_VM_Reset_Child_Cleans_Reused_State(t *testing.T) {
	oldCtx := plush.NewContext()
	newCtx := plush.NewContext()
	oldFrame := NewFrame(&object.Closure{Fn: &object.CompiledFunction{}}, 0)
	extraFrame := NewFrame(&object.Closure{Fn: &object.CompiledFunction{}}, 0)
	machine := &VM{
		stack:       []object.Object{&object.String{Value: "old"}, &object.String{Value: "stale"}},
		stackMax:    4,
		frames:      []*Frame{oldFrame, extraFrame},
		framesIndex: 2,
		ctx:         oldCtx,
		lastPopped:  &object.String{Value: "last"},
		lastIP:      99,
		halted:      true,
	}
	closure := &object.Closure{Fn: &object.CompiledFunction{NumLocals: 1}}
	arg := &object.String{Value: "arg"}

	machine.resetChild(closure, []object.Object{arg}, newCtx)

	require.Same(t, arg, machine.stack[0])
	require.Nil(t, machine.stack[1])
	require.Equal(t, 1, machine.sp)
	require.Equal(t, 1, machine.stackMax)
	require.Equal(t, 1, machine.framesIndex)
	require.NotNil(t, machine.frames[0])
	require.Nil(t, machine.frames[1])
	require.Same(t, newCtx, machine.ctx)
	require.Nil(t, machine.lastPopped)
	require.Zero(t, machine.lastIP)
	require.False(t, machine.halted)

	machine.frames[0] = nil
	machine.resetChild(closure, nil, newCtx)
	require.NotNil(t, machine.frames[0])
}

func Test_VM_Fast_Reflect_Arg_For_Call_Branches(t *testing.T) {
	value, err := fastReflectArgForCall("helper", 0, Null, stringType)
	require.NoError(t, err)
	require.Equal(t, "", value.Interface())

	value, err = fastReflectArgForCall("helper", 0, nil, reflect.TypeOf(0))
	require.NoError(t, err)
	require.Equal(t, 0, value.Interface())

	value, err = fastReflectArgForCall("helper", 0, &object.Native{Value: int32(3)}, reflect.TypeOf(int64(0)))
	require.NoError(t, err)
	require.Equal(t, int64(3), value.Interface())

	value, err = fastReflectArgForCall("helper", 0, int32(4), reflect.TypeOf(int64(0)))
	require.NoError(t, err)
	require.Equal(t, int64(4), value.Interface())

	value, err = fastReflectArgForCall("helper", 0, "raw", stringType)
	require.NoError(t, err)
	require.Equal(t, "raw", value.Interface())

	value, err = fastReflectArgForCall("helper", 0, &object.Array{Elements: []object.Object{&object.String{Value: "x"}}}, reflect.TypeOf([]interface{}{}))
	require.NoError(t, err)
	require.Equal(t, []interface{}{"x"}, value.Interface())

	section := vmPointerArgSection{Name: "hero"}
	value, err = fastReflectArgForCall("helper", 0, section, reflect.TypeOf(&vmPointerArgSection{}))
	require.NoError(t, err)
	require.Equal(t, "hero", value.Interface().(*vmPointerArgSection).Name)

	value, err = fastReflectArgForCall("helper", 0, &object.Native{Value: section}, reflect.TypeOf(&vmPointerArgSection{}))
	require.NoError(t, err)
	require.Equal(t, "hero", value.Interface().(*vmPointerArgSection).Name)

	_, err = fastReflectArgForCall("helper", 1, "bad", reflect.TypeOf(0))
	require.ErrorContains(t, err, "invalid argument")
}

func Test_VM_Reflect_Arg_And_Convertible_Value_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())

	value, err := machine.reflectArg("helper", 0, object.NullObject, stringType)
	require.NoError(t, err)
	require.Equal(t, "", value.Interface())

	value, err = machine.reflectArg("helper", 1, &object.Native{Value: nil}, reflect.TypeOf(0))
	require.NoError(t, err)
	require.Equal(t, 0, value.Interface())

	value, err = machine.reflectArg("helper", 2, &object.Native{Value: int32(7)}, reflect.TypeOf(int64(0)))
	require.NoError(t, err)
	require.Equal(t, int64(7), value.Interface())

	value, err = machine.reflectArg("helper", 3, &object.Array{Elements: []object.Object{&object.String{Value: "x"}}}, reflect.TypeOf([]interface{}{}))
	require.NoError(t, err)
	require.Equal(t, []interface{}{"x"}, value.Interface())

	value, err = machine.reflectArg("helper", 4, vmNativeIntObject(9), reflect.TypeOf(int64(0)))
	require.NoError(t, err)
	require.Equal(t, int64(9), value.Interface())

	value, err = machine.reflectArg("helper", 5, &object.Native{Value: vmPointerArgSection{Name: "hero"}}, reflect.TypeOf(&vmPointerArgSection{}))
	require.NoError(t, err)
	require.Equal(t, "hero", value.Interface().(*vmPointerArgSection).Name)

	_, err = machine.reflectArg("helper", 6, &object.Native{Value: struct{}{}}, stringType)
	require.ErrorContains(t, err, "invalid argument")

	value, ok := fastReflectArg(&object.Native{Value: nil}, stringType)
	require.True(t, ok)
	require.Equal(t, "", value.Interface())

	value, ok = fastReflectArg(&object.String{Value: "fast"}, stringType)
	require.True(t, ok)
	require.Equal(t, "fast", value.Interface())

	value, ok = fastReflectArg(object.TrueObject, reflect.TypeOf(false))
	require.True(t, ok)
	require.Equal(t, true, value.Interface())

	value, ok = fastReflectArg(&object.Integer{Value: 5}, reflect.TypeOf(int64(0)))
	require.True(t, ok)
	require.Equal(t, int64(5), value.Interface())

	value, ok = fastReflectArg(&object.Float{Value: 1.5}, reflect.TypeOf(float64(0)))
	require.True(t, ok)
	require.Equal(t, 1.5, value.Interface())

	_, ok = fastReflectArg(&object.Array{}, stringType)
	require.False(t, ok)

	value, ok = fastConvertibleValue(nil, stringType)
	require.True(t, ok)
	require.Equal(t, "", value.Interface())

	value, ok = fastConvertibleValue(vmFastOutputStringer("value"), reflect.TypeOf((*fmt.Stringer)(nil)).Elem())
	require.True(t, ok)
	require.Equal(t, "stringer:value", value.Interface().(fmt.Stringer).String())

	value, ok = fastConvertibleValue(vmPointerArgSection{Name: "copy"}, reflect.TypeOf(&vmPointerArgSection{}))
	require.True(t, ok)
	require.Equal(t, "copy", value.Interface().(*vmPointerArgSection).Name)

	_, ok = fastConvertibleValue(struct{}{}, stringType)
	require.False(t, ok)
}

func Test_VM_Closure_From_Stack_Branches(t *testing.T) {
	fn := &object.CompiledFunction{}
	machine := &VM{
		constants: []object.Object{fn},
		stack:     make([]object.Object, StackSize),
	}
	machine.stack[0] = &object.String{Value: "one"}
	machine.stack[1] = &object.String{Value: "two"}
	machine.sp = 2

	closure, err := machine.closureFromStack(0, 2)
	require.NoError(t, err)
	require.Same(t, fn, closure.Fn)
	require.Len(t, closure.Free, 2)
	require.Equal(t, &object.String{Value: "one"}, closure.Free[0])
	require.Equal(t, &object.String{Value: "two"}, closure.Free[1])
	require.Zero(t, machine.sp)

	machine.constants[0] = &object.String{Value: "nope"}
	_, err = machine.closureFromStack(0, 0)
	require.ErrorContains(t, err, "not a function")
}

func Test_VM_Optional_Arg_Branches(t *testing.T) {
	ctx := plush.NewContext()
	machine := newRuntimeHelperTestVM(ctx)

	value, ok := fastOptionalArg(optionalArgHelperContext, reflect.TypeOf(plush.HelperContext{}), ctx)
	require.True(t, ok)
	require.IsType(t, plush.HelperContext{}, value.Interface())

	value, ok = fastOptionalArg(optionalArgHelperContext, reflect.TypeOf(vmOptionalHelperContext{}), ctx)
	require.True(t, ok)
	require.IsType(t, vmOptionalHelperContext{}, value.Interface())

	value, ok = fastOptionalArg(optionalArgHelperContext, reflect.TypeOf((*hctx.HelperContext)(nil)).Elem(), ctx)
	require.True(t, ok)
	require.Implements(t, (*hctx.HelperContext)(nil), value.Interface())

	value, ok = fastOptionalArg(optionalArgMap, reflect.TypeOf(map[string]interface{}{}), ctx)
	require.True(t, ok)
	require.Equal(t, map[string]interface{}{}, value.Interface())

	value, ok = fastOptionalArg(optionalArgMap, reflect.TypeOf(vmOptionalMap{}), ctx)
	require.True(t, ok)
	require.Equal(t, map[string]interface{}{}, value.Interface())

	_, ok = fastOptionalArg(optionalArgNone, stringType, ctx)
	require.False(t, ok)

	_, ok = fastOptionalArg(optionalArgMap, stringType, ctx)
	require.False(t, ok)

	value, ok = machine.optionalArg(optionalArgHelperContext, reflect.TypeOf(plush.HelperContext{}), nil)
	require.True(t, ok)
	require.IsType(t, plush.HelperContext{}, value.Interface())

	value, ok = machine.optionalArg(optionalArgHelperContext, reflect.TypeOf(vmOptionalHelperContext{}), nil)
	require.True(t, ok)
	require.IsType(t, vmOptionalHelperContext{}, value.Interface())

	value, ok = machine.optionalArg(optionalArgHelperContext, reflect.TypeOf((*hctx.HelperContext)(nil)).Elem(), nil)
	require.True(t, ok)
	require.Implements(t, (*hctx.HelperContext)(nil), value.Interface())

	value, ok = machine.optionalArg(optionalArgMap, reflect.TypeOf(map[string]interface{}{}), nil)
	require.True(t, ok)
	require.Equal(t, map[string]interface{}{}, value.Interface())

	value, ok = machine.optionalArg(optionalArgMap, reflect.TypeOf(vmOptionalMap{}), nil)
	require.True(t, ok)
	require.Equal(t, map[string]interface{}{}, value.Interface())

	_, ok = machine.optionalArg(optionalArgNone, stringType, nil)
	require.False(t, ok)

	_, ok = machine.optionalArg(optionalArgMap, stringType, nil)
	require.False(t, ok)
}

func Test_VM_Write_Native_Return_Value_Kinds(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())
	frame := machine.currentFrame()

	tests := []struct {
		name     string
		fn       interface{}
		value    reflect.Value
		expected string
	}{
		{"string", func() string { return "" }, reflect.ValueOf("<x>"), "&lt;x&gt;"},
		{"html", func() template.HTML { return "" }, reflect.ValueOf(template.HTML("<b>")), "<b>"},
		{"bool", func() bool { return false }, reflect.ValueOf(true), "true"},
		{"int", func() int64 { return 0 }, reflect.ValueOf(int64(7)), "7"},
		{"uint", func() uint64 { return 0 }, reflect.ValueOf(uint64(8)), "8"},
		{"float", func() float32 { return 0 }, reflect.ValueOf(float32(1.5)), "1.5"},
		{"object", func() object.Object { return nil }, reflect.ValueOf(&object.String{Value: "<obj>"}), "&lt;obj&gt;"},
		{"generic", func() vmSegmentUser { return vmSegmentUser{} }, reflect.ValueOf(vmSegmentUser{Name: "ignored"}), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame.output.Reset()
			machine.writeNativeReturnValue(frame, tt.value, cachedCallPlan(reflect.TypeOf(tt.fn)))
			require.Equal(t, tt.expected, frame.output.String())
		})
	}

	frame.output.Reset()
	machine.writeNativeReturnValue(frame, reflect.Value{}, &callPlan{returnKind: callReturnNone})
	require.Empty(t, frame.output.String())

	frame.output.Reset()
	machine.writeNativeReturnValue(frame, reflect.ValueOf((*object.String)(nil)), cachedCallPlan(reflect.TypeOf(func() object.Object { return nil })))
	require.Empty(t, frame.output.String())
}

func Test_VM_Context_With_Frame_Locals(t *testing.T) {
	base := plush.NewContextWith(map[string]interface{}{"outer": "value"})
	machine := newRuntimeHelperTestVM(base)
	frame := machine.currentFrame()
	frame.cl.Fn.LocalNames = map[int]string{0: "local", 1: "skipNil", 99: "skipRange"}
	frame.cl.Fn.LocalNames[-1] = "skipNegative"
	frame.cl.Fn.LocalNames[len(machine.stack)+1] = "skipOutOfRange"
	machine.stack[0] = &object.String{Value: "local-value"}

	scoped := machine.contextWithFrameLocals()
	require.NotSame(t, base, scoped)
	require.Equal(t, "local-value", scoped.Value("local"))
	require.Equal(t, "value", scoped.Value("outer"))
	require.False(t, scoped.Has("skipNil"))
	require.False(t, scoped.Has("skipRange"))

	machine.ctx = nil
	require.Nil(t, machine.contextWithFrameLocals())

	machine.ctx = base
	frame.cl.Fn.LocalNames = nil
	require.Same(t, base, machine.contextWithFrameLocals())
}

func Test_VM_Optional_Arg_With_Helper_Context_Interface(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())
	value, ok := machine.optionalArg(optionalArgHelperContext, reflect.TypeOf((*hctx.HelperContext)(nil)).Elem(), nil)
	require.True(t, ok)
	require.Implements(t, (*hctx.HelperContext)(nil), value.Interface())

	value, ok = machine.optionalArg(optionalArgMap, emptyMapType, nil)
	require.True(t, ok)
	require.Equal(t, map[string]interface{}{}, value.Interface())

	_, ok = machine.optionalArg(optionalArgNone, stringType, nil)
	require.False(t, ok)
}

func Test_VM_Run_Block_Return_Output_And_Error_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())

	returnBlock := executeForClosure(executeForInstructions(
		code.Make(code.OpConstant, 4),
		code.Make(code.OpReturnValue),
	), 0)
	rendered, err := machine.runBlock(returnBlock, nil)
	require.NoError(t, err)
	require.Equal(t, "&lt;b&gt;", rendered)

	outputBlock := executeForClosure(executeForInstructions(
		code.Make(code.OpWriteConstant, 4),
		code.Make(code.OpReturn),
	), 0)
	rendered, err = machine.runBlock(outputBlock, plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "&lt;b&gt;", rendered)

	errorBlock := executeForClosure(executeForInstructions(
		code.Make(code.OpGetName, 0),
		code.Make(code.OpReturnValue),
	), 0)
	_, err = machine.runBlock(errorBlock, plush.NewContext())
	require.Error(t, err)

}
