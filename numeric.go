package plush

import (
	"fmt"
	"math"
	"reflect"
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

func isNumericOperator(op string) bool {
	switch op {
	case "+", "-", "*", "/", "<", ">", "<=", ">=", "==", "!=":
		return true
	default:
		return false
	}
}

func numericValueFromGo(value interface{}) (numericValue, bool) {
	if value == nil {
		return numericValue{}, false
	}

	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return numericValue{}, false
		}
		rv = rv.Elem()
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

func evalNumericOperator(left, right numericValue, op string) (interface{}, error) {
	switch op {
	case "==":
		return compareNumericEquality(left, right), nil
	case "!=":
		return !compareNumericEquality(left, right), nil
	case "<", ">", "<=", ">=":
		return compareNumericOrdered(op, left, right)
	default:
		return numericArithmetic(left, right, op)
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

func compareNumericOrdered(op string, left, right numericValue) (bool, error) {
	if left.kind == numericFloat || right.kind == numericFloat {
		l := left.float64()
		r := right.float64()
		switch op {
		case "<":
			return l < r, nil
		case ">":
			return l > r, nil
		case "<=":
			return l <= r, nil
		case ">=":
			return l >= r, nil
		}
	}

	if left.kind == numericSigned && right.kind == numericSigned {
		switch op {
		case "<":
			return left.i < right.i, nil
		case ">":
			return left.i > right.i, nil
		case "<=":
			return left.i <= right.i, nil
		case ">=":
			return left.i >= right.i, nil
		}
	}
	if left.kind == numericUnsigned && right.kind == numericUnsigned {
		return compareUintOrdered(op, left.u, right.u)
	}
	if left.kind == numericSigned {
		if left.i < 0 {
			switch op {
			case "<":
				return true, nil
			case ">":
				return false, nil
			case "<=":
				return true, nil
			case ">=":
				return false, nil
			}
		}
		return compareUintOrdered(op, uint64(left.i), right.u)
	}

	if right.i < 0 {
		switch op {
		case "<":
			return false, nil
		case ">":
			return true, nil
		case "<=":
			return false, nil
		case ">=":
			return true, nil
		}
	}
	return compareUintOrdered(op, left.u, uint64(right.i))
}

func compareUintOrdered(op string, left, right uint64) (bool, error) {
	switch op {
	case "<":
		return left < right, nil
	case ">":
		return left > right, nil
	case "<=":
		return left <= right, nil
	case ">=":
		return left >= right, nil
	default:
		return false, fmt.Errorf("unknown operator for numeric %s", op)
	}
}

func numericArithmetic(left, right numericValue, op string) (interface{}, error) {
	if left.kind == numericFloat || right.kind == numericFloat {
		l := left.float64()
		r := right.float64()
		switch op {
		case "+":
			return l + r, nil
		case "-":
			return l - r, nil
		case "*":
			return l * r, nil
		case "/":
			if r == 0 {
				return nil, fmt.Errorf("division by zero %v %s %v", l, op, r)
			}
			return l / r, nil
		default:
			return nil, fmt.Errorf("unknown operator for numeric %s", op)
		}
	}

	lSigned, lok := left.int64()
	rSigned, rok := right.int64()
	if lok && rok {
		switch op {
		case "+":
			return signedNumericResult(lSigned + rSigned), nil
		case "-":
			return signedNumericResult(lSigned - rSigned), nil
		case "*":
			return signedNumericResult(lSigned * rSigned), nil
		case "/":
			if rSigned == 0 {
				return nil, fmt.Errorf("division by zero %v %s %v", lSigned, op, rSigned)
			}
			return signedNumericResult(lSigned / rSigned), nil
		default:
			return nil, fmt.Errorf("unknown operator for numeric %s", op)
		}
	}

	lUnsigned, lok := left.uint64()
	rUnsigned, rok := right.uint64()
	if !lok || !rok {
		return nil, fmt.Errorf("unsupported mixed signed/unsigned operation")
	}

	switch op {
	case "+":
		if math.MaxUint64-lUnsigned < rUnsigned {
			return nil, fmt.Errorf("integer overflow %v + %v", lUnsigned, rUnsigned)
		}
		return lUnsigned + rUnsigned, nil
	case "-":
		if lUnsigned < rUnsigned {
			return nil, fmt.Errorf("integer underflow %v - %v", lUnsigned, rUnsigned)
		}
		return lUnsigned - rUnsigned, nil
	case "*":
		if rUnsigned != 0 && lUnsigned > math.MaxUint64/rUnsigned {
			return nil, fmt.Errorf("integer overflow %v * %v", lUnsigned, rUnsigned)
		}
		return lUnsigned * rUnsigned, nil
	case "/":
		if rUnsigned == 0 {
			return nil, fmt.Errorf("division by zero %v / %v", lUnsigned, rUnsigned)
		}
		return lUnsigned / rUnsigned, nil
	default:
		return nil, fmt.Errorf("unknown operator for numeric %s", op)
	}
}

func signedNumericResult(value int64) interface{} {
	asInt := int(value)
	if int64(asInt) == value {
		return asInt
	}
	return value
}
