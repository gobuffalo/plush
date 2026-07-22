package vm

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/helpers/meta"
	"github.com/gobuffalo/plush/v5/parser"
	"github.com/gobuffalo/plush/v5/templatecache/inmemory"
	"github.com/stretchr/testify/require"
)

type vmTestCase struct {
	input    string
	expected interface{}
}

type vmInlineCacheUser struct {
	Name string
}

type vmInlineCacheOtherUser struct {
	Name string
}

type lookupTestContext struct {
	context.Context
	values map[string]interface{}
	lookup int
	has    int
	value  int
}

func newFastRenderBindings(plan *compiler.FastRenderPlan, ctx hctx.Context) fastRenderBindings {
	return newFastRenderBindingsWithPlan(plan, ctx, nil)
}

func fastStructFieldLoopParts(loop *compiler.FastLoopPlan, elemType reflect.Type) bool {
	_, ok := fastStructLoopWriterPlanFor(loop, elemType)
	return ok
}

func newLookupTestContext(values map[string]interface{}) *lookupTestContext {
	return &lookupTestContext{Context: context.Background(), values: values}
}

func (c *lookupTestContext) New() hctx.Context {
	return newLookupTestContext(c.values)
}

func (c *lookupTestContext) Lookup(key string) (interface{}, bool) {
	c.lookup++
	value, ok := c.values[key]
	return value, ok
}

func (c *lookupTestContext) Has(key string) bool {
	c.has++
	_, ok := c.values[key]
	return ok
}

func (c *lookupTestContext) Value(key interface{}) interface{} {
	c.value++
	if key, ok := key.(string); ok {
		return c.values[key]
	}
	return nil
}

func (c *lookupTestContext) Set(key string, value interface{}) {
	c.values[key] = value
}

func (c *lookupTestContext) Update(key string, value interface{}) bool {
	if _, ok := c.values[key]; !ok {
		return false
	}
	c.values[key] = value
	return true
}

type idLookupTestContext struct {
	context.Context
	values     map[string]interface{}
	stringToID map[string]int
	idToString []string
	internID   int
	lookupID   int
	setID      int
	updateID   int
	lookup     int
	has        int
	value      int
}

func newIDLookupTestContext(values map[string]interface{}) *idLookupTestContext {
	ctx := &idLookupTestContext{
		Context:    context.Background(),
		values:     values,
		stringToID: map[string]int{},
	}
	for key := range values {
		ctx.InternID(key)
	}
	ctx.internID = 0
	return ctx
}

func (c *idLookupTestContext) New() hctx.Context {
	return c
}

func (c *idLookupTestContext) InternID(key string) int {
	c.internID++
	if id, ok := c.stringToID[key]; ok {
		return id
	}
	id := len(c.idToString)
	c.stringToID[key] = id
	c.idToString = append(c.idToString, key)
	return id
}

func (c *idLookupTestContext) LookupID(id int) (interface{}, bool) {
	c.lookupID++
	if id < 0 || id >= len(c.idToString) {
		return nil, false
	}
	value, ok := c.values[c.idToString[id]]
	return value, ok
}

func (c *idLookupTestContext) SetID(id int, value interface{}) {
	c.setID++
	if id >= 0 && id < len(c.idToString) {
		c.values[c.idToString[id]] = value
	}
}

func (c *idLookupTestContext) UpdateID(id int, value interface{}) bool {
	c.updateID++
	if id < 0 || id >= len(c.idToString) {
		return false
	}
	key := c.idToString[id]
	if _, ok := c.values[key]; !ok {
		return false
	}
	c.values[key] = value
	return true
}

func (c *idLookupTestContext) Lookup(key string) (interface{}, bool) {
	c.lookup++
	value, ok := c.values[key]
	return value, ok
}

func (c *idLookupTestContext) Has(key string) bool {
	c.has++
	_, ok := c.values[key]
	return ok
}

func (c *idLookupTestContext) Value(key interface{}) interface{} {
	c.value++
	if key, ok := key.(string); ok {
		return c.values[key]
	}
	return nil
}

func (c *idLookupTestContext) Set(key string, value interface{}) {
	c.values[key] = value
}

func (c *idLookupTestContext) Update(key string, value interface{}) bool {
	if _, ok := c.values[key]; !ok {
		return false
	}
	c.values[key] = value
	return true
}

func Test_Integer_Arithmetic(t *testing.T) {
	tests := []vmTestCase{
		{"1", 1},
		{"1 + 2", 3},
		{"5 * (2 + 10)", 60},
		{"-50 + 100 + -50", 0},
	}

	runVMTests(t, tests)
}

func Test_Booleans_And_Conditionals(t *testing.T) {
	tests := []vmTestCase{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"!true", false},
		{"if (true) { 10 } else { 20 }", 10},
		{"if (false) { 10 }", Null},
	}

	runVMTests(t, tests)
}

func Test_Functions_And_Closures(t *testing.T) {
	tests := []vmTestCase{
		{`let identity = fn(a) { a; }; identity(4);`, 4},
		{`let sum = fn(a, b) { a + b; }; sum(1, 2);`, 3},
		{`let newAdder = fn(a, b) { fn(c) { a + b + c }; }; let add = newAdder(1, 2); add(8);`, 11},
	}

	runVMTests(t, tests)
}

func Test_VM_Runtime_Coverage(t *testing.T) {
	tests := []vmTestCase{
		{`"plu" + "sh"`, "plush"},
		{`[1, 2, 3]`, []int{1, 2, 3}},
		{`[1 + 2, 3 * 4, 5 + 6]`, []int{3, 12, 11}},
		{`[1, 2, 3][1]`, 2},
		{
			`{1: 2, 2: 3}`,
			map[object.HashKey]int{
				(&object.Integer{Value: 1}).HashKey(): 2,
				(&object.Integer{Value: 2}).HashKey(): 3,
			},
		},
		{
			`{true: 1, false: 2, "three": 3}`,
			map[object.HashKey]int{
				(&object.Boolean{Value: true}).HashKey():   1,
				(&object.Boolean{Value: false}).HashKey():  2,
				(&object.String{Value: "three"}).HashKey(): 3,
			},
		},
		{`{1: 1, 2: 2}[2]`, 2},
		{`{"foo": 5}["foo"]`, 5},
		{`{"foo": 5}["bar"]`, Null},
		{`let globalSeed = 50; let minusOne = fn(num) { globalSeed - num; }; minusOne(1);`, 49},
		{`let returnsOneReturner = fn() { fn() { 1; }; }; returnsOneReturner()();`, 1},
		{`let newAdder = fn(a, b) { fn(c) { a + b + c }; }; let add = newAdder(1, 2); add(8);`, 11},
		{`let countDown = fn(x) { if (x == 0) { 0 } else { countDown(x - 1) } }; countDown(3);`, 0},
		{`len("")`, 0},
		{`len("hello world")`, 11},
		{`len([1, 2, 3])`, 3},
		{`first([1, 2, 3])`, 1},
		{`last([1, 2, 3])`, 3},
		{`rest([1, 2, 3])`, []int{2, 3}},
		{`push([], 1)`, []int{1}},
	}

	runVMTests(t, tests)

	program, err := compiler.ParseScript(`[][0]`)
	require.NoError(t, err)
	comp := compiler.New()
	require.NoError(t, comp.Compile(program))
	machine := New(comp.Bytecode())
	require.ErrorContains(t, machine.Run(), "array index out of bounds")
}

func Test_Phase_14_Wrong_Function_Argument_Counts(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`fn() { 1; }(1);`, `wrong number of arguments: want=0, got=1`},
		{`fn(a) { a; }();`, `wrong number of arguments: want=1, got=0`},
		{`fn(a, b) { a + b; }(1);`, `wrong number of arguments: want=2, got=1`},
	}

	for _, tt := range tests {
		program, err := compiler.ParseScript(tt.input)
		require.NoError(t, err)

		comp := compiler.New()
		require.NoError(t, comp.Compile(program))

		machine := New(comp.Bytecode())
		err = machine.Run()
		require.Error(t, err)
		require.EqualError(t, err, tt.expected)
	}
}

