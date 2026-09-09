package vm

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/vm/code"
	"github.com/gobuffalo/plush/v5/vm/compiler"
	"github.com/gobuffalo/plush/v5/vm/object"
)

func evalFastValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, base interface{}) (interface{}, bool, error) {
	if value == nil {
		return nil, true, nil
	}
	switch value.Kind {
	case compiler.FastValueName:
		if value.Value == "nil" {
			return nil, true, nil
		}
		raw, ok := bindings.value(value.NameIndex)
		if !ok {
			if value.NullOnMissing {
				return nil, true, nil
			}
			return nil, false, nil
		}
		return raw, true, nil
	case compiler.FastValueString:
		return value.Value, true, nil
	case compiler.FastValueInteger:
		return int(value.IntValue), true, nil
	case compiler.FastValueFloat:
		return value.FloatValue, true, nil
	case compiler.FastValueBool:
		return value.BoolValue, true, nil
	case compiler.FastValueLoopKey:
		return nil, false, nil
	case compiler.FastValueInfix:
		return evalFastInfixValue(value, ctx, bindings, nil, nil, base, false)
	case compiler.FastValuePrefix:
		return evalFastPrefixValue(value, ctx, bindings, nil, nil, base, false)
	case compiler.FastValueConcat:
		return evalFastConcatValue(value, ctx, bindings, nil, nil, base, false)
	case compiler.FastValueCall:
		result, ok, err := evalFastCallValuePlan(value.Call, nil, ctx, bindings, nil, nil, nil)
		if err != nil || !ok || len(value.Path) == 0 {
			return result, ok, err
		}
		result, err = evalFastPathSteps(result, value.Path, ctx, bindings, nil, nil, false)
		if err != nil {
			return nil, true, err
		}
		return result, true, nil
	case compiler.FastValueArray:
		return evalFastArrayValue(value.Elements, ctx, bindings, base)
	case compiler.FastValueHash:
		return evalFastHashValue(value.Pairs, ctx, bindings, base)
	case compiler.FastValuePath:
		return evalFastPathValue(value, ctx, bindings, base, nil, nil, false)
	case compiler.FastValueIndex:
		return evalFastIndexPlanValue(value, ctx, bindings, nil, nil, base, false)
	default:
		return nil, false, nil
	}
}

func evalFastArrayValue(elements []compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, base interface{}) ([]interface{}, bool, error) {
	out := make([]interface{}, 0, len(elements))
	for i := range elements {
		value, ok, err := evalFastValue(&elements[i], ctx, bindings, base)
		if err != nil || !ok {
			return nil, ok, err
		}
		out = append(out, value)
	}
	return out, true, nil
}

func evalFastHashValue(pairs []compiler.FastValuePair, ctx hctx.Context, bindings fastRenderBindings, base interface{}) (interface{}, bool, error) {
	if fastValuePairsUseStaticStringKeys(pairs) {
		out := make(map[string]interface{}, len(pairs))
		for i := range pairs {
			value, ok, err := evalFastValue(&pairs[i].Value, ctx, bindings, base)
			if err != nil || !ok {
				return nil, ok, err
			}
			out[pairs[i].Key] = value
		}
		return out, true, nil
	}
	out := make(map[object.HashKey]object.HashPair, len(pairs))
	for i := range pairs {
		value, ok, err := evalFastValue(&pairs[i].Value, ctx, bindings, base)
		if err != nil || !ok {
			return nil, ok, err
		}
		key, ok, err := evalFastHashPairKey(&pairs[i], ctx, bindings, base)
		if err != nil || !ok {
			return nil, ok, err
		}
		hashKey, ok := key.(object.Hashable)
		if !ok {
			return nil, true, fastLineError(pairs[i].Line, fmt.Errorf("unusable as hash key: %s", key.Type()))
		}
		out[hashKey.HashKey()] = object.HashPair{
			Key:   key,
			Value: object.Wrap(value),
		}
	}
	return &object.Hash{Pairs: out}, true, nil
}

func evalFastLoopValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue interface{}) (interface{}, bool, error) {
	if value == nil {
		return nil, true, nil
	}
	switch value.Kind {
	case compiler.FastValueLoopKey:
		return loopKey, true, nil
	case compiler.FastValueInfix:
		return evalFastInfixValue(value, ctx, bindings, loopKey, loopValue, nil, true)
	case compiler.FastValuePrefix:
		return evalFastPrefixValue(value, ctx, bindings, loopKey, loopValue, nil, true)
	case compiler.FastValueConcat:
		return evalFastConcatValue(value, ctx, bindings, loopKey, loopValue, nil, true)
	case compiler.FastValueCall:
		result, ok, err := evalFastLoopCallValuePlan(value.Call, ctx, bindings, loopKey, loopValue)
		if err != nil || !ok || len(value.Path) == 0 {
			return result, ok, err
		}
		result, err = evalFastPathSteps(result, value.Path, ctx, bindings, loopKey, loopValue, true)
		if err != nil {
			return nil, true, err
		}
		return result, true, nil
	case compiler.FastValueArray:
		return evalFastLoopArrayValue(value.Elements, ctx, bindings, loopKey, loopValue)
	case compiler.FastValueHash:
		return evalFastLoopHashValue(value.Pairs, ctx, bindings, loopKey, loopValue)
	case compiler.FastValuePath:
		return evalFastPathValue(value, ctx, bindings, loopValue, loopKey, loopValue, true)
	case compiler.FastValueIndex:
		return evalFastIndexPlanValue(value, ctx, bindings, loopKey, loopValue, nil, true)
	default:
		return evalFastValue(value, ctx, bindings, loopValue)
	}
}

func evalFastLoopArrayValue(elements []compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue interface{}) ([]interface{}, bool, error) {
	out := make([]interface{}, 0, len(elements))
	for i := range elements {
		value, ok, err := evalFastLoopValue(&elements[i], ctx, bindings, loopKey, loopValue)
		if err != nil || !ok {
			return nil, ok, err
		}
		out = append(out, value)
	}
	return out, true, nil
}

func evalFastLoopHashValue(pairs []compiler.FastValuePair, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue interface{}) (interface{}, bool, error) {
	if fastValuePairsUseStaticStringKeys(pairs) {
		out := make(map[string]interface{}, len(pairs))
		for i := range pairs {
			value, ok, err := evalFastLoopValue(&pairs[i].Value, ctx, bindings, loopKey, loopValue)
			if err != nil || !ok {
				return nil, ok, err
			}
			out[pairs[i].Key] = value
		}
		return out, true, nil
	}
	out := make(map[object.HashKey]object.HashPair, len(pairs))
	for i := range pairs {
		value, ok, err := evalFastLoopValue(&pairs[i].Value, ctx, bindings, loopKey, loopValue)
		if err != nil || !ok {
			return nil, ok, err
		}
		key, ok, err := evalFastLoopHashPairKey(&pairs[i], ctx, bindings, loopKey, loopValue)
		if err != nil || !ok {
			return nil, ok, err
		}
		hashKey, ok := key.(object.Hashable)
		if !ok {
			return nil, true, fastLineError(pairs[i].Line, fmt.Errorf("unusable as hash key: %s", key.Type()))
		}
		out[hashKey.HashKey()] = object.HashPair{
			Key:   key,
			Value: object.Wrap(value),
		}
	}
	return &object.Hash{Pairs: out}, true, nil
}

func fastValuePairsUseStaticStringKeys(pairs []compiler.FastValuePair) bool {
	for i := range pairs {
		if pairs[i].KeyPlan != nil {
			return false
		}
	}
	return true
}

func evalFastHashPairKey(pair *compiler.FastValuePair, ctx hctx.Context, bindings fastRenderBindings, base interface{}) (object.Object, bool, error) {
	if pair == nil {
		return object.NullObject, true, nil
	}
	if pair.KeyPlan == nil {
		return &object.String{Value: pair.Key}, true, nil
	}
	key, ok, err := evalFastValue(pair.KeyPlan, ctx, bindings, base)
	if err != nil || !ok {
		return nil, ok, err
	}
	return object.Wrap(key), true, nil
}

func evalFastLoopHashPairKey(pair *compiler.FastValuePair, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue interface{}) (object.Object, bool, error) {
	if pair == nil {
		return object.NullObject, true, nil
	}
	if pair.KeyPlan == nil {
		return &object.String{Value: pair.Key}, true, nil
	}
	key, ok, err := evalFastLoopValue(pair.KeyPlan, ctx, bindings, loopKey, loopValue)
	if err != nil || !ok {
		return nil, ok, err
	}
	return object.Wrap(key), true, nil
}

func evalFastPathValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, base interface{}, loopKey, loopValue interface{}, loopAware bool) (interface{}, bool, error) {
	if value == nil || value.Kind != compiler.FastValuePath {
		return nil, false, nil
	}
	raw := base
	if value.NameIndex >= 0 {
		var ok bool
		raw, ok = bindings.value(value.NameIndex)
		if !ok {
			if value.NullOnMissing {
				return nil, true, nil
			}
			return nil, false, nil
		}
	}
	if !fastPathStepsNeedRuntimeBindings(value.Path) {
		if result, handled, err := evalFastFieldChainValue(value, raw, ctx, bindings.runtimePlans); handled || err != nil {
			return result, true, err
		}
		if result, handled, err := evalFastAccessChainValue(value, raw, ctx, bindings.runtimePlans); handled || err != nil {
			return result, true, err
		}
	}
	result, err := evalFastPathSteps(raw, value.Path, ctx, bindings, loopKey, loopValue, loopAware)
	if err != nil {
		return nil, true, err
	}
	return result, true, nil
}

func evalFastIndexPlanValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue interface{}, base interface{}, loopAware bool) (interface{}, bool, error) {
	if value == nil || value.Kind != compiler.FastValueIndex || value.Left == nil || value.Right == nil {
		return nil, false, nil
	}
	left, ok, err := evalFastCompoundOperand(value.Left, ctx, bindings, loopKey, loopValue, base, loopAware)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		if value.NullOnMissing {
			return nil, true, nil
		}
		return nil, false, nil
	}
	index, ok, err := evalFastCompoundOperand(value.Right, ctx, bindings, loopKey, loopValue, base, loopAware)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, false, nil
	}
	result, err := fastIndexGoValue(left, index)
	if err != nil {
		return nil, true, fastLineError(value.Line, err)
	}
	if len(value.Path) == 0 {
		return result, true, nil
	}
	result, err = evalFastPathSteps(result, value.Path, ctx, bindings, loopKey, loopValue, loopAware)
	if err != nil {
		return nil, true, err
	}
	return result, true, nil
}

func evalFastLoopCallValuePlan(call *compiler.FastCallPlan, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue interface{}) (interface{}, bool, error) {
	if call == nil {
		return nil, true, nil
	}
	raw, ok := bindings.value(call.NameIndex)
	if !ok {
		return nil, false, nil
	}
	if err := spendFastFunctionCall(ctx, call.Name, call.Line); err != nil {
		return nil, true, err
	}
	var args *fastCallArgs
	var argStore *fastCallArgs
	if len(call.Args) > 0 {
		argStore = borrowFastCallArgs()
		var err error
		args, err = evalFastLoopCallArgsInto(call.Args, ctx, bindings, loopKey, loopValue, argStore)
		if err != nil {
			releaseFastCallArgs(argStore)
			return nil, true, err
		}
	}
	result, err := fastCallValueWithDiagnostics(call.Name, raw, args, ctx, &call.Cache, bindings.vmHotspots)
	releaseFastCallArgs(argStore)
	if err != nil {
		return nil, true, fastLineError(call.Line, err)
	}
	return result, true, nil
}

func evalFastPrefixValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue, base interface{}, loopAware bool) (interface{}, bool, error) {
	if value == nil || value.Kind != compiler.FastValuePrefix || value.Right == nil {
		return nil, false, nil
	}
	right, ok, err := evalFastCompoundOperand(value.Right, ctx, bindings, loopKey, loopValue, base, loopAware)
	if err != nil {
		return nil, true, err
	}
	switch value.Operator {
	case "!":
		return !(ok && isTruthyFastValue(right)), true, nil
	case "-":
		if !ok {
			right = nil
		}
		result, err := evalFastMinusOperator(right)
		if err != nil {
			return nil, true, fastLineError(value.Line, err)
		}
		return result, true, nil
	default:
		return nil, true, fastLineError(value.Line, fmt.Errorf("unknown fast prefix operator: %s", value.Operator))
	}
}

func evalFastConcatValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue, base interface{}, loopAware bool) (interface{}, bool, error) {
	if value == nil || value.Kind != compiler.FastValueConcat || value.Left == nil || value.Right == nil {
		return nil, false, nil
	}
	left, ok, err := evalFastCompoundOperand(value.Left, ctx, bindings, loopKey, loopValue, base, loopAware)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, false, nil
	}
	right, ok, err := evalFastCompoundOperand(value.Right, ctx, bindings, loopKey, loopValue, base, loopAware)
	if err != nil {
		return nil, true, err
	}
	if !ok {
		return nil, false, nil
	}
	result, err := evalFastAddOperator(left, right)
	if err != nil {
		return nil, true, fastLineError(value.Line, err)
	}
	return result, true, nil
}

func evalFastCompoundOperand(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue, base interface{}, loopAware bool) (interface{}, bool, error) {
	if loopAware {
		return evalFastLoopValue(value, ctx, bindings, loopKey, loopValue)
	}
	return evalFastValue(value, ctx, bindings, base)
}

func evalFastAddOperator(left, right interface{}) (interface{}, error) {
	left = fastAddGoValue(left)
	right = fastAddGoValue(right)
	if appended, handled, err := fastNativeSliceAppend(left, right); handled || err != nil {
		return appended, err
	}
	if _, ok := left.(string); ok {
		return fmt.Sprint(left) + fmt.Sprint(right), nil
	}
	if _, ok := right.(string); ok {
		return fmt.Sprint(left) + fmt.Sprint(right), nil
	}
	if _, ok := left.(bool); ok {
		return isTruthyFastValue(left) && isTruthyFastValue(right), nil
	}
	if _, ok := right.(bool); ok {
		return isTruthyFastValue(left) && isTruthyFastValue(right), nil
	}
	if lnum, ok := numericValueFromGo(left); ok {
		if rnum, ok := numericValueFromGo(right); ok {
			result, err := numericOperationObject(code.OpAdd, lnum, rnum)
			if err != nil {
				return nil, err
			}
			return object.ToGo(result), nil
		}
	}
	return nil, fmt.Errorf("unable to operate (+) on %T and %T", left, right)
}

func fastNativeSliceAppend(left, right interface{}) (interface{}, bool, error) {
	if left == nil {
		return nil, false, nil
	}
	rv := reflect.ValueOf(left)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, false, nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return nil, false, nil
	}
	elem := rv.Type().Elem()
	val := reflect.ValueOf(right)
	if !val.IsValid() {
		val = reflect.Zero(elem)
	} else if elem.Kind() != reflect.Interface && !val.Type().AssignableTo(elem) {
		return nil, true, fmt.Errorf("cannot append '%v' (untyped %s constant) as %s value in assignment", right, val.Type(), elem)
	}
	appended := reflect.Append(rv, val)
	return appended.Interface(), true, nil
}

func fastAddGoValue(value interface{}) interface{} {
	if obj, ok := value.(object.Object); ok {
		return object.ToGo(obj)
	}
	return value
}

func evalFastInfixValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue, base interface{}, loopAware bool) (interface{}, bool, error) {
	if value == nil || value.Kind != compiler.FastValueInfix || value.Left == nil || value.Right == nil {
		return nil, false, nil
	}
	if value.Operator == "&&" || value.Operator == "||" {
		return evalFastLogicalInfixValue(value, ctx, bindings, loopKey, loopValue, base, loopAware)
	}
	var left interface{}
	var right interface{}
	var ok bool
	var err error
	if loopAware {
		left, ok, err = evalFastLoopValue(value.Left, ctx, bindings, loopKey, loopValue)
	} else {
		left, ok, err = evalFastValue(value.Left, ctx, bindings, base)
	}
	if err != nil {
		return nil, true, err
	}
	leftOK := ok
	if !leftOK {
		left = nil
	}
	if loopAware {
		right, ok, err = evalFastLoopValue(value.Right, ctx, bindings, loopKey, loopValue)
	} else {
		right, ok, err = evalFastValue(value.Right, ctx, bindings, base)
	}
	if err != nil {
		return nil, true, err
	}
	rightOK := ok
	if !rightOK {
		right = nil
	}
	result, err := evalFastInfixOperator(value.Operator, left, right, value.RegexCache)
	if err != nil {
		return nil, true, fastLineError(value.Line, annotateFastInfixError(value, leftOK, rightOK, left, right, err))
	}
	return result, true, nil
}

