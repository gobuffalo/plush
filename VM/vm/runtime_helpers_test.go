package vm

import (
	"errors"
	"html/template"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/parser"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

type vmHelperStruct struct {
	Prop string
}

type vmHelperNestedStruct struct {
	Child *vmHelperStruct
}

type vmHelperInt32 int32

type runtimeSwitchingBytecodeCache struct {
	astKey   string
	bytecode *compiler.Bytecode
	gets     int
}

func (c *runtimeSwitchingBytecodeCache) Get(key string) (*plush.Template, bool) {
	if key != c.astKey {
		return nil, false
	}
	c.gets++
	if c.gets < 3 {
		return &plush.Template{}, true
	}
	return &plush.Template{VMBytecode: c.bytecode}, true
}

func (c *runtimeSwitchingBytecodeCache) Set(key string, t *plush.Template) {}

func (c *runtimeSwitchingBytecodeCache) Delete(key ...string) {}

func (c *runtimeSwitchingBytecodeCache) Clear() {}

func requireCompiledBytecode(t *testing.T, input string) *compiler.Bytecode {
	t.Helper()
	tmpl, err := Compile(input)
	require.NoError(t, err)
	return tmpl.bytecode
}

func newRuntimeHelperTestVM(ctx hctx.Context) *VM {
	fn := &object.CompiledFunction{
		LineNumbers:    map[int]int{3: 88},
		Properties:     map[int]object.PropertyAccess{},
		PropertyCaches: object.NewInlineCacheSlots(8),
	}
	frame := NewFrame(&object.Closure{Fn: fn}, 0)
	return &VM{
		constants: []object.Object{
			&object.String{Value: "name"},
			&object.String{Value: "global"},
			&object.String{Value: "Prop"},
			&object.String{Value: "missing"},
			&object.String{Value: "<b>"},
		},
		stack:       make([]object.Object, StackSize),
		globals:     make([]object.Object, 2),
		globalNames: map[int]string{1: "global"},
		frames:      []*Frame{frame},
		framesIndex: 1,
		ctx:         ctx,
		holes:       &[]plush.HoleMarker{},
	}
}

func Test_VM_Runtime_Line_Context_And_Write_Helpers(t *testing.T) {
	ctx := newIDLookupTestContext(map[string]interface{}{
		"name":   "old",
		"global": vmHelperStruct{Prop: "from-context"},
	})
	machine := newRuntimeHelperTestVM(ctx)
	machine.lastIP = 3

	require.EqualError(t, machine.wrapRuntimeError(errors.New("boom")), "line 88: boom")
	require.EqualError(t, machine.wrapRuntimeError(errors.New("line 7: already wrapped")), "line 7: already wrapped")
	require.NoError(t, machine.wrapRuntimeError(nil))
	require.Equal(t, 88, machine.currentLineNumber())

	machine.setName(0, &object.String{Value: "new"})
	require.Equal(t, "new", ctx.values["name"])
	require.Positive(t, machine.nameIDCacheLen)

	fallbackSetCtx := newLookupTestContext(map[string]interface{}{"name": "old"})
	machine.ctx = fallbackSetCtx
	machine.setName(0, &object.String{Value: "fallback-new"})
	require.Equal(t, "fallback-new", fallbackSetCtx.values["name"])
	value, ok := machine.contextValueByNameIndex(0)
	require.True(t, ok)
	require.Equal(t, "fallback-new", value)
	machine.ctx = ctx

	machine.nameIDOverflow = append(machine.nameIDOverflow, nameIDEntry{name: "overflow", id: 99})
	machine.clearNameIDCache()
	require.Zero(t, machine.nameIDCacheLen)
	require.Empty(t, machine.nameIDOverflow)

	require.NoError(t, machine.assignName(0, &object.String{Value: "assigned"}))
	require.Equal(t, "assigned", ctx.values["name"])
	require.ErrorContains(t, machine.assignName(3, &object.String{Value: "nope"}), `"missing": unknown identifier`)

	frame := machine.currentFrame()
	machine.writeConstant(frame, 4)
	machine.writeConstant(frame, 99)
	require.Equal(t, "&lt;b&gt;", frame.output.String())

	frame.output.Reset()
	machine.writeStringConstant(nil, 4)
	machine.writeStringConstant(frame, 4)
	require.Equal(t, "&lt;b&gt;", frame.output.String())

	frame.output.Reset()
	machine.stack[0] = &object.Native{Value: vmHelperStruct{Prop: "from-local"}}
	require.NoError(t, machine.writeLocalProperty(frame, 0, 2, 0))
	require.Equal(t, "from-local", frame.output.String())
	require.NoError(t, machine.writeLocalProperty(nil, 0, 2, 0))
	require.NoError(t, machine.writeLocalProperty(frame, StackSize+1, 2, 0))

	frame.output.Reset()
	require.NoError(t, machine.writeGlobalProperty(frame, 1, 2, 0))
	require.Equal(t, "from-context", frame.output.String())
	machine.globalNames[0] = "not-present"
	require.NoError(t, machine.writeGlobalProperty(frame, 0, 2, 0))
	require.NoError(t, machine.writeGlobalProperty(frame, 99, 2, 0))
}

func Test_VM_Runtime_State_And_Cache_Edge_Branches(t *testing.T) {
	machine := &VM{frames: []*Frame{nil}, framesIndex: 1}
	require.Empty(t, machine.Rendered())
	require.Nil(t, machine.PunchHoles())
	require.Nil(t, machine.currentFrameOutputValues())
	machine.writeFrameOutput(nil, &object.String{Value: "ignored"})
	require.Zero(t, machine.currentLineNumber())
	require.Equal(t, anonymousCallName, machine.currentCallName(0))
	require.Equal(t, object.PropertyAccess{}, machine.currentPropertyAccess(0))
	require.Nil(t, machine.currentPropertyCacheSlot(0))
	require.Nil(t, machine.currentCallCacheSlot(0))
	require.EqualError(t, machine.wrapRuntimeError(errors.New("boom")), "line 1: boom")

	holes := []plush.HoleMarker{plush.NewHoleMarker("hole", `"name"`, 1, 2)}
	machine.holes = &holes
	gotHoles := machine.PunchHoles()
	require.Len(t, gotHoles, 1)
	gotHoles[0] = plush.NewHoleMarker("changed", `"changed"`, 3, 4)
	require.Equal(t, "hole", holes[0].MarkerName())

	frame := NewFrame(&object.Closure{Fn: &object.CompiledFunction{
		CallNames:      map[int]string{2: "named"},
		LineNumbers:    map[int]int{3: 99},
		Properties:     map[int]object.PropertyAccess{4: {Receiver: "robot", Full: "robot.Name"}},
		PropertyCaches: object.NewInlineCacheSlots(1),
		CallCaches:     object.NewInlineCacheSlots(1),
	}}, 0)
	frame.output.WriteString("frame-output")
	machine = &VM{frames: []*Frame{frame}, framesIndex: 1}
	require.Equal(t, "frame-output", machine.Rendered())
	require.Nil(t, machine.currentFrameOutputValues())
	machine.writeFrameOutput(frame, &object.String{Value: "<extra>"})
	outputValues := machine.currentFrameOutputValues()
	require.Len(t, outputValues, 1)
	require.Equal(t, template.HTML("frame-output&lt;extra&gt;"), object.ToGo(outputValues[0]))
	require.Equal(t, anonymousCallName, machine.currentCallName(1))
	require.Equal(t, "named", machine.currentCallName(2))
	require.Zero(t, machine.currentLineNumber())
	machine.lastIP = 3
	require.Equal(t, 99, machine.currentLineNumber())
	require.Equal(t, object.PropertyAccess{Receiver: "robot", Full: "robot.Name"}, machine.currentPropertyAccess(4))
	require.NotNil(t, machine.currentPropertyCacheSlot(0))
	require.Nil(t, machine.currentPropertyCacheSlot(-1))
	require.Nil(t, machine.currentPropertyCacheSlot(1))
	require.NotNil(t, machine.currentCallCacheSlot(0))
	require.Nil(t, machine.currentCallCacheSlot(-1))
	require.Nil(t, machine.currentCallCacheSlot(1))

	machine.halted = true
	machine.lastPopped = &object.String{Value: "<done>"}
	require.Equal(t, "&lt;done&gt;", machine.Rendered())
	machine.lastPopped = object.NullObject
	require.Equal(t, "frame-output&lt;extra&gt;", machine.Rendered())
}

func Test_VM_Runtime_Stack_And_Budget_Edge_Branches(t *testing.T) {
	machine := &VM{stack: make([]object.Object, StackSize)}
	require.NoError(t, machine.push(nil))
	require.Same(t, Null, machine.stack[0])
	require.Equal(t, 1, machine.sp)
	require.Equal(t, 1, machine.stackMax)

	machine.sp = StackSize
	require.EqualError(t, machine.push(&object.Integer{Value: 1}), "stack overflow")

	machine = &VM{}
	require.Nil(t, machine.budget())

	releaseStack(make([]object.Object, 1), 1)
	releaseStack(make([]object.Object, StackSize), StackSize+10)
	releaseFrames(nil)
	releaseHoles(nil)

	var nilMachine *VM
	require.NotPanics(t, nilMachine.Release)

	machine = NewWithContext(&compiler.Bytecode{}, nil)
	require.NotNil(t, machine.ctx)
	machine.Release()

	require.Same(t, Null, machine.combineFrameOutput(&Frame{}, nil))

	ctx := plush.NewContext().WithBudget(plush.NewBudget(100))
	machine = &VM{ctx: ctx}
	require.NoError(t, machine.spendFunctionCall(""))
	stats := ctx.Budget().Stats()
	require.Contains(t, stats.ByFunction, anonymousCallName)
}

func Test_VM_Loop_Context_Property_Assignment_And_Cache_Edges(t *testing.T) {
	ctx := newIDLookupTestContext(map[string]interface{}{
		"name": vmHelperStruct{Prop: "<prop>"},
	})
	machine := newRuntimeHelperTestVM(ctx)
	machine.constants = append(machine.constants, &object.String{Value: "nil"})
	nilIndex := len(machine.constants) - 1
	frame := machine.currentFrame()

	require.NoError(t, machine.writeNameProperty(frame, nilIndex, 2, 0))
	require.Empty(t, frame.output.String())

	require.ErrorContains(t, machine.writeNameProperty(frame, 3, 2, 0), `"missing": unknown identifier`)
	require.NoError(t, machine.writePropertyValue(nil, vmHelperStruct{Prop: "ignored"}, 2, 0))

	budgetMachine := newRuntimeHelperTestVM(plush.NewContext().WithBudget(plush.NewBudget(0)))
	require.ErrorContains(t, budgetMachine.writePropertyValue(budgetMachine.currentFrame(), vmHelperStruct{Prop: "blocked"}, 2, 0), "budget")

	require.ErrorContains(t, machine.writePropertyValue(frame, vmHelperStruct{Prop: "value"}, 3, 0), "missing")

	fallbackCtx := newLookupTestContext(map[string]interface{}{"name": "old"})
	machine.ctx = fallbackCtx
	require.NoError(t, machine.assignName(0, &object.String{Value: "new"}))
	require.Equal(t, "new", fallbackCtx.values["name"])

	assignmentCosts := plush.ZeroCosts()
	assignmentCosts.Assignment = 1
	budgetMachine.ctx = plush.NewContext().WithBudget(plush.NewBudgetWithCosts(0, assignmentCosts))
	budgetMachine.constants = machine.constants
	require.ErrorContains(t, budgetMachine.assignName(0, &object.String{Value: "blocked"}), "budget")

	head := &propertyInlineCacheEntry{typ: reflect.TypeOf(vmHelperStruct{})}
	head.next = &propertyInlineCacheEntry{typ: reflect.TypeOf(vmHelperInt32(0))}
	require.Nil(t, clonePropertyInlineCache(head, 0))
	clone := clonePropertyInlineCache(head, 2)
	require.NotNil(t, clone)
	require.NotSame(t, head, clone)
	require.Equal(t, head.typ, clone.typ)
	require.NotNil(t, clone.next)
	require.Equal(t, head.next.typ, clone.next.typ)
	require.Nil(t, clone.next.next)

	slot := object.NewInlineCacheSlots(1)
	lookup := inlinePropertyLookup(&slot[0], reflect.TypeOf(vmHelperStruct{}), "Prop")
	require.Equal(t, propertyLookupField, lookup.kind)
	lookup = inlinePropertyLookup(&slot[0], reflect.TypeOf(vmHelperStruct{}), "Prop")
	require.Equal(t, propertyLookupField, lookup.kind)
	lookup = inlinePropertyLookup(&slot[0], reflect.TypeOf(vmHelperNestedStruct{}), "Child")
	require.Equal(t, propertyLookupField, lookup.kind)
	lookup = inlinePropertyLookup(nil, reflect.TypeOf(vmHelperStruct{}), "Prop")
	require.Equal(t, propertyLookupField, lookup.kind)
	require.Nil(t, buildFastPropertyWriter(reflect.TypeOf(vmHelperStruct{}), propertyLookup{kind: propertyLookupField, fieldIndex: []int{99}}))

	field, ok := fieldByIndex(reflect.TypeOf(vmHelperStruct{}), []int{0})
	require.True(t, ok)
	require.Equal(t, "Prop", field.Name)
	field, ok = fieldByIndex(reflect.TypeOf(vmHelperNestedStruct{}), []int{0, 0})
	require.True(t, ok)
	require.Equal(t, "Prop", field.Name)
	_, ok = fieldByIndex(reflect.TypeOf(1), []int{0})
	require.False(t, ok)
	_, ok = fieldByIndex(reflect.TypeOf(vmHelperStruct{}), nil)
	require.False(t, ok)
	_, ok = fieldByIndex(reflect.TypeOf(vmHelperStruct{}), []int{2})
	require.False(t, ok)

	machine = newRuntimeHelperTestVM(plush.NewContext())
	require.NoError(t, machine.pushField(reflect.ValueOf(vmHelperStruct{Prop: "field"}).FieldByName("Prop"), object.PropertyAccess{}, "Prop"))
	require.Equal(t, &object.String{Value: "field"}, machine.pop())
	require.NoError(t, machine.pushField(reflect.ValueOf((*vmHelperStruct)(nil)), object.PropertyAccess{}, "Prop"))
	require.Same(t, Null, machine.pop())
	hidden := reflect.ValueOf(vmFastPropertyUser{hidden: "secret"}).FieldByName("hidden")
	require.ErrorContains(t, machine.pushField(hidden, object.PropertyAccess{Receiver: "user", Full: "user.hidden"}, "hidden"), "user.hidden")
}

func Test_VM_Name_Global_And_Truthiness_Helpers(t *testing.T) {
	ctx := newIDLookupTestContext(map[string]interface{}{
		"name":   "<Mido>",
		"global": "<Global>",
	})
	machine := newRuntimeHelperTestVM(ctx)
	machine.constants = append(machine.constants, &object.String{Value: "nil"})
	frame := machine.currentFrame()

	require.NoError(t, machine.writeName(frame, 0, false))
	require.Equal(t, "&lt;Mido&gt;", frame.output.String())

	require.NoError(t, machine.writeName(nil, 0, false))

	frame.output.Reset()
	require.NoError(t, machine.writeName(frame, 3, true))
	require.Empty(t, frame.output.String())
	require.ErrorContains(t, machine.writeName(frame, 3, false), `"missing": unknown identifier`)
	require.NoError(t, machine.writeName(frame, 5, false))

	ctx.values["name"] = struct{}{}
	frame.output.Reset()
	require.NoError(t, machine.writeName(frame, 0, false))
	require.Empty(t, frame.output.String())
	ctx.values["name"] = "<Mido>"

	frame.output.Reset()
	machine.writeHTMLConstant(frame, 4)
	require.Equal(t, "<b>", frame.output.String())
	machine.writeHTMLConstant(nil, 4)

	frame.output.Reset()
	machine.globals[0] = &object.String{Value: "<local-global>"}
	machine.writeGlobal(frame, 0)
	require.Equal(t, "&lt;local-global&gt;", frame.output.String())

	frame.output.Reset()
	machine.writeGlobal(frame, 1)
	require.Equal(t, "&lt;Global&gt;", frame.output.String())
	machine.writeGlobal(nil, 1)
	machine.writeGlobal(frame, 99)

	machine.updateNamedGlobal(1, &object.String{Value: "updated"})
	require.Equal(t, "updated", ctx.values["global"])
	require.Equal(t, &object.String{Value: "updated"}, machine.globalFromContext(1))
	missingGlobalCtx := newIDLookupTestContext(map[string]interface{}{})
	machine.ctx = missingGlobalCtx
	require.Nil(t, machine.globalFromContext(1))
	machine.ctx = ctx
	require.Nil(t, machine.globalFromContext(99))

	require.Same(t, True, nativeBoolToBooleanObject(true))
	require.Same(t, False, nativeBoolToBooleanObject(false))
	require.False(t, isTruthy(nil))
	require.False(t, isTruthy(False))
	require.False(t, isTruthy(Null))
	require.False(t, isTruthy(&object.String{}))
	require.False(t, isTruthy(&object.Native{Value: nil}))
	require.False(t, isTruthy(&object.Native{Value: (*vmHelperStruct)(nil)}))
	require.True(t, isTruthy(&object.String{Value: "x"}))
	require.True(t, isTruthy(&object.Integer{Value: 0}))

	require.False(t, isTruthyFastValue(nil))
	require.False(t, isTruthyFastValue(""))
	require.False(t, isTruthyFastValue(false))
	require.False(t, isTruthyFastValue((*vmHelperStruct)(nil)))
	require.True(t, isTruthyFastValue("x"))
	require.True(t, isTruthyFastValue(true))
}

func Test_VM_Loop_Raw_Object_Helpers(t *testing.T) {
	key, value := loopObjects(3, "value", false)
	require.IsType(t, &object.Integer{}, key)
	require.IsType(t, &object.String{}, value)

	nativeKey, nativeValue := loopObjects(3, "value", true)
	require.IsType(t, &object.Native{}, nativeKey)
	require.IsType(t, &object.Native{}, nativeValue)
	require.Equal(t, 3, object.ToGo(nativeKey))
	require.Equal(t, "value", object.ToGo(nativeValue))

	obj := &object.String{Value: "kept"}
	require.Same(t, obj, rawLoopObject(obj))
	require.Same(t, Null, rawLoopObject(nil))
}

func Test_VM_Set_Index_Helpers(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())

	array := &object.Array{Elements: []object.Object{&object.String{Value: "a"}}}
	require.NoError(t, machine.executeSetIndex(array, &object.Integer{Value: 0}, &object.String{Value: "b"}))
	require.Equal(t, "b", array.Elements[0].(*object.String).Value)
	require.ErrorContains(t, machine.executeSetIndex(array, &object.String{Value: "bad"}, &object.String{Value: "x"}), "non int")
	require.ErrorContains(t, machine.executeSetIndex(array, &object.Integer{Value: 2}, &object.String{Value: "x"}), "out of bounds")

	hashKey := &object.String{Value: "key"}
	hash := &object.Hash{Pairs: map[object.HashKey]object.HashPair{}}
	require.NoError(t, machine.executeSetIndex(hash, hashKey, &object.Integer{Value: 7}))
	require.Equal(t, int64(7), hash.Pairs[hashKey.HashKey()].Value.(*object.Integer).Value)
	require.ErrorContains(t, machine.executeSetIndex(hash, &object.Array{}, &object.Integer{Value: 1}), "unusable as hash key")

	rawMap := map[string]interface{}{}
	require.NoError(t, machine.executeSetIndex(&object.Native{Value: rawMap}, hashKey, &object.String{Value: "mapped"}))
	require.Equal(t, "mapped", rawMap["key"])

	rawMapPtr := map[string]interface{}{}
	require.NoError(t, machine.executeSetIndex(&object.Native{Value: &rawMapPtr}, hashKey, &object.String{Value: "mapped-ptr"}))
	require.Equal(t, "mapped-ptr", rawMapPtr["key"])

	rawSlice := []string{"zero"}
	require.NoError(t, machine.executeSetIndex(&object.Native{Value: rawSlice}, &object.Integer{Value: 0}, &object.String{Value: "one"}))
	require.Equal(t, "one", rawSlice[0])
	require.ErrorContains(t, machine.executeSetIndex(&object.Native{Value: rawSlice}, &object.String{Value: "bad"}, &object.String{Value: "x"}), "non int")
	require.ErrorContains(t, machine.executeSetIndex(&object.Native{Value: rawSlice}, &object.Integer{Value: 3}, &object.String{Value: "x"}), "out of bounds")
	require.ErrorContains(t, machine.executeSetIndex(&object.Native{Value: rawSlice}, &object.Integer{Value: 0}, &object.Integer{Value: 1}), "cannot use")

	var nilRawSlice *[]string
	require.ErrorContains(t, machine.executeSetIndex(&object.Native{Value: nilRawSlice}, &object.Integer{Value: 0}, &object.String{Value: "x"}), "could not index")

	require.ErrorContains(t, machine.executeSetIndex(&object.Native{Value: vmHelperStruct{}}, hashKey, &object.String{Value: "x"}), "could not index")
}