func Test_Render_Plush_Features(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "mark",
		"hi": func(name string) string {
			return "hi " + name
		},
	})

	out, err := Render(`<p><%= hi(name) %></p>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "<p>hi mark</p>", out)

	out, err = Render(`<% let total = "" %><%= for (i,v) in ["a", "b", "c"] { return v } %>`, plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "abc", out)

	out, err = Render(`<% if (true) { %>hidden<% } %>`, plush.NewContext())
	require.NoError(t, err)
	require.Empty(t, out)

	out, err = Render(`<% if (true) { %><%= "hidden" %><% } %>`, plush.NewContext())
	require.NoError(t, err)
	require.Empty(t, out)

	out, err = Render(`<%= if (true) { %>shown<% } %>`, plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "shown", out)

	_, err = Render(`<% let a = [] %><%= a[0] %>`, plush.NewContext())
	require.ErrorContains(t, err, "array index out of bounds")
}

func Test_VM_Allocates_Globals_From_Bytecode(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int
	}{
		{
			name:          "no globals",
			input:         `1 + 2;`,
			expectedCount: 0,
		},
		{
			name:          "two globals",
			input:         `let one = 1; let two = 2; one + two;`,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := compiler.ParseScript(tt.input)
			require.NoError(t, err)

			comp := compiler.New()
			require.NoError(t, comp.Compile(program))

			machine := New(comp.Bytecode())
			require.Len(t, machine.globals, tt.expectedCount)
		})
	}
}

func Test_VM_Grows_Undersized_Globals_Store(t *testing.T) {
	instructions := append(code.Make(code.OpConstant, 0), code.Make(code.OpSetGlobal, 3)...)
	bytecode := &compiler.Bytecode{
		Instructions: instructions,
		Constants:    []object.Object{&object.Integer{Value: 7}},
	}

	machine := NewWithContext(bytecode, plush.NewContext())
	require.Empty(t, machine.globals)

	require.NoError(t, machine.Run())
	require.Len(t, machine.globals, 4)
	testIntegerObject(t, 7, machine.globals[3])
}

func Test_VM_Get_Global_Outside_Store_Falls_Back_To_Null(t *testing.T) {
	instructions := append(code.Make(code.OpGetGlobal, 7), code.Make(code.OpPop)...)
	bytecode := &compiler.Bytecode{Instructions: instructions}

	machine := NewWithContext(bytecode, plush.NewContext())
	require.Empty(t, machine.globals)

	require.NoError(t, machine.Run())
	require.Same(t, Null, machine.LastPoppedStackElem())
}

func Test_VM_Name_Lookup_Uses_Context_Lookup_Fast_Path(t *testing.T) {
	program, err := compiler.ParseScript(`name`)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))

	ctx := newLookupTestContext(map[string]interface{}{"name": "mido"})
	machine := NewWithContext(comp.Bytecode(), ctx)
	require.NoError(t, machine.Run())

	require.Equal(t, 1, ctx.lookup)
	require.Zero(t, ctx.has)
	require.Zero(t, ctx.value)
}

func Test_VM_Write_Name_Uses_Context_ID_Lookup_Fast_Path(t *testing.T) {
	program, err := parser.Parse(`<%= name %><%= name %>`)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))
	require.Contains(t, comp.Bytecode().Instructions.String(), "OpWriteName")

	ctx := newIDLookupTestContext(map[string]interface{}{"name": "mido"})
	machine := NewWithContext(comp.Bytecode(), ctx)
	require.NoError(t, machine.Run())

	require.Equal(t, "midomido", machine.Rendered())
	require.Equal(t, 2, ctx.lookupID)
	require.Equal(t, 1, ctx.internID)
	require.Zero(t, ctx.lookup)
	require.Zero(t, ctx.has)
	require.Zero(t, ctx.value)
}

func Test_VM_Property_Inline_Cache_Reuses_Instruction_Slot(t *testing.T) {
	program, err := parser.Parse(`<%= user.Name %><%= user.Name %>`)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))
	bytecode := comp.Bytecode()
	positions := opcodePositions(bytecode.Instructions, code.OpWriteNameProperty)
	require.Len(t, positions, 2)
	for _, pos := range positions {
		require.Nil(t, bytecode.PropertyCaches[pos].Load())
	}

	machine := NewWithContext(bytecode, plush.NewContextWith(map[string]interface{}{
		"user": vmInlineCacheUser{Name: "Ada"},
	}))
	require.NoError(t, machine.Run())
	require.Equal(t, "AdaAda", machine.Rendered())

	firstEntries := make([]*propertyInlineCacheEntry, 0, len(positions))
	for _, pos := range positions {
		entry, ok := bytecode.PropertyCaches[pos].Load().(*propertyInlineCacheEntry)
		require.True(t, ok)
		require.Equal(t, reflect.TypeOf(vmInlineCacheUser{}), entry.typ)
		require.Equal(t, propertyLookupField, entry.lookup.kind)
		firstEntries = append(firstEntries, entry)
	}

	machine = NewWithContext(bytecode, plush.NewContextWith(map[string]interface{}{
		"user": vmInlineCacheUser{Name: "Grace"},
	}))
	require.NoError(t, machine.Run())
	require.Equal(t, "GraceGrace", machine.Rendered())

	for i, pos := range positions {
		entry, ok := bytecode.PropertyCaches[pos].Load().(*propertyInlineCacheEntry)
		require.True(t, ok)
		require.Same(t, firstEntries[i], entry)
	}
}

func Test_VM_Property_Inline_Cache_Keeps_Polymorphic_Receivers(t *testing.T) {
	program, err := parser.Parse(`<%= user.Name %>`)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))
	bytecode := comp.Bytecode()
	positions := opcodePositions(bytecode.Instructions, code.OpWriteNameProperty)
	require.Len(t, positions, 1)
	slot := &bytecode.PropertyCaches[positions[0]]

	machine := NewWithContext(bytecode, plush.NewContextWith(map[string]interface{}{
		"user": vmInlineCacheUser{Name: "Ada"},
	}))
	require.NoError(t, machine.Run())
	require.Equal(t, "Ada", machine.Rendered())

	machine = NewWithContext(bytecode, plush.NewContextWith(map[string]interface{}{
		"user": vmInlineCacheOtherUser{Name: "Grace"},
	}))
	require.NoError(t, machine.Run())
	require.Equal(t, "Grace", machine.Rendered())

	head, ok := slot.Load().(*propertyInlineCacheEntry)
	require.True(t, ok)
	require.Equal(t, reflect.TypeOf(vmInlineCacheOtherUser{}), head.typ)
	require.NotNil(t, head.next)
	require.Equal(t, reflect.TypeOf(vmInlineCacheUser{}), head.next.typ)
}

func Test_VM_Regex_Cache_Reuses_Compiled_Pattern(t *testing.T) {
	regexCache.Delete(`^mido$`)

	first, err := cachedRegex(`^mido$`)
	require.NoError(t, err)
	second, err := cachedRegex(`^mido$`)
	require.NoError(t, err)
	require.Same(t, first, second)
}

func Test_VM_Render_Size_Estimation(t *testing.T) {
	machine := New(&compiler.Bytecode{})
	objects := []object.Object{
		&object.String{Value: "hello"},
		&object.Array{Elements: []object.Object{
			&object.String{Value: "world"},
			&object.Integer{Value: 10},
		}},
		&object.Native{Value: template.HTML("<strong>x</strong>")},
	}

	require.GreaterOrEqual(t, machine.estimatedRenderedObjectsSize(objects), len("helloworld10<strong>x</strong>"))
	var out strings.Builder
	for _, obj := range objects {
		machine.writeObject(&out, obj)
	}
	require.Equal(t, "helloworld10<strong>x</strong>", out.String())
}

func Test_VM_Fast_Helper_Registry_Writer_And_Args(t *testing.T) {
	ctx := plush.NewContext()
	args := &fastCallArgs{}
	obj := &object.String{Value: "obj"}
	args.Append("<tag>")
	args.Append(int32(-7))
	args.Append(uint32(9))
	args.Append(float32(2.5))
	args.Append(true)
	args.Append(obj)

	SetFastHelper(nil, "ignored", func(FastWriter, FastArgs) error { return nil })
	SetFastHelper(ctx, "", func(FastWriter, FastArgs) error { return nil })
	require.Nil(t, fastHelperRegistryForContext(nil, false))
	_, ok := fastHelperForContext(ctx, "")
	require.False(t, ok)

	SetFastHelper(ctx, "custom", func(w FastWriter, got FastArgs) error {
		require.Same(t, ctx, w.Context())
		require.Equal(t, 6, got.Len())
		raw, ok := got.Raw(1)
		require.True(t, ok)
		require.Equal(t, int32(-7), raw)
		text, ok := got.String(0)
		require.True(t, ok)
		require.Equal(t, "<tag>", text)
		i, ok := got.Int64(1)
		require.True(t, ok)
		require.Equal(t, int64(-7), i)
		u, ok := got.Uint64(2)
		require.True(t, ok)
		require.Equal(t, uint64(9), u)
		f, ok := got.Float64(3)
		require.True(t, ok)
		require.InDelta(t, 2.5, f, 0.001)
		b, ok := got.Bool(4)
		require.True(t, ok)
		require.True(t, b)
		gotObj, ok := got.Object(5)
		require.True(t, ok)
		require.Same(t, obj, gotObj)
		_, ok = got.Raw(99)
		require.False(t, ok)

		w.WriteEscapedString("<x>")
		w.WriteHTML(template.HTML("<b>raw</b>"))
		w.WriteHTMLString("<i>raw</i>")
		require.True(t, w.WriteGoValue("<go>"))
		return nil
	})

	helper, ok := fastHelperForContext(ctx, "custom")
	require.True(t, ok)
	var out strings.Builder
	handled, err := writeRegisteredFastHelper(&out, ctx, helper, args)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "&lt;x&gt;<b>raw</b><i>raw</i>&lt;go&gt;", out.String())

	handled, err = writeRegisteredFastHelper(&out, ctx, func(FastWriter, FastArgs) error {
		return ErrFastUnsupported
	}, nil)
	require.NoError(t, err)
	require.False(t, handled)

	handled, err = writeRegisteredFastHelper(nil, ctx, helper, args)
	require.NoError(t, err)
	require.False(t, handled)
	handled, err = writeRegisteredFastHelper(&out, ctx, nil, args)
	require.NoError(t, err)
	require.False(t, handled)

	require.False(t, FastArgs{}.Len() != 0)
	_, ok = FastArgs{}.String(0)
	require.False(t, ok)
	_, ok = FastArgs{}.Bool(0)
	require.False(t, ok)
	_, ok = FastArgs{}.Int64(0)
	require.False(t, ok)
	_, ok = FastArgs{}.Uint64(0)
	require.False(t, ok)
	_, ok = FastArgs{}.Float64(0)
	require.False(t, ok)
	_, ok = FastArgs{}.Object(0)
	require.False(t, ok)

	ClearFastHelper(ctx, "custom")
	_, ok = fastHelperForContext(ctx, "custom")
	require.False(t, ok)
}

func Test_VM_Fast_Call_Args_Objects_And_Frame_Writers(t *testing.T) {
	args := &fastCallArgs{}
	args.Append("name")
	args.Append(3)

	objects := args.Objects()
	require.Len(t, objects, 2)
	require.Equal(t, objects, args.Objects())
	require.Equal(t, "name", objects[0].(*object.String).Value)
	require.Equal(t, int64(3), objects[1].(*object.Integer).Value)

	var nilArgs *fastCallArgs
	require.Nil(t, nilArgs.Objects())
	require.Nil(t, args.Raw(99))

	frame := &Frame{}
	writeFastString(frame, "<name>")
	writeFastHTML(frame, template.HTML("<b>html</b>"))
	writeFastBool(frame, true)
	writeFastInt(frame, -12)
	writeFastUint(frame, 42)
	writeFastFloat(frame, 2.5, 64)

	require.True(t, frame.hasOutput)
	require.Equal(t, "&lt;name&gt;<b>html</b>true-12422.5", frame.output.String())

	writeFastString(nil, "ignored")
	writeFastHTML(nil, template.HTML("ignored"))
	writeFastBool(nil, false)
	writeFastInt(nil, 0)
	writeFastUint(nil, 0)
	writeFastFloat(nil, 0, 64)
}

func Test_VM_Partial_Overlay_Context_Methods(t *testing.T) {
	parent := plush.NewContextWith(map[string]interface{}{
		"parent": "value",
	})
	budget := plush.NewBudget(100)
	parent.WithBudget(budget)

	ctx := borrowPartialOverlayContext(parent)
	defer releasePartialOverlayContext(ctx)

	require.False(t, (*partialOverlayContext)(nil).stableBindingIDs())
	require.True(t, ctx.stableBindingIDs())
	deadline, ok := ctx.Deadline()
	require.False(t, ok)
	require.True(t, deadline.IsZero())
	require.Nil(t, ctx.Done())
	require.NoError(t, ctx.Err())
	require.Same(t, budget, ctx.Budget())

	ctx.Set("local", "first")
	require.True(t, ctx.Has("local"))
	require.True(t, ctx.Has("parent"))
	require.Equal(t, "first", ctx.Value("local"))
	value, ok := ctx.Lookup("parent")
	require.True(t, ok)
	require.Equal(t, "value", value)
	require.True(t, ctx.Update("local", "second"))
	value, ok = ctx.Lookup("local")
	require.True(t, ok)
	require.Equal(t, "second", value)

	id := ctx.InternID("local")
	value, ok = ctx.LookupID(id)
	require.True(t, ok)
	require.Equal(t, "second", value)
	ctx.SetID(id, "third")
	value, ok = ctx.LookupID(id)
	require.True(t, ok)
	require.Equal(t, "third", value)
	require.True(t, ctx.UpdateID(id, "fourth"))
	value, ok = ctx.LookupID(id)
	require.True(t, ok)
	require.Equal(t, "fourth", value)

	ids := make([]int, 2)
	ctx.InternIDs([]string{"local", "another"}, ids)
	require.NotEqual(t, ids[0], ids[1])
	ctx.SetID(ids[1], "extra")
	value, ok = ctx.Lookup("another")
	require.True(t, ok)
	require.Equal(t, "extra", value)

	child := ctx.New()
	require.NotNil(t, child)
	child.Set("child", "value")
	require.True(t, child.Has("child"))
	releasePartialOverlayContext(child.(*partialOverlayContext))

	partialChild, cleanup := partialHelperChildContext(parent)
	require.NotNil(t, partialChild)
	require.NotNil(t, cleanup)
	cleanup()
}

func Test_VM_Partial_Cache_Trim_And_Data_Application_Helpers(t *testing.T) {
	cache := newPartialBytecodeLinkCache()
	bytecode := &compiler.Bytecode{}
	link := cache.Set("row", 7, bytecode)
	require.NotNil(t, link)
	require.Equal(t, 1, cache.Len())
	got, ok := cache.Get("row", 7)
	require.True(t, ok)
	require.Same(t, bytecode, got)
	_, ok = cache.Get("row", 8)
	require.False(t, ok)

	require.Equal(t, "hello", string(trimWhitespaceSuffix([]byte("hello \n\t"))))
	require.True(t, isTrimWhitespace(' '))
	require.True(t, isTrimWhitespace('\n'))
	require.False(t, isTrimWhitespace('x'))

	parent := plush.NewContextWith(map[string]interface{}{
		"title": "Engineer",
	})
	partialCtx := borrowPartialOverlayContext(parent)
	defer releasePartialOverlayContext(partialCtx)

	partial := &compiler.FastPartialPlan{
		Data: []compiler.FastPartialDataPair{
			{
				Key:   "name",
				Value: compiler.FastValuePlan{Kind: compiler.FastValueString, Value: "Mido", Line: 1},
				Line:  1,
			},
			{
				Key:   "title",
				Value: compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "title", NameIndex: 0, Line: 1},
				Line:  1,
			},
		},
	}
	bindings := fastRenderBindings{ctx: parent, names: []string{"title"}}
	require.NoError(t, applyFastPartialDataSlow(partialCtx, partial, parent, bindings))
	value, ok := partialCtx.Lookup("name")
	require.True(t, ok)
	require.Equal(t, "Mido", value)
	value, ok = partialCtx.Lookup("title")
	require.True(t, ok)
	require.Equal(t, "Engineer", value)

	dataPlan := buildFastPartialDataBindingPlan(partial)
	require.NotNil(t, dataPlan)
	applied, err := applyFastPartialDataBindingPlan(partialCtx, dataPlan, parent, bindings)
	require.NoError(t, err)
	require.True(t, applied)
}

func Test_VM_Render_Linked_Partial_Inline(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{"name": "<Mido>"})
	var out strings.Builder

	ok, err := renderLinkedPartialInline(&out, `<span><%= name %></span>`, ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, `<span>&lt;Mido&gt;</span>`, out.String())
}

func Test_VM_Fast_Value_Evaluation_Helpers(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"left":  true,
		"right": false,
		"name":  "Mido",
	})
	plan := &compiler.FastRenderPlan{Bindings: []string{"left", "right", "name"}}
	bindings := newFastRenderBindings(plan, ctx)

	value, ok, err := evalFastValue(&compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "name", NameIndex: 2}, ctx, bindings, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "Mido", value)

	value, ok, err = evalFastLoopValue(&compiler.FastValuePlan{Kind: compiler.FastValueLoopKey}, ctx, bindings, "key", "value")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "key", value)

	value, ok, err = evalFastInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "||",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "left", NameIndex: 0},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueName, Value: "right", NameIndex: 1},
	}, ctx, bindings, nil, nil, nil, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, value)

	value, ok, err = evalFastInfixValue(&compiler.FastValuePlan{
		Kind:     compiler.FastValueInfix,
		Operator: "==",
		Left:     &compiler.FastValuePlan{Kind: compiler.FastValueInteger, IntValue: 3},
		Right:    &compiler.FastValuePlan{Kind: compiler.FastValueFloat, FloatValue: 3.0},
	}, ctx, bindings, nil, nil, nil, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, true, value)
}

func Test_VM_Op_Write_Streams_Frame_Output(t *testing.T) {
	program, err := parser.Parse(`<p><%= name %></p>`)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))

	machine := NewWithContext(comp.Bytecode(), plush.NewContextWith(map[string]interface{}{"name": "<Mido>"}))
	require.NoError(t, machine.Run())

	require.Equal(t, "<p>&lt;Mido&gt;</p>", machine.frames[0].output.String())
	require.True(t, machine.frames[0].hasOutput)
	require.Equal(t, machine.frames[0].output.String(), machine.Rendered())
}

func Test_VM_Raw_Write_Opcodes_Preserve_Escaping(t *testing.T) {
	program, err := parser.Parse(`<%= "<x>" %><%= name %><strong></strong>`)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))
	require.NotEmptyf(t, opcodePositions(comp.Bytecode().Instructions, code.OpWriteHTML), "expected OpWriteHTML:\n%s", comp.Bytecode().Instructions.String())
	require.NotEmptyf(t, opcodePositions(comp.Bytecode().Instructions, code.OpWriteString), "expected OpWriteString:\n%s", comp.Bytecode().Instructions.String())

	machine := NewWithContext(comp.Bytecode(), plush.NewContextWith(map[string]interface{}{"name": "<Mido>"}))
	require.NoError(t, machine.Run())
	require.Equal(t, "&lt;x&gt;&lt;Mido&gt;<strong></strong>", machine.Rendered())
}

func Test_VM_Direct_Write_Call_Opcodes(t *testing.T) {
	input := `<%= greet(name) %>|<%= robot.Name.Echo() %>|<% let make = fn() { return "closure" } %><%= make() %>|<%= noop() %>done`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))
	bytecode := comp.Bytecode()
	require.Equalf(t, 2, len(opcodePositions(bytecode.Instructions, code.OpWriteNameCall)), "expected OpWriteNameCall:\n%s", bytecode.Instructions.String())
	require.Equalf(t, 2, len(opcodePositions(bytecode.Instructions, code.OpWriteCall)), "expected OpWriteCall:\n%s", bytecode.Instructions.String())
	require.Emptyf(t, callWritePairs(bytecode.Instructions), "expected call/write pairs to be fused:\n%s", bytecode.Instructions.String())

	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"greet": func(name string) string {
			return "hi " + name
		},
		"robot": struct {
			Name vmEchoName
		}{Name: "bender"},
		"noop": func() {},
	})
	out, err := Render(input, ctx)
	require.NoError(t, err)
	require.Equal(t, "hi Mido|bender|closure|done", out)
}

func Test_VM_Static_Only_Template_Render_Fast_Path(t *testing.T) {
	tmpl, err := Compile(`<strong><%= "<x>" %></strong>`)
	require.NoError(t, err)
	require.True(t, tmpl.bytecode.Static)
	require.Equal(t, `<strong>&lt;x&gt;</strong>`, tmpl.bytecode.StaticOutput)

	out, err := tmpl.Render(nil)
	require.NoError(t, err)
	require.Equal(t, `<strong>&lt;x&gt;</strong>`, out)

	tmpl, err = Compile(`<%= 10 %><span>items</span>`)
	require.NoError(t, err)
	require.True(t, tmpl.bytecode.Static)
	require.Equal(t, `10<span>items</span>`, tmpl.bytecode.StaticOutput)

	out, err = tmpl.Render(nil)
	require.NoError(t, err)
	require.Equal(t, `10<span>items</span>`, out)
}

func Test_VM_Compile_And_Render_Error_Branches(t *testing.T) {
	_, err := Compile(`<%= "bad".Name %>`)
	require.ErrorContains(t, err, "no prefix parse function")

	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer func() {
		plush.ClearTemplateCache()
		plush.PlushCacheSetup(nil)
	}()

	filename := "edge_compile_error.plush"
	badProgram := &ast.Program{Statements: []ast.Statement{
		&ast.ExpressionStatement{Expression: &ast.PrefixExpression{
			Operator: "??",
			Right:    &ast.Boolean{Value: true},
		}},
	}}
	plush.CacheVMBytecodeForCleanFilename(filename, badProgram, "not-bytecode")
	ctx := plush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	_, err = Render(`<%= true %>`, ctx)
	require.ErrorContains(t, err, "unknown operator ??")
}

func Test_VM_Mixed_Static_Name_Render_Fast_Path(t *testing.T) {
	tmpl, err := Compile(`<p>Hello <%= name %></p><p><%= title %></p>`)
	require.NoError(t, err)
	require.False(t, tmpl.bytecode.Static)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Equal(t, 2, tmpl.bytecode.FastRenderPlan.NameCount)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"name":  "<Mido>",
		"title": template.HTML("<strong>Engineer</strong>"),
	}))
	require.NoError(t, err)
	require.Equal(t, `<p>Hello &lt;Mido&gt;</p><p><strong>Engineer</strong></p>`, out)
}

func Test_VM_Fast_Static_Name_Plan_Renders_Simple_Template(t *testing.T) {
	tmpl, err := Compile(`<p><%= name %></p><%= title %>!`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.NotNil(t, mixed.staticName)
	require.Len(t, mixed.staticName.ops, 3)
	require.Equal(t, []int{0, 1}, mixed.staticName.nameIndexes)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"name":  "<Mido>",
		"title": template.HTML("<strong>Engineer</strong>"),
	}))
	require.NoError(t, err)
	require.Equal(t, `<p>&lt;Mido&gt;</p><strong>Engineer</strong>!`, out)
	require.NotNil(t, mixed.staticName.ops[0].outputCache.Load())
	require.NotNil(t, mixed.staticName.ops[1].outputCache.Load())
}

func Test_VM_Fast_Static_Name_Plan_Looks_Up_Repeated_Name_Once(t *testing.T) {
	tmpl, err := Compile(`<%= name %>|<%= name %>|<%= name %>`)
	require.NoError(t, err)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed.staticName)
	require.Equal(t, []int{0}, mixed.staticName.nameIndexes)

	ctx := newIDLookupTestContext(map[string]interface{}{"name": "Mido"})
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `Mido|Mido|Mido`, out)
	require.Equal(t, 1, ctx.lookupID)
}

func Test_VM_Fast_Static_Name_Plan_Rejects_Non_Name_Segments(t *testing.T) {
	type user struct {
		Name string
	}

	tmpl, err := Compile(`<p><%= user.Name %></p>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Nil(t, mixed.staticName)
	require.NotNil(t, mixed.simple)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"user": user{Name: "<Mido>"},
	}))
	require.NoError(t, err)
	require.Equal(t, `<p>&lt;Mido&gt;</p>`, out)
}

func Test_VM_Fast_Static_Name_Plan_Unknown_Identifier_Falls_Back_To_Error(t *testing.T) {
	tmpl, err := Compile(`<p><%= missing %></p>`)
	require.NoError(t, err)
	require.NotNil(t, prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan).staticName)

	_, err = tmpl.Render(plush.NewContext())
	require.Error(t, err)
	require.Contains(t, err.Error(), `"missing": unknown identifier`)
}

func Test_VM_Fast_Simple_Plan_Renders_Static_Property_Access_And_Infix(t *testing.T) {
	type profile struct {
		Name string
	}
	type user struct {
		Profile profile
	}

	tmpl, err := Compile(`<p><%= name %>|<%= user.Profile.Name %>|<%= labels["status"] %>|<%= count == 1 %></p>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Nil(t, mixed.staticName)
	require.NotNil(t, mixed.simple)
	require.Equal(t, []int{0, 1, 2, 3}, mixed.simple.nameIndexes)
	require.Len(t, mixed.simple.ops, 5)
	require.Equal(t, fastMixedOpName, mixed.simple.ops[0].op.kind)
	require.Equal(t, fastMixedOpAccessChain, mixed.simple.ops[1].op.kind)
	require.Equal(t, fastMixedOpAccessChain, mixed.simple.ops[2].op.kind)
	require.Equal(t, fastMixedOpValue, mixed.simple.ops[3].op.kind)
	require.Equal(t, compiler.FastValueInfix, mixed.simple.ops[3].op.valuePlan.Kind)
	require.NotNil(t, mixed.simple.ops[3].value)
	require.Equal(t, 3, mixed.simple.ops[3].value.left.lookupIndex)
	require.Equal(t, -1, mixed.simple.ops[3].value.right.lookupIndex)
	require.Equal(t, fastMixedOpStatic, mixed.simple.ops[4].op.kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"name":   "<Mido>",
		"user":   user{Profile: profile{Name: "<Bender>"}},
		"labels": map[string]string{"status": "<ok>"},
		"count":  uint32(1),
	}))
	require.NoError(t, err)
	require.Equal(t, `<p>&lt;Mido&gt;|&lt;Bender&gt;|&lt;ok&gt;|true</p>`, out)
}

func Test_VM_Fast_Simple_Plan_Looks_Up_Repeated_Root_Once(t *testing.T) {
	type user struct {
		Name string
	}

	tmpl, err := Compile(`<%= user.Name %>|<%= user.Name %>|<%= user.Name == "Mido" %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Nil(t, mixed.staticName)
	require.NotNil(t, mixed.simple)
	require.Equal(t, []int{0}, mixed.simple.nameIndexes)
	require.NotNil(t, mixed.simple.ops[2].value)
	require.Equal(t, 0, mixed.simple.ops[2].value.left.lookupIndex)

	ctx := newIDLookupTestContext(map[string]interface{}{"user": user{Name: "Mido"}})
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `Mido|Mido|true`, out)
	require.Equal(t, 1, ctx.lookupID)
}

