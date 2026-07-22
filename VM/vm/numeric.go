package vm

import (
	"fmt"
	"math"
	"reflect"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
)

type numericKind int

const (
	numericSigned numericKind = iota
	numericUnsigned
	numericFloat
)

type numericValue struct {
	kind numericKind
	i    int64
	u    uint64
	f    float64
}

func numericValueFromObject(obj object.Object) (numericValue, bool) {
	switch obj := obj.(type) {
	case *object.Integer:
		return numericValue{kind: numericSigned, i: obj.Value}, true
	case *object.Float:
		return numericValue{kind: numericFloat, f: obj.Value}, true
	case *object.Native:
		return numericValueFromGo(obj.Value)
	default:
		return numericValue{}, false
	}
}

func numericValueFromGo(value interface{}) (numericValue, bool) {
	if value == nil {
		return numericValue{}, false
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return numericValue{kind: numericSigned, i: rv.Int()}, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return numericValue{kind: numericUnsigned, u: rv.Uint()}, true
	case reflect.Float32, reflect.Float64:
		return numericValue{kind: numericFloat, f: rv.Convert(reflect.TypeOf(float64(0))).Float()}, true
	default:
		return numericValue{}, false
	}
}

func numericValueFromReflectValue(rv reflect.Value) (numericValue, bool) {
	if !rv.IsValid() || isNilReflectValue(rv) {
		return numericValue{}, false
	}
	for rv.Kind() == reflect.Interface {
		rv = rv.Elem()
		if !rv.IsValid() || isNilReflectValue(rv) {
			return numericValue{}, false
		}
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return numericValue{kind: numericSigned, i: rv.Int()}, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return numericValue{kind: numericUnsigned, u: rv.Uint()}, true
	case reflect.Float32, reflect.Float64:
		return numericValue{kind: numericFloat, f: rv.Convert(reflect.TypeOf(float64(0))).Float()}, true
	default:
		return numericValue{}, false
	}
}

func (n numericValue) float64() float64 {
	switch n.kind {
	case numericFloat:
		return n.f
	case numericUnsigned:
		return float64(n.u)
	default:
		return float64(n.i)
	}
}

func (n numericValue) int64() (int64, bool) {
	switch n.kind {
	case numericSigned:
		return n.i, true
	case numericUnsigned:
		if n.u <= uint64(math.MaxInt64) {
			return int64(n.u), true
		}
	}
	return 0, false
}

func (n numericValue) uint64() (uint64, bool) {
	switch n.kind {
	case numericUnsigned:
		return n.u, true
	case numericSigned:
		if n.i >= 0 {
			return uint64(n.i), true
		}
	}
	return 0, false
}

func compareNumericEquality(left, right numericValue) bool {
	if left.kind == numericFloat || right.kind == numericFloat {
		return left.float64() == right.float64()
	}

	if l, ok := left.int64(); ok {
		if r, ok := right.int64(); ok {
			return l == r
		}
	}

	if left.kind == numericSigned && left.i < 0 {
		return false
	}
	if right.kind == numericSigned && right.i < 0 {
		return false
	}

	l, lok := left.uint64()
	r, rok := right.uint64()
	return lok && rok && l == r
}

func compareNumericOrdered(op code.Opcode, left, right numericValue) (bool, error) {
	if left.kind == numericFloat || right.kind == numericFloat {
		return compareFloatOrdered(op, left.float64(), right.float64())
	}

	if left.kind == numericSigned && right.kind == numericSigned {
		return compareIntOrdered(op, left.i, right.i)
	}
	if left.kind == numericUnsigned && right.kind == numericUnsigned {
		return compareUintOrdered(op, left.u, right.u)
	}
	if left.kind == numericSigned {
		if left.i < 0 {
			switch op {
			case code.OpGreaterThan:
				return false, nil
			case code.OpGreaterEqual:
				return false, nil
			}
		}
		return compareUintOrdered(op, uint64(left.i), right.u)
	}

	if right.i < 0 {
		switch op {
		case code.OpGreaterThan:
			return true, nil
		case code.OpGreaterEqual:
			return true, nil
		}
	}
	return compareUintOrdered(op, left.u, uint64(right.i))
}