func Test_VM_Native_Index_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())

	require.NoError(t, machine.executeNativeIndex(object.NullObject, &object.String{Value: "name"}))
	require.Same(t, Null, machine.pop())

	require.NoError(t, machine.executeNativeIndex(&object.Native{Value: (*vmHelperStruct)(nil)}, &object.String{Value: "Prop"}))
	require.Same(t, Null, machine.pop())

	require.NoError(t, machine.executeNativeIndex(&object.Native{Value: map[string]int{"count": 3}}, &object.String{Value: "count"}))
	require.Equal(t, &object.Integer{Value: 3}, machine.pop())

	require.NoError(t, machine.executeNativeIndex(&object.Native{Value: map[interface{}]string{7: "seven"}}, &object.Integer{Value: 7}))
	require.Equal(t, &object.String{Value: "seven"}, machine.pop())

	rawMapPtr := map[string]int{"count": 4}
	require.NoError(t, machine.executeNativeIndex(&object.Native{Value: &rawMapPtr}, &object.String{Value: "count"}))
	require.Equal(t, &object.Integer{Value: 4}, machine.pop())

	require.NoError(t, machine.executeNativeIndex(&object.Native{Value: map[string]int{}}, &object.String{Value: "missing"}))
	require.Same(t, Null, machine.pop())

	err := machine.executeNativeIndex(&object.Native{Value: map[string]int{}}, &object.Integer{Value: 1})
	require.ErrorContains(t, err, "cannot use")

	require.NoError(t, machine.executeNativeIndex(&object.Native{Value: []string{"zero", "one"}}, &object.Integer{Value: 1}))
	require.Equal(t, &object.String{Value: "one"}, machine.pop())

	require.NoError(t, machine.executeNativeIndex(&object.Native{Value: [2]int{4, 5}}, &object.Integer{Value: 0}))
	require.Equal(t, &object.Integer{Value: 4}, machine.pop())

	err = machine.executeNativeIndex(&object.Native{Value: []string{"zero"}}, &object.String{Value: "bad"})
	require.ErrorContains(t, err, "non int")

	err = machine.executeNativeIndex(&object.Native{Value: []string{"zero"}}, &object.Integer{Value: 2})
	require.ErrorContains(t, err, "out of bounds")

	err = machine.executeNativeIndex(&object.Native{Value: vmHelperStruct{}}, &object.String{Value: "Prop"})
	require.ErrorContains(t, err, "could not index")
}