func Test_VM_Mixed_Static_Property_Render_Fast_Path(t *testing.T) {
	type user struct {
		Name string
	}

	costs := plush.ZeroCosts()
	costs.ObjectTraversal = 3
	budget := plush.NewBudgetWithCosts(20, costs)
	ctx := plush.NewContextWith(map[string]interface{}{
		"user": user{Name: "<Mido>"},
	})
	ctx.WithBudget(budget)

	tmpl, err := Compile(`<p><%= user.Name %></p>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<p>&lt;Mido&gt;</p>`, out)
	require.Equal(t, int64(3), budget.Stats().ObjectTraversals)
}

func Test_VM_Simple_Loop_Render_Fast_Path(t *testing.T) {
	type product struct {
		Name string
	}

	costs := plush.ZeroCosts()
	costs.LoopIteration = 2
	costs.ObjectTraversal = 3
	budget := plush.NewBudgetWithCosts(100, costs)
	ctx := plush.NewContextWith(map[string]interface{}{
		"items":    []string{"a", "b"},
		"products": []product{{Name: "Bender"}, {Name: "<Fry>"}},
	})
	ctx.WithBudget(budget)

	tmpl, err := Compile(`<%= for (i, item) in items { %><%= i %>:<%= item %>;<% } %>|<%= for (i, product) in products { %><%= product.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `0:a;1:b;|Bender;&lt;Fry&gt;;`, out)

	stats := budget.Stats()
	require.Equal(t, int64(8), stats.LoopIterations)
	require.Equal(t, int64(6), stats.ObjectTraversals)
}

func Test_VM_Fast_Render_Loop_Break_And_Continue(t *testing.T) {
	type variant struct {
		OnSale bool
		Price  int
	}
	type product struct {
		Name     string
		Skip     bool
		Variants []variant
	}

	tmpl, err := Compile(`<%= for (_, product) in products { %><%= if product.Skip { %><%= continue %><% } %><%= product.Name %>:<%= for (_, variant) in product.Variants { %><%= if variant.OnSale { %><span><%= variant.Price %></span><%= break %><% } %><% } %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"products": []product{
			{Name: "A", Variants: []variant{{OnSale: false, Price: 1}, {OnSale: true, Price: 2}, {OnSale: true, Price: 3}}},
			{Name: "B", Skip: true, Variants: []variant{{OnSale: true, Price: 4}}},
			{Name: "C", Variants: []variant{{OnSale: true, Price: 5}}},
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `A:<span>2</span>;C:<span>5</span>;`, out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
}

func Test_VM_Fast_Render_Script_Loop_Control(t *testing.T) {
	tmpl, err := Compile(`<%= for (i, v) in [1, 2, 3] { %><%= i %><% break %><% } %>|<%= for (i, v) in [1, 2, 3] { %><% continue %><%= v %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContext()
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `0|`, out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
}

func Test_VM_Fast_Render_Loop_Block_Helper_Sees_Current_Value(t *testing.T) {
	type product struct {
		ID   int
		Name string
	}

	tmpl, err := Compile(`<%= for (_, product) in products { %><%= form({id: product.ID, path: cartPath()}) { %><span><%= product.Name %></span><% } %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"products": []product{{ID: 1, Name: "<A>"}, {ID: 2, Name: "B"}},
		"cartPath": func() string {
			return "/cart"
		},
		"form": func(data map[string]interface{}, help plush.HelperContext) (template.HTML, error) {
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<form action="%s" id="%v">%s</form>`, data["path"], data["id"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<form action="/cart" id="1"><span>&lt;A&gt;</span></form><form action="/cart" id="2"><span>B</span></form>`, out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
	require.GreaterOrEqual(t, diagnostics.FastPlan.HelperCalls, 1)
}

func Test_VM_Helper_Call_Render_Fast_Plan(t *testing.T) {
	tmpl, err := Compile(`<%= greet(name) %><%= greet(title) %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Len(t, tmpl.bytecode.FastRenderPlan.Segments, 2)
	require.Equal(t, compiler.FastRenderSegmentCall, tmpl.bytecode.FastRenderPlan.Segments[0].Kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"name":  "<Mido>",
		"title": "Engineer",
		"greet": func(value string) string {
			return "hi " + value
		},
	}))
	require.NoError(t, err)
	require.Equal(t, `hi &lt;Mido&gt;hi Engineer`, out)

	if entry, ok := tmpl.bytecode.FastRenderPlan.Segments[0].Call.Cache.Load().(*fastBuilderCallCacheEntry); ok {
		require.NotNil(t, entry.invoker)
	}
}

func Test_VM_Helper_Call_Value_Argument_Can_Satisfy_Pointer_Parameter(t *testing.T) {
	tmpl, err := Compile(`<%= render_section(section) %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"section": vmPointerArgSection{Name: "hero"},
		"render_section": func(section *vmPointerArgSection) string {
			return section.Name
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "hero", out)
}

func Test_VM_Block_Helper_Call_And_Array_Arguments_Render_Fast_Plan(t *testing.T) {
	tmpl, err := Compile(`<%= multiple_menu_items([menu_nav, menu_categories_string]) %>|<%= form({content_for: "head", class: klass}) { %><span><%= name %></span><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"menu_nav":               "main",
		"menu_categories_string": "categories",
		"klass":                  "hero",
		"name":                   "<Mido>",
		"multiple_menu_items": func(values []interface{}) string {
			return fmt.Sprintf("%s,%s", values[0], values[1])
		},
		"form": func(data map[string]interface{}, help plush.HelperContext) (template.HTML, error) {
			body, err := help.Block()
			if err != nil {
				return "", err
			}
			return template.HTML(fmt.Sprintf(`<div data-for="%s" class="%s">%s</div>`, data["content_for"], data["class"], body)), nil
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `main,categories|<div data-for="head" class="hero"><span>&lt;Mido&gt;</span></div>`, out)

	diagnostics, ok := plush.RenderDiagnosticsFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, plush.RenderFastPathFast, diagnostics.FastPath)
	require.Empty(t, diagnostics.FastReject)
	require.GreaterOrEqual(t, diagnostics.FastPlan.HelperCalls, 2)
}

func Test_VM_Block_Helper_Call_Renders_Block_With_Script_Let(t *testing.T) {
	tmpl, err := Compile(`<%= wrap() { %><% let label = name %><span><%= label %></span><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Empty(t, tmpl.bytecode.FastReject)

	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "<Mido>",
		"wrap": func(help plush.HelperContext) (template.HTML, error) {
			body, err := help.Block()
			return template.HTML("<div>" + body + "</div>"), err
		},
	})

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<div><span>&lt;Mido&gt;</span></div>`, out)
}

func Test_VM_Fast_Helper_Call_Uses_Direct_String_Path(t *testing.T) {
	calls := 0
	tmpl, err := Compile(`<%= greet(name) %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"name": "<Mido>",
		"greet": func(value string) string {
			calls++
			return "hi " + value
		},
	}))
	require.NoError(t, err)
	require.Equal(t, `hi &lt;Mido&gt;`, out)
	require.Equal(t, 1, calls)
	require.Nil(t, tmpl.bytecode.FastRenderPlan.Segments[0].Call.Cache.Load())
}

func Test_VM_Fast_Helper_Args_Stay_Raw_For_Direct_Invoker(t *testing.T) {
	args := &fastCallArgs{}
	args.Append("<Mido>")
	var out strings.Builder

	err := writeFastCallValue(&out, plush.NewContext(), "greet", func(value string) string {
		return "hi " + value
	}, args, &object.InlineCacheSlot{})

	require.NoError(t, err)
	require.Equal(t, "hi &lt;Mido&gt;", out.String())
	require.Nil(t, args.objects)
}

func Test_VM_Fast_Helper_Call_Uses_Direct_Scalar_Paths(t *testing.T) {
	type name string

	tests := []struct {
		name     string
		args     []interface{}
		helper   interface{}
		expected string
	}{
		{
			name: "named string to base string",
			args: []interface{}{name("<Mido>")},
			helper: func(value string) string {
				return "hi " + value
			},
			expected: "hi &lt;Mido&gt;",
		},
		{
			name: "int and string",
			args: []interface{}{int32(7), "items"},
			helper: func(count int, suffix string) string {
				return fmt.Sprintf("%d %s", count, suffix)
			},
			expected: "7 items",
		},
		{
			name: "uint32 and string",
			args: []interface{}{uint32(3), "left"},
			helper: func(count uint32, suffix string) string {
				return fmt.Sprintf("%d %s", count, suffix)
			},
			expected: "3 left",
		},
		{
			name: "bool and string",
			args: []interface{}{true, "enabled"},
			helper: func(enabled bool, suffix string) string {
				return fmt.Sprintf("%t %s", enabled, suffix)
			},
			expected: "true enabled",
		},
		{
			name: "float64 and string",
			args: []interface{}{float32(1.5), "rate"},
			helper: func(rate float64, suffix string) string {
				return fmt.Sprintf("%.1f %s", rate, suffix)
			},
			expected: "1.5 rate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args fastCallArgs
			for _, arg := range tt.args {
				args.Append(arg)
			}
			var out strings.Builder
			slot := &object.InlineCacheSlot{}

			err := writeFastCallValue(&out, plush.NewContext(), "helper", tt.helper, &args, slot)

			require.NoError(t, err)
			require.Equal(t, tt.expected, out.String())
			require.Nil(t, args.objects)
			entry, ok := slot.Load().(*fastBuilderCallCacheEntry)
			require.Truef(t, ok, "expected fast builder cache entry, got %T", slot.Load())
			require.NotNil(t, entry.invoker)
		})
	}
}

func Test_VM_Write_Fast_Invoker_Uses_Direct_Scalar_Paths(t *testing.T) {
	invoker := writeFastInvokerForRaw(func(count int, suffix string) string {
		return fmt.Sprintf("%d %s", count, suffix)
	})
	require.NotNil(t, invoker)

	frame := &Frame{}
	err := invoker(nil, frame, "label", func(count int, suffix string) string {
		return fmt.Sprintf("%d %s", count, suffix)
	}, []object.Object{
		object.Wrap(int32(9)),
		object.Wrap("<items>"),
	})

	require.NoError(t, err)
	require.True(t, frame.hasOutput)
	require.Equal(t, "9 &lt;items&gt;", frame.output.String())
}

func Test_VM_Indexed_Property_Chain_Render_Fast_Plan(t *testing.T) {
	tmpl, err := Compile(`<%= robots[0].Name.Echo() %>|<%= robots[1].Name.Echo() %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Equal(t, compiler.FastRenderSegmentValue, tmpl.bytecode.FastRenderPlan.Segments[0].Kind)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Equal(t, fastMixedOpAccessChain, mixed.ops[0].kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"robots": []vmAccessRobot{
			{Name: "bender"},
			{Name: "<fry>"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, `bender|&lt;fry&gt;`, out)

	entry, ok := mixed.ops[0].accessCache.Load().(*fastTopLevelAccessCacheEntry)
	require.True(t, ok)
	require.Equal(t, fastTopLevelAccessMethodCall, entry.kind)
	require.Equal(t, reflect.TypeOf([]vmAccessRobot{}), entry.typ)
	require.NotNil(t, entry.method)
}

func Test_VM_Conditional_Render_Fast_Plan(t *testing.T) {
	costs := plush.ZeroCosts()
	costs.ConditionCheck = 5
	budget := plush.NewBudgetWithCosts(100, costs)
	ctx := plush.NewContextWith(map[string]interface{}{
		"enabled":  false,
		"fallback": true,
		"name":     "<fallback>",
	})
	ctx.WithBudget(budget)

	tmpl, err := Compile(`<%= if enabled { %>yes<% } else if fallback { %><%= name %><% } else { %>no<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Equal(t, compiler.FastRenderSegmentConditional, tmpl.bytecode.FastRenderPlan.Segments[0].Kind)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `&lt;fallback&gt;`, out)
	require.Equal(t, int64(10), budget.Stats().ConditionChecks)
}

func Test_VM_Fast_Simple_Conditional_Plan_Renders_Simple_Branches(t *testing.T) {
	type robot struct {
		Name  string
		Stock uint32
	}

	costs := plush.ZeroCosts()
	costs.ConditionCheck = 5
	costs.ObjectTraversal = 2
	budget := plush.NewBudgetWithCosts(100, costs)
	ctx := plush.NewContextWith(map[string]interface{}{
		"robot":    robot{Name: "<Bender>", Stock: 0},
		"fallback": true,
		"labels":   map[string]string{"status": "<ready>"},
		"count":    uint32(1),
	})
	ctx.WithBudget(budget)

	tmpl, err := Compile(`<%= if robot.Stock > 0 { %><%= robot.Name %><% } else if fallback { %><%= labels["status"] %>:<%= count == 1 %><% } else { %>no<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 1)
	require.Equal(t, fastMixedOpConditional, mixed.ops[0].kind)
	require.NotNil(t, mixed.ops[0].simpleCond)
	require.Len(t, mixed.ops[0].simpleCond.branches, 2)
	require.NotNil(t, mixed.ops[0].simpleCond.branches[0].condition)
	require.NotNil(t, mixed.ops[0].simpleCond.branches[0].segments)
	require.NotNil(t, mixed.ops[0].simpleCond.branches[1].segments)
	require.NotNil(t, mixed.ops[0].simpleCond.elseSegments)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `&lt;ready&gt;:true`, out)
	require.Equal(t, int64(10), budget.Stats().ConditionChecks)
	require.Equal(t, int64(2), budget.Stats().ObjectTraversals)
}

func Test_VM_Fast_Simple_Conditional_Plan_Rejects_Complex_Branches(t *testing.T) {
	tmpl, err := Compile(`<%= if enabled { %><%= greet(name) %><% } else { %>no<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 1)
	require.Equal(t, fastMixedOpConditional, mixed.ops[0].kind)
	require.Nil(t, mixed.ops[0].simpleCond)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"enabled": true,
		"name":    "<Mido>",
		"greet": func(name string) string {
			return "hi " + name
		},
	}))
	require.NoError(t, err)
	require.Equal(t, `hi &lt;Mido&gt;`, out)
}

func Test_VM_Partial_Render_Fast_Plan_Reuses_Linked_Bytecode(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})

	tmpl, err := Compile(`<%= partial("row.plush") %><%= partial("row.plush") %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Equal(t, compiler.FastRenderSegmentPartial, tmpl.bytecode.FastRenderPlan.Segments[0].Kind)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<span>Mido</span><span>Mido</span>`, out)

	links, ok := ctx.Value(vmPartialBytecodeLinksKey).(*partialBytecodeLinkCache)
	require.True(t, ok)
	require.Equal(t, 1, links.Len())
	require.True(t, links.hasFeederID)
}

func Test_VM_Partial_Render_Binding_Plan_Reuses_Linked_Binding_I_Ds(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})

	tmpl, err := Compile(`<%= partial("row.plush") %><%= partial("row.plush") %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<span>Mido</span><span>Mido</span>`, out)

	link := onlyPartialBytecodeLink(t, ctx)
	require.NotNil(t, link.bindingPlan)
	require.Equal(t, []string{"name"}, link.bindingPlan.names)
}

func Test_VM_Fast_Loop_Partial_Sees_Current_Loop_Value(t *testing.T) {
	type product struct {
		Name string
	}

	ctx := plush.NewContextWith(map[string]interface{}{
		"products": []product{{Name: "Pizza"}, {Name: "Pasta"}},
		"partialFeeder": func(name string) (string, error) {
			require.Equal(t, "partials/product-card.plush.html", name)
			return `<span><%= product.Name %></span>`, nil
		},
	})

	tmpl, err := Compile(`<%= for (_, product) in products { %><%= partial("partials/product-card.plush.html") %><% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.Len(t, tmpl.bytecode.FastRenderPlan.Segments, 1)
	segment := tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 1)
	require.Equal(t, compiler.FastLoopPartPartial, segment.Loop.Parts[0].Kind)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<span>Pizza</span><span>Pasta</span>`, out)
	require.False(t, ctx.Has("product"))
}

func Test_VM_Partial_Render_Binding_Plan_Keeps_Data_Values_Live(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})

	out, err := Render(`<%= partial("row.plush", {name: "Mido"}) %><%= partial("row.plush", {name: "Fry"}) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, `<span>Mido</span><span>Fry</span>`, out)

	link := onlyPartialBytecodeLink(t, ctx)
	require.NotNil(t, link.bindingPlan)
	require.Equal(t, []string{"name"}, link.bindingPlan.names)
}

func Test_VM_Fast_Data_Partial_Uses_Overlay_And_Keeps_Values_Live(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name":   "Outer",
		"first":  "Mido",
		"second": "Fry",
		"robot":  vmAccessRobot{Name: "Bender"},
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %>:<%= title %></span>`, nil
		},
	})

	tmpl, err := Compile(`<%= partial("row.plush", {name: first, title: robot.Name}) %>|<%= partial("row.plush", {name: second, title: "literal"}) %>|<%= name %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.NotEmpty(t, tmpl.bytecode.FastRenderPlan.Segments)
	require.Equal(t, compiler.FastRenderSegmentPartial, tmpl.bytecode.FastRenderPlan.Segments[0].Kind)
	require.Len(t, tmpl.bytecode.FastRenderPlan.Segments[0].Partial.Data, 2)

	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 3)
	require.Equal(t, fastMixedOpPartial, mixed.ops[0].kind)
	require.NotNil(t, mixed.ops[0].partialData)
	require.Equal(t, []string{"name", "title"}, mixed.ops[0].partialData.keys)
	require.Equal(t, []int{0, 1}, mixed.ops[0].partialData.nameIndexes)
	require.Equal(t, 0, mixed.ops[0].partialData.pairs[0].value.lookupIndex)
	require.Equal(t, 1, mixed.ops[0].partialData.pairs[1].value.lookupIndex)
	require.Equal(t, fastMixedOpPartial, mixed.ops[1].kind)
	require.NotNil(t, mixed.ops[1].partialData)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<span>Mido:Bender</span>|<span>Fry:literal</span>|Outer`, out)

	link := onlyPartialBytecodeLink(t, ctx)
	require.NotNil(t, link.bindingPlan)
	require.Equal(t, []string{"name", "title"}, link.bindingPlan.names)
}

