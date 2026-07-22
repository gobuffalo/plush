package object

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"html/template"
	"math"
	"reflect"
	"strings"
	"sync/atomic"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/ast"
)

type BuiltinFunction func(args ...Object) Object

type ObjectType string

const (
	NULL_OBJ  = "NULL"
	ERROR_OBJ = "ERROR"

	INTEGER_OBJ = "INTEGER"
	FLOAT_OBJ   = "FLOAT"
	BOOLEAN_OBJ = "BOOLEAN"
	STRING_OBJ  = "STRING"

	BUILTIN_OBJ  = "BUILTIN"
	FUNCTION_OBJ = "FUNCTION"

	ARRAY_OBJ = "ARRAY"
	HASH_OBJ  = "HASH"

	COMPILED_FUNCTION_OBJ = "COMPILED_FUNCTION"
	CLOSURE_OBJ           = "CLOSURE"

	NATIVE_OBJ  = "NATIVE"
	CONTROL_OBJ = "CONTROL"
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) HashKey() HashKey {
	return HashKey{Type: i.Type(), Value: uint64(i.Value)}
}

type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return fmt.Sprintf("%v", f.Value) }

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) HashKey() HashKey {
	if b.Value {
		return HashKey{Type: b.Type(), Value: 1}
	}
	return HashKey{Type: b.Type(), Value: 0}
}

type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "null" }

var (
	NullObject  = &Null{}
	TrueObject  = &Boolean{Value: true}
	FalseObject = &Boolean{Value: false}
)

type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }

type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }
func (s *String) HashKey() HashKey {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s.Value))

	return HashKey{Type: s.Type(), Value: h.Sum64()}
}

type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }

type FunctionLiteral struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
}

func (f *FunctionLiteral) Type() ObjectType { return FUNCTION_OBJ }
func (f *FunctionLiteral) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range f.Parameters {
		params = append(params, p.String())
	}

	out.WriteString("fn")
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	if f.Body != nil {
		out.WriteString(f.Body.String())
	}
	out.WriteString("\n}")

	return out.String()
}

type Array struct {
	Elements []Object
}

func (ao *Array) Type() ObjectType { return ARRAY_OBJ }
func (ao *Array) Inspect() string {
	elements := make([]string, 0, len(ao.Elements))
	for _, e := range ao.Elements {
		elements = append(elements, e.Inspect())
	}

	return "[" + strings.Join(elements, ", ") + "]"
}

type HashKey struct {
	Type  ObjectType
	Value uint64
}

type Hashable interface {
	HashKey() HashKey
}

type HashPair struct {
	Key   Object
	Value Object
}

type Hash struct {
	Pairs map[HashKey]HashPair
}

func (h *Hash) Type() ObjectType { return HASH_OBJ }
func (h *Hash) Inspect() string {
	var out bytes.Buffer

	pairs := []string{}
	for _, pair := range h.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s",
			pair.Key.Inspect(), pair.Value.Inspect()))
	}

	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")

	return out.String()
}

type CompiledFunction struct {
	Instructions   code.Instructions
	CallNames      map[int]string
	LocalNames     map[int]string
	LineNumbers    map[int]int
	Properties     map[int]PropertyAccess
	PropertyCaches []InlineCacheSlot
	CallCaches     []InlineCacheSlot
	NumLocals      int
	NumParameters  int
}

func (cf *CompiledFunction) Type() ObjectType { return COMPILED_FUNCTION_OBJ }
func (cf *CompiledFunction) Inspect() string {
	return fmt.Sprintf("CompiledFunction[%p]", cf)
}

type PropertyAccess struct {
	Receiver string
	Full     string
	Method   bool
}

type InlineCacheSlot struct {
	value atomic.Value
}

func NewInlineCacheSlots(size int) []InlineCacheSlot {
	if size <= 0 {
		return nil
	}
	return make([]InlineCacheSlot, size)
}