func compareFloatOrdered(op code.Opcode, left, right float64) (bool, error) {
	switch op {
	case code.OpGreaterThan:
		return left > right, nil
	case code.OpGreaterEqual:
		return left >= right, nil
	default:
		return false, fmt.Errorf("unknown ordered comparison: %d", op)
	}
}

func compareIntOrdered(op code.Opcode, left, right int64) (bool, error) {
	switch op {
	case code.OpGreaterThan:
		return left > right, nil
	case code.OpGreaterEqual:
		return left >= right, nil
	default:
		return false, fmt.Errorf("unknown ordered comparison: %d", op)
	}
}

func compareUintOrdered(op code.Opcode, left, right uint64) (bool, error) {
	switch op {
	case code.OpGreaterThan:
		return left > right, nil
	case code.OpGreaterEqual:
		return left >= right, nil
	default:
		return false, fmt.Errorf("unknown ordered comparison: %d", op)
	}
}

func numericOperationObject(op code.Opcode, left, right numericValue) (object.Object, error) {
	if left.kind == numericFloat || right.kind == numericFloat {
		l := left.float64()
		r := right.float64()
		switch op {
		case code.OpAdd:
			return &object.Float{Value: l + r}, nil
		case code.OpSub:
			return &object.Float{Value: l - r}, nil
		case code.OpMul:
			return &object.Float{Value: l * r}, nil
		case code.OpDiv:
			if r == 0 {
				return nil, fmt.Errorf("division by zero %v / %v", l, r)
			}
			return &object.Float{Value: l / r}, nil
		default:
			return nil, fmt.Errorf("unknown numeric operator: %d", op)
		}
	}

	lSigned, lok := left.int64()
	rSigned, rok := right.int64()
	if lok && rok {
		switch op {
		case code.OpAdd:
			return &object.Integer{Value: lSigned + rSigned}, nil
		case code.OpSub:
			return &object.Integer{Value: lSigned - rSigned}, nil
		case code.OpMul:
			return &object.Integer{Value: lSigned * rSigned}, nil
		case code.OpDiv:
			if rSigned == 0 {
				return nil, fmt.Errorf("division by zero %v / %v", lSigned, rSigned)
			}
			return &object.Integer{Value: lSigned / rSigned}, nil
		default:
			return nil, fmt.Errorf("unknown numeric operator: %d", op)
		}
	}

	lUnsigned, lok := left.uint64()
	rUnsigned, rok := right.uint64()
	if !lok || !rok {
		return nil, fmt.Errorf("unsupported mixed signed/unsigned operation")
	}

	switch op {
	case code.OpAdd:
		if math.MaxUint64-lUnsigned < rUnsigned {
			return nil, fmt.Errorf("integer overflow %v + %v", lUnsigned, rUnsigned)
		}
		return unsignedNumericObject(lUnsigned + rUnsigned), nil
	case code.OpSub:
		if lUnsigned < rUnsigned {
			return nil, fmt.Errorf("integer underflow %v - %v", lUnsigned, rUnsigned)
		}
		return unsignedNumericObject(lUnsigned - rUnsigned), nil
	case code.OpMul:
		if rUnsigned != 0 && lUnsigned > math.MaxUint64/rUnsigned {
			return nil, fmt.Errorf("integer overflow %v * %v", lUnsigned, rUnsigned)
		}
		return unsignedNumericObject(lUnsigned * rUnsigned), nil
	case code.OpDiv:
		if rUnsigned == 0 {
			return nil, fmt.Errorf("division by zero %v / %v", lUnsigned, rUnsigned)
		}
		return unsignedNumericObject(lUnsigned / rUnsigned), nil
	default:
		return nil, fmt.Errorf("unknown numeric operator: %d", op)
	}
}

func unsignedNumericObject(value uint64) object.Object {
	if value <= uint64(math.MaxInt64) {
		return &object.Integer{Value: int64(value)}
	}
	return &object.Native{Value: value}
}

func orderedOperatorString(op code.Opcode) string {
	switch op {
	case code.OpGreaterThan:
		return ">"
	case code.OpGreaterEqual:
		return ">="
	default:
		return fmt.Sprint(op)
	}
}