func Test_VM_Fast_Data_Partial_Helper_Call_Value(t *testing.T) {
	type product struct {
		Name string
	}

	ctx := plush.NewContextWith(map[string]interface{}{
		"product": product{Name: "<Bender>"},
		"prefix":  "robot",
		"label": func(name string, prefix string) string {
			return prefix + ":" + name
		},
		"partialFeeder": func(string) (string, error) {
			return `<span><%= label %></span>`, nil
		},
	})

	tmpl, err := Compile(`<%= partial("row.plush", {label: label(product.Name, prefix)}) %>|<%= product.Name %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	require.NotEmpty(t, tmpl.bytecode.FastRenderPlan.Segments)
	segment := tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentPartial, segment.Kind)
	require.NotNil(t, segment.Partial)
	require.Len(t, segment.Partial.Data, 1)
	require.Equal(t, compiler.FastValueCall, segment.Partial.Data[0].Value.Kind)
	require.NotNil(t, segment.Partial.Data[0].Value.Call)

	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 2)
	require.Equal(t, fastMixedOpPartial, mixed.ops[0].kind)
	require.NotNil(t, mixed.ops[0].partialData)
	require.Equal(t, []string{"label"}, mixed.ops[0].partialData.keys)
	require.Equal(t, []int{0, 1, 2}, mixed.ops[0].partialData.nameIndexes)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<span>robot:&lt;Bender&gt;</span>|&lt;Bender&gt;`, out)

	entry, ok := segment.Partial.Data[0].Value.Call.Cache.Load().(*fastBuilderCallCacheEntry)
	require.Truef(t, ok, "expected fast value call cache entry, got %T", segment.Partial.Data[0].Value.Call.Cache.Load())
	require.NotNil(t, entry)
	require.NotNil(t, entry.plan)
	require.NotNil(t, entry.valueInvoker)
}

func Test_VM_Fast_Data_Partial_Helper_Call_Value_Missing_Helper_Names_Error(t *testing.T) {
	type product struct {
		Name string
	}

	ctx := plush.NewContextWith(map[string]interface{}{
		"product": product{Name: "Bender"},
		"prefix":  "robot",
		"partialFeeder": func(string) (string, error) {
			return `<span>static</span>`, nil
		},
	})

	tmpl, err := Compile(`<%= partial("row.plush", {label: label(product.Name, prefix)}) %>`)
	require.NoError(t, err)

	_, err = tmpl.Render(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), `"label": unknown identifier`)
}

func Test_VM_Fast_Data_Partial_Direct_Binding_Renders_Simple_Linked_Partial(t *testing.T) {
	type robot struct {
		Name string
	}

	tmpl, err := Compile(`<%= partial("row.plush", {name: first, title: robot.Name}) %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 1)
	require.Equal(t, fastMixedOpPartial, mixed.ops[0].kind)
	require.NotNil(t, mixed.ops[0].partialData)

	ctx := plush.NewContextWith(map[string]interface{}{
		"first": "Mido",
		"robot": robot{Name: "<Bender>"},
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %>|<%= title %></span>`, nil
		},
	})
	bindings := newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx)
	var out strings.Builder
	ok, err := renderFastDataPartialDirectInto(&out, mixed.ops[0].partial, ctx, bindings, mixed.ops[0].partialData)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, `<span>Mido|&lt;Bender&gt;</span>`, out.String())

	links, ok := ctx.Value(vmPartialBytecodeLinksKey).(*partialBytecodeLinkCache)
	require.True(t, ok)
	require.Equal(t, 1, links.Len())
	link := onlyPartialBytecodeLink(t, ctx)
	require.NotNil(t, link.bytecode.FastRenderPlan)
}

func Test_VM_Fast_Data_Partial_Direct_Binding_Evaluates_Unused_Data(t *testing.T) {
	tmpl, err := Compile(`<%= partial("row.plush", {name: missing}) %>`)
	require.NoError(t, err)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 1)

	ctx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<span>static</span>`, nil
		},
	})
	var out strings.Builder
	ok, err := renderFastDataPartialDirectInto(&out, mixed.ops[0].partial, ctx, newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx), mixed.ops[0].partialData)
	require.True(t, ok)
	require.Error(t, err)
	require.Contains(t, err.Error(), `"missing": unknown identifier`)
	require.Empty(t, out.String())
}

func Test_VM_Inline_Partial_Fast_Path_Writes_Linked_Static_Name_Bytecode(t *testing.T) {
	partial, err := Compile(`<span><%= name %></span>`)
	require.NoError(t, err)
	require.NotNil(t, partial.bytecode.FastRenderPlan)
	require.NotNil(t, prepareFastMixedPlan(partial.bytecode.FastRenderPlan).staticName)

	ctx := plush.NewContextWith(map[string]interface{}{"name": "Mido"})
	link := &partialBytecodeLink{bytecode: partial.bytecode}
	var out strings.Builder

	ok, err := renderLinkedPartialBytecodeInline(&out, link, ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, `<span>Mido</span>`, out.String())
	require.NotNil(t, link.bindingPlan)
}

func Test_VM_Inline_Partial_Fast_Path_Writes_Linked_Simple_Bytecode(t *testing.T) {
	type profile struct {
		Name string
	}
	type user struct {
		Profile profile
	}

	partial, err := Compile(`<span><%= user.Profile.Name %>|<%= labels["status"] %>|<%= count == 1 %></span>`)
	require.NoError(t, err)
	require.NotNil(t, partial.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(partial.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Nil(t, mixed.staticName)
	require.NotNil(t, mixed.simple)

	ctx := plush.NewContextWith(map[string]interface{}{
		"user":   user{Profile: profile{Name: "<Mido>"}},
		"labels": map[string]string{"status": "<ready>"},
		"count":  uint32(1),
	})
	link := &partialBytecodeLink{bytecode: partial.bytecode}
	var out strings.Builder

	ok, err := renderLinkedPartialBytecodeInline(&out, link, ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, `<span>&lt;Mido&gt;|&lt;ready&gt;|true</span>`, out.String())
	require.NotNil(t, link.bindingPlan)
	require.Equal(t, []string{"user", "labels", "count"}, link.bindingPlan.names)
}

func Test_VM_Inline_Partial_Fast_Path_Writes_Linked_Mixed_Bytecode(t *testing.T) {
	partial, err := Compile(`<%= for (item) in items { %><%= item %>|<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, partial.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(partial.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Nil(t, mixed.staticName)
	require.Nil(t, mixed.simple)

	ctx := plush.NewContextWith(map[string]interface{}{
		"items": []string{"a", "b"},
	})
	link := &partialBytecodeLink{bytecode: partial.bytecode}
	var out strings.Builder

	ok, err := renderLinkedPartialBytecodeInline(&out, link, ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, `a|b|`, out.String())
	require.NotNil(t, link.bindingPlan)
	require.Equal(t, []string{"items"}, link.bindingPlan.names)
}

func Test_VM_Inline_Partial_Fast_Path_Rejects_Unsupported_Block_Helper_Before_Writing(t *testing.T) {
	partial, err := Compile(`<%= form({}) { %>body<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, partial.bytecode.FastRenderPlan)

	ctx := plush.NewContextWith(map[string]interface{}{
		"form": func(interface{}, plush.HelperContext, map[string]interface{}, string) template.HTML {
			return template.HTML("unsupported")
		},
	})
	link := &partialBytecodeLink{bytecode: partial.bytecode}
	var out strings.Builder

	ok, err := renderLinkedPartialBytecodeInline(&out, link, ctx)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, out.String())
}

func Test_VM_Inline_Partial_Fast_Path_Rejects_Unsupported_Simple_Before_Writing(t *testing.T) {
	partial, err := Compile(`<span><%= labels["status"] %></span>`)
	require.NoError(t, err)
	require.NotNil(t, partial.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(partial.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Nil(t, mixed.staticName)
	require.NotNil(t, mixed.simple)

	ctx := plush.NewContextWith(map[string]interface{}{
		"labels": 12,
	})
	link := &partialBytecodeLink{bytecode: partial.bytecode}
	var out strings.Builder

	ok, err := renderLinkedPartialBytecodeInline(&out, link, ctx)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, out.String())
}

func Test_VM_Inline_Partial_Fast_Path_Rejects_Unsupported_Output_Before_Writing(t *testing.T) {
	partial, err := Compile(`<span><%= name %></span>`)
	require.NoError(t, err)

	ctx := plush.NewContextWith(map[string]interface{}{
		"name": &object.Builtin{},
	})
	link := &partialBytecodeLink{bytecode: partial.bytecode}
	var out strings.Builder

	ok, err := renderLinkedPartialBytecodeInline(&out, link, ctx)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, out.String())
}

func Test_VM_Inline_Partial_Fast_Path_Skips_When_JS_Escape_Is_Needed(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"contentType": "application/javascript",
		"partialFeeder": func(string) (string, error) {
			return `<%= "<tag>" %>`, nil
		},
	})
	var out strings.Builder

	ok, err := renderFastNoDataPartialInto(&out, "row.html", ctx, 1)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, template.JSEscapeString("&lt;tag&gt;"), out.String())
}

func Test_VM_Fast_No_Data_Partial_Direct_Renders_Simple_Linked_Partial(t *testing.T) {
	type robot struct {
		Name string
	}

	ctx := plush.NewContextWith(map[string]interface{}{
		"name":  "<Mido>",
		"robot": robot{Name: "<Bender>"},
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %>|<%= robot.Name %></span>`, nil
		},
	})
	var out strings.Builder

	ok, err := renderFastNoDataPartialDirectInto(&out, "row.plush", ctx, 1)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, `<span>&lt;Mido&gt;|&lt;Bender&gt;</span>`, out.String())

	links, ok := ctx.Value(vmPartialBytecodeLinksKey).(*partialBytecodeLinkCache)
	require.True(t, ok)
	require.Equal(t, 1, links.Len())
	link := onlyPartialBytecodeLink(t, ctx)
	require.NotNil(t, link.bytecode.FastRenderPlan)
}

func Test_VM_Fast_No_Data_Partial_Direct_Rejects_Metadata_Bindings(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			return `<%= ` + meta.TemplateFileKey + ` %>`, nil
		},
	})
	var out strings.Builder

	ok, err := renderFastNoDataPartialDirectInto(&out, "row.plush", ctx, 1)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, out.String())
}

func Test_VM_Partial_Overlay_Pool_Clears_Values(t *testing.T) {
	parent := plush.NewContextWith(map[string]interface{}{"name": "parent"})

	ctx := borrowPartialOverlayContext(parent)
	ctx.Set("name", "child")
	for i := 0; i < 10; i++ {
		ctx.Set(fmt.Sprintf("extra%d", i), i)
	}
	value, ok := ctx.Lookup("name")
	require.True(t, ok)
	require.Equal(t, "child", value)
	releasePartialOverlayContext(ctx)

	reused := borrowPartialOverlayContext(parent)
	defer releasePartialOverlayContext(reused)

	value, ok = reused.Lookup("name")
	require.True(t, ok)
	require.Equal(t, "parent", value)
	for i := 0; i < 10; i++ {
		_, ok := reused.localValue(fmt.Sprintf("extra%d", i))
		require.False(t, ok)
	}
}

func Test_VM_Partial_Feeder_ID_Cache_Uses_Current_Feeder(t *testing.T) {
	ctx := plush.NewContext()
	tmpl, err := Compile(`<%= partial("row") %>`)
	require.NoError(t, err)

	ctx.Set("partialFeeder", func(string) (string, error) {
		return `<%= "a" %>`, nil
	})
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "a", out)

	links, ok := ctx.Value(vmPartialBytecodeLinksKey).(*partialBytecodeLinkCache)
	require.True(t, ok)
	require.True(t, links.hasFeederID)

	ctx.Set("partialFeeder", func(string) (string, error) {
		return `<%= "b" %>`, nil
	})
	out, err = tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "b", out)
	require.True(t, links.hasFeederID)
}

func onlyPartialBytecodeLink(t *testing.T, ctx hctx.Context) *partialBytecodeLink {
	t.Helper()
	links, ok := ctx.Value(vmPartialBytecodeLinksKey).(*partialBytecodeLinkCache)
	require.True(t, ok)
	require.Equal(t, 1, links.Len())

	links.mu.RLock()
	defer links.mu.RUnlock()
	for _, link := range links.entries {
		return link
	}
	require.FailNow(t, "expected partial bytecode link")
	return nil
}

type trackingNewContext struct {
	hctx.Context
	newCalls int
}

func (c *trackingNewContext) New() hctx.Context {
	c.newCalls++
	return c.Context.New()
}

func Test_VM_Fast_No_Data_Partial_Avoids_Parent_Context_New(t *testing.T) {
	base := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})
	ctx := &trackingNewContext{Context: base}

	tmpl, err := Compile(`<%= partial("row.plush") %><%= partial("row.plush") %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<span>Mido</span><span>Mido</span>`, out)
	require.Zero(t, ctx.newCalls)

	links, ok := base.Value(vmPartialBytecodeLinksKey).(*partialBytecodeLinkCache)
	require.True(t, ok)
	require.Equal(t, 1, links.Len())
}

func Test_VM_Fast_Mixed_Plan_Fuses_Static_Prefixes(t *testing.T) {
	type user struct {
		Name string
	}

	tmpl, err := Compile(`<p>Hello <%= name %>, <%= user.Name %></p>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 3)
	require.Equal(t, fastMixedOpName, mixed.ops[0].kind)
	require.Equal(t, "<p>Hello ", mixed.ops[0].prefix)
	require.Equal(t, fastMixedOpProperty, mixed.ops[1].kind)
	require.Equal(t, ", ", mixed.ops[1].prefix)
	require.Equal(t, fastMixedOpStatic, mixed.ops[2].kind)
	require.Equal(t, "</p>", mixed.ops[2].prefix)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"user": user{Name: "<Bender>"},
	}))
	require.NoError(t, err)
	require.Equal(t, `<p>Hello Mido, &lt;Bender&gt;</p>`, out)
}

func Test_VM_Fast_Output_Grow_Size_Includes_Loop_Estimate(t *testing.T) {
	tmpl, err := Compile(`<ul><%= for (i, item) in items { %><li><%= i %>:<%= item %></li><% } %></ul>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)

	ctx := plush.NewContextWith(map[string]interface{}{
		"items": []string{"one", "two", "three"},
	})
	bindings := newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx)

	base := mixed.staticSize + mixed.nameCount*16
	require.Greater(t, fastOutputGrowSize(mixed, bindings), base)
}

func Test_VM_Fast_Struct_Field_Chain_Plan_Writes_Output(t *testing.T) {
	type profile struct {
		Name string
	}
	type user struct {
		Profile profile
	}

	costs := plush.ZeroCosts()
	costs.ObjectTraversal = 2
	budget := plush.NewBudgetWithCosts(10, costs)
	ctx := plush.NewContextWith(map[string]interface{}{
		"user": user{Profile: profile{Name: "<Mido>"}},
	})
	ctx.WithBudget(budget)

	tmpl, err := Compile(`<p><%= user.Profile.Name %></p>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 2)
	require.Equal(t, fastMixedOpAccessChain, mixed.ops[0].kind)

	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `<p>&lt;Mido&gt;</p>`, out)
	require.Equal(t, int64(4), budget.Stats().ObjectTraversals)

	entry, ok := mixed.ops[0].accessCache.Load().(*fastTopLevelAccessCacheEntry)
	require.True(t, ok)
	require.Equal(t, fastTopLevelAccessFieldChain, entry.kind)
	require.Equal(t, reflect.TypeOf(user{}), entry.typ)
	require.Len(t, entry.fieldChain.steps, 2)
}

func Test_VM_Fast_Struct_Field_Chain_Plan_Handles_Nil_Pointers(t *testing.T) {
	type profile struct {
		Name string
	}
	type user struct {
		Profile *profile
	}

	tmpl, err := Compile(`<%= user.Profile.Name %>|<%= user.Profile.Name %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"user": user{},
	}))
	require.NoError(t, err)
	require.Equal(t, `|`, out)
}

