package vm

import (
	"fmt"
	"html/template"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

func writeBuilderFastInt(out *strings.Builder, value int64) {
	var buf [20]byte
	out.Write(strconv.AppendInt(buf[:0], value, 10))
}

func writeBuilderFastUint(out *strings.Builder, value uint64) {
	var buf [20]byte
	out.Write(strconv.AppendUint(buf[:0], value, 10))
}

func writeBuilderFastFloat(out *strings.Builder, value float64, bitSize int) {
	var buf [32]byte
	out.Write(strconv.AppendFloat(buf[:0], value, 'g', -1, bitSize))
}

func writeFastPropertyOutput(out *strings.Builder, ctx hctx.Context, base interface{}, name string, access object.PropertyAccess, cacheSlot *object.InlineCacheSlot) error {
	if hash, ok := base.(*object.Hash); ok {
		key := (&object.String{Value: name}).HashKey()
		if pair, ok := hash.Pairs[key]; ok {
			writeFastGoValue(out, ctx, pair.Value)
		}
		return nil
	}

	rv, _, ok, err := fastPropertyReflectValue(base)
	if err != nil || !ok {
		return err
	}
	return writeFastPropertyReflectOutput(out, ctx, rv, name, access, cacheSlot)
}

func writeFastPropertyReflectOutput(out *strings.Builder, ctx hctx.Context, rv reflect.Value, name string, access object.PropertyAccess, cacheSlot *object.InlineCacheSlot) error {
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	entry := inlinePropertyEntry(cacheSlot, rv.Type(), name)
	if entry != nil && entry.writer != nil {
		written, err := entry.writer(out, ctx, rv, access, name)
		if err != nil || written {
			return err
		}
	}

	raw := interface{}(nil)
	if rv.CanInterface() {
		raw = rv.Interface()
	}
	value, err := fastPropertyValueFromReflect(rv, raw, name, access, entry)
	if err != nil {
		return err
	}
	writeFastGoValue(out, ctx, value)
	return nil
}

func fastPropertyValue(base interface{}, name string, access object.PropertyAccess, cacheSlot *object.InlineCacheSlot) (interface{}, error) {
	if hash, ok := base.(*object.Hash); ok {
		key := (&object.String{Value: name}).HashKey()
		if pair, ok := hash.Pairs[key]; ok {
			return pair.Value, nil
		}
		return nil, nil
	}

	rv, raw, ok, err := fastPropertyReflectValue(base)
	if err != nil || !ok {
		return nil, err
	}
	return fastPropertyValueFromReflect(rv, raw, name, access, inlinePropertyEntry(cacheSlot, rv.Type(), name))
}

func fastPropertyReflectValue(base interface{}) (reflect.Value, interface{}, bool, error) {
	if obj, ok := base.(object.Object); ok {
		if object.IsNull(obj) {
			return reflect.Value{}, nil, false, nil
		}
		base = object.ToGo(obj)
	}
	if base == nil {
		return reflect.Value{}, nil, false, nil
	}

	rv := reflect.ValueOf(base)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return reflect.Value{}, nil, false, nil
		}
		rv = rv.Elem()
	}
	return rv, base, true, nil
}

func fastPropertyValueFromReflect(rv reflect.Value, raw interface{}, name string, access object.PropertyAccess, entry *propertyInlineCacheEntry) (interface{}, error) {
	if entry == nil {
		return nil, propertyMissingError(access, raw, name)
	}
	if entry.reader != nil {
		return entry.reader(rv, access, name)
	}
	lookup := entry.lookup
	switch lookup.kind {
	case propertyLookupValueMethod:
		method := rv.Method(lookup.index)
		if method.IsValid() {
			return method.Interface(), nil
		}
	case propertyLookupPointerMethod:
		var method reflect.Value
		if rv.CanAddr() {
			method = rv.Addr().Method(lookup.index)
		} else {
			ptr := reflect.New(rv.Type())
			ptr.Elem().Set(rv)
			method = ptr.Method(lookup.index)
		}
		if method.IsValid() {
			return method.Interface(), nil
		}
	case propertyLookupField:
		if access.Method {
			return nil, propertyMissingError(access, raw, name)
		}
		field := rv.FieldByIndex(lookup.fieldIndex)
		return fastFieldValue(field, access, name)
	}

	if access.Method || rv.Kind() != reflect.Struct {
		return nil, propertyMissingError(access, raw, name)
	}
	return nil, propertyMissingError(access, raw, name)
}

