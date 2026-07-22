package vm

import (
	"html/template"
	"reflect"
	"strings"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/stretchr/testify/require"
)

type vmFastPropertyChild struct {
	Name string
}

func (c vmFastPropertyChild) Echo() string {
	return c.Name + "!"
}

type vmFastPropertyUser struct {
	Name   string
	Count  int32
	Markup template.HTML
	Child  *vmFastPropertyChild
	hidden string
}

func (u vmFastPropertyUser) Echo() string {
	return u.Name + "!"
}

func (u *vmFastPropertyUser) PointerEcho() string {
	return u.Name + "?"
}

func Test_VM_Fast_Condition_Operand_Branches(t *testing.T) {
	var iface interface{} = true
	var nilIface interface{} = (*vmFastPropertyUser)(nil)

	boolValue, ok := (fastConditionOperandValue{raw: true}).boolValue()
	require.True(t, ok)
	require.True(t, boolValue)

	boolValue, ok = (fastConditionOperandValue{raw: object.FalseObject}).boolValue()
	require.True(t, ok)
	require.False(t, boolValue)

	_, ok = (fastConditionOperandValue{raw: "true"}).boolValue()
	require.False(t, ok)

	boolValue, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(iface)}).boolValue()
	require.True(t, ok)
	require.True(t, boolValue)

	boolValue, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(&iface).Elem()}).boolValue()
	require.True(t, ok)
	require.True(t, boolValue)

	var nilConcreteIface interface{}
	_, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(&nilConcreteIface).Elem()}).boolValue()
	require.False(t, ok)

	_, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(nilIface)}).boolValue()
	require.False(t, ok)

	stringValue, ok := (fastConditionOperandValue{raw: "<name>"}).stringValue()
	require.True(t, ok)
	require.Equal(t, "<name>", stringValue)

	stringValue, ok = (fastConditionOperandValue{raw: &object.String{Value: "object"}}).stringValue()
	require.True(t, ok)
	require.Equal(t, "object", stringValue)

	var stringIface interface{} = "iface"
	stringValue, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(stringIface)}).stringValue()
	require.True(t, ok)
	require.Equal(t, "iface", stringValue)

	stringValue, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(&stringIface).Elem()}).stringValue()
	require.True(t, ok)
	require.Equal(t, "iface", stringValue)

	_, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(&nilConcreteIface).Elem()}).stringValue()
	require.False(t, ok)

	_, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(3)}).stringValue()
	require.False(t, ok)

	numericValue, ok := (fastConditionOperandValue{raw: int32(7)}).numericValue()
	require.True(t, ok)
	require.Equal(t, int64(7), numericValue.i)

	numericValue, ok = (fastConditionOperandValue{raw: &object.Float{Value: 1.5}}).numericValue()
	require.True(t, ok)
	require.Equal(t, 1.5, numericValue.f)

	numericValue, ok = (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(uint32(8))}).numericValue()
	require.True(t, ok)
	require.Equal(t, uint64(8), numericValue.u)

	require.Equal(t, "go", (fastConditionOperandValue{raw: &object.String{Value: "go"}}).goValue())
	require.Equal(t, "iface", (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(stringIface)}).goValue())
	require.Nil(t, (fastConditionOperandValue{hasReflect: true, reflect: reflect.Value{}}).goValue())

	require.True(t, (fastConditionOperandValue{raw: nil}).isNil())
	require.True(t, (fastConditionOperandValue{raw: object.NullObject}).isNil())
	require.True(t, (fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(nilIface)}).isNil())
	require.False(t, (fastConditionOperandValue{raw: "value"}).isNil())

	require.True(t, isTruthyFastConditionOperand(fastConditionOperandValue{raw: &object.String{Value: "x"}}))
	require.False(t, isTruthyFastConditionOperand(fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf("")}))

	require.Equal(t, "string", fastConditionTypeName(fastConditionOperandValue{raw: "x"}))
	require.Equal(t, "int32", fastConditionTypeName(fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf(int32(1))}))
}