func Test_VM_Fast_Indexed_Access_Chain_Plan_Writes_Slice_Output(t *testing.T) {
	type robot struct {
		Name string
	}

	tmpl, err := Compile(`<%= robots[0].Name %>|<%= robots[1].Name %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 2)
	require.Equal(t, fastMixedOpAccessChain, mixed.ops[0].kind)

	chain, ok := fastAccessChainPlanFor(&mixed.ops[0].valuePlan, reflect.TypeOf([]robot{}))
	require.True(t, ok)
	require.Len(t, chain.steps, 2)
	require.Equal(t, fastAccessStepIndex, chain.steps[0].kind)
	require.Equal(t, fastAccessStepField, chain.steps[1].kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"robots": []robot{
			{Name: "Bender"},
			{Name: "<Fry>"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, `Bender|&lt;Fry&gt;`, out)

	entry, ok := mixed.ops[0].accessCache.Load().(*fastTopLevelAccessCacheEntry)
	require.True(t, ok)
	require.Equal(t, fastTopLevelAccessChain, entry.kind)
	require.Equal(t, reflect.TypeOf([]robot{}), entry.typ)
}

func Test_VM_Fast_Indexed_Access_Chain_Plan_Writes_Map_Output(t *testing.T) {
	type robot struct {
		Name string
	}

	tmpl, err := Compile(`<%= robots[0].Name %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 1)

	chain, ok := fastAccessChainPlanFor(&mixed.ops[0].valuePlan, reflect.TypeOf(map[int]robot{}))
	require.True(t, ok)
	require.Len(t, chain.steps, 2)
	require.Equal(t, fastAccessStepIndex, chain.steps[0].kind)
	require.Equal(t, fastAccessStepField, chain.steps[1].kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"robots": map[int]robot{
			0: {Name: "<Bender>"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, `&lt;Bender&gt;`, out)
}

func Test_VM_Fast_String_Indexed_Access_Chain_Plan_Writes_Map_Output(t *testing.T) {
	tmpl, err := Compile(`<%= labels["status"] %>|<%= counts["active"] %>|<%= ints["count"] %>|<%= any["label"] %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 4)
	for i := range mixed.ops {
		require.Equal(t, fastMixedOpAccessChain, mixed.ops[i].kind)
	}

	labelChain, ok := fastAccessChainPlanFor(&mixed.ops[0].valuePlan, reflect.TypeOf(map[string]string{}))
	require.True(t, ok)
	require.Len(t, labelChain.steps, 1)
	require.Equal(t, fastAccessStepIndex, labelChain.steps[0].kind)
	require.True(t, labelChain.steps[0].mapKey.IsValid())
	require.Equal(t, "status", labelChain.steps[0].mapKey.Interface())
	require.Equal(t, fastMapDirectStringString, labelChain.steps[0].mapDirect)
	var directOut strings.Builder
	handled, err := writeFastDirectMapIndexOutput(&directOut, plush.NewContext(), reflect.ValueOf(map[string]string{"status": "<ok>"}), &labelChain.steps[0])
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "&lt;ok&gt;", directOut.String())

	countChain, ok := fastAccessChainPlanFor(&mixed.ops[1].valuePlan, reflect.TypeOf(map[string]uint32{}))
	require.True(t, ok)
	require.Len(t, countChain.steps, 1)
	require.Equal(t, fastAccessStepIndex, countChain.steps[0].kind)
	require.True(t, countChain.steps[0].mapKey.IsValid())
	require.Equal(t, "active", countChain.steps[0].mapKey.Interface())
	require.Equal(t, fastMapDirectStringUint32, countChain.steps[0].mapDirect)
	directOut.Reset()
	handled, err = writeFastDirectMapIndexOutput(&directOut, plush.NewContext(), reflect.ValueOf(map[string]uint32{"active": 7}), &countChain.steps[0])
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "7", directOut.String())

	intChain, ok := fastAccessChainPlanFor(&mixed.ops[2].valuePlan, reflect.TypeOf(map[string]int{}))
	require.True(t, ok)
	require.Len(t, intChain.steps, 1)
	require.Equal(t, fastMapDirectStringInt, intChain.steps[0].mapDirect)
	directOut.Reset()
	handled, err = writeFastDirectMapIndexOutput(&directOut, plush.NewContext(), reflect.ValueOf(map[string]int{"count": 3}), &intChain.steps[0])
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "3", directOut.String())

	interfaceChain, ok := fastAccessChainPlanFor(&mixed.ops[3].valuePlan, reflect.TypeOf(map[string]interface{}{}))
	require.True(t, ok)
	require.Len(t, interfaceChain.steps, 1)
	require.Equal(t, fastMapDirectStringInterface, interfaceChain.steps[0].mapDirect)
	directOut.Reset()
	handled, err = writeFastDirectMapIndexOutput(&directOut, plush.NewContext(), reflect.ValueOf(map[string]interface{}{"label": "<any>"}), &interfaceChain.steps[0])
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "&lt;any&gt;", directOut.String())

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"labels": map[string]string{"status": "<ok>"},
		"counts": map[string]uint32{"active": 7},
		"ints":   map[string]int{"count": 3},
		"any":    map[string]interface{}{"label": "<any>"},
	}))
	require.NoError(t, err)
	require.Equal(t, `&lt;ok&gt;|7|3|&lt;any&gt;`, out)
}

func Test_VM_Fast_String_Indexed_Access_Chain_Plan_Writes_Nested_Map_Struct_Output(t *testing.T) {
	type robot struct {
		Name string
	}

	tmpl, err := Compile(`<%= robots["bender"].Name %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 1)
	require.Equal(t, fastMixedOpAccessChain, mixed.ops[0].kind)

	chain, ok := fastAccessChainPlanFor(&mixed.ops[0].valuePlan, reflect.TypeOf(map[string]robot{}))
	require.True(t, ok)
	require.Len(t, chain.steps, 2)
	require.Equal(t, fastAccessStepIndex, chain.steps[0].kind)
	require.Equal(t, "bender", chain.steps[0].mapKey.Interface())
	require.Equal(t, fastAccessStepField, chain.steps[1].kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"robots": map[string]robot{
			"bender": {Name: "<Bender>"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, `&lt;Bender&gt;`, out)
}

func Test_VM_Fast_String_Indexed_Access_Chain_Plan_Handles_Interface_Map_Keys(t *testing.T) {
	tmpl, err := Compile(`<%= labels["status"] %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 1)
	require.Equal(t, fastMixedOpAccessChain, mixed.ops[0].kind)

	chain, ok := fastAccessChainPlanFor(&mixed.ops[0].valuePlan, reflect.TypeOf(map[interface{}]string{}))
	require.True(t, ok)
	require.Len(t, chain.steps, 1)
	require.Equal(t, fastAccessStepIndex, chain.steps[0].kind)
	require.True(t, chain.steps[0].mapKey.IsValid())
	require.Equal(t, "status", chain.steps[0].mapKey.Interface())

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"labels": map[interface{}]string{"status": "<ok>"},
	}))
	require.NoError(t, err)
	require.Equal(t, `&lt;ok&gt;`, out)
}

func Test_VM_Fast_String_Indexed_Access_Chain_Preserves_Bad_Key_Error(t *testing.T) {
	_, err := Render(`<%= lookup["bad"] %>`, plush.NewContextWith(map[string]interface{}{
		"lookup": map[int]string{1: "one"},
	}))
	require.Error(t, err)
	require.Contains(t, err.Error(), `cannot use bad (string constant) as int value in map index`)
}

func Test_VM_Fast_Top_Level_Access_Chain_Plan_Handles_Method_Chain(t *testing.T) {
	tmpl, err := Compile(`<%= robot.Name.Echo() %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 1)
	require.Equal(t, fastMixedOpAccessChain, mixed.ops[0].kind)

	_, ok := fastFieldChainPlanFor(&mixed.ops[0].valuePlan, reflect.TypeOf(vmAccessRobot{}))
	require.False(t, ok)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"robot": vmAccessRobot{Name: "Bender"},
	}))
	require.NoError(t, err)
	require.Equal(t, `Bender`, out)

	entry, ok := mixed.ops[0].accessCache.Load().(*fastTopLevelAccessCacheEntry)
	require.True(t, ok)
	require.Equal(t, fastTopLevelAccessMethodCall, entry.kind)
	require.Equal(t, reflect.TypeOf(vmAccessRobot{}), entry.typ)
	require.NotNil(t, entry.method)
}

func Test_VM_Fast_Top_Level_Access_Chain_Skips_Method_In_Middle(t *testing.T) {
	tmpl, err := Compile(`<%= robot.GetFriends()[0].Name.Echo() %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 1)
	require.Equal(t, fastMixedOpValue, mixed.ops[0].kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"robot": vmAccessRobot{Friends: []vmAccessRobot{{Name: "Fry"}}},
	}))
	require.NoError(t, err)
	require.Equal(t, `Fry`, out)
}

func Test_VM_Loop_Fast_Plan_Uses_Typed_Field_Cache(t *testing.T) {
	type product struct {
		Name string
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= product.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 2)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"products": []product{{Name: "Bender"}, {Name: "Fry"}},
	}))
	require.NoError(t, err)
	require.Equal(t, `Bender;Fry;`, out)

	cache := segment.Loop.Parts[0].PropertyCache.Load()
	require.NotNil(t, cache)
	entry, ok := cache.(*propertyInlineCacheEntry)
	require.True(t, ok)
	require.NotNil(t, entry.reader)
}

func Test_VM_Fast_Loop_Helper_Call_Part(t *testing.T) {
	type product struct {
		Name string
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= label(product.Name, prefix) %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 2)
	require.Equal(t, compiler.FastLoopPartCall, segment.Loop.Parts[0].Kind)
	require.NotNil(t, segment.Loop.Parts[0].Call)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"prefix":   "robot",
		"products": []product{{Name: "Bender"}, {Name: "<Fry>"}},
		"label": func(name string, prefix string) string {
			return prefix + ":" + name
		},
	}))
	require.NoError(t, err)
	require.Equal(t, `robot:Bender;robot:&lt;Fry&gt;;`, out)

	entry, ok := segment.Loop.Parts[0].Call.Cache.Load().(*fastBuilderCallCacheEntry)
	require.Truef(t, ok, "expected fast loop call cache entry, got %T", segment.Loop.Parts[0].Call.Cache.Load())
	require.NotNil(t, entry)
	require.NotNil(t, entry.plan)
	require.NotNil(t, entry.invoker)

	argPlan := &segment.Loop.Parts[0].Call.Args[0]
	chain, ok := fastFieldChainPlanFor(argPlan, reflect.TypeOf(product{}))
	require.True(t, ok)
	require.Len(t, chain.steps, 1)
	require.Equal(t, "Name", chain.steps[0].name)
}

func Test_VM_Fast_Struct_Loop_Specializes_Helper_Call_Part_Automatically(t *testing.T) {
	type product struct {
		Name string
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= label(product.Name, prefix) %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)

	elemType, ok := fastStructLoopElementType(reflect.TypeOf([]product{}))
	require.True(t, ok)
	writerPlan, ok := fastStructLoopWriterPlanFor(segment.Loop, elemType)
	require.True(t, ok)
	require.Len(t, writerPlan.ops, 2)
	require.Equal(t, fastStructLoopWriterCall, writerPlan.ops[0].kind)
	require.NotNil(t, writerPlan.ops[0].call)

	ctx := plush.NewContextWith(map[string]interface{}{
		"prefix":   "robot",
		"products": []product{{Name: "Bender"}, {Name: "<Fry>"}},
		"label": func(name string, prefix string) string {
			return prefix + ":" + name
		},
	})
	bindings := newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx)
	state := &fastStructLoopRenderState{}
	resolved, err := state.resolvedCall(writerPlan.ops[0].call, bindings)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.NotNil(t, resolved.directWriter)

	var out strings.Builder
	handled, err := renderFastStructFieldLoop(&out, ctx, bindings, segment.Loop, reflect.ValueOf([]product{{Name: "Bender"}, {Name: "<Fry>"}}))
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `robot:Bender;robot:&lt;Fry&gt;;`, out.String())

	rendered, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `robot:Bender;robot:&lt;Fry&gt;;`, rendered)
}

func Test_VM_Fast_Struct_Loop_Helper_Call_Learns_Reflect_Arg_Plan(t *testing.T) {
	type productName string
	type product struct {
		Name productName
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= label(product.Name, prefix) %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)

	elemType, ok := fastStructLoopElementType(reflect.TypeOf([]product{}))
	require.True(t, ok)
	writerPlan, ok := fastStructLoopWriterPlanFor(segment.Loop, elemType)
	require.True(t, ok)
	require.Len(t, writerPlan.ops, 2)
	require.Equal(t, fastStructLoopWriterCall, writerPlan.ops[0].kind)
	require.NotNil(t, writerPlan.ops[0].call)
	require.Len(t, writerPlan.ops[0].call.args, 2)
	require.Equal(t, fastStructLoopCallArgAccessChain, writerPlan.ops[0].call.args[0].kind)
	require.Equal(t, fastStructLoopCallArgBinding, writerPlan.ops[0].call.args[1].kind)

	label := func(name productName, prefix string) string {
		return prefix + ":" + string(name)
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"prefix":   "robot",
		"products": []product{{Name: "Bender"}, {Name: "<Fry>"}},
		"label":    label,
	})
	bindings := newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx)
	state := &fastStructLoopRenderState{}
	resolved, err := state.resolvedCall(writerPlan.ops[0].call, bindings)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.True(t, resolved.canReflect)
	require.Equal(t, reflect.TypeOf(label), resolved.entry.rt)

	var out strings.Builder
	handled, err := renderFastStructFieldLoop(&out, ctx, bindings, segment.Loop, reflect.ValueOf([]product{{Name: "Bender"}, {Name: "<Fry>"}}))
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `robot:Bender;robot:&lt;Fry&gt;;`, out.String())

	rendered, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `robot:Bender;robot:&lt;Fry&gt;;`, rendered)
}

func Test_VM_Fast_Loop_Key_Values_For_Helper_Calls(t *testing.T) {
	type product struct {
		Name string
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= label(i) %>:<%= product.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Equal(t, compiler.FastLoopPartCall, segment.Loop.Parts[0].Kind)
	require.Equal(t, compiler.FastValueLoopKey, segment.Loop.Parts[0].Call.Args[0].Kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"products": []product{{Name: "Bender"}, {Name: "Fry"}},
		"label": func(index int) string {
			return fmt.Sprintf("idx-%d", index)
		},
	}))
	require.NoError(t, err)
	require.Equal(t, `idx-0:Bender;idx-1:Fry;`, out)

	entry, ok := segment.Loop.Parts[0].Call.Cache.Load().(*fastBuilderCallCacheEntry)
	require.Truef(t, ok, "expected fast loop call cache entry, got %T", segment.Loop.Parts[0].Call.Cache.Load())
	require.NotNil(t, entry.invoker)
}

func Test_VM_Fast_Loop_Conditional_Part(t *testing.T) {
	type product struct {
		Name    string
		Enabled bool
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= if product.Enabled { %><%= product.Name %><% } else if fallback { %>fallback<% } else { %>hidden<% } %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Len(t, segment.Loop.Parts, 2)
	require.Equal(t, compiler.FastLoopPartConditional, segment.Loop.Parts[0].Kind)
	require.NotNil(t, segment.Loop.Parts[0].Conditional)

	products := []product{
		{Name: "<Bender>", Enabled: true},
		{Name: "Fry", Enabled: false},
	}
	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"products": products,
		"fallback": false,
	}))
	require.NoError(t, err)
	require.Equal(t, `&lt;Bender&gt;;hidden;`, out)

	out, err = tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"products": products,
		"fallback": true,
	}))
	require.NoError(t, err)
	require.Equal(t, `&lt;Bender&gt;;fallback;`, out)
}

func Test_VM_Fast_Loop_Infix_Conditional_Part(t *testing.T) {
	type product struct {
		Name  string
		Stock uint32
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= if product.Stock > min { %><%= product.Name %><% } else if i == 0 { %>first<% } else { %>low<% } %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)
	require.Equal(t, compiler.FastLoopPartConditional, segment.Loop.Parts[0].Kind)
	require.Equal(t, compiler.FastValueInfix, segment.Loop.Parts[0].Conditional.Branches[0].Condition.Kind)
	require.Equal(t, compiler.FastValueInfix, segment.Loop.Parts[0].Conditional.Branches[1].Condition.Kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"min": uint64(2),
		"products": []product{
			{Name: "Bender", Stock: 0},
			{Name: "<Fry>", Stock: 3},
			{Name: "Leela", Stock: 1},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, `first;&lt;Fry&gt;;low;`, out)
}

func Test_VM_Fast_Top_Level_Infix_Output(t *testing.T) {
	type robot struct {
		Stock uint32
	}

	tmpl, err := Compile(`<%= i32 == 0 %>|<%= u32 == 0 %>|<%= u64one == 1.0 %>|<%= f32i == 3 %>|<%= f64i == i32v %>|<%= robot.Stock == 3.0 %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 6)
	for i := range mixed.ops {
		require.Equal(t, fastMixedOpValue, mixed.ops[i].kind)
		require.Equal(t, compiler.FastValueInfix, mixed.ops[i].valuePlan.Kind)
	}

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"i32":    int32(0),
		"i32v":   int32(3),
		"u32":    uint32(0),
		"u64one": uint64(1),
		"f32i":   float32(3),
		"f64i":   float64(3),
		"robot":  robot{Stock: uint32(3)},
	}))
	require.NoError(t, err)
	require.Equal(t, `true|true|true|true|true|true`, out)
}