func annotateFastInfixError(value *compiler.FastValuePlan, leftOK, rightOK bool, left, right interface{}, err error) error {
	if err == nil || value == nil {
		return err
	}
	expression := fastValuePlanExpression(value)
	if expression == "" {
		return err
	}
	notes := make([]string, 0, 2)
	if note := fastInfixNilOperandNote(value.Left, leftOK, left); note != "" {
		notes = append(notes, note)
	}
	if note := fastInfixNilOperandNote(value.Right, rightOK, right); note != "" {
		notes = append(notes, note)
	}
	if len(notes) == 0 {
		return err
	}
	message := strings.TrimSpace(err.Error()) + " while evaluating " + expression + "; " + strings.Join(notes, "; ")
	return fmt.Errorf("%s", message)
}

func fastInfixNilOperandNote(value *compiler.FastValuePlan, ok bool, raw interface{}) string {
	if ok && !(fastConditionOperandValue{raw: raw}).isNil() {
		return ""
	}
	name := fastValuePlanExpression(value)
	if name == "" {
		return ""
	}
	return name + " is nil or missing"
}

func fastValuePlanExpression(value *compiler.FastValuePlan) string {
	if value == nil {
		return ""
	}
	switch value.Kind {
	case compiler.FastValueName, compiler.FastValueLoopKey:
		return value.Value
	case compiler.FastValueString:
		return strconv.Quote(value.Value)
	case compiler.FastValueInteger:
		return strconv.FormatInt(value.IntValue, 10)
	case compiler.FastValueFloat:
		return strconv.FormatFloat(value.FloatValue, 'f', -1, 64)
	case compiler.FastValueBool:
		return strconv.FormatBool(value.BoolValue)
	case compiler.FastValuePath:
		return fastPathValueExpression(value)
	case compiler.FastValueIndex:
		return fastIndexValueExpression(value)
	case compiler.FastValueInfix, compiler.FastValueConcat:
		left := fastValuePlanExpression(value.Left)
		right := fastValuePlanExpression(value.Right)
		if left == "" || right == "" || value.Operator == "" {
			return ""
		}
		return left + " " + value.Operator + " " + right
	case compiler.FastValuePrefix:
		right := fastValuePlanExpression(value.Right)
		if right == "" {
			return ""
		}
		return value.Operator + right
	case compiler.FastValueCall:
		if value.Call != nil && value.Call.Name != "" {
			return value.Call.Name + "(...)"
		}
	}
	return ""
}

func fastPathValueExpression(value *compiler.FastValuePlan) string {
	if value == nil {
		return ""
	}
	expression := value.Value
	for i := range value.Path {
		step := value.Path[i]
		switch step.Kind {
		case compiler.FastPathStepProperty:
			if step.Full != "" {
				expression = step.Full
			} else if expression == "" {
				expression = step.Value
			} else {
				expression += "." + step.Value
			}
		case compiler.FastPathStepIndexInteger:
			expression += "[" + strconv.Itoa(step.Index) + "]"
		case compiler.FastPathStepIndexString:
			expression += "[" + strconv.Quote(step.Value) + "]"
		case compiler.FastPathStepCall:
			if expression == "" {
				expression = step.Value + "()"
			} else {
				expression += "()"
			}
		}
	}
	return expression
}

func fastIndexValueExpression(value *compiler.FastValuePlan) string {
	if value == nil {
		return ""
	}
	left := fastValuePlanExpression(value.Left)
	right := fastValuePlanExpression(value.Right)
	if left == "" || right == "" {
		return ""
	}
	return left + "[" + right + "]"
}

func evalFastLogicalInfixValue(value *compiler.FastValuePlan, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue, base interface{}, loopAware bool) (interface{}, bool, error) {
	var left interface{}
	var ok bool
	var err error
	if loopAware {
		left, ok, err = evalFastLoopValue(value.Left, ctx, bindings, loopKey, loopValue)
	} else {
		left, ok, err = evalFastValue(value.Left, ctx, bindings, base)
	}
	if err != nil {
		return nil, true, err
	}
	leftTruthy := ok && isTruthyFastValue(left)
	switch value.Operator {
	case "&&":
		if !leftTruthy {
			return false, true, nil
		}
	case "||":
		if leftTruthy {
			return true, true, nil
		}
	default:
		return nil, false, nil
	}

	var right interface{}
	if loopAware {
		right, ok, err = evalFastLoopValue(value.Right, ctx, bindings, loopKey, loopValue)
	} else {
		right, ok, err = evalFastValue(value.Right, ctx, bindings, base)
	}
	if err != nil {
		return nil, true, err
	}
	return ok && isTruthyFastValue(right), true, nil
}

func evalFastInfixOperator(operator string, left, right interface{}, regexSlot *object.InlineCacheSlot) (interface{}, error) {
	leftValue := fastConditionOperandValue{raw: left}
	rightValue := fastConditionOperandValue{raw: right}
	switch operator {
	case "&&":
		return isTruthyFastValue(left) && isTruthyFastValue(right), nil
	case "||":
		return isTruthyFastValue(left) || isTruthyFastValue(right), nil
	case "==":
		return evalFastConditionInfixOperator(operator, leftValue, rightValue)
	case "!=":
		return evalFastConditionInfixOperator(operator, leftValue, rightValue)
	case ">":
		return evalFastConditionInfixOperator(operator, leftValue, rightValue)
	case ">=":
		return evalFastConditionInfixOperator(operator, leftValue, rightValue)
	case "<":
		return evalFastConditionInfixOperator(operator, leftValue, rightValue)
	case "<=":
		return evalFastConditionInfixOperator(operator, leftValue, rightValue)
	case "~=":
		pattern := fmt.Sprint(fastAddGoValue(right))
		re, err := cachedRegex(regexSlot, pattern)
		if err != nil {
			return nil, fmt.Errorf("couldn't compile regex %s", fastAddGoValue(right))
		}
		return re.MatchString(fmt.Sprint(fastAddGoValue(left))), nil
	case "-", "*", "/":
		return evalFastNumericArithmeticOperator(operator, left, right)
	default:
		return nil, fmt.Errorf("unknown fast infix operator: %s", operator)
	}
}

func evalFastMinusOperator(value interface{}) (interface{}, error) {
	value = fastAddGoValue(value)
	if f, ok := value.(float32); ok {
		return -float64(f), nil
	}
	if f, ok := value.(float64); ok {
		return -f, nil
	}
	n, ok := numericValueFromGo(value)
	if !ok {
		return nil, fmt.Errorf("unsupported type for negation: %T", value)
	}
	i, ok := n.int64()
	if !ok {
		return nil, fmt.Errorf("unsupported type for negation: %T", value)
	}
	return -i, nil
}

func evalFastNumericArithmeticOperator(operator string, left, right interface{}) (interface{}, error) {
	left = fastAddGoValue(left)
	right = fastAddGoValue(right)
	lnum, lok := numericValueFromGo(left)
	rnum, rok := numericValueFromGo(right)
	if !lok || !rok {
		return nil, fmt.Errorf("unable to operate (%s) on %T and %T", operator, left, right)
	}
	op, ok := fastArithmeticOpcode(operator)
	if !ok {
		return nil, fmt.Errorf("unknown fast arithmetic operator: %s", operator)
	}
	result, err := numericOperationObject(op, lnum, rnum)
	if err != nil {
		return nil, err
	}
	return object.ToGo(result), nil
}

func fastArithmeticOpcode(operator string) (code.Opcode, bool) {
	switch operator {
	case "-":
		return code.OpSub, true
	case "*":
		return code.OpMul, true
	case "/":
		return code.OpDiv, true
	default:
		return 0, false
	}
}

func writeFastValuePlanOutput(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, value *compiler.FastValuePlan) (bool, bool, error) {
	if value == nil {
		return false, false, nil
	}
	if value.Kind == compiler.FastValueInfix {
		result, ok, err := evalFastInfixValue(value, ctx, bindings, nil, nil, nil, false)
		if err != nil || !ok {
			return true, ok, err
		}
		if truth, ok := result.(bool); ok {
			out.WriteString(strconv.FormatBool(truth))
		} else {
			writeFastGoValue(out, ctx, result)
		}
		return true, true, nil
	}
	if value.Kind == compiler.FastValuePrefix || value.Kind == compiler.FastValueConcat {
		result, ok, err := evalFastValue(value, ctx, bindings, nil)
		if err != nil || !ok {
			return true, ok, err
		}
		writeFastGoValue(out, ctx, result)
		return true, true, nil
	}
	if value.Kind != compiler.FastValuePath || value.NameIndex < 0 {
		return false, false, nil
	}
	raw, ok := bindings.value(value.NameIndex)
	if !ok {
		if value.NullOnMissing {
			return true, true, nil
		}
		return true, false, nil
	}
	if handled, err := writeFastFieldChainValue(out, ctx, value, raw, bindings.runtimePlans); handled || err != nil {
		return handled, true, err
	}
	if handled, err := writeFastAccessChainValue(out, ctx, value, raw, bindings.runtimePlans); handled || err != nil {
		return handled, true, err
	}
	return false, false, nil
}