func Test_VM_Hash_Index_Branches(t *testing.T) {
	machine := newRuntimeHelperTestVM(plush.NewContext())
	key := &object.String{Value: "key"}
	hash := &object.Hash{Pairs: map[object.HashKey]object.HashPair{
		key.HashKey(): {Key: key, Value: &object.Integer{Value: 7}},
	}}

	require.NoError(t, machine.executeHashIndex(hash, key))
	require.Equal(t, &object.Integer{Value: 7}, machine.pop())

	require.NoError(t, machine.executeHashIndex(hash, &object.String{Value: "missing"}))
	require.Same(t, Null, machine.pop())

	require.ErrorContains(t, machine.executeHashIndex(hash, &object.Array{}), "unusable as hash key")

	array := &object.Array{Elements: []object.Object{&object.String{Value: "zero"}}}
	require.NoError(t, machine.executeIndexExpression(array, &object.Integer{Value: 0}))
	require.Equal(t, &object.String{Value: "zero"}, machine.pop())
	require.ErrorContains(t, machine.executeIndexExpression(array, &object.Integer{Value: 9}), "array index out of bounds")
}

func Test_VM_Write_Builtin_And_Native_Calls(t *testing.T) {
	builtin := &object.Builtin{Fn: func(args ...object.Object) object.Object {
		return &object.String{Value: strings.Join([]string{args[0].Inspect(), args[1].Inspect()}, ":")}
	}}
	machine := newRuntimeHelperTestVM(plush.NewContext())
	machine.stack[0] = builtin
	machine.stack[1] = &object.String{Value: "a"}
	machine.stack[2] = &object.String{Value: "b"}
	machine.sp = 3
	require.NoError(t, machine.writeBuiltinCall(builtin, 2, true))
	require.Equal(t, 0, machine.sp)
	require.Equal(t, "a:b", machine.currentFrame().output.String())

	nilBuiltin := &object.Builtin{Fn: func(args ...object.Object) object.Object { return nil }}
	machine.currentFrame().output.Reset()
	machine.stack[0] = &object.String{Value: "ignored"}
	machine.sp = 1
	require.NoError(t, machine.writeBuiltinCall(nilBuiltin, 1, false))
	require.Equal(t, 0, machine.sp)
	require.Empty(t, machine.currentFrame().output.String())

	machine.stack[0] = nilBuiltin
	machine.sp = 1
	require.NoError(t, machine.callBuiltin(nilBuiltin, 0))
	require.Same(t, Null, machine.pop())

	fn := func(value string) string { return value + "!" }
	callee := &object.Native{Value: &fn}
	machine.currentFrame().output.Reset()
	machine.stack[0] = callee
	machine.stack[1] = &object.String{Value: "go"}
	machine.sp = 2
	require.NoError(t, machine.writeNativeCall("bang", callee, 1, nil, true))
	require.Equal(t, 0, machine.sp)
	require.Equal(t, "go!", machine.currentFrame().output.String())
}