func fastFieldValue(field reflect.Value, access object.PropertyAccess, name string) (interface{}, error) {
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil, nil
		}
		field = field.Elem()
	}
	if !field.CanInterface() {
		if access.Receiver != "" && access.Full != "" {
			return nil, fmt.Errorf("'%s'cannot return value obtained from unexported field or method '%s' (%s)", access.Receiver, name, access.Full)
		}
		return nil, fmt.Errorf("cannot return value obtained from unexported field or method '%s'", name)
	}
	return field.Interface(), nil
}

type fastOutputKind uint8

const (
	fastOutputString fastOutputKind = iota + 1
	fastOutputHTML
	fastOutputBool
	fastOutputInt
	fastOutputInt8
	fastOutputInt16
	fastOutputInt32
	fastOutputInt64
	fastOutputUint
	fastOutputUint8
	fastOutputUint16
	fastOutputUint32
	fastOutputUint64
	fastOutputUintptr
	fastOutputFloat
	fastOutputFloat64
)

type fastOutputCacheEntry struct {
	kind fastOutputKind
}

func writeFastBindingOutput(out *strings.Builder, ctx hctx.Context, value interface{}, cacheSlot *object.InlineCacheSlot) bool {
	if entry, ok := cacheSlot.Load().(*fastOutputCacheEntry); ok && entry != nil && entry.matches(value) {
		entry.write(out, value)
		return true
	}
	if entry := newFastOutputCacheEntry(value); entry != nil {
		cacheSlot.Store(entry)
		entry.write(out, value)
		return true
	}
	return writeFastGoValue(out, ctx, value)
}

func canWriteFastBindingOutput(value interface{}) bool {
	if value == nil {
		return true
	}
	if newFastOutputCacheEntry(value) != nil {
		return true
	}
	return canWriteFastGoValue(value)
}

func (e *fastOutputCacheEntry) matches(value interface{}) bool {
	if e == nil || value == nil {
		return false
	}
	switch e.kind {
	case fastOutputString:
		_, ok := value.(string)
		return ok
	case fastOutputHTML:
		_, ok := value.(template.HTML)
		return ok
	case fastOutputBool:
		_, ok := value.(bool)
		return ok
	case fastOutputInt:
		_, ok := value.(int)
		return ok
	case fastOutputInt8:
		_, ok := value.(int8)
		return ok
	case fastOutputInt16:
		_, ok := value.(int16)
		return ok
	case fastOutputInt32:
		_, ok := value.(int32)
		return ok
	case fastOutputInt64:
		_, ok := value.(int64)
		return ok
	case fastOutputUint:
		_, ok := value.(uint)
		return ok
	case fastOutputUint8:
		_, ok := value.(uint8)
		return ok
	case fastOutputUint16:
		_, ok := value.(uint16)
		return ok
	case fastOutputUint32:
		_, ok := value.(uint32)
		return ok
	case fastOutputUint64:
		_, ok := value.(uint64)
		return ok
	case fastOutputUintptr:
		_, ok := value.(uintptr)
		return ok
	case fastOutputFloat:
		_, ok := value.(float32)
		return ok
	case fastOutputFloat64:
		_, ok := value.(float64)
		return ok
	default:
		return false
	}
}