func canUseFastTopLevelAccessChain(value *compiler.FastValuePlan) bool {
	if value == nil ||
		value.Kind != compiler.FastValuePath ||
		value.NameIndex < 0 ||
		len(value.Path) == 0 {
		return false
	}
	for i := range value.Path {
		step := &value.Path[i]
		switch step.Kind {
		case compiler.FastPathStepProperty:
			if step.Method {
				return i == len(value.Path)-2 &&
					value.Path[len(value.Path)-1].Kind == compiler.FastPathStepCall &&
					len(value.Path[len(value.Path)-1].Args) == 0
			}
		case compiler.FastPathStepIndexInteger, compiler.FastPathStepIndexString:
			continue
		case compiler.FastPathStepCall:
			return i == len(value.Path)-1 &&
				len(step.Args) == 0 &&
				i > 0 &&
				value.Path[i-1].Kind == compiler.FastPathStepProperty &&
				value.Path[i-1].Method
		default:
			return false
		}
	}
	return true
}

func writeFastTopLevelAccessChainOutput(out *strings.Builder, ctx hctx.Context, bindings fastRenderBindings, value *compiler.FastValuePlan, cacheSlot *object.InlineCacheSlot) (bool, bool, error) {
	if !canUseFastTopLevelAccessChain(value) {
		return false, false, nil
	}
	raw, ok := bindings.value(value.NameIndex)
	if !ok {
		if value.NullOnMissing {
			return true, true, nil
		}
		return true, false, nil
	}
	handled, err := writeFastTopLevelAccessChainRaw(out, ctx, value, raw, cacheSlot, bindings.runtimePlans)
	return handled, true, err
}

func writeFastTopLevelAccessChainRaw(out *strings.Builder, ctx hctx.Context, value *compiler.FastValuePlan, raw interface{}, cacheSlot *object.InlineCacheSlot, runtimePlans ...*fastRuntimePlanCache) (bool, error) {
	if !canUseFastTopLevelAccessChain(value) {
		return false, nil
	}
	rv, nilValue, ok := fastAccessChainRootValue(raw)
	if nilValue {
		return true, nil
	}
	if !ok {
		return false, nil
	}
	entry := fastTopLevelAccessCacheEntryFor(cacheSlot, value, rv.Type(), runtimePlans...)
	if entry == nil || entry.kind == fastTopLevelAccessUnsupported {
		return false, nil
	}
	switch entry.kind {
	case fastTopLevelAccessFieldChain:
		return true, writeFastFieldChainPlanOutput(out, ctx, entry.fieldChain, rv)
	case fastTopLevelAccessChain:
		_, err := writeFastAccessChainPlanOutput(out, ctx, entry.chain, rv)
		return true, err
	case fastTopLevelAccessMethodCall:
		return true, writeFastLoopMethodCall(out, ctx, entry.method, rv)
	default:
		return false, nil
	}
}

func fastTopLevelAccessCacheEntryFor(slot *object.InlineCacheSlot, value *compiler.FastValuePlan, rt reflect.Type, runtimePlans ...*fastRuntimePlanCache) *fastTopLevelAccessCacheEntry {
	if slot == nil {
		return buildFastTopLevelAccessCacheEntry(value, rt, runtimePlans...)
	}
	if cached, ok := slot.Load().(*fastTopLevelAccessCacheEntry); ok {
		for entry := cached; entry != nil; entry = entry.next {
			if entry.typ == rt {
				return entry
			}
		}
	}
	current, _ := slot.Load().(*fastTopLevelAccessCacheEntry)
	entry := buildFastTopLevelAccessCacheEntry(value, rt, runtimePlans...)
	entry.next = cloneFastTopLevelAccessCache(current, propertyInlineCacheDepth-1)
	slot.Store(entry)
	return entry
}

func buildFastTopLevelAccessCacheEntry(value *compiler.FastValuePlan, rt reflect.Type, runtimePlans ...*fastRuntimePlanCache) *fastTopLevelAccessCacheEntry {
	entry := &fastTopLevelAccessCacheEntry{
		typ:  rt,
		kind: fastTopLevelAccessUnsupported,
	}
	if method, ok := buildFastLoopMethodCallPlan(value, rt); ok {
		entry.kind = fastTopLevelAccessMethodCall
		entry.method = method
		return entry
	}
	if chain, ok := fastFieldChainPlanFor(value, rt, runtimePlans...); ok {
		entry.kind = fastTopLevelAccessFieldChain
		entry.fieldChain = chain
		return entry
	}
	if chain, ok := fastAccessChainPlanFor(value, rt, runtimePlans...); ok {
		entry.kind = fastTopLevelAccessChain
		entry.chain = chain
		return entry
	}
	return entry
}

func cloneFastTopLevelAccessCache(head *fastTopLevelAccessCacheEntry, limit int) *fastTopLevelAccessCacheEntry {
	if head == nil || limit <= 0 {
		return nil
	}
	clone := &fastTopLevelAccessCacheEntry{
		typ:        head.typ,
		kind:       head.kind,
		fieldChain: head.fieldChain,
		chain:      head.chain,
		method:     head.method,
	}
	tail := clone
	count := 1
	for entry := head.next; entry != nil && count < limit; entry = entry.next {
		tail.next = &fastTopLevelAccessCacheEntry{
			typ:        entry.typ,
			kind:       entry.kind,
			fieldChain: entry.fieldChain,
			chain:      entry.chain,
			method:     entry.method,
		}
		tail = tail.next
		count++
	}
	return clone
}

func evalFastFieldChainValue(value *compiler.FastValuePlan, raw interface{}, ctx hctx.Context, runtimePlans ...*fastRuntimePlanCache) (interface{}, bool, error) {
	rv, nilValue, ok := fastFieldChainRootValue(raw)
	if nilValue {
		return nil, true, nil
	}
	if !ok {
		return nil, false, nil
	}
	chain, ok := fastFieldChainPlanFor(value, rv.Type(), runtimePlans...)
	if !ok {
		return nil, false, nil
	}
	for i := range chain.steps {
		step := &chain.steps[i]
		if err := spendFastTraversal(ctx, step.line); err != nil {
			return nil, true, err
		}
		rv = unwrapFastFieldChainValue(rv)
		field := rv.FieldByIndex(step.lookup.fieldIndex)
		if i == len(chain.steps)-1 {
			result, err := fastFieldValue(field, object.PropertyAccess{
				Receiver: step.receiver,
				Full:     step.full,
			}, step.name)
			if err != nil {
				return nil, true, fastLineError(step.line, err)
			}
			return result, true, nil
		}
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				return nil, true, nil
			}
			field = field.Elem()
		}
		if !field.CanInterface() {
			return nil, true, fastLineError(step.line, fieldAccessError(step.receiver, step.full, step.name))
		}
		rv = field
	}
	return raw, true, nil
}

func writeFastFieldChainValue(out *strings.Builder, ctx hctx.Context, value *compiler.FastValuePlan, raw interface{}, runtimePlans ...*fastRuntimePlanCache) (bool, error) {
	rv, nilValue, ok := fastFieldChainRootValue(raw)
	if nilValue {
		return true, nil
	}
	if !ok {
		return false, nil
	}
	chain, ok := fastFieldChainPlanFor(value, rv.Type(), runtimePlans...)
	if !ok {
		return false, nil
	}
	return true, writeFastFieldChainPlanOutput(out, ctx, chain, rv)
}

