package vm

import (
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/object"
)

func (vm *VM) stringConstant(index int) string {
	if index < 0 || index >= len(vm.constants) {
		return ""
	}
	if str, ok := vm.constants[index].(*object.String); ok {
		return str.Value
	}
	return vm.constants[index].Inspect()
}

func (vm *VM) htmlConstantString(index int) string {
	if index < 0 || index >= len(vm.constants) {
		return ""
	}
	switch obj := vm.constants[index].(type) {
	case *object.String:
		return obj.Value
	case *object.Native:
		if html, ok := obj.Value.(template.HTML); ok {
			return string(html)
		}
	}
	return vm.constants[index].Inspect()
}

func (vm *VM) renderObject(obj object.Object) string {
	var out strings.Builder
	if size := vm.estimatedRenderedObjectSize(obj); size > 0 {
		out.Grow(size)
	}
	vm.writeObject(&out, obj)
	return out.String()
}

func (vm *VM) estimatedRenderedObjectsSize(objects []object.Object) int {
	total := 0
	for _, obj := range objects {
		total += vm.estimatedRenderedObjectSize(obj)
	}
	return total
}

func (vm *VM) estimatedRenderedObjectSize(obj object.Object) int {
	if obj == nil || object.IsNull(obj) {
		return 0
	}

	switch obj := obj.(type) {
	case *object.Array:
		return vm.estimatedRenderedObjectsSize(obj.Elements)
	case *object.Integer:
		return 20
	case *object.Float:
		return 24
	case *object.Boolean:
		return 5
	case *object.String:
		return len(obj.Value)
	case *object.Native:
		return vm.estimatedRenderedGoValueSize(obj.Value)
	default:
		return len(obj.Inspect())
	}
}

func (vm *VM) estimatedRenderedGoValueSize(value interface{}) int {
	switch value := value.(type) {
	case nil:
		return 0
	case vmHole:
		return len(plush.PunchHoleMarkerName(0))
	case time.Time:
		if vm.ctx != nil {
			if dtf, ok := vm.ctx.Value("TIME_FORMAT").(string); ok {
				return len(value.Format(dtf))
			}
		}
		return len(value.Format(plush.DefaultTimeFormat))
	case *time.Time:
		if value == nil {
			return 0
		}
		return vm.estimatedRenderedGoValueSize(*value)
	case template.HTML:
		return len(value)
	case string:
		return len(value)
	case bool:
		return 5
	case uint, uint8, uint16, uint32, uint64, int, int8, int16, int32, int64:
		return 20
	case float32, float64:
		return 24
	case []string:
		total := 0
		for _, el := range value {
			total += len(el)
		}
		return total
	case []interface{}:
		total := 0
		for _, el := range value {
			total += vm.estimatedRenderedGoValueSize(el)
		}
		return total
	default:
		rv := reflect.ValueOf(value)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			total := 0
			for i := 0; i < rv.Len(); i++ {
				total += vm.estimatedRenderedGoValueSize(rv.Index(i).Interface())
			}
			return total
		}
	}
	return 0
}

func (vm *VM) writeObject(out *strings.Builder, obj object.Object) {
	if obj == nil || object.IsNull(obj) {
		return
	}

	switch obj := obj.(type) {
	case *object.Array:
		for _, el := range obj.Elements {
			vm.writeObject(out, el)
		}
		return
	case *object.Integer, *object.Float, *object.Boolean:
		out.WriteString(template.HTMLEscaper(obj.Inspect()))
		return
	case *object.String:
		out.WriteString(template.HTMLEscapeString(obj.Value))
		return
	}

	vm.writeGoValue(out, object.ToGo(obj))
}

func (vm *VM) writeGoValue(out *strings.Builder, value interface{}) {
	switch t := value.(type) {
	case vmHole:
		if vm.holes == nil {
			holes := []plush.HoleMarker{}
			vm.holes = &holes
		}
		markerName := plush.PunchHoleMarkerName(len(*vm.holes))
		start, end := out.Len(), out.Len()+len(markerName)
		if vm.deferHolePositions {
			start, end = -1, -1
		}
		*vm.holes = append(*vm.holes, plush.NewHoleMarker(markerName, t.input, start, end))
		out.WriteString(markerName)
	case nil:
		return
	case object.Object:
		vm.writeObject(out, t)
	case time.Time:
		if vm.ctx != nil {
			if dtf, ok := vm.ctx.Value("TIME_FORMAT").(string); ok {
				out.WriteString(t.Format(dtf))
				return
			}
		}
		out.WriteString(t.Format(plush.DefaultTimeFormat))
	case *time.Time:
		if t != nil {
			vm.writeGoValue(out, *t)
		}
	case interface{ Interface() interface{} }:
		vm.writeGoValue(out, t.Interface())
	case template.HTML:
		out.WriteString(string(t))
	case interface{ HTML() template.HTML }:
		out.WriteString(string(t.HTML()))
	case string:
		out.WriteString(template.HTMLEscapeString(t))
	case bool:
		out.WriteString(template.HTMLEscaper(t))
	case uint, uint8, uint16, uint32, uint64, int, int8, int16, int32, int64, float32, float64:
		out.WriteString(fmt.Sprint(t))
	case fmt.Stringer:
		out.WriteString(t.String())
	case []string:
		for _, el := range t {
			vm.writeGoValue(out, el)
		}
	case []interface{}:
		for _, el := range t {
			vm.writeGoValue(out, el)
		}
	default:
		rv := reflect.ValueOf(value)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			for i := 0; i < rv.Len(); i++ {
				vm.writeGoValue(out, rv.Index(i).Interface())
			}
			return
		}
	}
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	}
	return False
}

func isTruthy(obj object.Object) bool {
	if obj == nil {
		return false
	}

	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return false
	case *object.String:
		return obj.Value != ""
	case *object.Native:
		if obj.Value == nil {
			return false
		}
		rv := reflect.ValueOf(obj.Value)
		if rv.Kind() == reflect.Ptr && rv.IsNil() {
			return false
		}
		return true
	default:
		return true
	}
}

func isTruthyFastValue(value interface{}) bool {
	switch value := value.(type) {
	case nil:
		return false
	case object.Object:
		return isTruthy(value)
	case bool:
		return value
	case string:
		return value != ""
	default:
		rv := reflect.ValueOf(value)
		if rv.IsValid() && rv.Kind() == reflect.Ptr && rv.IsNil() {
			return false
		}
		return true
	}
}

func toInt(obj object.Object) (int64, bool) {
	value, ok := numericValueFromObject(obj)
	if !ok {
		return 0, false
	}
	return value.int64()
}