func Test_VM_Fast_Top_Level_Logical_Infix_Output_Short_Circuits(t *testing.T) {
	tmpl, err := Compile(`<%= enabled && missing.Value %>|<%= enabled || missing.Value %>|<%= count == 1 && enabled %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"enabled": false,
		"count":   uint32(1),
	}))
	require.NoError(t, err)
	require.Equal(t, `false|false|false`, out)

	out, err = tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"enabled": true,
		"count":   uint32(1),
	}))
	require.NoError(t, err)
	require.Equal(t, `false|true|true`, out)
}

func Test_VM_Fast_Top_Level_Infix_Output_Preserves_Missing_Ordered_Error(t *testing.T) {
	_, err := Render(`<%= undefined > 3 %>`, plush.NewContext())
	require.Error(t, err)
	require.Contains(t, err.Error(), `unable to operate (>)`)
}

func Test_VM_Fast_Struct_Loop_Specializes_Conditional_Part_Automatically(t *testing.T) {
	type product struct {
		Name  string
		Stock uint32
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= if product.Stock > min { %><%= product.Name %><% } else if i == 0 { %>first<% } else { %>low<% } %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)

	elemType, ok := fastStructLoopElementType(reflect.TypeOf([]product{}))
	require.True(t, ok)
	writerPlan, ok := fastStructLoopWriterPlanFor(segment.Loop, elemType)
	require.True(t, ok)
	require.Len(t, writerPlan.ops, 2)
	require.Equal(t, fastStructLoopWriterConditional, writerPlan.ops[0].kind)
	require.NotNil(t, writerPlan.ops[0].conditional)
	require.Len(t, writerPlan.ops[0].conditional.branches, 2)
	firstCondition := writerPlan.ops[0].conditional.branches[0].conditionPlan
	require.NotNil(t, firstCondition)
	require.Equal(t, fastStructLoopConditionInfix, firstCondition.kind)
	require.Equal(t, ">", firstCondition.operator)
	require.Equal(t, fastStructLoopCallArgAccessChain, firstCondition.leftValue.kind)
	require.Equal(t, fastStructLoopCallArgBinding, firstCondition.rightValue.kind)
	secondCondition := writerPlan.ops[0].conditional.branches[1].conditionPlan
	require.NotNil(t, secondCondition)
	require.Equal(t, fastStructLoopConditionInfix, secondCondition.kind)
	require.Equal(t, "==", secondCondition.operator)
	require.Equal(t, fastStructLoopCallArgKey, secondCondition.leftValue.kind)
	require.Equal(t, fastStructLoopCallArgInt, secondCondition.rightValue.kind)

	products := []product{
		{Name: "Bender", Stock: 0},
		{Name: "<Fry>", Stock: 3},
		{Name: "Leela", Stock: 1},
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"min":      uint64(2),
		"products": products,
	})
	var out strings.Builder
	handled, err := renderFastStructFieldLoop(&out, ctx, newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx), segment.Loop, reflect.ValueOf(products))
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `first;&lt;Fry&gt;;low;`, out.String())

	rendered, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `first;&lt;Fry&gt;;low;`, rendered)
}

func Test_VM_Fast_Struct_Loop_Specializes_Logical_Infix_Conditional_Part(t *testing.T) {
	type product struct {
		Name  string
		Stock uint32
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= if product.Stock > min && i != 0 { %><%= product.Name %><% } else { %>skip<% } %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)

	elemType, ok := fastStructLoopElementType(reflect.TypeOf([]product{}))
	require.True(t, ok)
	writerPlan, ok := fastStructLoopWriterPlanFor(segment.Loop, elemType)
	require.True(t, ok)
	require.Len(t, writerPlan.ops, 2)
	require.Equal(t, fastStructLoopWriterConditional, writerPlan.ops[0].kind)
	require.NotNil(t, writerPlan.ops[0].conditional)
	require.Len(t, writerPlan.ops[0].conditional.branches, 1)

	condition := writerPlan.ops[0].conditional.branches[0].conditionPlan
	require.NotNil(t, condition)
	require.Equal(t, fastStructLoopConditionLogical, condition.kind)
	require.Equal(t, "&&", condition.operator)
	require.NotNil(t, condition.left)
	require.NotNil(t, condition.right)
	require.Equal(t, fastStructLoopConditionInfix, condition.left.kind)
	require.Equal(t, fastStructLoopConditionInfix, condition.right.kind)

	products := []product{
		{Name: "Bender", Stock: 5},
		{Name: "<Fry>", Stock: 3},
		{Name: "Leela", Stock: 1},
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"min":      uint64(2),
		"products": products,
	})
	var out strings.Builder
	handled, err := renderFastStructFieldLoop(&out, ctx, newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx), segment.Loop, reflect.ValueOf(products))
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `skip;&lt;Fry&gt;;skip;`, out.String())

	rendered, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `skip;&lt;Fry&gt;;skip;`, rendered)
}

func Test_VM_Fast_Binding_Output_Inline_Cache(t *testing.T) {
	tmpl, err := Compile(`<%= name %>|<%= count %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"name":  "<Mido>",
		"count": uint32(7),
	}))
	require.NoError(t, err)
	require.Equal(t, `&lt;Mido&gt;|7`, out)

	mixed := prepareFastMixedPlan(tmpl.bytecode.FastRenderPlan)
	require.NotNil(t, mixed)
	require.Len(t, mixed.ops, 2)
	require.NotNil(t, mixed.ops[0].outputCache.Load())
	require.NotNil(t, mixed.ops[1].outputCache.Load())

	out, err = tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"name":  template.HTML("<b>Mido</b>"),
		"count": uint64(9),
	}))
	require.NoError(t, err)
	require.Equal(t, `<b>Mido</b>|9`, out)
}

func Test_VM_Top_Level_Fast_Binding_Plan_Caches_Root_Context_I_Ds(t *testing.T) {
	tmpl, err := Compile(`<%= name %>|<%= title %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"z":     true,
		"name":  "Mido",
		"title": "Engineer",
	}))
	require.NoError(t, err)
	require.Equal(t, `Mido|Engineer`, out)

	cached, ok := tmpl.bytecode.FastRenderPlan.BindingPrepared.Load().(*fastRenderBindingPlan)
	require.True(t, ok)
	require.NotNil(t, cached)
	require.True(t, cached.matches(tmpl.bytecode.FastRenderPlan.Bindings))

	out, err = tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"a":     true,
		"title": "Pilot",
		"name":  "Fry",
	}))
	require.NoError(t, err)
	require.Equal(t, `Fry|Pilot`, out)

	reused, ok := tmpl.bytecode.FastRenderPlan.BindingPrepared.Load().(*fastRenderBindingPlan)
	require.True(t, ok)
	require.Same(t, cached, reused)
}

func Test_VM_Top_Level_Fast_Binding_Plan_Skips_Custom_ID_Context(t *testing.T) {
	tmpl, err := Compile(`<%= name %>|<%= title %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	ctx := newIDLookupTestContext(map[string]interface{}{
		"name":  "Mido",
		"title": "Engineer",
	})
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, `Mido|Engineer`, out)
	require.Nil(t, tmpl.bytecode.FastRenderPlan.BindingPrepared.Load())
	require.Equal(t, 2, ctx.lookupID)
}

func Test_VM_Fast_Binding_Output_Inline_Cache_Scalar_Types(t *testing.T) {
	cases := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{name: "string", value: "<x>", expected: "&lt;x&gt;"},
		{name: "html", value: template.HTML("<b>x</b>"), expected: "<b>x</b>"},
		{name: "bool", value: true, expected: "true"},
		{name: "int", value: int(-1), expected: "-1"},
		{name: "int8", value: int8(-2), expected: "-2"},
		{name: "int16", value: int16(-3), expected: "-3"},
		{name: "int32", value: int32(-4), expected: "-4"},
		{name: "int64", value: int64(-5), expected: "-5"},
		{name: "uint", value: uint(1), expected: "1"},
		{name: "uint8", value: uint8(2), expected: "2"},
		{name: "uint16", value: uint16(3), expected: "3"},
		{name: "uint32", value: uint32(4), expected: "4"},
		{name: "uint64", value: uint64(5), expected: "5"},
		{name: "uintptr", value: uintptr(6), expected: "6"},
		{name: "float32", value: float32(1.25), expected: "1.25"},
		{name: "float64", value: float64(2.5), expected: "2.5"},
	}

	ctx := plush.NewContext()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var slot object.InlineCacheSlot
			var out strings.Builder
			handled := writeFastBindingOutput(&out, ctx, tc.value, &slot)
			require.True(t, handled)
			require.Equal(t, tc.expected, out.String())

			entry, ok := slot.Load().(*fastOutputCacheEntry)
			require.True(t, ok)
			require.True(t, entry.matches(tc.value))

			out.Reset()
			handled = writeFastBindingOutput(&out, ctx, tc.value, &slot)
			require.True(t, handled)
			require.Equal(t, tc.expected, out.String())
		})
	}
}

func Test_VM_Fast_Struct_Slice_Loop_Specialization(t *testing.T) {
	type product struct {
		Name  string
		Price int
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= i %>:<%= product.Name %>=<%= product.Price %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)

	elemType, ok := fastStructLoopElementType(reflect.TypeOf([]product{}))
	require.True(t, ok)
	require.Equal(t, reflect.TypeOf(product{}), elemType)
	require.True(t, fastStructFieldLoopParts(segment.Loop, elemType))
	writerPlan, ok := fastStructLoopWriterPlanFor(segment.Loop, elemType)
	require.True(t, ok)
	require.Len(t, writerPlan.ops, 6)
	require.Equal(t, fastStructLoopWriterKey, writerPlan.ops[0].kind)
	require.Equal(t, fastStructLoopWriterField, writerPlan.ops[2].kind)
	require.Equal(t, "Name", writerPlan.ops[2].name)
	require.Equal(t, fastStructLoopWriterField, writerPlan.ops[4].kind)
	require.Equal(t, "Price", writerPlan.ops[4].name)

	products := []product{
		{Name: "Bender", Price: 10},
		{Name: "<Fry>", Price: 20},
	}
	var out strings.Builder
	ctx := plush.NewContext()
	handled, err := renderFastStructFieldLoop(&out, ctx, newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx), segment.Loop, reflect.ValueOf(products))
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `0:Bender=10;1:&lt;Fry&gt;=20;`, out.String())

	rendered, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"products": products,
	}))
	require.NoError(t, err)
	require.Equal(t, `0:Bender=10;1:&lt;Fry&gt;=20;`, rendered)
}

func Test_VM_Fast_Struct_Pointer_Slice_Loop_Specialization(t *testing.T) {
	type product struct {
		Name string
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= product.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)

	elemType, ok := fastStructLoopElementType(reflect.TypeOf([]*product{}))
	require.True(t, ok)
	require.Equal(t, reflect.TypeOf(product{}), elemType)
	require.True(t, fastStructFieldLoopParts(segment.Loop, elemType))

	products := []*product{{Name: "Bender"}, nil, {Name: "<Fry>"}}
	var out strings.Builder
	ctx := plush.NewContext()
	handled, err := renderFastStructFieldLoop(&out, ctx, newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx), segment.Loop, reflect.ValueOf(products))
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `Bender;;&lt;Fry&gt;;`, out.String())

	rendered, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"products": products,
	}))
	require.NoError(t, err)
	require.Equal(t, `Bender;;&lt;Fry&gt;;`, rendered)
}

func Test_VM_Fast_Struct_Slice_Loop_Specializes_Nested_Value_Path(t *testing.T) {
	type category struct {
		Name string
	}
	type product struct {
		Category category
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= product.Category.Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)

	elemType, ok := fastStructLoopElementType(reflect.TypeOf([]product{}))
	require.True(t, ok)
	require.True(t, fastStructFieldLoopParts(segment.Loop, elemType))
	writerPlan, ok := fastStructLoopWriterPlanFor(segment.Loop, elemType)
	require.True(t, ok)
	require.Len(t, writerPlan.ops, 2)
	require.Equal(t, fastStructLoopWriterAccessChain, writerPlan.ops[0].kind)
	require.NotNil(t, writerPlan.ops[0].accessPlan)
	require.Len(t, writerPlan.ops[0].accessPlan.steps, 2)

	products := []product{
		{Category: category{Name: "Drinks"}},
		{Category: category{Name: "<Snacks>"}},
	}
	var out strings.Builder
	ctx := plush.NewContext()
	handled, err := renderFastStructFieldLoop(&out, ctx, newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx), segment.Loop, reflect.ValueOf(products))
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `Drinks;&lt;Snacks&gt;;`, out.String())

	rendered, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"products": products,
	}))
	require.NoError(t, err)
	require.Equal(t, `Drinks;&lt;Snacks&gt;;`, rendered)
}

func Test_VM_Fast_Struct_Slice_Loop_Specializes_Indexed_Value_Path(t *testing.T) {
	type friend struct {
		Name string
	}
	type product struct {
		Friends []friend
	}

	tmpl, err := Compile(`<%= for (i, product) in products { %><%= product.Friends[0].Name %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)

	elemType, ok := fastStructLoopElementType(reflect.TypeOf([]product{}))
	require.True(t, ok)
	require.True(t, fastStructFieldLoopParts(segment.Loop, elemType))
	writerPlan, ok := fastStructLoopWriterPlanFor(segment.Loop, elemType)
	require.True(t, ok)
	require.Len(t, writerPlan.ops, 2)
	require.Equal(t, fastStructLoopWriterAccessChain, writerPlan.ops[0].kind)
	require.NotNil(t, writerPlan.ops[0].accessPlan)
	require.Len(t, writerPlan.ops[0].accessPlan.steps, 3)
	require.Equal(t, fastAccessStepField, writerPlan.ops[0].accessPlan.steps[0].kind)
	require.Equal(t, fastAccessStepIndex, writerPlan.ops[0].accessPlan.steps[1].kind)
	require.Equal(t, fastAccessStepField, writerPlan.ops[0].accessPlan.steps[2].kind)

	products := []product{
		{Friends: []friend{{Name: "Bender"}}},
		{Friends: []friend{{Name: "<Fry>"}}},
	}
	var out strings.Builder
	ctx := plush.NewContext()
	handled, err := renderFastStructFieldLoop(&out, ctx, newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx), segment.Loop, reflect.ValueOf(products))
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `Bender;&lt;Fry&gt;;`, out.String())

	rendered, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"products": products,
	}))
	require.NoError(t, err)
	require.Equal(t, `Bender;&lt;Fry&gt;;`, rendered)
}

func Test_VM_Fast_Struct_Slice_Loop_Specializes_Method_Value_Path(t *testing.T) {
	tmpl, err := Compile(`<%= for (i, product) in products { %><%= product.Name.Echo() %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)

	elemType, ok := fastStructLoopElementType(reflect.TypeOf([]vmAccessRobot{}))
	require.True(t, ok)
	require.True(t, fastStructFieldLoopParts(segment.Loop, elemType))
	writerPlan, ok := fastStructLoopWriterPlanFor(segment.Loop, elemType)
	require.True(t, ok)
	require.Len(t, writerPlan.ops, 2)
	require.Equal(t, fastStructLoopWriterMethodCall, writerPlan.ops[0].kind)
	require.NotNil(t, writerPlan.ops[0].methodPlan)

	var out strings.Builder
	ctx := plush.NewContext()
	handled, err := renderFastStructFieldLoop(&out, ctx, newFastRenderBindings(tmpl.bytecode.FastRenderPlan, ctx), segment.Loop, reflect.ValueOf([]vmAccessRobot{{Name: "Bender"}, {Name: "<Fry>"}}))
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, `Bender;&lt;Fry&gt;;`, out.String())

	rendered, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"products": []vmAccessRobot{{Name: "Bender"}, {Name: "<Fry>"}},
	}))
	require.NoError(t, err)
	require.Equal(t, `Bender;&lt;Fry&gt;;`, rendered)
}

func Test_VM_Fast_String_Key_Value_Loop_Specialization(t *testing.T) {
	tmpl, err := Compile(`<%= for (i, item) in items { %><%= i %>:<%= item %>;<% } %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)
	segment := &tmpl.bytecode.FastRenderPlan.Segments[0]
	require.Equal(t, compiler.FastRenderSegmentLoop, segment.Kind)
	require.NotNil(t, segment.Loop)

	separator, suffix, ok := fastStringKeyValueLoopParts(segment.Loop)
	require.True(t, ok)
	require.Equal(t, ":", separator)
	require.Equal(t, ";", suffix)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"items": []string{"a", "<b>"},
	}))
	require.NoError(t, err)
	require.Equal(t, `0:a;1:&lt;b&gt;;`, out)
}

func Test_VM_Mixed_Static_Name_Fast_Path_Falls_Back_For_Unsupported_Values(t *testing.T) {
	tmpl, err := Compile(`<%= values %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"values": []int{1, 2, 3},
	}))
	require.NoError(t, err)
	require.Equal(t, "123", out)
}

func Test_VM_Static_Only_Render_Uses_Cached_Static_Bytecode(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer plush.ClearTemplateCache()

	filename := "vm-static-fast-path.plush"
	ctx := plush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := Render(`<%= 10 %><span>items</span>`, ctx)
	require.NoError(t, err)
	require.Equal(t, `10<span>items</span>`, out)

	entry, ok := cache.Get(plush.GenerateASTKey(filename))
	require.True(t, ok)
	bytecode, ok := entry.VMBytecode.(*compiler.Bytecode)
	require.True(t, ok)
	require.True(t, bytecode.Static)

	ctx = plush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)
	out, err = Render(`<%= 10 %><span>items</span>`, ctx)
	require.NoError(t, err)
	require.Equal(t, bytecode.StaticOutput, out)
}

func Test_VM_Cached_Bytecode_Entry_Renders_VM_Only_Bytecode(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)
	defer plush.ClearTemplateCache()

	filename := "vm-bytecode-entry.plush"
	input := `<% let forceBytecode = fn() { return "x" } %><% let name = "cached" %><%= name %>`
	ctx := plush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)

	out, err := Render(input, ctx)
	require.NoError(t, err)
	require.Equal(t, "cached", out)

	entry, ok := cache.Get(plush.GenerateASTKey(filename))
	require.True(t, ok)
	bytecode, ok := entry.VMBytecode.(*compiler.Bytecode)
	require.True(t, ok)
	require.False(t, bytecode.Static)
	require.Nil(t, bytecode.FastRenderPlan)

	ctx = plush.NewContext()
	ctx.Set(meta.TemplateFileKey, filename)
	out, err = Render(input, ctx)
	require.NoError(t, err)
	require.Equal(t, "cached", out)
}

func Test_VM_Direct_Write_Call_Renders_Helpers_Methods_And_Closures(t *testing.T) {
	input := `<%= greet(name) %>|<%= robot.Name() %>|<% let local = fn(value) { return "local " + value } %><%= local(name) %>|<%= echo.Echo() %>`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))
	bytecode := comp.Bytecode()
	require.Equal(t, 1, len(opcodePositions(bytecode.Instructions, code.OpWriteNameCall)))
	require.Equal(t, 3, len(opcodePositions(bytecode.Instructions, code.OpWriteCall)))
	require.Falsef(t, instructionContainsSequenceForVM(bytecode.Instructions, code.OpCall, code.OpWrite), "expected call/write pair to be fused:\n%s", bytecode.Instructions.String())

	out, err := Render(input, plush.NewContextWith(map[string]interface{}{
		"name":  "Mido",
		"greet": func(name string) string { return "hi " + name },
		"robot": &vmRobot{name: "Bender"},
		"echo":  vmEchoName("echoed"),
	}))
	require.NoError(t, err)
	require.Equal(t, "hi Mido|Bender|local Mido|echoed", out)
}