func (e *fastOutputCacheEntry) write(out *strings.Builder, value interface{}) {
	switch e.kind {
	case fastOutputString:
		writeFastEscapedString(out, value.(string))
	case fastOutputHTML:
		out.WriteString(string(value.(template.HTML)))
	case fastOutputBool:
		out.WriteString(strconv.FormatBool(value.(bool)))
	case fastOutputInt:
		writeBuilderFastInt(out, int64(value.(int)))
	case fastOutputInt8:
		writeBuilderFastInt(out, int64(value.(int8)))
	case fastOutputInt16:
		writeBuilderFastInt(out, int64(value.(int16)))
	case fastOutputInt32:
		writeBuilderFastInt(out, int64(value.(int32)))
	case fastOutputInt64:
		writeBuilderFastInt(out, value.(int64))
	case fastOutputUint:
		writeBuilderFastUint(out, uint64(value.(uint)))
	case fastOutputUint8:
		writeBuilderFastUint(out, uint64(value.(uint8)))
	case fastOutputUint16:
		writeBuilderFastUint(out, uint64(value.(uint16)))
	case fastOutputUint32:
		writeBuilderFastUint(out, uint64(value.(uint32)))
	case fastOutputUint64:
		writeBuilderFastUint(out, value.(uint64))
	case fastOutputUintptr:
		writeBuilderFastUint(out, uint64(value.(uintptr)))
	case fastOutputFloat:
		writeBuilderFastFloat(out, float64(value.(float32)), 32)
	case fastOutputFloat64:
		writeBuilderFastFloat(out, value.(float64), 64)
	}
}

func newFastOutputCacheEntry(value interface{}) *fastOutputCacheEntry {
	if value == nil {
		return nil
	}
	switch value.(type) {
	case string:
		return &fastOutputCacheEntry{kind: fastOutputString}
	case template.HTML:
		return &fastOutputCacheEntry{kind: fastOutputHTML}
	case bool:
		return &fastOutputCacheEntry{kind: fastOutputBool}
	case int:
		return &fastOutputCacheEntry{kind: fastOutputInt}
	case int8:
		return &fastOutputCacheEntry{kind: fastOutputInt8}
	case int16:
		return &fastOutputCacheEntry{kind: fastOutputInt16}
	case int32:
		return &fastOutputCacheEntry{kind: fastOutputInt32}
	case int64:
		return &fastOutputCacheEntry{kind: fastOutputInt64}
	case uint:
		return &fastOutputCacheEntry{kind: fastOutputUint}
	case uint8:
		return &fastOutputCacheEntry{kind: fastOutputUint8}
	case uint16:
		return &fastOutputCacheEntry{kind: fastOutputUint16}
	case uint32:
		return &fastOutputCacheEntry{kind: fastOutputUint32}
	case uint64:
		return &fastOutputCacheEntry{kind: fastOutputUint64}
	case uintptr:
		return &fastOutputCacheEntry{kind: fastOutputUintptr}
	case float32:
		return &fastOutputCacheEntry{kind: fastOutputFloat}
	case float64:
		return &fastOutputCacheEntry{kind: fastOutputFloat64}
	default:
		return nil
	}
}