func Test_VM_Native_Call_Execution_Edge_Branches(t *testing.T) {
	budgetMachine := newRuntimeHelperTestVM(plush.NewContext().WithBudget(plush.NewBudget(0)))
	require.ErrorContains(t, budgetMachine.executeCall("blocked", 0, nil, nil), "budget")
	require.ErrorContains(t, budgetMachine.executeWriteCall("blocked", 0, nil), "budget")

	machine := newRuntimeHelperTestVM(plush.NewContext())
	var nilFunc *func()
	require.ErrorContains(t, machine.callNativeValue("nilFunc", nilFunc, 0, nil, nil), "invalid function")
	require.ErrorContains(t, machine.callNativeValue("bad", 7, 0, nil, nil), "invalid function")

	machine.stack[0] = &object.Native{Value: func() {}}
	machine.sp = 1
	require.NoError(t, machine.callNativeValue("noop", func() {}, 0, nil, nil))
	require.Same(t, Null, machine.pop())

	machine.stack[0] = &object.Native{Value: func() (string, error) { return "", errors.New("boom") }}
	machine.sp = 1
	require.ErrorContains(t, machine.callNativeValue("fail", func() (string, error) { return "", errors.New("boom") }, 0, nil, nil), "could not call fail function")

	machine.stack[0] = &object.Native{Value: func(string) string { return "" }}
	machine.sp = 1
	require.ErrorContains(t, machine.callNativeValue("needsArg", func(string) string { return "" }, 0, nil, nil), "too few arguments")

	machine.stack[0] = &object.Native{Value: func() {}}
	machine.sp = 1
	require.NoError(t, machine.writeNativeValueCall("noop", func() {}, 0, nil, true))
	require.Equal(t, 0, machine.sp)
	require.Empty(t, machine.currentFrame().output.String())

	var nilWriteFunc *func()
	machine.stack[0] = &object.Native{Value: nilWriteFunc}
	machine.sp = 1
	require.ErrorContains(t, machine.writeNativeValueCall("nilWrite", nilWriteFunc, 0, nil, true), "invalid function")

	machine.stack[0] = &object.Native{Value: 7}
	machine.sp = 1
	require.ErrorContains(t, machine.writeNativeValueCall("badWrite", 7, 0, nil, true), "invalid function")

	machine.stack[0] = &object.Native{Value: func(string) string { return "" }}
	machine.sp = 1
	require.ErrorContains(t, machine.writeNativeValueCall("needsArg", func(string) string { return "" }, 0, nil, true), "too few arguments")

	machine.sp = 0
	handled, err := machine.tryFastWriteNativeValueCall("nilFast", nil, 0, nil, false)
	require.ErrorContains(t, err, "invalid function")
	require.False(t, handled)

	machine.sp = 0
	handled, err = machine.tryFastWriteNativeValueCall("needsArg", func(string) string { return "" }, 0, nil, false)
	require.NoError(t, err)
	require.False(t, handled)
	require.Zero(t, machine.sp)

	machine.stack[0] = &object.Native{Value: func() (string, error) { return "", errors.New("boom") }}
	machine.sp = 1
	require.ErrorContains(t, machine.writeNativeValueCall("fail", func() (string, error) { return "", errors.New("boom") }, 0, nil, true), "could not call fail function")

	machine = &VM{stack: make([]object.Object, StackSize), frames: []*Frame{nil}, framesIndex: 1}
	machine.stack[0] = &object.Native{Value: func() string { return "ignored" }}
	machine.sp = 1
	require.NoError(t, machine.writeNativeValueCall("noframe", func() string { return "ignored" }, 0, nil, true))
	require.Equal(t, 0, machine.sp)
}