func Test_VM_Direct_Write_Call_Renders_Fast_Return_Kinds(t *testing.T) {
	input := `<%= unsafe() %>|<%= safe() %>|<%= yes() %>|<%= count() %>|<%= ratio() %>`
	out, err := Render(input, plush.NewContextWith(map[string]interface{}{
		"unsafe": func() string { return "<x>" },
		"safe":   func() template.HTML { return template.HTML("<b>x</b>") },
		"yes":    func() bool { return true },
		"count":  func() int { return 7 },
		"ratio":  func() float64 { return 1.5 },
	}))
	require.NoError(t, err)
	require.Equal(t, "&lt;x&gt;|<b>x</b>|true|7|1.5", out)
}

func Test_VM_Callsite_Cache_For_Named_Helpers(t *testing.T) {
	input := `<% let forceBytecode = fn() { return "x" } %><%= greet(name) %><%= greet(name) %>`
	tmpl, err := Compile(input)
	require.NoError(t, err)

	positions := opcodePositions(tmpl.bytecode.Instructions, code.OpWriteNameCall)
	require.Len(t, positions, 2)
	for _, pos := range positions {
		require.Nil(t, tmpl.bytecode.CallCaches[pos].Load())
	}

	ctx := plush.NewContextWith(map[string]interface{}{
		"name":  "Mido",
		"greet": func(name string) string { return "hi " + name },
	})
	out, err := tmpl.Render(ctx)
	require.NoError(t, err)
	require.Equal(t, "hi Midohi Mido", out)

	for _, pos := range positions {
		entry := requireCallCacheEntry(t, tmpl.bytecode.CallCaches[pos].Load())
		require.NotNil(t, entry.plan)
		require.NotNil(t, entry.invoker)
		require.False(t, entry.noFast)
	}
}

func Test_VM_Callsite_Cache_For_Method_Write_Calls(t *testing.T) {
	input := `<% let forceBytecode = fn() { return "x" } %><%= echo.Echo() %><%= echo.Echo() %>`
	tmpl, err := Compile(input)
	require.NoError(t, err)

	positions := opcodePositions(tmpl.bytecode.Instructions, code.OpWriteCall)
	require.Len(t, positions, 2)
	for _, pos := range positions {
		require.Nil(t, tmpl.bytecode.CallCaches[pos].Load())
	}

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"echo": vmEchoName("<echo>"),
	}))
	require.NoError(t, err)
	require.Equal(t, "&lt;echo&gt;&lt;echo&gt;", out)

	for _, pos := range positions {
		entry := requireCallCacheEntry(t, tmpl.bytecode.CallCaches[pos].Load())
		require.NotNil(t, entry.plan)
		require.NotNil(t, entry.invoker)
		require.False(t, entry.noFast)
	}
}

func Test_VM_Write_Call_Falls_Back_When_Fast_Invoker_Is_Unavailable(t *testing.T) {
	input := `<% let forceBytecode = fn() { return "x" } %><%= robot.Generic() %>`
	tmpl, err := Compile(input)
	require.NoError(t, err)

	positions := opcodePositions(tmpl.bytecode.Instructions, code.OpWriteCall)
	require.Len(t, positions, 1)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"robot": vmGenericMethodRobot{},
	}))
	require.NoError(t, err)
	require.Equal(t, "generic", out)

	entry := requireCallCacheEntry(t, tmpl.bytecode.CallCaches[positions[0]].Load())
	require.NotNil(t, entry.plan)
	require.Nil(t, entry.invoker)
	require.True(t, entry.noFast)
}

func Test_VM_Direct_Property_Write_Fusion_And_Raw_Loop_Values(t *testing.T) {
	type product struct {
		Name string
	}
	input := `<%= user.Name %>|<%= for (i, product) in products { %><%= product.Name %>;<% } %>`
	program, err := parser.Parse(input)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))
	require.NotEmptyf(t, opcodePositions(comp.Bytecode().Instructions, code.OpWriteNameProperty), "expected OpWriteNameProperty:\n%s", comp.Bytecode().Instructions.String())

	loopFn := firstCompiledFunction(t, comp.Bytecode().Constants)
	require.NotEmptyf(t, opcodePositions(loopFn.Instructions, code.OpWriteLocalProperty), "expected OpWriteLocalProperty:\n%s", loopFn.Instructions.String())
	require.True(t, loopCanUseRawValues(&object.Closure{Fn: loopFn}))

	out, err := Render(input, plush.NewContextWith(map[string]interface{}{
		"user":     product{Name: "<Ada>"},
		"products": []product{{Name: "Bender"}, {Name: "<Fry>"}},
	}))
	require.NoError(t, err)
	require.Equal(t, "&lt;Ada&gt;|Bender;&lt;Fry&gt;;", out)
}

func Test_VM_Loop_Raw_Value_Detection_Rejects_Computed_Locals(t *testing.T) {
	program, err := compiler.ParseScript(`for (i, item) in items { item + "!" }`)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))

	loopFn := firstCompiledFunction(t, comp.Bytecode().Constants)
	require.False(t, loopCanUseRawValues(&object.Closure{Fn: loopFn}))
}

func Test_VM_Partial_Bytecode_Links_Reuse_Within_Render(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "Mido",
		"partialFeeder": func(string) (string, error) {
			return `<span><%= name %></span>`, nil
		},
	})

	out, err := Render(`<%= partial("row") %><%= partial("row") %><%= partial("row") %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "<span>Mido</span><span>Mido</span><span>Mido</span>", out)

	links, ok := ctx.Value(vmPartialBytecodeLinksKey).(*partialBytecodeLinkCache)
	require.True(t, ok)
	require.Equal(t, 1, links.Len())
}

func Test_VM_Partial_Bytecode_Links_Do_Not_Stale_Dynamic_Feeder_Source(t *testing.T) {
	parts := []string{`<%= "a" %>`, `<%= "b" %>`}
	calls := 0
	ctx := plush.NewContextWith(map[string]interface{}{
		"partialFeeder": func(string) (string, error) {
			part := parts[calls]
			calls++
			return part, nil
		},
	})

	out, err := Render(`<%= partial("dynamic") %><%= partial("dynamic") %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "ab", out)
	require.Equal(t, 2, calls)

	links, ok := ctx.Value(vmPartialBytecodeLinksKey).(*partialBytecodeLinkCache)
	require.True(t, ok)
	require.Equal(t, 1, links.Len())
}

func Test_VM_Loop_Context_Write_Detection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		template bool
		expected bool
	}{
		{
			name:     "pure stack locals",
			input:    `for (i, item) in items { item }`,
			expected: false,
		},
		{
			name:     "helper call",
			input:    `for (i, item) in items { helper(item) }`,
			expected: true,
		},
		{
			name:     "hole",
			input:    `<%= for (i, item) in items { %><%H item %><% } %>`,
			template: true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				program interface{}
				err     error
			)
			if tt.template {
				program, err = parser.Parse(tt.input)
			} else {
				program, err = compiler.ParseScript(tt.input)
			}
			require.NoError(t, err)

			comp := compiler.New()
			require.NoError(t, comp.Compile(program))

			fn := firstCompiledFunction(t, comp.Bytecode().Constants)
			require.Equal(t, tt.expected, loopNeedsContextWrites(&object.Closure{Fn: fn}))
		})
	}
}

func Test_Render_Helper_Block_And_Assignment(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"wrap": func(help plush.HelperContext) (template.HTML, error) {
			body, err := help.Block()
			return template.HTML("<div>" + body + "</div>"), err
		},
	})

	out, err := Render(`<% let message = "hello" %><%= wrap() { message = "bye" } %><%= message %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "<div></div>bye", out)
}

func Test_Render_Phase_6_Helper_Interop_Matrix(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name": "mark",
		"count": func(values ...int) int {
			return len(values)
		},
		"touch": func(value string) {},
		"pair": func() (string, error) {
			return "pair", nil
		},
		"wrap": func(data map[string]interface{}, help plush.HelperContext) (template.HTML, error) {
			child := help.New()
			for k, v := range data {
				child.Set(k, v)
			}
			body, err := help.BlockWith(child)
			return template.HTML(body), err
		},
		"iface": func(help hctx.HelperContext) string {
			if !help.HasBlock() {
				return "no block"
			}
			body, _ := help.Block()
			return body
		},
		"renderSnippet": func(help hctx.HelperContext) (template.HTML, error) {
			body, err := help.Render(`<%= name %>`)
			return template.HTML(body), err
		},
	})

	out, err := Render(`<%= count() %>|<%= count(1, 2) %>|<%= touch("x") %>|<%= pair() %>|<%= wrap({name: "child"}) { return name } %>|<%= name %>|<%= iface() { return "block" } %>|<%= renderSnippet() %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "0|2||pair|child|mark|block|mark", out)
}

func Test_Render_Struct_Traversal(t *testing.T) {
	ctx := plush.NewContext()
	ctx.Set("user", struct {
		Name string
	}{Name: "Mark"})

	out, err := Render(`<%= user.Name %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "Mark", out)
}

type vmRobot struct {
	name string
}

func (r *vmRobot) Name() string {
	return r.name
}

type vmHTMLer struct {
	value string
}

func (h vmHTMLer) HTML() template.HTML {
	return template.HTML(h.value)
}

type vmInterfaceValue struct {
	value interface{}
}

func (v vmInterfaceValue) Interface() interface{} {
	return v.value
}

type vmStringer string

func (s vmStringer) String() string {
	return string(s)
}

type vmGenericStringer string

func (s vmGenericStringer) String() string {
	return string(s)
}

type vmGenericMethodRobot struct{}

func (vmGenericMethodRobot) Generic() vmGenericStringer {
	return "generic"
}

type vmPhase9Robot struct {
	Name string
}

type vmBudgetGreeter struct{}

func (vmBudgetGreeter) Greet() string {
	return "hi"
}

type vmBudgetRobot struct {
	Name vmEchoName
}

func Test_Render_Pointer_Method_On_Struct_Value(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"robot": vmRobot{name: "Bender"},
	})

	out, err := Render(`<%= robot.Name() %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "Bender", out)
}

func Test_Render_Phase_7_Output_Safety_Matrix(t *testing.T) {
	tm := time.Date(2013, time.February, 3, 0, 0, 0, 0, time.UTC)
	ctx := plush.NewContextWith(map[string]interface{}{
		"unsafe":        `<strong>"x"</strong>`,
		"safeHTML":      template.HTML(`<strong>"x"</strong>`),
		"htmler":        vmHTMLer{value: `<em>x</em>`},
		"interfaceHTML": vmInterfaceValue{value: template.HTML(`<i>x</i>`)},
		"rawStringer":   vmStringer(`<b>stringer</b>`),
		"tm":            tm,
		"strings":       []string{"a", "<b>"},
		"mixed":         []interface{}{"x", template.HTML("<y>"), 7, false, nil},
	})

	out, err := Render(`<%= "<script>" %>|<%= unsafe %>|<%= safeHTML %>|<%= htmler %>|<%= interfaceHTML %>|<%= rawStringer %>|<%= tm %>|<%= strings %>|<%= mixed %>|<%= nil %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, `&lt;script&gt;|&lt;strong&gt;&#34;x&#34;&lt;/strong&gt;|<strong>"x"</strong>|<em>x</em>|<i>x</i>|<b>stringer</b>|February 03, 2013 00:00:00 +0000|a&lt;b&gt;|x<y>7false|`, out)

	ctx.Set("TIME_FORMAT", "2006-02-Jan")
	out, err = Render(`<%= tm %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "2013-03-Feb", out)
}

func Test_Render_Phase_9_Optional_If_Parentheses(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"name":   "mark",
		"admin":  false,
		"robots": []vmPhase9Robot{{Name: "mark"}},
		"lookup": map[string]string{"TESTING": "mark"},
	})

	out, err := Render(`<%= if true { %>a<% } %>|<%= if name == "mark" || admin { %>b<% } %>|<%= if (name == "ringo") || admin { %>x<% } else if robots[0].Name == "mark" { %>c<% } %>|<%= if lookup["TESTING"] == "mark" { %>d<% } %>|<%= if (true) { %>old<% } %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "a|b|c|d|old", out)
}

func Test_Render_If_Block_Output(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"show": true,
	})

	out, err := Render(`<%= if (show) { %>yes<% } %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "yes", out)
}

func Test_Render_If_Branch_Let_Does_Not_Overwrite_Outer(t *testing.T) {
	out, err := Render(`<p><% let username = "Hello World" %><%= if (username) {
		let username = "hi"
		username = "bye"
	} %><%= username %></p>`, plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "<p>Hello World</p>", out)
}

func Test_Render_If_Branch_Assignment_Updates_Outer(t *testing.T) {
	out, err := Render(`<p><% let username = "Hello World" %><%= if (username) {
		username = "hi"
		if (username == "hi") {
			username = "hi2"
		}
	} %><%= username %></p>`, plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "<p>hi2</p>", out)
}

func Test_VM_Fast_Render_Silent_If_Assignment_Updates_Outer(t *testing.T) {
	type listing struct {
		DisplayResult int
		TotalResult   int
	}

	tmpl, err := Compile(`<% let displayResult = listing.DisplayResult
if (listing.TotalResult < listing.DisplayResult) {
	displayResult = listing.TotalResult
}
%><%= displayResult %>`)
	require.NoError(t, err)
	require.NotNil(t, tmpl.bytecode.FastRenderPlan, tmpl.bytecode.FastReject)
	require.Empty(t, tmpl.bytecode.FastReject)
	require.Len(t, tmpl.bytecode.FastRenderPlan.Segments, 3)
	require.Equal(t, compiler.FastRenderSegmentLet, tmpl.bytecode.FastRenderPlan.Segments[0].Kind)
	require.Equal(t, compiler.FastRenderSegmentConditional, tmpl.bytecode.FastRenderPlan.Segments[1].Kind)
	require.Equal(t, compiler.FastRenderSegmentAssign, tmpl.bytecode.FastRenderPlan.Segments[1].Conditional.Branches[0].Segments[0].Kind)
	require.Equal(t, compiler.FastRenderSegmentName, tmpl.bytecode.FastRenderPlan.Segments[2].Kind)

	out, err := tmpl.Render(plush.NewContextWith(map[string]interface{}{
		"listing": listing{DisplayResult: 10, TotalResult: 3},
	}))
	require.NoError(t, err)
	require.Equal(t, "3", out)
}

type vmProductListing struct {
	Products []vmProduct
}

type vmProduct struct {
	Name []string
}

type vmEchoName string

func (n vmEchoName) Echo() string {
	return string(n)
}

func (n vmEchoName) String() string {
	return string(n)
}

type vmAccessNode struct {
	N    string
	Next *vmAccessNode
}

type vmAccessRobot struct {
	Name    vmEchoName
	Next    *vmAccessNode
	Friends []vmAccessRobot
	Map     map[string]vmAccessRobot
	Buckets map[string]vmAccessBucket
}

func (r vmAccessRobot) GetFriends() []vmAccessRobot {
	return r.Friends
}

type vmAccessBucket struct {
	Items []vmAccessRobot
}

type vmRobotFactory struct {
	values []vmAccessRobot
}

func (f vmRobotFactory) Robots() []vmAccessRobot {
	return f.values
}

type vmFunctionReturnFactory struct {
	robot vmFunctionReturnRobot
}

func (f vmFunctionReturnFactory) Robots() vmFunctionReturnRobot {
	return f.robot
}

type vmFunctionReturnRobot struct {
	name vmEchoName
}

func (r vmFunctionReturnRobot) Name() vmEchoName {
	return r.name
}

type vmArrayHolder struct {
	Items [2]vmAccessRobot
}

type vmAccessPerson struct {
	born time.Time
}

func (p vmAccessPerson) GetBorn() time.Time {
	return p.born
}

type vmNumericStats struct {
	Count uint64
}

type vmNumericRobot struct {
	Count uint32
	Stats vmNumericStats
}

func (r vmNumericRobot) MethodCount() uint32 {
	return r.Count
}

func Test_Render_Indexed_Receiver_Callee_Chain(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"product_listing": vmProductListing{
			Products: []vmProduct{{Name: []string{"Buffalo"}}},
		},
	})

	out, err := Render(`<% let a = product_listing.Products[0].Name[0] %><%= a %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "Buffalo", out)
}