func writeFastGoValue(out *strings.Builder, ctx hctx.Context, value interface{}) bool {
	switch value := value.(type) {
	case nil:
		return true
	case object.Object:
		return writeFastObject(out, ctx, value)
	case time.Time:
		writeFastTime(out, ctx, value)
		return true
	case *time.Time:
		if value != nil {
			writeFastTime(out, ctx, *value)
		}
		return true
	case interface{ Interface() interface{} }:
		return writeFastGoValue(out, ctx, value.Interface())
	case template.HTML:
		out.WriteString(string(value))
		return true
	case interface{ HTML() template.HTML }:
		out.WriteString(string(value.HTML()))
		return true
	case string:
		writeFastEscapedString(out, value)
		return true
	case bool:
		out.WriteString(strconv.FormatBool(value))
		return true
	case int:
		writeBuilderFastInt(out, int64(value))
		return true
	case int8:
		writeBuilderFastInt(out, int64(value))
		return true
	case int16:
		writeBuilderFastInt(out, int64(value))
		return true
	case int32:
		writeBuilderFastInt(out, int64(value))
		return true
	case int64:
		writeBuilderFastInt(out, value)
		return true
	case uint:
		writeBuilderFastUint(out, uint64(value))
		return true
	case uint8:
		writeBuilderFastUint(out, uint64(value))
		return true
	case uint16:
		writeBuilderFastUint(out, uint64(value))
		return true
	case uint32:
		writeBuilderFastUint(out, uint64(value))
		return true
	case uint64:
		writeBuilderFastUint(out, value)
		return true
	case uintptr:
		writeBuilderFastUint(out, uint64(value))
		return true
	case float32:
		writeBuilderFastFloat(out, float64(value), 32)
		return true
	case float64:
		writeBuilderFastFloat(out, value, 64)
		return true
	case fmt.Stringer:
		out.WriteString(value.String())
		return true
	case []string:
		for _, el := range value {
			writeFastEscapedString(out, el)
		}
		return true
	case []interface{}:
		for _, el := range value {
			if !writeFastGoValue(out, ctx, el) {
				return false
			}
		}
		return true
	default:
		rv := reflect.ValueOf(value)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			for i := 0; i < rv.Len(); i++ {
				writeFastGoValue(out, ctx, rv.Index(i).Interface())
			}
		}
		return true
	}
}

func canWriteFastGoValue(value interface{}) bool {
	switch value := value.(type) {
	case nil:
		return true
	case object.Object:
		return canWriteFastObject(value)
	case time.Time, *time.Time, template.HTML, string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64, fmt.Stringer:
		return true
	case interface{ Interface() interface{} }:
		return canWriteFastGoValue(value.Interface())
	case interface{ HTML() template.HTML }:
		return true
	case []string:
		return true
	case []interface{}:
		for _, el := range value {
			if !canWriteFastGoValue(el) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func writeFastObject(out *strings.Builder, ctx hctx.Context, obj object.Object) bool {
	if obj == nil || object.IsNull(obj) {
		return true
	}

	switch obj := obj.(type) {
	case *object.Array:
		for _, el := range obj.Elements {
			if !writeFastObject(out, ctx, el) {
				return false
			}
		}
		return true
	case *object.Integer:
		writeBuilderFastInt(out, obj.Value)
		return true
	case *object.Float:
		writeBuilderFastFloat(out, obj.Value, 64)
		return true
	case *object.Boolean:
		out.WriteString(strconv.FormatBool(obj.Value))
		return true
	case *object.String:
		writeFastEscapedString(out, obj.Value)
		return true
	case *object.Native:
		return writeFastGoValue(out, ctx, obj.Value)
	default:
		return false
	}
}

func canWriteFastObject(obj object.Object) bool {
	if obj == nil || object.IsNull(obj) {
		return true
	}
	switch obj := obj.(type) {
	case *object.Array:
		for _, el := range obj.Elements {
			if !canWriteFastObject(el) {
				return false
			}
		}
		return true
	case *object.Integer, *object.Float, *object.Boolean, *object.String:
		return true
	case *object.Native:
		return canWriteFastGoValue(obj.Value)
	default:
		return false
	}
}

func writeFastTime(out *strings.Builder, ctx hctx.Context, value time.Time) {
	if ctx != nil {
		if dtf, ok := ctx.Value("TIME_FORMAT").(string); ok {
			out.WriteString(value.Format(dtf))
			return
		}
	}
	out.WriteString(value.Format(plush.DefaultTimeFormat))
}

func writeFastEscapedString(out *strings.Builder, value string) {
	if !strings.ContainsAny(value, `&<>'"`) {
		out.WriteString(value)
		return
	}
	out.WriteString(template.HTMLEscapeString(value))
}