func writeFastFieldChainPlanOutput(out *strings.Builder, ctx hctx.Context, chain *fastFieldChainPlan, rv reflect.Value) error {
	if chain == nil {
		return nil
	}
	for i := range chain.steps {
		step := &chain.steps[i]
		if err := spendFastTraversal(ctx, step.line); err != nil {
			return err
		}
		rv = unwrapFastFieldChainValue(rv)
		if !rv.IsValid() {
			return nil
		}
		field := rv.FieldByIndex(step.lookup.fieldIndex)
		if i == len(chain.steps)-1 {
			written, err := writeFastField(out, ctx, field, object.PropertyAccess{
				Receiver: step.receiver,
				Full:     step.full,
			}, step.name, step.fieldType)
			if err != nil {
				return fastLineError(step.line, err)
			}
			if written {
				return nil
			}
			value, _ := fastFieldValue(field, object.PropertyAccess{
				Receiver: step.receiver,
				Full:     step.full,
			}, step.name)
			writeFastGoValue(out, ctx, value)
			return nil
		}
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				return nil
			}
			field = field.Elem()
		}
		if !field.CanInterface() {
			return fastLineError(step.line, fieldAccessError(step.receiver, step.full, step.name))
		}
		rv = field
	}
	return nil
}

func fastFieldChainRootValue(raw interface{}) (reflect.Value, bool, bool) {
	if obj, ok := raw.(object.Object); ok {
		if object.IsNull(obj) {
			return reflect.Value{}, true, true
		}
		raw = object.ToGo(obj)
	}
	if raw == nil {
		return reflect.Value{}, true, true
	}
	rv := reflect.ValueOf(raw)
	rv = unwrapFastFieldChainValue(rv)
	if !rv.IsValid() {
		return reflect.Value{}, true, true
	}
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}, false, false
	}
	return rv, false, true
}

func unwrapFastFieldChainValue(rv reflect.Value) reflect.Value {
	for rv.IsValid() && rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return reflect.Value{}
		}
		rv = rv.Elem()
	}
	return rv
}

func fastFieldChainPlanFor(value *compiler.FastValuePlan, root reflect.Type, runtimePlans ...*fastRuntimePlanCache) (*fastFieldChainPlan, bool) {
	if value == nil || value.Kind != compiler.FastValuePath || len(value.Path) == 0 {
		return nil, false
	}
	key := fastFieldChainPlanKey{plan: value, typ: root}
	runtimeCache := optionalFastRuntimePlanCache(runtimePlans)
	if chain, ok := runtimeCache.fieldChainPlan(key); ok {
		return chain, chain != nil
	}
	chain, ok := buildFastFieldChainPlan(value, root)
	if !ok {
		runtimeCache.storeFieldChainPlan(key, nil)
		return nil, false
	}
	chain = runtimeCache.storeFieldChainPlan(key, chain)
	return chain, chain != nil
}

func buildFastFieldChainPlan(value *compiler.FastValuePlan, root reflect.Type) (*fastFieldChainPlan, bool) {
	current := root
	chain := &fastFieldChainPlan{steps: make([]fastFieldChainStep, 0, len(value.Path))}
	for i := range value.Path {
		step := &value.Path[i]
		if step.Kind != compiler.FastPathStepProperty || step.Method {
			return nil, false
		}
		current = unwrapReflectType(current)
		if current.Kind() != reflect.Struct {
			return nil, false
		}
		lookup := cachedPropertyLookup(current, step.Value)
		if lookup.kind != propertyLookupField {
			return nil, false
		}
		field, _ := fieldByIndex(current, lookup.fieldIndex)
		chain.steps = append(chain.steps, fastFieldChainStep{
			name:      step.Value,
			receiver:  step.Receiver,
			full:      step.Full,
			line:      step.Line,
			fieldType: field.Type,
			lookup:    lookup,
		})
		current = field.Type
	}
	return chain, len(chain.steps) > 0
}

func evalFastAccessChainValue(value *compiler.FastValuePlan, raw interface{}, ctx hctx.Context, runtimePlans ...*fastRuntimePlanCache) (interface{}, bool, error) {
	rv, nilValue, ok := fastAccessChainRootValue(raw)
	if nilValue {
		return nil, true, nil
	}
	if !ok {
		return nil, false, nil
	}
	chain, ok := fastAccessChainPlanFor(value, rv.Type(), runtimePlans...)
	if !ok {
		return nil, false, nil
	}
	result, ok, err := evalFastAccessChainPlanValue(chain, rv, ctx)
	return result, ok, err
}

func writeFastAccessChainValue(out *strings.Builder, ctx hctx.Context, value *compiler.FastValuePlan, raw interface{}, runtimePlans ...*fastRuntimePlanCache) (bool, error) {
	rv, nilValue, ok := fastAccessChainRootValue(raw)
	if nilValue {
		return true, nil
	}
	if !ok {
		return false, nil
	}
	chain, ok := fastAccessChainPlanFor(value, rv.Type(), runtimePlans...)
	if !ok {
		return false, nil
	}
	return writeFastAccessChainPlanOutput(out, ctx, chain, rv)
}

func fastAccessChainRootValue(raw interface{}) (reflect.Value, bool, bool) {
	if obj, ok := raw.(object.Object); ok {
		if object.IsNull(obj) {
			return reflect.Value{}, true, true
		}
		raw = object.ToGo(obj)
	}
	if raw == nil {
		return reflect.Value{}, true, true
	}
	rv := reflect.ValueOf(raw)
	rv = unwrapFastFieldChainValue(rv)
	if !rv.IsValid() {
		return reflect.Value{}, true, true
	}
	switch rv.Kind() {
	case reflect.Struct, reflect.Array, reflect.Slice, reflect.Map:
		return rv, false, true
	default:
		return reflect.Value{}, false, false
	}
}

func fastAccessChainPlanFor(value *compiler.FastValuePlan, root reflect.Type, runtimePlans ...*fastRuntimePlanCache) (*fastAccessChainPlan, bool) {
	if value == nil || value.Kind != compiler.FastValuePath || len(value.Path) == 0 {
		return nil, false
	}
	key := fastAccessChainPlanKey{plan: value, typ: root}
	runtimeCache := optionalFastRuntimePlanCache(runtimePlans)
	if chain, ok := runtimeCache.accessChainPlan(key); ok {
		return chain, chain != nil
	}
	chain, _, ok := buildFastAccessChainPlanForSteps(value.Path, root)
	if !ok || len(chain.steps) == 0 {
		runtimeCache.storeAccessChainPlan(key, nil)
		return nil, false
	}
	chain = runtimeCache.storeAccessChainPlan(key, chain)
	return chain, chain != nil
}

func buildFastAccessChainPlanForSteps(steps []compiler.FastPathStep, root reflect.Type) (*fastAccessChainPlan, reflect.Type, bool) {
	current := root
	chain := &fastAccessChainPlan{steps: make([]fastAccessChainStep, 0, len(steps))}
	for i := range steps {
		step := &steps[i]
		current = unwrapReflectType(current)
		switch step.Kind {
		case compiler.FastPathStepProperty:
			if step.Method {
				return nil, nil, false
			}
			if current.Kind() == reflect.Map {
				indexStep, next, ok := buildFastIndexAccessStep(current, step)
				if !ok {
					return nil, nil, false
				}
				chain.steps = append(chain.steps, indexStep)
				current = next
				continue
			}
			if current.Kind() != reflect.Struct {
				return nil, nil, false
			}
			lookup := cachedPropertyLookup(current, step.Value)
			if lookup.kind != propertyLookupField {
				return nil, nil, false
			}
			field, _ := fieldByIndex(current, lookup.fieldIndex)
			chain.steps = append(chain.steps, fastAccessChainStep{
				kind:      fastAccessStepField,
				name:      step.Value,
				receiver:  step.Receiver,
				full:      step.Full,
				line:      step.Line,
				fieldType: field.Type,
				lookup:    lookup,
			})
			current = field.Type
		case compiler.FastPathStepIndexInteger, compiler.FastPathStepIndexString:
			indexStep, next, ok := buildFastIndexAccessStep(current, step)
			if !ok {
				return nil, nil, false
			}
			chain.steps = append(chain.steps, indexStep)
			current = next
		default:
			return nil, nil, false
		}
	}
	return chain, current, true
}