func Test_Render_Struct_Access_Phase_5_Chains(t *testing.T) {
	robot := vmAccessRobot{
		Name: "bender",
		Next: &vmAccessNode{N: "one", Next: &vmAccessNode{N: "two"}},
		Friends: []vmAccessRobot{
			{Name: "fry"},
			{Name: "leela"},
		},
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"robot":  robot,
		"robots": []vmAccessRobot{robot},
		"getRobots": func() []vmAccessRobot {
			return []vmAccessRobot{{Name: "scruffy"}}
		},
		"factory": func() vmRobotFactory {
			return vmRobotFactory{values: []vmAccessRobot{{Name: "factory"}}}
		},
	})

	out, err := Render(`<%= robot.Next.Next.N %>|<%= robots[0].Friends[1].Name.Echo() %>|<%= getRobots()[0].Name %>|<%= factory().Robots()[0].Name.Echo() %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "two|leela|scruffy|factory", out)
}

func Test_Render_Struct_Access_Phase_5_Map_And_Typed_Collection_Chains(t *testing.T) {
	robot := vmAccessRobot{
		Name: "bender",
		Map: map[string]vmAccessRobot{
			"owner": {Name: "fry"},
		},
		Buckets: map[string]vmAccessBucket{
			"team": {Items: []vmAccessRobot{{Name: "amy"}}},
		},
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"robot":     robot,
		"key":       "owner",
		"bucketKey": "team",
		"holder": vmArrayHolder{
			Items: [2]vmAccessRobot{{Name: "zero"}, {Name: "one"}},
		},
		"nour": vmAccessPerson{
			born: time.Date(1993, time.January, 11, 0, 0, 0, 0, time.UTC),
		},
	})

	out, err := Render(`<%= robot.Map[key].Name.Echo() %>|<%= robot.Buckets[bucketKey].Items[0].Name %>|<%= holder.Items[1].Name %>|<%= nour.GetBorn().Format("Jan 2, 2006") %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "fry|amy|one|Jan 11, 1993", out)

	out, err = Render(`<%= robot.Map["missing"].Name %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "", out)
}

func Test_Render_Struct_Access_Phase_5_Nested_Function_Returns(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"factory": func() vmFunctionReturnFactory {
			return vmFunctionReturnFactory{
				robot: vmFunctionReturnRobot{name: "nested"},
			}
		},
	})

	out, err := Render(`<%= factory().Robots().Name().Echo() %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "nested", out)
}

func Test_Render_Struct_Access_Phase_5_Errors(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"robot": vmAccessRobot{
			Name:    "bender",
			Friends: []vmAccessRobot{{Name: "fry"}},
		},
		"items":  []vmAccessRobot{{Name: "zero"}},
		"lookup": map[int]string{1: "one"},
	})

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "slice index type",
			input:    `<%= items["bad"].Name %>`,
			contains: `can't access Slice/Array with a non int Index`,
		},
		{
			name:     "slice out of bounds after method return",
			input:    `<%= robot.GetFriends()[9].Name %>`,
			contains: `array index out of bounds`,
		},
		{
			name:     "map key type",
			input:    `<%= lookup["bad"] %>`,
			contains: `cannot use bad (string constant) as int value in map index`,
		},
		{
			name:     "call non function field",
			input:    `<%= robot.Name() %>`,
			contains: `does not have a method named 'Name'`,
		},
		{
			name:     "invalid chained call",
			input:    `<%= robot.Name.Missing() %>`,
			contains: `does not have a method named 'Missing'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(tt.input, ctx)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.contains)
		})
	}
}

func Test_Render_Loop_Break_Stops_After_Output(t *testing.T) {
	out, err := Render(`<%= for (i,v) in [1, 2, 3, 4] {
		%>Start<%
		if (v == 3) {
			%>Stop<%
			break
		}
		return v
	} %>`, plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "Start1Start2StartStop", out)
}

func Test_Render_Loop_Continue_Keeps_Output_And_Skips_Value(t *testing.T) {
	out, err := Render(`<%= for (i,v) in [1, 2, 3, 4] {
		%>Start<%
		if (v == 1 || v == 3) {
			%>Odd<%
			continue
		}
		return v
	} %>`, plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "StartOddStart2StartOddStart4", out)
}

func Test_Render_Native_Slice_Append(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"ints": []int{1, 2},
	})

	out, err := Render(`<% let a = ints %><% a = a + 3 %><%= a[2] %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "3", out)
}

func Test_Render_Safe_Mixed_Numeric_Comparisons(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"i32":     int32(0),
		"neg":     int32(-1),
		"u32":     uint32(0),
		"u32v":    uint32(3),
		"u64":     uint64(0),
		"u64one":  uint64(1),
		"u64max":  uint64(math.MaxUint64),
		"f32":     float32(3.5),
		"f64":     float64(3.5),
		"counts":  map[string]uint32{"active": 0},
		"values":  []uint32{0},
		"robot":   vmNumericRobot{Count: 0, Stats: vmNumericStats{Count: 0}},
		"robots":  []vmNumericRobot{{Count: 1, Stats: vmNumericStats{Count: 0}}},
		"counter": func() uint32 { return 0 },
	})

	out, err := Render(`<%= i32 == 0 %>|<%= u32 == 0 %>|<%= u64 == 0 %>|<%= u64one > 0 %>|<%= u64max > 0 %>|<%= neg < 0 %>|<%= neg < u64 %>|<%= neg == u64 %>|<%= u32v == 3 %>|<%= f32 == 3.5 %>|<%= f64 > 3 %>|<%= robot.Count == 0 %>|<%= robots[0].Stats.Count == 0 %>|<%= counts["active"] == 0 %>|<%= values[0] == 0 %>|<%= counter() == 0 %>|<%= robot.MethodCount() == 0 %>|<%= robot.Count + 1 %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "true|true|true|true|true|true|true|false|true|true|true|true|true|true|true|true|true|1", out)
}

func Test_Render_Top_Level_Return(t *testing.T) {
	out, err := Render(`<% return "x" %>`, plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "x", out)
}

func Test_Render_Context_Value_Shadows_Builtin_Identifier(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"len": "shadow",
	})

	out, err := Render(`<%= len %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "shadow", out)
}

func Test_Render_Context_Function_Shadows_Builtin_Call(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"len": func() string {
			return "ctx"
		},
	})

	out, err := Render(`<%= len() %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "ctx", out)
}

func Test_Logical_Short_Circuit(t *testing.T) {
	out, err := Render(`<%= false && missing %>|<%= true || missing %>`, plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "false|true", out)
}

func Test_Render_Undefined_Equality_Does_Not_Error(t *testing.T) {
	out, err := Render(`<%= undefined == 3 %>|<%= 3 != unknown %>`, plush.NewContext())
	require.NoError(t, err)
	require.Equal(t, "false|true", out)
}

func Test_Render_Phase_11_Named_Function_Budget_Stats(t *testing.T) {
	costs := plush.ZeroCosts()
	costs.HelperCall = 1
	costs.FunctionCosts = map[string]int64{
		"expensive": 7,
		"Greet":     5,
		"Echo":      3,
	}
	costs.ObjectTraversal = 2

	budget := plush.NewBudgetWithCosts(100, costs)
	ctx := plush.NewContextWith(map[string]interface{}{
		"expensive": func() string { return "x" },
		"greeter":   vmBudgetGreeter{},
		"robot":     vmBudgetRobot{Name: vmEchoName("Bender")},
		"robots":    []vmBudgetRobot{{Name: vmEchoName("Fry")}},
	})
	ctx.WithBudget(budget)

	out, err := Render(`<%= expensive() %>|<%= greeter.Greet() %>|<%= robot.Name.Echo() %>|<%= robots[0].Name.Echo() %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "x|hi|Bender|Fry", out)

	stats := budget.Stats()
	require.Equal(t, int64(18), stats.FunctionCalls)
	require.Equal(t, int64(7), stats.ByFunction["expensive"])
	require.Equal(t, int64(5), stats.ByFunction["Greet"])
	require.Equal(t, int64(6), stats.ByFunction["Echo"])
	require.Equal(t, int64(10), stats.ObjectTraversals)
	require.Equal(t, stats.TotalUsed, stats.FunctionCalls+stats.ObjectTraversals)
}

func Test_Render_Phase_11_Named_Function_Budget_Exceeded(t *testing.T) {
	costs := plush.ZeroCosts()
	costs.FunctionCosts = map[string]int64{"expensive": 7}

	budget := plush.NewBudgetWithCosts(6, costs)
	ctx := plush.NewContextWith(map[string]interface{}{
		"expensive": func() string { return "x" },
	})
	ctx.WithBudget(budget)

	out, err := Render(`<%= expensive() %>`, ctx)
	require.ErrorIs(t, err, plush.ErrBudgetExceeded)
	require.Empty(t, out)

	stats := budget.Stats()
	require.Equal(t, int64(7), stats.FunctionCalls)
	require.Equal(t, int64(7), stats.ByFunction["expensive"])
	require.Equal(t, int64(7), stats.TotalUsed)
}

func Test_Render_Phase_11_User_Function_Budget_Name(t *testing.T) {
	costs := plush.ZeroCosts()
	costs.Assignment = 1
	costs.FunctionCosts = map[string]int64{"add": 9}

	budget := plush.NewBudgetWithCosts(100, costs)
	ctx := plush.NewContext()
	ctx.WithBudget(budget)

	out, err := Render(`<% let add = fn(x) { return x + 1 } %><%= add(2) %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "3", out)

	stats := budget.Stats()
	require.Equal(t, int64(9), stats.FunctionCalls)
	require.Equal(t, int64(9), stats.ByFunction["add"])
	require.Equal(t, int64(1), stats.Assignments)
	require.Equal(t, int64(10), stats.TotalUsed)
}

func Test_Render_Phase_11_Nested_Function_Budget_Names(t *testing.T) {
	costs := plush.ZeroCosts()
	costs.FunctionCosts = map[string]int64{
		"make":  2,
		"inner": 3,
		"greet": 5,
	}

	budget := plush.NewBudgetWithCosts(100, costs)
	ctx := plush.NewContextWith(map[string]interface{}{
		"greet": func() string { return "hi" },
	})
	ctx.WithBudget(budget)

	out, err := Render(`<% let make = fn() { let inner = fn() { return greet() }; return inner() } %><%= make() %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "hi", out)

	stats := budget.Stats()
	require.Equal(t, int64(10), stats.FunctionCalls)
	require.Equal(t, int64(2), stats.ByFunction["make"])
	require.Equal(t, int64(3), stats.ByFunction["inner"])
	require.Equal(t, int64(5), stats.ByFunction["greet"])
	require.Equal(t, int64(10), stats.TotalUsed)
}

func Test_Render_Hole_Statements(t *testing.T) {
	ctx := plush.NewContextWith(map[string]interface{}{
		"items": []string{"a", "b", "c"},
	})

	out, err := Render(`<%H "start" %><%H for (i,v) in items { %><%= v %><% } %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "<PLUSH_HOLE_0><PLUSH_HOLE_1>", out)

	ctx.Set(meta.TemplateFileKey, "holes.plush")
	out, err = Render(`<%H "start" %><%H for (i,v) in items { %><%= v %><% } %>`, ctx)
	require.NoError(t, err)
	require.Equal(t, "startabc", out)
}

func Test_Render_Phase_10_Hole_Skeleton_Records_Markers(t *testing.T) {
	program, err := parser.Parse(`<% let a = ["a", "b"] %><% a = a + "1" %><%= a %><%H "testing" %><%= a %><%H "sssss" %>`)
	require.NoError(t, err)

	comp := compiler.New()
	require.NoError(t, comp.Compile(program))

	machine := NewWithContext(comp.Bytecode(), plush.NewContext())
	require.NoError(t, machine.Run())

	skeleton := machine.Rendered()
	holes := machine.PunchHoles()
	require.Equal(t, `ab1<PLUSH_HOLE_0>ab1<PLUSH_HOLE_1>`, skeleton)
	require.Len(t, holes, 2)
	require.Equal(t, "<PLUSH_HOLE_0>", holes[0].MarkerName())
	require.Equal(t, `<%= "testing" %>`, holes[0].Input())
	require.Equal(t, 3, holes[0].Start())
	require.Equal(t, 17, holes[0].End())
	require.Empty(t, holes[0].Content())
	require.NoError(t, holes[0].Err())
	require.Equal(t, "<PLUSH_HOLE_1>", holes[1].MarkerName())
	require.Equal(t, `<%= "sssss" %>`, holes[1].Input())
	require.Equal(t, 20, holes[1].Start())
	require.Equal(t, 34, holes[1].End())
}

func Test_Render_Phase_10_Hole_Cache_Behavior(t *testing.T) {
	cache := inmemory.NewMemoryCache()
	plush.PlushCacheSetup(cache)

	ctx := plush.NewContextWith(map[string]interface{}{
		"myArray": []string{"a", "b"},
	})
	ctx.Set(meta.TemplateFileKey, "vm-cache-hole.plush")

	input := `<% let a = myArray %><% a = a + "1" %><%= a %><%H "testing" %><%= a %><%H "sssss" %>`
	out, err := Render(input, ctx)
	require.NoError(t, err)
	require.Equal(t, `ab1testingab1sssss`, out)

	cachedInput := `<% let a = myArray %><% a = a + "2" %><%= a %><%H "testing" %><%= a %><%H "sssss" %>`
	out, err = Render(cachedInput, ctx)
	require.NoError(t, err)
	require.Equal(t, `ab2testingab2sssss`, out)

	cache.Delete(plush.GenerateASTKey("vm-cache-hole.plush"))
	out, err = Render(cachedInput, ctx)
	require.NoError(t, err)
	require.Equal(t, `ab2testingab2sssss`, out)
}

func runVMTests(t *testing.T, tests []vmTestCase) {
	t.Helper()

	for _, tt := range tests {
		program, err := compiler.ParseScript(tt.input)
		require.NoError(t, err)

		comp := compiler.New()
		require.NoError(t, comp.Compile(program))

		machine := New(comp.Bytecode())
		require.NoError(t, machine.Run())

		testExpectedObject(t, tt.expected, machine.LastPoppedStackElem())
	}
}

func testExpectedObject(t *testing.T, expected interface{}, actual object.Object) {
	t.Helper()

	switch expected := expected.(type) {
	case int:
		testIntegerObject(t, int64(expected), actual)
	case string:
		testStringObject(t, expected, actual)
	case bool:
		testBooleanObject(t, expected, actual)
	case []int:
		testArrayObject(t, expected, actual)
	case map[object.HashKey]int:
		testHashObject(t, expected, actual)
	case *object.Null:
		require.Same(t, Null, actual)
	case *object.Error:
		result, ok := actual.(*object.Error)
		require.Truef(t, ok, "object is not Error. got=%T (%+v)", actual, actual)
		require.Equal(t, expected.Message, result.Message)
	default:
		require.Failf(t, "unsupported expected type", "unsupported expected type %T", expected)
	}
}

func testIntegerObject(t *testing.T, expected int64, actual object.Object) {
	t.Helper()

	result, ok := actual.(*object.Integer)
	require.Truef(t, ok, "object is not Integer. got=%T (%+v)", actual, actual)
	require.Equal(t, expected, result.Value)
}

func testBooleanObject(t *testing.T, expected bool, actual object.Object) {
	t.Helper()

	result, ok := actual.(*object.Boolean)
	require.Truef(t, ok, "object is not Boolean. got=%T (%+v)", actual, actual)
	require.Equal(t, expected, result.Value)
}

func testStringObject(t *testing.T, expected string, actual object.Object) {
	t.Helper()

	result, ok := actual.(*object.String)
	require.Truef(t, ok, "object is not String. got=%T (%+v)", actual, actual)
	require.Equal(t, expected, result.Value)
}

func testArrayObject(t *testing.T, expected []int, actual object.Object) {
	t.Helper()

	result, ok := actual.(*object.Array)
	require.Truef(t, ok, "object is not Array. got=%T (%+v)", actual, actual)
	require.Len(t, result.Elements, len(expected))
	for i, expectedElem := range expected {
		testIntegerObject(t, int64(expectedElem), result.Elements[i])
	}
}

func testHashObject(t *testing.T, expected map[object.HashKey]int, actual object.Object) {
	t.Helper()

	result, ok := actual.(*object.Hash)
	require.Truef(t, ok, "object is not Hash. got=%T (%+v)", actual, actual)
	require.Len(t, result.Pairs, len(expected))
	for expectedKey, expectedValue := range expected {
		pair, ok := result.Pairs[expectedKey]
		require.Truef(t, ok, "missing hash key %+v", expectedKey)
		testIntegerObject(t, int64(expectedValue), pair.Value)
	}
}

func firstCompiledFunction(t *testing.T, constants []object.Object) *object.CompiledFunction {
	t.Helper()

	for _, constant := range constants {
		if fn, ok := constant.(*object.CompiledFunction); ok {
			return fn
		}
	}
	require.Fail(t, "expected compiled function constant")
	return nil
}

func requireCallCacheEntry(t *testing.T, value interface{}) *callCacheEntry {
	t.Helper()

	entry, ok := value.(*callCacheEntry)
	require.Truef(t, ok, "expected call cache entry, got %T", value)
	require.NotNil(t, entry)
	return entry
}

func opcodePositions(instructions code.Instructions, target code.Opcode) []int {
	positions := []int{}
	for i := 0; i < len(instructions); {
		op := code.Opcode(instructions[i])
		if op == target {
			positions = append(positions, i)
		}
		def, err := code.Lookup(byte(op))
		if err != nil {
			i++
			continue
		}
		_, read := code.ReadOperands(def, instructions[i+1:])
		i += 1 + read
	}
	return positions
}

func callWritePairs(instructions code.Instructions) []int {
	positions := []int{}
	for i := 0; i < len(instructions); {
		op := code.Opcode(instructions[i])
		def, err := code.Lookup(byte(op))
		if err != nil {
			i++
			continue
		}
		_, read := code.ReadOperands(def, instructions[i+1:])
		next := i + 1 + read
		if op == code.OpCall && next < len(instructions) && code.Opcode(instructions[next]) == code.OpWrite {
			positions = append(positions, i)
		}
		i = next
	}
	return positions
}

func instructionContainsSequenceForVM(instructions code.Instructions, first, second code.Opcode) bool {
	for i := 0; i < len(instructions); {
		op := code.Opcode(instructions[i])
		def, err := code.Lookup(byte(op))
		if err != nil {
			i++
			continue
		}
		_, read := code.ReadOperands(def, instructions[i+1:])
		next := i + 1 + read
		if op == first && next < len(instructions) && code.Opcode(instructions[next]) == second {
			return true
		}
		i = next
	}
	return false
}