func Test_VM_Fast_Object_And_Optional_Arg_Helpers(t *testing.T) {
	var out strings.Builder
	require.True(t, writeFastObject(&out, plush.NewContext(), &object.Array{Elements: []object.Object{
		&object.String{Value: "<tag>"},
		&object.Integer{Value: 12},
		&object.Float{Value: 1.5},
		True,
		&object.Native{Value: template.HTML("<safe>")},
	}}))
	require.Equal(t, "&lt;tag&gt;121.5true<safe>", out.String())
	require.True(t, writeFastObject(&out, nil, nil))
	require.False(t, writeFastObject(&out, nil, &object.Builtin{}))
	require.True(t, canWriteFastObject(&object.Array{Elements: []object.Object{&object.Integer{Value: 1}}}))
	require.False(t, canWriteFastObject(&object.Array{Elements: []object.Object{&object.Builtin{}}}))

	value, ok := fastOptionalArg(optionalArgHelperContext, plushHelperContextType, plush.NewContext())
	require.True(t, ok)
	require.IsType(t, plush.HelperContext{}, value.Interface())

	value, ok = fastOptionalArg(optionalArgHelperContext, reflect.TypeOf((*hctx.HelperContext)(nil)).Elem(), plush.NewContext())
	require.True(t, ok)
	require.Implements(t, (*hctx.HelperContext)(nil), value.Interface())

	value, ok = fastOptionalArg(optionalArgMap, emptyMapType, nil)
	require.True(t, ok)
	require.IsType(t, map[string]interface{}{}, value.Interface())

	_, ok = fastOptionalArg(optionalArgNone, stringType, nil)
	require.False(t, ok)
}

func Test_VM_Fast_Raw_Numeric_Arg_Helpers(t *testing.T) {
	i, ok := fastWriteRawInt64Arg(int32(-7))
	require.True(t, ok)
	require.Equal(t, int64(-7), i)

	u, ok := fastWriteRawUintArg(uint32(12))
	require.True(t, ok)
	require.Equal(t, uint(12), u)

	u64, ok := fastWriteRawUint64Arg(uint64(42))
	require.True(t, ok)
	require.Equal(t, uint64(42), u64)

	require.True(t, fastUint64FitsUint(42))
}

func Test_VM_Fast_Arg_Numeric_Coercions(t *testing.T) {
	i, ok := fastArgInt64(&object.Integer{Value: 5})
	require.True(t, ok)
	require.Equal(t, int64(5), i)

	i, ok = fastArgInt64(vmHelperInt32(-3))
	require.True(t, ok)
	require.Equal(t, int64(-3), i)

	_, ok = fastArgInt64(uint64(math.MaxInt64) + 1)
	require.False(t, ok)
	_, ok = fastArgInt64("nope")
	require.False(t, ok)

	u, ok := fastArgUint64(&object.Integer{Value: 6})
	require.True(t, ok)
	require.Equal(t, uint64(6), u)

	u, ok = fastArgUint64(float64(7.5))
	require.True(t, ok)
	require.Equal(t, uint64(7), u)

	_, ok = fastArgUint64(int(-1))
	require.False(t, ok)
	_, ok = fastArgUint64("nope")
	require.False(t, ok)

	f, ok := fastArgFloat64(&object.Float{Value: 2.25})
	require.True(t, ok)
	require.Equal(t, 2.25, f)

	f, ok = fastArgFloat64(uint16(8))
	require.True(t, ok)
	require.Equal(t, float64(8), f)

	_, ok = fastArgFloat64("nope")
	require.False(t, ok)
}

func Test_VM_Rendered_Go_Value_Size_Branches(t *testing.T) {
	ctx := plush.NewContext()
	ctx.Set("TIME_FORMAT", "2006")
	machine := newRuntimeHelperTestVM(ctx)
	now := time.Date(2026, time.July, 7, 0, 0, 0, 0, time.UTC)

	require.Zero(t, machine.estimatedRenderedGoValueSize(nil))
	require.Equal(t, len(plush.PunchHoleMarkerName(0)), machine.estimatedRenderedGoValueSize(vmHole{input: "hole"}))
	require.Equal(t, 4, machine.estimatedRenderedGoValueSize(now))
	require.Equal(t, 4, machine.estimatedRenderedGoValueSize(&now))
	require.Equal(t, len("<b>"), machine.estimatedRenderedGoValueSize(template.HTML("<b>")))
	require.Equal(t, 3, machine.estimatedRenderedGoValueSize("abc"))
	require.Equal(t, 5, machine.estimatedRenderedGoValueSize(true))
	require.Equal(t, 20, machine.estimatedRenderedGoValueSize(uint32(1)))
	require.Equal(t, 24, machine.estimatedRenderedGoValueSize(float32(1)))
	require.Equal(t, 3, machine.estimatedRenderedGoValueSize([]string{"a", "bc"}))
	require.Equal(t, 6, machine.estimatedRenderedGoValueSize([]interface{}{"a", true}))
	require.Equal(t, 3, machine.estimatedRenderedGoValueSize([2]string{"a", "bc"}))
	require.Zero(t, machine.estimatedRenderedGoValueSize(struct{}{}))
}

func Test_VM_Numeric_Unsigned_Helpers(t *testing.T) {
	value := numericValue{kind: numericUnsigned, u: 9}
	u, ok := value.uint64()
	require.True(t, ok)
	require.Equal(t, uint64(9), u)

	value = numericValue{kind: numericSigned, i: 8}
	u, ok = value.uint64()
	require.True(t, ok)
	require.Equal(t, uint64(8), u)

	_, ok = numericValue{kind: numericSigned, i: -1}.uint64()
	require.False(t, ok)

	require.IsType(t, &object.Integer{}, unsignedNumericObject(uint64(math.MaxInt64)))
	require.IsType(t, &object.Native{}, unsignedNumericObject(uint64(math.MaxInt64)+1))
}