func buildFastIndexAccessStep(current reflect.Type, step *compiler.FastPathStep) (fastAccessChainStep, reflect.Type, bool) {
	current = unwrapReflectType(current)
	switch current.Kind() {
	case reflect.Array:
		if step.Kind != compiler.FastPathStepIndexInteger {
			return fastAccessChainStep{}, nil, false
		}
		if step.Index < 0 || step.Index >= current.Len() {
			return fastAccessChainStep{}, nil, false
		}
		return fastAccessChainStep{
			kind:       fastAccessStepIndex,
			line:       step.Line,
			index:      step.Index,
			resultType: current.Elem(),
		}, current.Elem(), true
	case reflect.Slice:
		if step.Kind != compiler.FastPathStepIndexInteger {
			return fastAccessChainStep{}, nil, false
		}
		if step.Index < 0 {
			return fastAccessChainStep{}, nil, false
		}
		return fastAccessChainStep{
			kind:       fastAccessStepIndex,
			line:       step.Line,
			index:      step.Index,
			resultType: current.Elem(),
		}, current.Elem(), true
	case reflect.Map:
		key, ok := fastAccessMapKeyValue(current.Key(), step)
		if !ok {
			return fastAccessChainStep{}, nil, false
		}
		return fastAccessChainStep{
			kind:       fastAccessStepIndex,
			line:       step.Line,
			index:      step.Index,
			mapKey:     key,
			mapString:  step.Value,
			mapDirect:  fastMapDirectKindFor(current, step),
			resultType: current.Elem(),
		}, current.Elem(), true
	default:
		return fastAccessChainStep{}, nil, false
	}
}

func fastMapDirectKindFor(mapType reflect.Type, step *compiler.FastPathStep) fastMapDirectKind {
	if mapType == nil || step == nil || mapType.Kind() != reflect.Map || mapType.Key() != stringType {
		return fastMapDirectNone
	}
	if step.Kind != compiler.FastPathStepIndexString && step.Kind != compiler.FastPathStepProperty {
		return fastMapDirectNone
	}
	switch mapType.Elem() {
	case stringType:
		return fastMapDirectStringString
	case reflect.TypeOf(int(0)):
		return fastMapDirectStringInt
	case reflect.TypeOf(uint32(0)):
		return fastMapDirectStringUint32
	case emptyInterfaceType:
		return fastMapDirectStringInterface
	default:
		return fastMapDirectNone
	}
}

func fastAccessMapKeyValue(keyType reflect.Type, step *compiler.FastPathStep) (reflect.Value, bool) {
	var key reflect.Value
	switch step.Kind {
	case compiler.FastPathStepIndexInteger:
		key = reflect.ValueOf(step.Index)
	case compiler.FastPathStepIndexString, compiler.FastPathStepProperty:
		key = reflect.ValueOf(step.Value)
	default:
		return reflect.Value{}, false
	}
	if !key.IsValid() || keyType == nil {
		return reflect.Value{}, false
	}
	if !key.Type().AssignableTo(keyType) {
		if key.Type().ConvertibleTo(keyType) {
			key = key.Convert(keyType)
		} else {
			return reflect.Value{}, false
		}
	}
	return key, true
}

func fastRuntimeMapKeyValue(keyType reflect.Type, step *fastAccessChainStep) (reflect.Value, bool) {
	if step == nil {
		return reflect.Value{}, false
	}
	if step.mapKey.IsValid() {
		return step.mapKey, true
	}
	key := reflect.ValueOf(step.index)
	if !key.IsValid() || keyType == nil {
		return reflect.Value{}, false
	}
	if key.Type() != keyType {
		if key.Type().ConvertibleTo(keyType) {
			key = key.Convert(keyType)
		} else if keyType.Kind() != reflect.Interface {
			return reflect.Value{}, false
		}
	}
	return key, true
}

func evalFastAccessChainPlanValue(chain *fastAccessChainPlan, rv reflect.Value, ctx hctx.Context) (interface{}, bool, error) {
	current := rv
	for i := range chain.steps {
		step := &chain.steps[i]
		last := i == len(chain.steps)-1
		switch step.kind {
		case fastAccessStepField:
			if err := spendFastTraversal(ctx, step.line); err != nil {
				return nil, true, err
			}
			field, ok, err := fastAccessFieldValue(current, step)
			if !ok || err != nil {
				return nil, true, err
			}
			if last {
				value, err := fastFieldValue(field, object.PropertyAccess{
					Receiver: step.receiver,
					Full:     step.full,
				}, step.name)
				if err != nil {
					return nil, true, fastLineError(step.line, err)
				}
				return value, true, nil
			}
			current, ok, err = fastAccessIntermediateValue(field, step)
			if !ok || err != nil {
				return nil, true, err
			}
		case fastAccessStepIndex:
			indexed, ok, err := fastAccessIndexValue(current, step)
			if err != nil {
				return nil, true, fastLineError(step.line, err)
			}
			if !ok {
				return nil, true, nil
			}
			if last {
				return fastReflectInterface(indexed), true, nil
			}
			current, ok, err = fastAccessIntermediateValue(indexed, step)
			if !ok || err != nil {
				return nil, true, err
			}
		}
	}
	return fastReflectInterface(current), true, nil
}

func writeFastAccessChainPlanOutput(out *strings.Builder, ctx hctx.Context, chain *fastAccessChainPlan, rv reflect.Value) (bool, error) {
	current := rv
	for i := range chain.steps {
		step := &chain.steps[i]
		last := i == len(chain.steps)-1
		switch step.kind {
		case fastAccessStepField:
			if err := spendFastTraversal(ctx, step.line); err != nil {
				return true, err
			}
			field, ok, err := fastAccessFieldValue(current, step)
			if !ok || err != nil {
				return true, err
			}
			if last {
				written, err := writeFastField(out, ctx, field, object.PropertyAccess{
					Receiver: step.receiver,
					Full:     step.full,
				}, step.name, step.fieldType)
				if err != nil {
					return true, fastLineError(step.line, err)
				}
				if written {
					return true, nil
				}
				value, _ := fastFieldValue(field, object.PropertyAccess{
					Receiver: step.receiver,
					Full:     step.full,
				}, step.name)
				writeFastGoValue(out, ctx, value)
				return true, nil
			}
			current, ok, err = fastAccessIntermediateValue(field, step)
			if !ok || err != nil {
				return true, err
			}
		case fastAccessStepIndex:
			if last {
				if handled, err := writeFastDirectMapIndexOutput(out, ctx, current, step); handled || err != nil {
					return true, err
				}
			}
			indexed, ok, err := fastAccessIndexValue(current, step)
			if err != nil {
				return true, fastLineError(step.line, err)
			}
			if !ok {
				return true, nil
			}
			if last {
				return true, writeFastReflectValue(out, ctx, indexed)
			}
			current, ok, err = fastAccessIntermediateValue(indexed, step)
			if !ok || err != nil {
				return true, err
			}
		}
	}
	return true, nil
}

func fastAccessFieldValue(current reflect.Value, step *fastAccessChainStep) (reflect.Value, bool, error) {
	current = unwrapFastFieldChainValue(current)
	if !current.IsValid() {
		return reflect.Value{}, false, nil
	}
	if current.Kind() != reflect.Struct {
		return reflect.Value{}, false, nil
	}
	return current.FieldByIndex(step.lookup.fieldIndex), true, nil
}

func fastAccessIndexValue(current reflect.Value, step *fastAccessChainStep) (reflect.Value, bool, error) {
	current = unwrapFastFieldChainValue(current)
	if !current.IsValid() {
		return reflect.Value{}, false, nil
	}
	switch current.Kind() {
	case reflect.Array, reflect.Slice:
		if step.index < 0 || step.index >= current.Len() {
			return reflect.Value{}, false, fmt.Errorf("array index out of bounds, got index %d, while array size is %d", step.index, current.Len())
		}
		return current.Index(step.index), true, nil
	case reflect.Map:
		if value, found, handled := fastAccessDirectMapIndex(current, step); handled {
			return value, found, nil
		}
		key, ok := fastRuntimeMapKeyValue(current.Type().Key(), step)
		if !ok {
			return reflect.Value{}, false, nil
		}
		value := current.MapIndex(key)
		if !value.IsValid() {
			return reflect.Value{}, false, nil
		}
		return value, true, nil
	default:
		return reflect.Value{}, false, nil
	}
}