func Test_VM_Fast_Condition_Comparisons(t *testing.T) {
	_, ok := numericValueFromGo(nil)
	require.False(t, ok)
	_, ok = numericValueFromGo("nope")
	require.False(t, ok)
	value, ok := numericValueFromGo(uintptr(9))
	require.True(t, ok)
	require.Equal(t, numericUnsigned, value.kind)
	require.Equal(t, uint64(9), value.u)
	value, ok = numericValueFromGo(float32(1.25))
	require.True(t, ok)
	require.Equal(t, numericFloat, value.kind)
	require.InDelta(t, 1.25, value.f, 0.0001)

	equal, err := compareFastConditionEquality(
		"==",
		fastConditionOperandValue{raw: uint32(0)},
		fastConditionOperandValue{raw: &object.Integer{Value: 0}},
	)
	require.NoError(t, err)
	require.True(t, equal)

	equal, err = compareFastConditionEquality(
		"==",
		fastConditionOperandValue{raw: true},
		fastConditionOperandValue{raw: "truthy"},
	)
	require.NoError(t, err)
	require.True(t, equal)

	equal, err = compareFastConditionEquality(
		"==",
		fastConditionOperandValue{raw: &object.String{Value: "same"}},
		fastConditionOperandValue{hasReflect: true, reflect: reflect.ValueOf("same")},
	)
	require.NoError(t, err)
	require.True(t, equal)

	_, err = compareFastConditionEquality(
		"!=",
		fastConditionOperandValue{raw: uint32(0)},
		fastConditionOperandValue{raw: "0"},
	)
	require.ErrorContains(t, err, "unable to operate (!=)")

	equal, err = compareFastConditionEquality(
		"==",
		fastConditionOperandValue{raw: []int{1, 2}},
		fastConditionOperandValue{raw: []int{1, 2}},
	)
	require.NoError(t, err)
	require.True(t, equal)

	ordered, err := compareFastConditionOrdered(
		code.OpGreaterEqual,
		fastConditionOperandValue{raw: int32(5)},
		fastConditionOperandValue{raw: uint64(5)},
	)
	require.NoError(t, err)
	require.True(t, ordered)

	ordered, err = compareFastConditionOrdered(
		code.OpGreaterThan,
		fastConditionOperandValue{raw: "z"},
		fastConditionOperandValue{raw: "a"},
	)
	require.NoError(t, err)
	require.True(t, ordered)

	ordered, err = compareFastConditionOrdered(
		code.OpGreaterEqual,
		fastConditionOperandValue{raw: "same"},
		fastConditionOperandValue{raw: "same"},
	)
	require.NoError(t, err)
	require.True(t, ordered)

	ordered, err = compareFloatOrdered(code.OpGreaterThan, 2, 1)
	require.NoError(t, err)
	require.True(t, ordered)

	ordered, err = compareFloatOrdered(code.OpGreaterEqual, 2, 2)
	require.NoError(t, err)
	require.True(t, ordered)

	_, err = compareFastConditionOrdered(
		code.OpGreaterThan,
		fastConditionOperandValue{raw: float64(1.5)},
		fastConditionOperandValue{raw: "x"},
	)
	require.ErrorContains(t, err, "unable to operate (>)")

	_, err = compareFastConditionOrdered(
		code.OpAdd,
		fastConditionOperandValue{raw: "a"},
		fastConditionOperandValue{raw: "b"},
	)
	require.ErrorContains(t, err, "unknown ordered comparison")

	_, err = compareFloatOrdered(code.OpAdd, 1, 2)
	require.ErrorContains(t, err, "unknown ordered comparison")
}

func Test_VM_Fast_Struct_Loop_Element_Type_Branches(t *testing.T) {
	elemType, ok := fastStructLoopElementType(reflect.TypeOf([]vmFastPropertyUser{}))
	require.True(t, ok)
	require.Equal(t, reflect.TypeOf(vmFastPropertyUser{}), elemType)

	elemType, ok = fastStructLoopElementType(reflect.TypeOf([]*vmFastPropertyUser{}))
	require.True(t, ok)
	require.Equal(t, reflect.TypeOf(vmFastPropertyUser{}), elemType)

	_, ok = fastStructLoopElementType(reflect.TypeOf([]string{}))
	require.False(t, ok)

	_, ok = fastStructLoopElementType(reflect.TypeOf(map[string]vmFastPropertyUser{}))
	require.False(t, ok)
}