func (s *InlineCacheSlot) Load() interface{} {
	if s == nil {
		return nil
	}
	return s.value.Load()
}

func (s *InlineCacheSlot) Store(value interface{}) {
	if s == nil || value == nil {
		return
	}
	s.value.Store(value)
}

type Closure struct {
	Fn   *CompiledFunction
	Free []Object
}

func (c *Closure) Type() ObjectType { return CLOSURE_OBJ }
func (c *Closure) Inspect() string  { return fmt.Sprintf("Closure[%p]", c) }

type Native struct {
	Value interface{}
}

func (n *Native) Type() ObjectType { return NATIVE_OBJ }
func (n *Native) Inspect() string  { return fmt.Sprint(n.Value) }

type ControlKind string

const (
	ControlBreak    ControlKind = "break"
	ControlContinue ControlKind = "continue"
)

type Control struct {
	Kind  ControlKind
	Value []Object
}

func (c *Control) Type() ObjectType { return CONTROL_OBJ }
func (c *Control) Inspect() string  { return string(c.Kind) }

func NewError(format string, a ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}

func Wrap(v interface{}) Object {
	switch v := v.(type) {
	case nil:
		return NullObject
	case Object:
		return v
	case int:
		return &Integer{Value: int64(v)}
	case int8:
		return &Integer{Value: int64(v)}
	case int16:
		return &Integer{Value: int64(v)}
	case int32:
		return &Integer{Value: int64(v)}
	case int64:
		return &Integer{Value: v}
	case uint:
		if uint64(v) <= uint64(math.MaxInt64) {
			return &Integer{Value: int64(v)}
		}
		return &Native{Value: v}
	case uint8:
		return &Integer{Value: int64(v)}
	case uint16:
		return &Integer{Value: int64(v)}
	case uint32:
		return &Integer{Value: int64(v)}
	case uint64:
		if v <= uint64(math.MaxInt64) {
			return &Integer{Value: int64(v)}
		}
		return &Native{Value: v}
	case float32:
		return &Float{Value: float64(v)}
	case float64:
		return &Float{Value: v}
	case bool:
		if v {
			return TrueObject
		}
		return FalseObject
	case string:
		return &String{Value: v}
	case []Object:
		return &Array{Elements: v}
	case []interface{}:
		elements := make([]Object, 0, len(v))
		for _, el := range v {
			elements = append(elements, Wrap(el))
		}
		return &Array{Elements: elements}
	default:
		return wrapReflect(v)
	}
}

func wrapReflect(v interface{}) Object {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return NullObject
	}
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return NullObject
	}

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		return &Native{Value: v}
	case reflect.Map:
		return &Native{Value: v}
	default:
		return &Native{Value: v}
	}
}

func ToGo(obj Object) interface{} {
	switch obj := obj.(type) {
	case nil:
		return nil
	case *Null:
		return nil
	case *Integer:
		return int(obj.Value)
	case *Float:
		return obj.Value
	case *Boolean:
		return obj.Value
	case *String:
		return obj.Value
	case *Array:
		out := make([]interface{}, 0, len(obj.Elements))
		for _, el := range obj.Elements {
			out = append(out, ToGo(el))
		}
		return out
	case *Hash:
		out := map[string]interface{}{}
		for _, pair := range obj.Pairs {
			out[hashKeyString(pair.Key)] = ToGo(pair.Value)
		}
		return out
	case *Native:
		return obj.Value
	default:
		return obj
	}
}

func hashKeyString(key Object) string {
	switch key := key.(type) {
	case *String:
		return key.Value
	case *Integer:
		return key.Inspect()
	case *Boolean:
		return key.Inspect()
	default:
		return key.Inspect()
	}
}

func IsNull(obj Object) bool {
	if obj == nil {
		return true
	}
	_, ok := obj.(*Null)
	return ok
}

func IsHTML(obj Object) bool {
	_, ok := ToGo(obj).(template.HTML)
	return ok
}