func fastAccessDirectMapIndex(current reflect.Value, step *fastAccessChainStep) (reflect.Value, bool, bool) {
	if step == nil || step.mapDirect == fastMapDirectNone || !current.IsValid() || current.Kind() != reflect.Map || !current.CanInterface() {
		return reflect.Value{}, false, false
	}
	key := step.mapString
	switch step.mapDirect {
	case fastMapDirectStringString:
		m, ok := current.Interface().(map[string]string)
		if !ok {
			return reflect.Value{}, false, false
		}
		value, found := m[key]
		if !found {
			return reflect.Value{}, false, true
		}
		return reflect.ValueOf(value), true, true
	case fastMapDirectStringInt:
		m, ok := current.Interface().(map[string]int)
		if !ok {
			return reflect.Value{}, false, false
		}
		value, found := m[key]
		if !found {
			return reflect.Value{}, false, true
		}
		return reflect.ValueOf(value), true, true
	case fastMapDirectStringUint32:
		m, ok := current.Interface().(map[string]uint32)
		if !ok {
			return reflect.Value{}, false, false
		}
		value, found := m[key]
		if !found {
			return reflect.Value{}, false, true
		}
		return reflect.ValueOf(value), true, true
	case fastMapDirectStringInterface:
		m, ok := current.Interface().(map[string]interface{})
		if !ok {
			return reflect.Value{}, false, false
		}
		value, found := m[key]
		if !found {
			return reflect.Value{}, false, true
		}
		if value == nil {
			return reflect.Zero(emptyInterfaceType), true, true
		}
		return reflect.ValueOf(value), true, true
	default:
		return reflect.Value{}, false, false
	}
}

func writeFastDirectMapIndexOutput(out *strings.Builder, ctx hctx.Context, current reflect.Value, step *fastAccessChainStep) (bool, error) {
	if step == nil || step.mapDirect == fastMapDirectNone || !current.IsValid() || current.Kind() != reflect.Map || !current.CanInterface() {
		return false, nil
	}
	key := step.mapString
	switch step.mapDirect {
	case fastMapDirectStringString:
		m, ok := current.Interface().(map[string]string)
		if !ok {
			return false, nil
		}
		value, found := m[key]
		if found {
			writeFastEscapedString(out, value)
		}
		return true, nil
	case fastMapDirectStringInt:
		m, ok := current.Interface().(map[string]int)
		if !ok {
			return false, nil
		}
		value, found := m[key]
		if found {
			writeBuilderFastInt(out, int64(value))
		}
		return true, nil
	case fastMapDirectStringUint32:
		m, ok := current.Interface().(map[string]uint32)
		if !ok {
			return false, nil
		}
		value, found := m[key]
		if found {
			writeBuilderFastUint(out, uint64(value))
		}
		return true, nil
	case fastMapDirectStringInterface:
		m, ok := current.Interface().(map[string]interface{})
		if !ok {
			return false, nil
		}
		value, found := m[key]
		if found {
			writeFastGoValue(out, ctx, value)
		}
		return true, nil
	default:
		return false, nil
	}
}

func fastAccessIntermediateValue(value reflect.Value, step *fastAccessChainStep) (reflect.Value, bool, error) {
	value = unwrapFastFieldChainValue(value)
	if !value.IsValid() {
		return reflect.Value{}, false, nil
	}
	if !value.CanInterface() {
		return reflect.Value{}, false, fastLineError(step.line, fieldAccessError(step.receiver, step.full, step.name))
	}
	return value, true, nil
}

func fastReflectInterface(value reflect.Value) interface{} {
	if !value.IsValid() || isNilReflectValue(value) {
		return nil
	}
	if value.Kind() == reflect.Interface {
		value = value.Elem()
	}
	if !value.CanInterface() {
		return nil
	}
	return value.Interface()
}

func writeFastReflectValue(out *strings.Builder, ctx hctx.Context, value reflect.Value) error {
	value = unwrapFastFieldChainValue(value)
	if !value.IsValid() || isNilReflectValue(value) {
		return nil
	}
	if value.Kind() == reflect.Interface {
		value = value.Elem()
	}
	if !value.CanInterface() {
		return nil
	}
	if value.Type() == templateHTMLType {
		out.WriteString(value.String())
		return nil
	}
	if value.Type().Implements(objectInterfaceType) {
		if obj, ok := value.Interface().(object.Object); ok {
			writeFastObject(out, ctx, obj)
			return nil
		}
	}
	if value.Type().PkgPath() == "" {
		switch value.Kind() {
		case reflect.String:
			writeFastEscapedString(out, value.String())
			return nil
		case reflect.Bool:
			out.WriteString(strconv.FormatBool(value.Bool()))
			return nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			writeBuilderFastInt(out, value.Int())
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			writeBuilderFastUint(out, value.Uint())
			return nil
		case reflect.Float32, reflect.Float64:
			writeBuilderFastFloat(out, value.Float(), int(value.Type().Bits()))
			return nil
		}
	}
	raw := value.Interface()
	writeFastGoValue(out, ctx, raw)
	return nil
}

func unwrapReflectType(rt reflect.Type) reflect.Type {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	return rt
}

func fieldAccessError(receiver, full, name string) error {
	if receiver != "" && full != "" {
		return fmt.Errorf("'%s'cannot return value obtained from unexported field or method '%s' (%s)", receiver, name, full)
	}
	return fmt.Errorf("cannot return value obtained from unexported field or method '%s'", name)
}

func fastPathStepsNeedRuntimeBindings(steps []compiler.FastPathStep) bool {
	for i := range steps {
		step := &steps[i]
		if len(step.Args) > 0 {
			return true
		}
	}
	return false
}

func evalFastPathSteps(base interface{}, steps []compiler.FastPathStep, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue interface{}, loopAware bool) (interface{}, error) {
	raw := base
	for i := range steps {
		step := &steps[i]
		var err error
		raw, err = evalFastPathStepWithBindings(raw, step, ctx, bindings, loopKey, loopValue, loopAware)
		if err != nil {
			return nil, err
		}
	}
	return raw, nil
}

func evalFastPathStep(base interface{}, step *compiler.FastPathStep, ctx hctx.Context) (interface{}, error) {
	bindings := fastRenderBindings{
		ctx:        ctx,
		vmHotspots: plush.CaptureRenderVMHotspotDiagnostics(ctx),
	}
	return evalFastPathStepWithBindings(base, step, ctx, bindings, nil, nil, false)
}

func evalFastPathStepWithBindings(base interface{}, step *compiler.FastPathStep, ctx hctx.Context, bindings fastRenderBindings, loopKey, loopValue interface{}, loopAware bool) (interface{}, error) {
	switch step.Kind {
	case compiler.FastPathStepProperty:
		if err := spendFastTraversal(ctx, step.Line); err != nil {
			return nil, err
		}
		value, err := fastPropertyValue(base, step.Value, object.PropertyAccess{
			Receiver: step.Receiver,
			Full:     step.Full,
			Method:   step.Method,
		}, &step.PropertyCache)
		if err != nil {
			return nil, fastLineError(step.Line, err)
		}
		return value, nil
	case compiler.FastPathStepIndexInteger:
		value, err := fastIndexValue(base, step.Index)
		if err != nil {
			return nil, fastLineError(step.Line, err)
		}
		return value, nil
	case compiler.FastPathStepIndexString:
		value, err := fastStringIndexValue(base, step.Value)
		if err != nil {
			return nil, fastLineError(step.Line, err)
		}
		return value, nil
	case compiler.FastPathStepCall:
		if err := spendFastFunctionCall(ctx, step.Value, step.Line); err != nil {
			return nil, err
		}
		var args *fastCallArgs
		var argStore fastCallArgs
		if len(step.Args) > 0 {
			for i := range step.Args {
				arg, ok, err := evalFastCompoundOperand(&step.Args[i], ctx, bindings, loopKey, loopValue, nil, loopAware)
				if err != nil {
					return nil, err
				}
				if !ok {
					if step.Args[i].NullOnMissing {
						argStore.Append(nil)
						continue
					}
					return nil, fastLineError(step.Args[i].Line, fmt.Errorf("%q: unknown identifier", fastValueMissingName(&step.Args[i])))
				}
				argStore.Append(arg)
			}
			args = &argStore
		}
		value, err := fastCallValueWithDiagnostics(step.Value, base, args, ctx, &step.CallCache, bindings.vmHotspots)
		if err != nil {
			return nil, fastLineError(step.Line, err)
		}
		return value, nil
	default:
		return nil, nil
	}
}