func Test_VM_Fast_Loop_Conditional_And_Call_Helpers(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"items":  []string{"a", "b"},
		"prefix": "pre:",
		"label": func(value, prefix string) string {
			return prefix + value
		},
	})
	plan := &compiler.FastRenderPlan{Bindings: []string{"items", "label", "prefix"}}
	bindings := newFastRenderBindings(plan, ctx)
	loop := &compiler.FastLoopPlan{IterableName: "items", IterableNameIndex: 0, Line: 1}

	call := &compiler.FastCallPlan{
		Name:      "label",
		NameIndex: 1,
		Args: []compiler.FastValuePlan{
			{Kind: compiler.FastValuePath, NameIndex: -1},
			{Kind: compiler.FastValueName, NameIndex: 2, Value: "prefix"},
		},
		Line: 1,
	}
	var args fastCallArgs
	evaluated, err := evalFastLoopCallArgsInto(call.Args, ctx, bindings, 1, "item", &args)
	require.NoError(t, err)
	require.Equal(t, 2, evaluated.Len())
	require.Equal(t, "item", evaluated.Raw(0))
	require.Equal(t, "pre:", evaluated.Raw(1))

	var out strings.Builder
	require.NoError(t, writeFastLoopCallPart(&out, ctx, bindings, call, 1, "item"))
	require.Equal(t, "pre:item", out.String())
	require.NoError(t, writeFastLoopCallPart(&out, ctx, bindings, nil, 1, "item"))

	out.Reset()
	conditional := &compiler.FastLoopConditionalPlan{
		Branches: []compiler.FastLoopConditionalBranch{
			{
				Condition: compiler.FastValuePlan{
					Kind:     compiler.FastValueInfix,
					Operator: "==",
					Left:     &compiler.FastValuePlan{Kind: compiler.FastValueLoopKey},
					Right:    &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 1},
				},
				Parts: []compiler.FastLoopPart{
					{Kind: compiler.FastLoopPartStatic, Value: "one:"},
					{Kind: compiler.FastLoopPartValue},
				},
				Line: 1,
			},
		},
		ElseParts: []compiler.FastLoopPart{
			{Kind: compiler.FastLoopPartStatic, Value: "other:"},
			{Kind: compiler.FastLoopPartKey},
		},
	}
	require.NoError(t, renderFastLoopConditional(&out, ctx, bindings, loop, conditional, 1, "hit"))
	require.Equal(t, "one:hit", out.String())

	out.Reset()
	require.NoError(t, renderFastLoopConditional(&out, ctx, bindings, loop, conditional, 2, "miss"))
	require.Equal(t, "other:2", out.String())
	require.NoError(t, renderFastLoopConditional(&out, ctx, bindings, loop, nil, 2, "miss"))
}

func Test_VM_Fast_Struct_Loop_Value_Helpers(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"name": "bound"})
	bindings := newFastRenderBindings(&compiler.FastRenderPlan{Bindings: []string{"name"}}, ctx)
	item := reflect.ValueOf(vmHelperStruct{Prop: "field"})
	path := &compiler.FastValuePlan{
		Kind:      compiler.FastValuePath,
		NameIndex: -1,
		Path: []compiler.FastPathStep{
			{Kind: compiler.FastPathStepProperty, Value: "Prop", Receiver: "item", Full: "item.Prop"},
		},
	}

	value, ok, err := evalFastStructLoopValue(path, ctx, bindings, 7, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "field", value)

	truthy, ok, err := isTruthyFastStructLoopValue(path, ctx, bindings, 7, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, truthy)

	key, ok, err := evalFastStructLoopValue(&compiler.FastValuePlan{Kind: compiler.FastValueLoopKey}, ctx, bindings, 7, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 7, key)

	bound, ok, err := evalFastStructLoopValue(&compiler.FastValuePlan{Kind: compiler.FastValueName, NameIndex: 0, Value: "name"}, ctx, bindings, 7, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "bound", bound)

	infix := &compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     path,
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "field"},
	}
	result, ok, err := evalFastStructLoopInfixValue(infix, ctx, bindings, 7, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, result)

	logical := &compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "&&",
		Left:     infix,
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueBool, BoolValue: true},
	}
	result, ok, err = evalFastStructLoopLogicalInfixValue(logical, ctx, bindings, 7, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, result)

	whole, ok, err := evalFastStructLoopPathValue(&compiler.FastValuePlan{Kind: compiler.FastValuePath, NameIndex: -1}, ctx, bindings, item)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, vmHelperStruct{Prop: "field"}, whole)

	require.False(t, isTruthyFastReflectValue(reflect.ValueOf("")))
	require.True(t, isTruthyFastReflectValue(reflect.ValueOf("x")))
	require.False(t, isTruthyFastReflectValue(reflect.ValueOf((*vmHelperStruct)(nil))))
	require.Contains(t, fieldAccessError("item", "item.hidden", "hidden").Error(), "item.hidden")
	require.Contains(t, fieldAccessError("", "", "hidden").Error(), "hidden")
}

func Test_VM_Partial_Setup_And_Helper_Rendering(t *testing.T) {
	ctx := plush.NewContext()
	ctx.Set(meta.TemplateBaseFileNameKey, "show")
	ctx.Set(meta.TemplateExtensionKey, "plush")
	ctx.Set(meta.TemplateFileKey, "templates/users/show.plush")

	require.NoError(t, setupPartialTemplateFile(ctx, "row.plush"))
	require.Equal(t, "templates/users/row.plush", ctx.Value(meta.TemplateFileKey))

	setupPartialNesting(ctx, "row.plush")
	require.Equal(t, "row.plush", ctx.Value(vmAlreadyInPartial))
	setupPartialNesting(ctx, "nested.html")
	require.Equal(t, "nested", ctx.Value(meta.TemplateBaseFileNameKey))
	require.Equal(t, "html", ctx.Value(meta.TemplateExtensionKey))

	ctx.Set(vmPartialFeederName, func(name string) (string, error) {
		switch name {
		case "row.plush":
			return `<span><%= name %></span>`, nil
		case "layout.plush":
			return `<main><%= yield %></main>`, nil
		default:
			return "", errors.New("missing partial")
		}
	})

	rendered, err := vmPartialHelper("row.plush", map[string]interface{}{"name": "Mido"}, plush.NewHelperContext(ctx, nil))
	require.NoError(t, err)
	require.Equal(t, template.HTML(`<span>Mido</span>`), rendered)

	rendered, err = vmPartialHelper("row.plush", map[string]interface{}{"name": "Mido", "layout": "layout.plush"}, plush.NewHelperContext(ctx, nil))
	require.NoError(t, err)
	require.Equal(t, template.HTML(`<main><span>Mido</span></main>`), rendered)

	freshCtx := plush.NewContext()
	freshCtx.Set(vmPartialFeederName, func(string) (string, error) {
		return `<span>fresh</span>`, nil
	})
	rendered, err = vmPartialHelper("fresh.plush", nil, plush.NewHelperContext(freshCtx, nil))
	require.NoError(t, err)
	require.Equal(t, template.HTML(`<span>fresh</span>`), rendered)
	require.Nil(t, freshCtx.Value(vmAlreadyInPartial))

	_, err = vmPartialHelper("row.plush", nil, plush.NewHelperContext(plush.NewContext(), nil))
	require.ErrorContains(t, err, "could not find partial feeder")

	badFileKeyCtx := plush.NewContext()
	badFileKeyCtx.Set(meta.TemplateBaseFileNameKey, "show")
	badFileKeyCtx.Set(meta.TemplateExtensionKey, "plush")
	badFileKeyCtx.Set(meta.TemplateFileKey, 12)
	badFileKeyCtx.Set(vmPartialFeederName, func(string) (string, error) {
		return `<span>bad</span>`, nil
	})
	_, err = vmPartialHelper("row.plush", nil, plush.NewHelperContext(badFileKeyCtx, nil))
	require.ErrorContains(t, err, "expected fileKey to be a string")

	_, err = vmPartialHelper("missing.plush", nil, plush.NewHelperContext(ctx, nil))
	require.ErrorContains(t, err, "missing partial")

	parseErrCtx := plush.NewContext()
	parseErrCtx.Set(vmPartialFeederName, func(string) (string, error) {
		return `<%=`, nil
	})
	_, err = vmPartialHelper("bad.plush", nil, plush.NewHelperContext(parseErrCtx, nil))
	require.Error(t, err)

	jsCtx := plush.NewContext()
	jsCtx.Set("contentType", "application/javascript")
	jsCtx.Set(vmPartialFeederName, func(string) (string, error) {
		return `<span>"quoted"</span>`, nil
	})
	rendered, err = vmPartialHelper("row.plush", nil, plush.NewHelperContext(jsCtx, nil))
	require.NoError(t, err)
	require.Equal(t, template.HTML(`\u003Cspan\u003E\"quoted\"\u003C/span\u003E`), rendered)

	_, err = vmPartialHelper("row.plush", nil, plush.HelperContext{})
	require.ErrorContains(t, err, "invalid context")

	budgetCtx := plush.NewContext().WithBudget(plush.NewBudget(0))
	budgetCtx.Set(vmPartialFeederName, func(string) (string, error) {
		return `<span>never</span>`, nil
	})
	_, err = vmPartialHelper("row.plush", nil, plush.NewHelperContext(budgetCtx, nil))
	require.Error(t, err)
}