func Test_VM_Fast_Property_Value_And_Output_Branches(t *testing.T) {
	ctx := plush.NewContext()
	user := vmFastPropertyUser{
		Name:   "<mido>",
		Count:  7,
		Markup: template.HTML("<b>safe</b>"),
		Child:  &vmFastPropertyChild{Name: "kid"},
		hidden: "private",
	}
	access := object.PropertyAccess{Receiver: "user", Full: "user.Name"}
	var nameSlot object.InlineCacheSlot
	var countSlot object.InlineCacheSlot
	var echoSlot object.InlineCacheSlot
	var pointerEchoSlot object.InlineCacheSlot
	var markupSlot object.InlineCacheSlot
	var missingSlot object.InlineCacheSlot

	value, err := fastPropertyValue(user, "Name", access, &nameSlot)
	require.NoError(t, err)
	require.Equal(t, "<mido>", value)

	value, err = fastPropertyValue(&user, "Count", object.PropertyAccess{Receiver: "user", Full: "user.Count"}, &countSlot)
	require.NoError(t, err)
	require.Equal(t, int32(7), value)

	value, err = fastPropertyValue(user, "Echo", object.PropertyAccess{Receiver: "user", Full: "user.Echo()", Method: true}, &echoSlot)
	require.NoError(t, err)
	require.IsType(t, func() string { return "" }, value)

	value, err = fastPropertyValue(user, "PointerEcho", object.PropertyAccess{Receiver: "user", Full: "user.PointerEcho()", Method: true}, &pointerEchoSlot)
	require.NoError(t, err)
	require.IsType(t, func() string { return "" }, value)

	value, err = fastPropertyValue((*vmFastPropertyUser)(nil), "Name", access, &nameSlot)
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = fastPropertyValue(object.NullObject, "Name", access, &nameSlot)
	require.NoError(t, err)
	require.Nil(t, value)

	hashKey := (&object.String{Value: "Name"}).HashKey()
	hash := &object.Hash{Pairs: map[object.HashKey]object.HashPair{
		hashKey: {Key: &object.String{Value: "Name"}, Value: &object.String{Value: "<hash>"}},
	}}
	value, err = fastPropertyValue(hash, "Name", access, &nameSlot)
	require.NoError(t, err)
	require.Equal(t, &object.String{Value: "<hash>"}, value)

	value, err = fastPropertyValue(hash, "Missing", access, &missingSlot)
	require.NoError(t, err)
	require.Nil(t, value)

	_, err = fastPropertyValue(user, "Name", object.PropertyAccess{Receiver: "user", Full: "user.Name()", Method: true}, &nameSlot)
	require.ErrorContains(t, err, "does not have a method")

	_, err = fastPropertyValue(user, "Missing", object.PropertyAccess{Receiver: "user", Full: "user.Missing"}, &missingSlot)
	require.ErrorContains(t, err, "does not have a field or method")

	var out strings.Builder
	require.NoError(t, writeFastPropertyOutput(&out, ctx, hash, "Name", access, &nameSlot))
	require.Equal(t, "&lt;hash&gt;", out.String())

	out.Reset()
	require.NoError(t, writeFastPropertyOutput(&out, ctx, user, "Name", access, &nameSlot))
	require.Equal(t, "&lt;mido&gt;", out.String())

	out.Reset()
	require.NoError(t, writeFastPropertyOutput(&out, ctx, user, "Markup", object.PropertyAccess{Receiver: "user", Full: "user.Markup"}, &markupSlot))
	require.Equal(t, "<b>safe</b>", out.String())

	out.Reset()
	require.NoError(t, writeFastPropertyOutput(&out, ctx, (*vmFastPropertyUser)(nil), "Name", access, &nameSlot))
	require.Empty(t, out.String())

	out.Reset()
	require.NoError(t, writeFastPropertyOutput(&out, ctx, hash, "Missing", access, &missingSlot))
	require.Empty(t, out.String())

	entry := inlinePropertyEntry(nil, reflect.TypeOf(user), "Name")
	value, err = fastPropertyValueFromReflect(reflect.ValueOf(user), user, "Name", access, entry)
	require.NoError(t, err)
	require.Equal(t, "<mido>", value)

	_, err = fastPropertyValueFromReflect(reflect.ValueOf(user), user, "Missing", access, nil)
	require.ErrorContains(t, err, "does not have a field or method")

	_, err = fastFieldValue(reflect.ValueOf(user).FieldByName("hidden"), object.PropertyAccess{Receiver: "user", Full: "user.hidden"}, "hidden")
	require.ErrorContains(t, err, "unexported field")
}