func fastIndexValue(left interface{}, index int) (interface{}, error) {
	if obj, ok := left.(object.Object); ok {
		switch obj := obj.(type) {
		case *object.Array:
			if index < 0 || index >= len(obj.Elements) {
				return nil, nil
			}
			return obj.Elements[index], nil
		case *object.Hash:
			key := (&object.Integer{Value: int64(index)}).HashKey()
			if pair, ok := obj.Pairs[key]; ok {
				return pair.Value, nil
			}
			return nil, nil
		default:
			left = object.ToGo(obj)
		}
	}
	if left == nil {
		return nil, nil
	}
	rv := reflect.ValueOf(left)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		if index < 0 || index >= rv.Len() {
			return nil, fmt.Errorf("array index out of bounds, got index %d, while array size is %d", index, rv.Len())
		}
		return rv.Index(index).Interface(), nil
	case reflect.Map:
		key := reflect.ValueOf(index)
		if key.Type() != rv.Type().Key() {
			if key.Type().ConvertibleTo(rv.Type().Key()) {
				key = key.Convert(rv.Type().Key())
			} else if rv.Type().Key().Kind() != reflect.Interface {
				return nil, fmt.Errorf("cannot use %v (%s constant) as %s value in map index", index, key.Kind().String(), rv.Type().Key().Kind().String())
			}
		}
		val := rv.MapIndex(key)
		if !val.IsValid() {
			return nil, nil
		}
		return val.Interface(), nil
	default:
		return nil, fmt.Errorf("could not index %T with %T", left, index)
	}
}

func fastStringIndexValue(left interface{}, index string) (interface{}, error) {
	if obj, ok := left.(object.Object); ok {
		switch obj := obj.(type) {
		case *object.Hash:
			key := (&object.String{Value: index}).HashKey()
			if pair, ok := obj.Pairs[key]; ok {
				return pair.Value, nil
			}
			return nil, nil
		default:
			left = object.ToGo(obj)
		}
	}
	if left == nil {
		return nil, nil
	}
	rv := reflect.ValueOf(left)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Map:
		key := reflect.ValueOf(index)
		if key.Type() != rv.Type().Key() {
			if key.Type().ConvertibleTo(rv.Type().Key()) {
				key = key.Convert(rv.Type().Key())
			} else if rv.Type().Key().Kind() != reflect.Interface {
				return nil, fmt.Errorf("cannot use %v (%s constant) as %s value in map index", index, key.Kind().String(), rv.Type().Key().Kind().String())
			}
		}
		val := rv.MapIndex(key)
		if !val.IsValid() {
			return nil, nil
		}
		return val.Interface(), nil
	case reflect.Array, reflect.Slice:
		return nil, fmt.Errorf("can't access Slice/Array with a non int Index (%v)", index)
	default:
		return nil, fmt.Errorf("could not index %T with %T", left, index)
	}
}

func fastIndexGoValue(left, index interface{}) (interface{}, error) {
	switch index := index.(type) {
	case int:
		return fastIndexValue(left, index)
	case int8:
		return fastIndexValue(left, int(index))
	case int16:
		return fastIndexValue(left, int(index))
	case int32:
		return fastIndexValue(left, int(index))
	case int64:
		return fastIndexValue(left, int(index))
	case uint:
		return fastIndexValue(left, int(index))
	case uint8:
		return fastIndexValue(left, int(index))
	case uint16:
		return fastIndexValue(left, int(index))
	case uint32:
		return fastIndexValue(left, int(index))
	case uint64:
		return fastIndexValue(left, int(index))
	case string:
		return fastStringIndexValue(left, index)
	}
	if obj, ok := left.(object.Object); ok {
		switch obj := obj.(type) {
		case *object.Hash:
			key := object.Wrap(index)
			hashKey, ok := key.(object.Hashable)
			if !ok {
				return nil, fmt.Errorf("unusable as hash key: %s", key.Type())
			}
			if pair, ok := obj.Pairs[hashKey.HashKey()]; ok {
				return object.ToGo(pair.Value), nil
			}
			return nil, nil
		default:
			left = object.ToGo(obj)
		}
	}
	rv, nilValue, ok := fastIndexRootValue(left)
	if nilValue {
		return nil, nil
	}
	if !ok {
		return nil, fmt.Errorf("could not index %T with %T", left, index)
	}
	if rv.Kind() != reflect.Map {
		return nil, fmt.Errorf("can't access Slice/Array with a non int Index (%v)", index)
	}
	key, err := fastReflectMapKey(rv.Type().Key(), index)
	if err != nil {
		return nil, err
	}
	val := rv.MapIndex(key)
	if !val.IsValid() {
		return nil, nil
	}
	return fastReflectInterface(val), nil
}

func setFastIndexGoValue(left, index, value interface{}) error {
	raw := left
	switch left := raw.(type) {
	case *object.Array:
		i, ok := toInt(object.Wrap(index))
		if !ok {
			return fmt.Errorf("can't access Slice/Array with a non int Index (%v)", index)
		}
		if i < 0 || int(i) >= len(left.Elements) {
			return fmt.Errorf("array index out of bounds, got index %d, while array size is %d", i, len(left.Elements))
		}
		left.Elements[int(i)] = object.Wrap(value)
		return nil
	case *object.Hash:
		key := object.Wrap(index)
		hashKey, ok := key.(object.Hashable)
		if !ok {
			return fmt.Errorf("unusable as hash key: %s", key.Type())
		}
		left.Pairs[hashKey.HashKey()] = object.HashPair{Key: key, Value: object.Wrap(value)}
		return nil
	case object.Object:
		raw = object.ToGo(left)
	}
	rv, nilValue, ok := fastIndexRootValue(raw)
	if nilValue || !ok {
		return fmt.Errorf("could not index %T with %T", raw, index)
	}
	switch rv.Kind() {
	case reflect.Map:
		key, err := fastReflectMapKey(rv.Type().Key(), index)
		if err != nil {
			return err
		}
		val, err := fastReflectAssignableValue(value, rv.Type().Elem())
		if err != nil {
			return err
		}
		rv.SetMapIndex(key, val)
		return nil
	case reflect.Array, reflect.Slice:
		i, ok := fastReflectIndexInt(index)
		if !ok {
			return fmt.Errorf("can't access Slice/Array with a non int Index (%v)", index)
		}
		if i < 0 || rv.Len()-1 < i {
			return fmt.Errorf("array index out of bounds, got index %d, while array size is %d", i, rv.Len())
		}
		slot := rv.Index(i)
		if !slot.CanSet() {
			return fmt.Errorf("cannot assign to index %d of %T", i, raw)
		}
		val, err := fastReflectAssignableValue(value, slot.Type())
		if err != nil {
			return err
		}
		slot.Set(val)
		return nil
	default:
		return fmt.Errorf("could not index %T with %T", raw, index)
	}
}

func fastIndexRootValue(raw interface{}) (reflect.Value, bool, bool) {
	if raw == nil {
		return reflect.Value{}, true, true
	}
	rv := reflect.ValueOf(raw)
	rv = unwrapFastFieldChainValue(rv)
	if !rv.IsValid() {
		return reflect.Value{}, true, true
	}
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		return rv, false, true
	default:
		return reflect.Value{}, false, false
	}
}

func fastReflectMapKey(keyType reflect.Type, index interface{}) (reflect.Value, error) {
	key := reflect.ValueOf(index)
	if !key.IsValid() {
		return reflect.Value{}, fmt.Errorf("cannot use <nil> as %s value in map index", keyType)
	}
	if key.Type().AssignableTo(keyType) {
		return key, nil
	}
	if key.Type().ConvertibleTo(keyType) {
		return key.Convert(keyType), nil
	}
	if keyType.Kind() != reflect.Interface && key.Kind() != keyType.Kind() {
		return reflect.Value{}, fmt.Errorf("cannot use %v (%s constant) as %s value in map index", index, key.Kind().String(), keyType.String())
	}
	return reflect.Value{}, fmt.Errorf("cannot use %v (%s constant) as %s value in map index", index, key.Type().String(), keyType.String())
}

func fastReflectIndexInt(index interface{}) (int, bool) {
	switch index := index.(type) {
	case int:
		return index, true
	case int8:
		return int(index), true
	case int16:
		return int(index), true
	case int32:
		return int(index), true
	case int64:
		return int(index), true
	case uint:
		return int(index), true
	case uint8:
		return int(index), true
	case uint16:
		return int(index), true
	case uint32:
		return int(index), true
	case uint64:
		return int(index), true
	default:
		return 0, false
	}
}

func fastReflectAssignableValue(value interface{}, target reflect.Type) (reflect.Value, error) {
	if value == nil {
		switch target.Kind() {
		case reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice, reflect.Func:
			return reflect.Zero(target), nil
		default:
			return reflect.Value{}, fmt.Errorf("cannot use '<nil>' as %s value in assignment", target)
		}
	}
	val := reflect.ValueOf(value)
	if val.Type().AssignableTo(target) {
		return val, nil
	}
	if val.Type().ConvertibleTo(target) {
		return val.Convert(target), nil
	}
	return reflect.Value{}, fmt.Errorf("cannot use '%v' (untyped %s constant) as %s value in assignment", value, val.Type(), target)
}