func Test_VM_Render_Fast_Partial_Segment(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})

	var out strings.Builder
	require.NoError(t, renderFastPartialSegment(&out, ctx, fastRenderBindings{}, &compiler.FastPartialPlan{Name: "row.plush", Line: 1}))
	require.Equal(t, `<span>Mido</span>`, out.String())
	require.NoError(t, renderFastPartialSegment(&out, ctx, fastRenderBindings{}, nil))

	out.Reset()
	require.NoError(t, renderFastPartialSegmentWithDataPlan(&out, ctx, fastRenderBindings{}, &compiler.FastPartialPlan{
		Name: "row.plush",
		Data: []compiler.FastPartialDataPair{{Key: "name", Value: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "Leela"}, Line: 2}},
		Line: 2,
	}, nil))
	require.Equal(t, `<span>Leela</span>`, out.String())

	err := renderFastPartialSegmentWithDataPlan(&out, plush.NewContext().WithBudget(plush.NewBudget(0)), fastRenderBindings{}, &compiler.FastPartialPlan{Name: "row.plush", Line: 3}, nil)
	require.ErrorContains(t, err, "render budget exceeded")

	handled, err := renderFastNoDataPartialInto(nil, "row.plush", ctx, 4)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = renderFastNoDataPartialInto(&out, "row.plush", nil, 4)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 4")

	handled, err = renderFastNoDataPartialInto(&out, "row.plush", plush.NewContext(), 5)
	require.True(t, handled)
	require.ErrorContains(t, err, "line 5")

	require.False(t, fastPartialDataCanDirect(nil))
	require.False(t, fastPartialDataCanDirect(&fastPartialDataBindingPlan{keys: []string{"layout"}}))
	require.True(t, fastPartialDataCanDirect(&fastPartialDataBindingPlan{keys: []string{"name"}}))
	require.False(t, fastPartialRenderPlanCanDirect(nil))
	require.False(t, fastPartialRenderPlanCanDirect(&compiler.FastRenderPlan{Bindings: []string{meta.TemplateFileKey}}))
	require.True(t, fastPartialRenderPlanCanDirect(&compiler.FastRenderPlan{Bindings: []string{"name"}}))
}

func Test_VM_Preprocess_Trim_Tags_Branches(t *testing.T) {
	require.Equal(t, "plain", preprocessTrimTags("plain"))
	require.Equal(t, "a<%= name %>b", preprocessTrimTags("a \n\t<%- name %> \n\tb"))
	require.Equal(t, "a<%= name", preprocessTrimTags("a <%- name"))
}

func Test_VM_New_With_Globals_Store_Uses_Provided_Globals(t *testing.T) {
	globals := []object.Object{&object.String{Value: "shared"}}
	bytecode := &compiler.Bytecode{NumGlobals: 1}

	machine := NewWithGlobalsStore(bytecode, globals)
	require.Same(t, globals[0], machine.globals[0])
}

func Test_VM_Runtime_Template_Render_Compile_And_Global_Size_Edges(t *testing.T) {
	var nilTemplate *Template
	_, err := nilTemplate.Render(nil)
	require.ErrorContains(t, err, "cannot render nil compiled template")

	_, err = (&Template{}).Render(nil)
	require.ErrorContains(t, err, "cannot render nil compiled template")

	staticTemplate := &Template{bytecode: &compiler.Bytecode{Static: true, StaticOutput: "static"}}
	rendered, err := staticTemplate.Render(nil)
	require.NoError(t, err)
	require.Equal(t, "static", rendered)

	rendered, err = renderBytecode(&compiler.Bytecode{Static: true, StaticOutput: "bytecode"}, nil)
	require.NoError(t, err)
	require.Equal(t, "bytecode", rendered)

	compiled, err := Compile("hello")
	require.NoError(t, err)
	rendered, err = compiled.Render(nil)
	require.NoError(t, err)
	require.Equal(t, "hello", rendered)

	dynamic, err := Compile(`<%= name %>`)
	require.NoError(t, err)
	rendered, err = renderBytecode(dynamic.bytecode, plush.NewContextWith(map[string]interface{}{"name": "Mido"}))
	require.NoError(t, err)
	require.Equal(t, "Mido", rendered)

	_, err = renderBytecode(dynamic.bytecode, nil)
	require.ErrorContains(t, err, "unknown identifier")

	_, err = Compile(`<% if (`)
	require.Error(t, err)

	require.Equal(t, 0, globalStoreSize(&compiler.Bytecode{NumGlobals: -1}))
	require.Equal(t, 4, globalStoreSize(&compiler.Bytecode{
		NumGlobals:  1,
		GlobalNames: map[int]string{3: "late"},
	}))
}

func Test_VM_Runtime_Render_Cache_And_Error_Branches(t *testing.T) {
	rendered, err := Render("plain", nil)
	require.NoError(t, err)
	require.Equal(t, "plain", rendered)

	_, err = Render(`<% if (`, plush.NewContext())
	require.Error(t, err)

	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	staticCtx := plush.NewContext()
	staticCtx.Set(meta.TemplateFileKey, "runtime_static.plush")
	plush.CacheVMBytecodeForCleanFilename("runtime_static.plush", nil, &compiler.Bytecode{Static: true, StaticOutput: "cached static"})
	rendered, err = Render("ignored", staticCtx)
	require.NoError(t, err)
	require.Equal(t, "cached static", rendered)

	cachedProgram, err := parser.Parse("cached ast")
	require.NoError(t, err)
	plush.CacheVMBytecodeForCleanFilename("runtime_ast.plush", cachedProgram, &compiler.Bytecode{Static: true, StaticOutput: "cached ast"})
	parsedProgram, parsedCachedProgram, err := parseProgram("ignored", "runtime_ast.plush", plush.NewContext())
	require.NoError(t, err)
	require.Same(t, cachedProgram, parsedProgram)
	require.Same(t, cachedProgram, parsedCachedProgram)

	fastTemplate, err := Compile(`<%= name %>`)
	require.NoError(t, err)
	fastCtx := plush.NewContextWith(map[string]interface{}{"name": "Mido"})
	fastCtx.Set(meta.TemplateFileKey, "runtime_fast.plush")
	plush.CacheVMBytecodeForCleanFilename("runtime_fast.plush", nil, fastTemplate.bytecode)
	rendered, err = Render("ignored", fastCtx)
	require.NoError(t, err)
	require.Equal(t, "Mido", rendered)

	dynamicTemplate, err := Compile(`<%= name %>`)
	require.NoError(t, err)
	dynamicTemplate.bytecode.FastRenderPlan = nil
	dynamicTemplate.bytecode.HasPartials = true
	dynamicCtx := plush.NewContextWith(map[string]interface{}{
		"name":    "Leela",
		"partial": plush.PartialHelper,
	})
	dynamicCtx.Set(meta.TemplateFileKey, "runtime_dynamic.plush")
	plush.CacheVMBytecodeForCleanFilename("runtime_dynamic.plush", nil, dynamicTemplate.bytecode)
	rendered, err = Render("ignored", dynamicCtx)
	require.NoError(t, err)
	require.Equal(t, "Leela", rendered)
	require.True(t, sameFunction(dynamicCtx.Value("partial"), plush.PartialHelper))

	rendered, err = renderBytecode(dynamicTemplate.bytecode, dynamicCtx)
	require.NoError(t, err)
	require.Equal(t, "Leela", rendered)
	require.True(t, sameFunction(dynamicCtx.Value("partial"), plush.PartialHelper))
}