func Test_VM_Fast_Output_Cache_Entry_Branches(t *testing.T) {
	ctx := plush.NewContext()
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"string", "<x>", "&lt;x&gt;"},
		{"html", template.HTML("<b>"), "<b>"},
		{"bool", true, "true"},
		{"int", int(-1), "-1"},
		{"int8", int8(-2), "-2"},
		{"int16", int16(-3), "-3"},
		{"int32", int32(-4), "-4"},
		{"int64", int64(-5), "-5"},
		{"uint", uint(1), "1"},
		{"uint8", uint8(2), "2"},
		{"uint16", uint16(3), "3"},
		{"uint32", uint32(4), "4"},
		{"uint64", uint64(5), "5"},
		{"uintptr", uintptr(6), "6"},
		{"float32", float32(1.5), "1.5"},
		{"float64", float64(2.5), "2.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := newFastOutputCacheEntry(tt.value)
			require.NotNil(t, entry)
			require.True(t, entry.matches(tt.value))
			require.False(t, entry.matches(nil))

			var out strings.Builder
			require.True(t, writeFastBindingOutput(&out, ctx, tt.value, &object.InlineCacheSlot{}))
			require.Equal(t, tt.expected, out.String())
		})
	}

	require.Nil(t, newFastOutputCacheEntry(struct{}{}))
	require.True(t, canWriteFastBindingOutput(nil))
	require.True(t, canWriteFastBindingOutput(struct{}{}))

	var out strings.Builder
	require.True(t, writeFastBindingOutput(&out, ctx, nil, &object.InlineCacheSlot{}))
	require.Empty(t, out.String())
}

func Test_VM_Execution_Comparison_Helper_Branches(t *testing.T) {
	equal, err := compareEquality("==", object.NullObject, Null)
	require.NoError(t, err)
	require.True(t, equal)

	equal, err = compareEquality("==", &object.Array{Elements: []object.Object{&object.String{Value: "x"}}}, &object.Array{Elements: []object.Object{&object.String{Value: "x"}}})
	require.NoError(t, err)
	require.True(t, equal)

	equal, err = compareEquality("==", &object.Native{Value: uint32(0)}, &object.Integer{Value: 0})
	require.NoError(t, err)
	require.True(t, equal)

	equal, err = compareEquality("==", object.TrueObject, &object.String{Value: "x"})
	require.NoError(t, err)
	require.True(t, equal)

	equal, err = compareEquality("==", &object.String{Value: "7"}, &object.Integer{Value: 7})
	require.NoError(t, err)
	require.True(t, equal)

	_, err = compareEquality("!=", &object.Integer{Value: 1}, &object.String{Value: "1"})
	require.ErrorContains(t, err, "unable to operate (!=)")

	ordered, err := compareOrdered(code.OpGreaterEqual, &object.Native{Value: int32(2)}, &object.Integer{Value: 2})
	require.NoError(t, err)
	require.True(t, ordered)

	ordered, err = compareOrdered(code.OpGreaterThan, &object.String{Value: "b"}, &object.String{Value: "a"})
	require.NoError(t, err)
	require.True(t, ordered)

	_, err = compareOrdered(code.OpGreaterThan, &object.Integer{Value: 1}, &object.String{Value: "x"})
	require.ErrorContains(t, err, "unable to operate")

	_, err = compareOrdered(code.OpAdd, &object.String{Value: "a"}, &object.String{Value: "b"})
	require.ErrorContains(t, err, "unknown ordered comparison")

	require.Equal(t, "bool", plushTypeName(object.TrueObject))
	require.Equal(t, "int", plushTypeName(&object.Integer{Value: 1}))
	require.Equal(t, "float64", plushTypeName(&object.Float{Value: 1}))
	require.Equal(t, "string", plushTypeName(&object.String{Value: "x"}))
	require.Equal(t, "<nil>", plushTypeName(object.NullObject))
	require.Equal(t, "<nil>", plushTypeName(&object.Native{}))
	require.Equal(t, "uint32", plushTypeName(&object.Native{Value: uint32(1)}))
	require.Equal(t, "[]interface {}", plushTypeName(&object.Array{}))
}