func Test_VM_Runtime_Render_No_Filename_Uses_Source_Bytecode_Cache(t *testing.T) {
	clearSourceBytecodeCacheForTest()
	defer clearSourceBytecodeCacheForTest()

	source := `<%= name %>`
	firstCtx := plush.NewContextWith(map[string]interface{}{"name": "first"})
	rendered, err := Render(source, firstCtx)
	require.NoError(t, err)
	require.Equal(t, "first", rendered)
	firstDiagnostics, ok := plush.RenderDiagnosticsFromContext(firstCtx)
	require.True(t, ok)
	require.Equal(t, plush.VMBytecodeCacheMissStoreSource, firstDiagnostics.VMBytecodeCache)

	secondCtx := plush.NewContextWith(map[string]interface{}{"name": "second"})
	rendered, err = Render(source, secondCtx)
	require.NoError(t, err)
	require.Equal(t, "second", rendered)
	secondDiagnostics, ok := plush.RenderDiagnosticsFromContext(secondCtx)
	require.True(t, ok)
	require.Equal(t, plush.VMBytecodeCacheHitSource, secondDiagnostics.VMBytecodeCache)
	require.Equal(t, plush.RenderFastPathFast, secondDiagnostics.FastPath)
}

func Test_VM_Source_Bytecode_Cache_Is_Bounded(t *testing.T) {
	cache := newSourceBytecodeCache(2)
	first := &compiler.Bytecode{Static: true, StaticOutput: "first"}
	second := &compiler.Bytecode{Static: true, StaticOutput: "second"}
	third := &compiler.Bytecode{Static: true, StaticOutput: "third"}

	cache.Set("one", first)
	cache.Set("two", second)
	cached, ok := cache.Get("one")
	require.True(t, ok)
	require.Same(t, first, cached)

	cache.Set("three", third)
	_, ok = cache.Get("one")
	require.False(t, ok)
	cached, ok = cache.Get("two")
	require.True(t, ok)
	require.Same(t, second, cached)
	cached, ok = cache.Get("three")
	require.True(t, ok)
	require.Same(t, third, cached)
}

func Test_VM_Runtime_Render_Returns_Cached_Punch_Hole_Skeleton_Before_Compile(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	ctx := plush.NewContextWith(map[string]interface{}{"name": "cached"})
	ctx.Set(meta.TemplateFileKey, "runtime_cached_hole.plush")
	holes := []plush.HoleMarker{
		plush.NewHoleMarker(plush.PunchHoleMarkerName(0), `<%= name %>`, 1, 15),
	}
	plush.CachePunchHoleSkeleton("runtime_cached_hole.plush", ctx, "A<PLUSH_HOLE_0>B", holes, true)

	rendered, err := Render("ignored", ctx)
	require.NoError(t, err)
	require.Equal(t, "AcachedB", rendered)
}

func Test_VM_Runtime_Render_Uses_Post_Preprocess_Bytecode_Cache(t *testing.T) {
	tests := []struct {
		name     string
		bytecode *compiler.Bytecode
		ctx      hctx.Context
		expected string
	}{
		{
			name:     "Static_Bytecode",
			bytecode: &compiler.Bytecode{Static: true, StaticOutput: "static cache"},
			ctx:      plush.NewContext(),
			expected: "static cache",
		},
		{
			name:     "Fast_Bytecode",
			bytecode: requireCompiledBytecode(t, `<%= name %>`),
			ctx:      plush.NewContextWith(map[string]interface{}{"name": "fast cache"}),
			expected: "fast cache",
		},
		{
			name: "VM_Bytecode_With_Partial_Helper_Restore",
			bytecode: func() *compiler.Bytecode {
				bytecode := requireCompiledBytecode(t, `<%= name %>`)
				bytecode.FastRenderPlan = nil
				bytecode.HasPartials = true
				return bytecode
			}(),
			ctx: plush.NewContextWith(map[string]interface{}{
				"name":    "vm cache",
				"partial": plush.PartialHelper,
			}),
			expected: "vm cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := "runtime_post_preprocess_" + tt.name + ".plush"
			cache := &runtimeSwitchingBytecodeCache{
				astKey:   plush.GenerateASTKey(filename),
				bytecode: tt.bytecode,
			}
			plush.PlushCacheSetup(cache)
			defer plush.PlushCacheSetup(nil)

			if setter, ok := tt.ctx.(interface{ Set(string, interface{}) }); ok {
				setter.Set(meta.TemplateFileKey, filename)
			}
			rendered, err := Render("ignored", tt.ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, rendered)
			require.GreaterOrEqual(t, cache.gets, 3)
		})
	}
}

func Test_VM_Runtime_Render_Bytecode_Uses_Cached_Punch_Hole_Skeleton(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	ctx := plush.NewContextWith(map[string]interface{}{"name": "bytecode cache"})
	ctx.Set(meta.TemplateFileKey, "runtime_render_bytecode_hole.plush")
	holes := []plush.HoleMarker{
		plush.NewHoleMarker(plush.PunchHoleMarkerName(0), `<%= name %>`, 1, 15),
	}
	plush.CachePunchHoleSkeleton("runtime_render_bytecode_hole.plush", ctx, "A<PLUSH_HOLE_0>B", holes, true)

	rendered, err := renderBytecode(&compiler.Bytecode{HasHoles: true}, ctx)
	require.NoError(t, err)
	require.Equal(t, "Abytecode cacheB", rendered)
}

func Test_VM_Runtime_Hole_Render_Returns_Nested_Hole_Skeleton(t *testing.T) {
	nested := requireCompiledBytecode(t, `<%H "inner" %>`)
	parent := plush.NewContext()
	parent.Set(meta.TemplateFileKey, "runtime_nested_hole.plush")
	outer := []plush.HoleMarker{
		plush.NewHoleMarker(plush.PunchHoleMarkerName(0), `<%= "outer" %>`, 0, len(plush.PunchHoleMarkerName(0))),
	}

	rendered := plush.RenderPunchHolesConcurrentlyWith(outer, parent, func(input string, ctx hctx.Context) (string, error) {
		if !plush.IsHoleRender(ctx) {
			return "", errors.New("expected hole render context")
		}
		return renderBytecodeVMWithState(nested, ctx, "runtime_nested_hole.plush", false, "")
	})

	require.Len(t, rendered, 1)
	require.Equal(t, plush.PunchHoleMarkerName(0), rendered[0].Content())
	require.NoError(t, rendered[0].Err())
}
