package vm

import (
	"errors"
	"fmt"
	"html/template"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
)

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.lastPopped
}

func (vm *VM) PunchHoles() []plush.HoleMarker {
	if vm.holes == nil {
		return nil
	}
	return append([]plush.HoleMarker(nil), (*vm.holes)...)
}

func (vm *VM) ensureGlobal(index int) {
	if index < 0 || index < len(vm.globals) {
		return
	}
	globals := make([]object.Object, index+1)
	copy(globals, vm.globals)
	vm.globals = globals
}

func (vm *VM) globalValue(index int) object.Object {
	if index < 0 || index >= len(vm.globals) {
		return nil
	}
	return vm.globals[index]
}

func (vm *VM) Rendered() string {
	if vm.frames[0] == nil {
		return ""
	}
	if vm.halted && !object.IsNull(vm.lastPopped) {
		return vm.renderObject(vm.lastPopped)
	}
	return vm.frames[0].output.String()
}

func (vm *VM) Run() error {
	var ip int
	var ins code.Instructions
	var op code.Opcode

	for !vm.halted && vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		vm.currentFrame().ip++

		ip = vm.currentFrame().ip
		vm.lastIP = ip
		ins = vm.currentFrame().Instructions()
		op = code.Opcode(ins[ip])

		switch op {
		case code.OpConstant:
			constIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2
			if err := vm.push(vm.constants[constIndex]); err != nil {
				return err
			}

		case code.OpPop:
			vm.discard()

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv:
			if err := vm.executeBinaryOperation(op); err != nil {
				return err
			}

		case code.OpTrue:
			if err := vm.push(True); err != nil {
				return err
			}

		case code.OpFalse:
			if err := vm.push(False); err != nil {
				return err
			}

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan,
			code.OpGreaterEqual,
			code.OpMatches, code.OpAnd, code.OpOr:
			if err := vm.executeComparisonOrLogical(op); err != nil {
				return err
			}

		case code.OpBang:
			_ = vm.executeBangOperator()

		case code.OpMinus:
			if err := vm.executeMinusOperator(); err != nil {
				return err
			}

		case code.OpJump:
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip = pos - 1

		case code.OpJumpNotTruthy:
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			if err := vm.spendCondition(); err != nil {
				return err
			}

			condition := vm.pop()
			if !isTruthy(condition) {
				vm.currentFrame().ip = pos - 1
			}

		case code.OpNull:
			if err := vm.push(Null); err != nil {
				return err
			}

		case code.OpSetGlobal:
			globalIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			if err := vm.spendAssignment(); err != nil {
				return err
			}
			value := vm.pop()
			vm.ensureGlobal(globalIndex)
			vm.globals[globalIndex] = value
			vm.updateNamedGlobal(globalIndex, value)

		case code.OpGetGlobal:
			globalIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			value := vm.globalValue(globalIndex)
			if value == nil {
				value = vm.globalFromContext(globalIndex)
			}
			if value == nil {
				value = Null
			}
			if err := vm.push(value); err != nil {
				return err
			}

		case code.OpArray:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			array := vm.buildArray(vm.sp-numElements, vm.sp)
			vm.sp = vm.sp - numElements
			if err := vm.push(array); err != nil {
				return err
			}

		case code.OpHash:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			hash, err := vm.buildHash(vm.sp-numElements, vm.sp)
			if err != nil {
				return err
			}
			vm.sp = vm.sp - numElements
			if err := vm.push(hash); err != nil {
				return err
			}

		case code.OpIndex:
			index := vm.pop()
			left := vm.pop()
			if err := vm.executeIndexExpression(left, index); err != nil {
				return err
			}

		case code.OpSetIndex:
			value := vm.pop()
			index := vm.pop()
			left := vm.pop()
			if err := vm.executeSetIndex(left, index, value); err != nil {
				return err
			}
			_ = vm.push(Null)

		case code.OpCall:
			numArgs := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			if err := vm.executeCall(vm.currentCallName(ip), int(numArgs), nil, vm.currentCallCacheSlot(ip)); err != nil {
				return err
			}

		case code.OpWriteCall:
			numArgs := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			if err := vm.executeWriteCall(vm.currentCallName(ip), int(numArgs), vm.currentCallCacheSlot(ip)); err != nil {
				return err
			}

		case code.OpWriteNameCall:
			nameIndex := int(code.ReadUint16(ins[ip+1:]))
			numArgs := code.ReadUint8(ins[ip+3:])
			vm.currentFrame().ip += 3
			if err := vm.executeWriteNameCall(nameIndex, int(numArgs), vm.currentCallCacheSlot(ip)); err != nil {
				return err
			}

		case code.OpCallBlock:
			numArgs := code.ReadUint8(ins[ip+1:])
			blockIndex := int(code.ReadUint16(ins[ip+2:]))
			numFree := int(code.ReadUint8(ins[ip+4:]))
			vm.currentFrame().ip += 4
			block, err := vm.closureFromStack(blockIndex, numFree)
			if err != nil {
				return err
			}
			if err := vm.executeCall(vm.currentCallName(ip), int(numArgs), block, vm.currentCallCacheSlot(ip)); err != nil {
				return err
			}

		case code.OpReturnValue:
			returnValue := vm.pop()
			if err := vm.returnFromFrame(returnValue); err != nil {
				return err
			}

		case code.OpReturn:
			if err := vm.returnFromFrameValue(vm.currentFrameOutput(), false); err != nil {
				return err
			}

		case code.OpSetLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			if err := vm.spendAssignment(); err != nil {
				return err
			}
			frame := vm.currentFrame()
			vm.stack[frame.basePointer+int(localIndex)] = vm.pop()

		case code.OpGetLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			frame := vm.currentFrame()
			if err := vm.push(vm.stack[frame.basePointer+int(localIndex)]); err != nil {
				return err
			}

		case code.OpGetBuiltin:
			builtinIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			definition := object.Builtins[builtinIndex]
			if vm.ctx != nil && vm.ctx.Has(definition.Name) {
				if err := vm.push(object.Wrap(vm.ctx.Value(definition.Name))); err != nil {
					return err
				}
				break
			}
			if err := vm.push(definition.Builtin); err != nil {
				return err
			}

		case code.OpClosure:
			constIndex := code.ReadUint16(ins[ip+1:])
			numFree := code.ReadUint8(ins[ip+3:])
			vm.currentFrame().ip += 3
			if err := vm.pushClosure(int(constIndex), int(numFree)); err != nil {
				return err
			}

		case code.OpGetFree:
			freeIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			currentClosure := vm.currentFrame().cl
			if err := vm.push(currentClosure.Free[freeIndex]); err != nil {
				return err
			}

		case code.OpCurrentClosure:
			if err := vm.push(vm.currentFrame().cl); err != nil {
				return err
			}

		case code.OpGetName:
			nameIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			if err := vm.pushName(nameIndex); err != nil {
				return err
			}

		case code.OpGetNameOrNull:
			nameIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			if err := vm.pushNameOrNull(nameIndex); err != nil {
				return err
			}

		case code.OpSetName:
			nameIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			vm.setName(nameIndex, vm.pop())

		case code.OpAssignName:
			nameIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			if err := vm.assignName(nameIndex, vm.pop()); err != nil {
				return err
			}
			_ = vm.push(Null)

		case code.OpGetProperty:
			nameIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			obj := vm.pop()
			if err := vm.getProperty(obj, vm.stringConstant(nameIndex), vm.currentPropertyAccess(ip), vm.currentPropertyCacheSlot(ip)); err != nil {
				return err
			}

		case code.OpWrite:
			value := vm.pop()
			vm.writeFrameOutput(vm.currentFrame(), value)

		case code.OpWriteConstant:
			constIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			vm.writeConstant(vm.currentFrame(), constIndex)

		case code.OpWriteName:
			nameIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			if err := vm.writeName(vm.currentFrame(), nameIndex, false); err != nil {
				return err
			}

		case code.OpWriteNameOrNull:
			nameIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			_ = vm.writeName(vm.currentFrame(), nameIndex, true)

		case code.OpWriteLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			frame := vm.currentFrame()
			vm.writeFrameOutput(frame, vm.stack[frame.basePointer+int(localIndex)])

		case code.OpWriteGlobal:
			globalIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			vm.writeGlobal(vm.currentFrame(), globalIndex)

		case code.OpWriteLocalProperty:
			localIndex := code.ReadUint8(ins[ip+1:])
			nameIndex := int(code.ReadUint16(ins[ip+2:]))
			vm.currentFrame().ip += 3
			if err := vm.writeLocalProperty(vm.currentFrame(), int(localIndex), nameIndex, ip); err != nil {
				return err
			}

		case code.OpWriteGlobalProperty:
			globalIndex := int(code.ReadUint16(ins[ip+1:]))
			nameIndex := int(code.ReadUint16(ins[ip+3:]))
			vm.currentFrame().ip += 4
			if err := vm.writeGlobalProperty(vm.currentFrame(), globalIndex, nameIndex, ip); err != nil {
				return err
			}

		case code.OpWriteNameProperty:
			baseNameIndex := int(code.ReadUint16(ins[ip+1:]))
			propertyNameIndex := int(code.ReadUint16(ins[ip+3:]))
			vm.currentFrame().ip += 4
			if err := vm.writeNameProperty(vm.currentFrame(), baseNameIndex, propertyNameIndex, ip); err != nil {
				return err
			}

		case code.OpWriteString:
			constIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			vm.writeStringConstant(vm.currentFrame(), constIndex)

		case code.OpWriteHTML:
			constIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			vm.writeHTMLConstant(vm.currentFrame(), constIndex)

		case code.OpRenderTemplate:
			tmpl := fmt.Sprint(object.ToGo(vm.pop()))
			rendered, err := Render(tmpl, vm.ctx)
			if err != nil {
				return err
			}
			_ = vm.push(object.Wrap(template.HTML(rendered)))

		case code.OpHole:
			inputIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			vm.writeFrameOutput(vm.currentFrame(), &object.Native{
				Value: vmHole{input: vm.stringConstant(inputIndex)},
			})

		case code.OpFor:
			blockIndex := int(code.ReadUint16(ins[ip+1:]))
			keyNameIndex := int(code.ReadUint16(ins[ip+3:]))
			valueNameIndex := int(code.ReadUint16(ins[ip+5:]))
			numFree := int(code.ReadUint8(ins[ip+7:]))
			vm.currentFrame().ip += 7
			block, err := vm.closureFromStack(blockIndex, numFree)
			if err != nil {
				return err
			}
			iterable := vm.pop()
			result, err := vm.executeFor(iterable, block, vm.stringConstant(keyNameIndex), vm.stringConstant(valueNameIndex))
			if err != nil {
				return err
			}
			_ = vm.push(result)

		case code.OpBreak:
			control := &object.Control{Kind: object.ControlBreak, Value: vm.currentFrameOutputValues()}
			if err := vm.returnFromFrame(control); err != nil {
				return err
			}

		case code.OpContinue:
			control := &object.Control{Kind: object.ControlContinue, Value: vm.currentFrameOutputValues()}
			if err := vm.returnFromFrame(control); err != nil {
				return err
			}
		}
	}

	return nil
}

func (vm *VM) push(o object.Object) error {
	if o == nil {
		o = Null
	}
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}

	vm.stack[vm.sp] = o
	vm.sp++
	vm.markStack()
	return nil
}

func (vm *VM) pop() object.Object {
	o := vm.stack[vm.sp-1]
	vm.sp--
	vm.lastPopped = o
	vm.stack[vm.sp] = nil
	return o
}

func (vm *VM) markStack() {
	if vm.sp > vm.stackMax {
		vm.stackMax = vm.sp
	}
}

func (vm *VM) discard() {
	frame := vm.currentFrame()
	stackBase := frame.basePointer + frame.cl.Fn.NumLocals
	if vm.sp <= stackBase {
		vm.lastPopped = Null
		return
	}
	vm.pop()
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

func (vm *VM) currentCallName(ip int) string {
	frame := vm.currentFrame()
	if frame != nil && frame.cl != nil && frame.cl.Fn != nil {
		if name := frame.cl.Fn.CallNames[ip]; name != "" {
			return name
		}
	}
	return anonymousCallName
}

func (vm *VM) wrapRuntimeError(err error) error {
	if err == nil {
		return nil
	}
	if strings.HasPrefix(err.Error(), "line ") {
		return err
	}
	line := vm.currentLineNumber()
	if line <= 0 {
		line = 1
	}
	return fmt.Errorf("line %d: %w", line, err)
}

func (vm *VM) currentLineNumber() int {
	frame := vm.currentFrame()
	if frame == nil || frame.cl == nil || frame.cl.Fn == nil {
		return 0
	}
	if line := frame.cl.Fn.LineNumbers[vm.lastIP]; line > 0 {
		return line
	}
	return 0
}

func (vm *VM) currentPropertyAccess(ip int) object.PropertyAccess {
	frame := vm.currentFrame()
	if frame == nil || frame.cl == nil || frame.cl.Fn == nil {
		return object.PropertyAccess{}
	}
	return frame.cl.Fn.Properties[ip]
}

func (vm *VM) currentPropertyCacheSlot(ip int) *object.InlineCacheSlot {
	frame := vm.currentFrame()
	if frame == nil || frame.cl == nil || frame.cl.Fn == nil {
		return nil
	}
	if ip < 0 || ip >= len(frame.cl.Fn.PropertyCaches) {
		return nil
	}
	return &frame.cl.Fn.PropertyCaches[ip]
}

func (vm *VM) currentCallCacheSlot(ip int) *object.InlineCacheSlot {
	frame := vm.currentFrame()
	if frame == nil || frame.cl == nil || frame.cl.Fn == nil {
		return nil
	}
	if ip < 0 || ip >= len(frame.cl.Fn.CallCaches) {
		return nil
	}
	return &frame.cl.Fn.CallCaches[ip]
}

func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}

func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	frame := vm.frames[vm.framesIndex]
	vm.frames[vm.framesIndex] = nil
	return frame
}

func (vm *VM) returnFromFrame(returnValue object.Object) error {
	return vm.returnFromFrameValue(returnValue, true)
}

func (vm *VM) returnFromFrameValue(returnValue object.Object, combineOutput bool) error {
	frame := vm.currentFrame()
	writeReturn := frame != nil && frame.writeReturn
	if combineOutput {
		if _, isControl := returnValue.(*object.Control); !isControl {
			returnValue = vm.combineFrameOutput(frame, returnValue)
		}
	}

	if vm.framesIndex == 1 {
		vm.lastPopped = returnValue
		vm.halted = true
		return nil
	}

	frame = vm.popFrame()
	basePointer := frame.basePointer
	calleeOnStack := frame.calleeOnStack
	releaseFrame(frame)
	vm.sp = basePointer
	if calleeOnStack {
		vm.sp--
	}
	if writeReturn {
		vm.writeFrameOutput(vm.currentFrame(), returnValue)
		return nil
	}
	return vm.push(returnValue)
}

func (vm *VM) currentFrameOutput() object.Object {
	return vm.frameOutputObject(vm.currentFrame())
}

func (vm *VM) combineFrameOutput(frame *Frame, returnValue object.Object) object.Object {
	if !frame.hasOutput {
		if returnValue == nil {
			return Null
		}
		return returnValue
	}

	values := []object.Object{vm.frameOutputObject(frame)}
	if !object.IsNull(returnValue) {
		values = append(values, returnValue)
	}
	return &object.Array{Elements: values}
}

func (vm *VM) frameOutputObject(frame *Frame) object.Object {
	if frame == nil || !frame.hasOutput {
		return Null
	}
	return &object.Native{Value: template.HTML(frame.output.String())}
}

func (vm *VM) currentFrameOutputValues() []object.Object {
	frame := vm.currentFrame()
	if frame == nil || !frame.hasOutput {
		return nil
	}
	return []object.Object{vm.frameOutputObject(frame)}
}

func (vm *VM) writeFrameOutput(frame *Frame, value object.Object) {
	if frame == nil {
		return
	}
	frame.hasOutput = true
	vm.writeObject(&frame.output, value)
}

func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	switch op {
	case code.OpAdd:
		return vm.executeAdd(left, right)
	case code.OpSub, code.OpMul, code.OpDiv:
		return vm.executeNumericOperation(op, left, right)
	default:
		return fmt.Errorf("unknown binary operator: %d", op)
	}
}

func (vm *VM) executeAdd(left, right object.Object) error {
	if left.Type() == object.ARRAY_OBJ {
		arr := left.(*object.Array)
		elements := append([]object.Object(nil), arr.Elements...)
		elements = append(elements, right)
		return vm.push(&object.Array{Elements: elements})
	}

	if handled, err := vm.executeNativeSliceAppend(left, right); handled || err != nil {
		return err
	}

	if left.Type() == object.STRING_OBJ || right.Type() == object.STRING_OBJ {
		return vm.push(&object.String{Value: fmt.Sprint(object.ToGo(left)) + fmt.Sprint(object.ToGo(right))})
	}

	if left.Type() == object.BOOLEAN_OBJ || right.Type() == object.BOOLEAN_OBJ {
		return vm.push(nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right)))
	}

	return vm.executeNumericOperation(code.OpAdd, left, right)
}

func (vm *VM) executeNativeSliceAppend(left, right object.Object) (bool, error) {
	raw := object.ToGo(left)
	if raw == nil {
		return false, nil
	}

	rv := reflect.ValueOf(raw)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return false, nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return false, nil
	}

	value := object.ToGo(right)
	val := reflect.ValueOf(value)
	elem := rv.Type().Elem()
	if !val.IsValid() {
		val = reflect.Zero(elem)
	} else if elem.Kind() != reflect.Interface && !val.Type().AssignableTo(elem) {
		return true, fmt.Errorf("cannot append '%v' (untyped %s constant) as %s value in assignment", value, val.Type(), elem)
	}

	appended := reflect.Append(rv, val)
	return true, vm.push(object.Wrap(appended.Interface()))
}

func (vm *VM) executeNumericOperation(op code.Opcode, left, right object.Object) error {
	l, lok := numericValueFromObject(left)
	r, rok := numericValueFromObject(right)
	if !lok || !rok {
		return fmt.Errorf("unsupported types for binary operation: %s %s", left.Type(), right.Type())
	}

	result, err := numericOperationObject(op, l, r)
	if err != nil {
		return err
	}
	return vm.push(result)
}

func (vm *VM) executeComparisonOrLogical(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	switch op {
	case code.OpEqual:
		result, err := compareEquality("==", left, right)
		if err != nil {
			return err
		}
		return vm.push(nativeBoolToBooleanObject(result))
	case code.OpNotEqual:
		result, err := compareEquality("!=", left, right)
		if err != nil {
			return err
		}
		return vm.push(nativeBoolToBooleanObject(!result))
	case code.OpGreaterThan, code.OpGreaterEqual:
		result, err := compareOrdered(op, left, right)
		if err != nil {
			return err
		}
		return vm.push(nativeBoolToBooleanObject(result))
	case code.OpMatches:
		pattern := fmt.Sprint(object.ToGo(right))
		re, err := cachedRegex(pattern)
		if err != nil {
			return fmt.Errorf("couldn't compile regex %s", object.ToGo(right))
		}
		return vm.push(nativeBoolToBooleanObject(re.MatchString(fmt.Sprint(object.ToGo(left)))))
	case code.OpAnd:
		return vm.push(nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right)))
	case code.OpOr:
		return vm.push(nativeBoolToBooleanObject(isTruthy(left) || isTruthy(right)))
	default:
		return fmt.Errorf("unknown comparison operator: %d", op)
	}
}

func compareEquality(operator string, left, right object.Object) (bool, error) {
	if object.IsNull(left) || object.IsNull(right) {
		return object.IsNull(left) && object.IsNull(right), nil
	}

	switch left := left.(type) {
	case *object.Boolean:
		return left.Value == isTruthy(right), nil
	case *object.String:
		return left.Value == fmt.Sprint(object.ToGo(right)), nil
	}

	l, lok := numericValueFromObject(left)
	r, rok := numericValueFromObject(right)
	if lok && rok {
		return compareNumericEquality(l, r), nil
	}
	if lok || rok {
		return false, fmt.Errorf("unable to operate (%s) on %s and %s ", operator, plushTypeName(left), plushTypeName(right))
	}

	return reflect.DeepEqual(object.ToGo(left), object.ToGo(right)), nil
}

func cachedRegex(pattern string) (*regexp.Regexp, error) {
	if cached, ok := regexCache.Load(pattern); ok {
		entry := cached.(regexCacheEntry)
		return entry.re, entry.err
	}

	re, err := regexp.Compile(pattern)
	entry := regexCacheEntry{re: re, err: err}
	actual, _ := regexCache.LoadOrStore(pattern, entry)
	entry = actual.(regexCacheEntry)
	return entry.re, entry.err
}

func compareOrdered(op code.Opcode, left, right object.Object) (bool, error) {
	lNumeric, lok := numericValueFromObject(left)
	rNumeric, rok := numericValueFromObject(right)
	if lok && rok {
		return compareNumericOrdered(op, lNumeric, rNumeric)
	}
	if lok || rok {
		return false, fmt.Errorf("unable to operate (%s) on %s and %s ", orderedOperatorString(op), plushTypeName(left), plushTypeName(right))
	}

	l := fmt.Sprint(object.ToGo(left))
	r := fmt.Sprint(object.ToGo(right))
	switch op {
	case code.OpGreaterThan:
		return l > r, nil
	case code.OpGreaterEqual:
		return l >= r, nil
	}
	return false, fmt.Errorf("unknown ordered comparison: %d", op)
}

func plushTypeName(obj object.Object) string {
	switch obj.(type) {
	case *object.Boolean:
		return "bool"
	case *object.Integer:
		return "int"
	case *object.Float:
		return "float64"
	case *object.String:
		return "string"
	case *object.Null:
		return "<nil>"
	case *object.Native:
		raw := object.ToGo(obj)
		if raw == nil {
			return "<nil>"
		}
		return reflect.TypeOf(raw).String()
	default:
		return fmt.Sprintf("%T", object.ToGo(obj))
	}
}

func (vm *VM) executeBangOperator() error {
	operand := vm.pop()
	return vm.push(nativeBoolToBooleanObject(!isTruthy(operand)))
}

func (vm *VM) executeMinusOperator() error {
	operand := vm.pop()

	if f, ok := operand.(*object.Float); ok {
		return vm.push(&object.Float{Value: -f.Value})
	}

	value, ok := toInt(operand)
	if !ok {
		return fmt.Errorf("unsupported type for negation: %s", operand.Type())
	}

	return vm.push(&object.Integer{Value: -value})
}

func (vm *VM) buildArray(startIndex, endIndex int) object.Object {
	elements := make([]object.Object, endIndex-startIndex)
	for i := startIndex; i < endIndex; i++ {
		elements[i-startIndex] = vm.stack[i]
	}
	return &object.Array{Elements: elements}
}

func (vm *VM) buildHash(startIndex, endIndex int) (object.Object, error) {
	hashedPairs := make(map[object.HashKey]object.HashPair)

	for i := startIndex; i < endIndex; i += 2 {
		key := vm.stack[i]
		value := vm.stack[i+1]

		hashKey, ok := key.(object.Hashable)
		if !ok {
			return nil, fmt.Errorf("unusable as hash key: %s", key.Type())
		}

		hashedPairs[hashKey.HashKey()] = object.HashPair{Key: key, Value: value}
	}

	return &object.Hash{Pairs: hashedPairs}, nil
}

func (vm *VM) executeIndexExpression(left, index object.Object) error {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		arrayObject := left.(*object.Array)
		i := index.(*object.Integer).Value
		max := int64(len(arrayObject.Elements) - 1)
		if i < 0 || i > max {
			return fmt.Errorf("array index out of bounds, got index %d, while array size is %d", i, len(arrayObject.Elements))
		}
		return vm.push(arrayObject.Elements[i])
	case left.Type() == object.HASH_OBJ:
		return vm.executeHashIndex(left.(*object.Hash), index)
	default:
		return vm.executeNativeIndex(left, index)
	}
}

func (vm *VM) executeHashIndex(hash *object.Hash, index object.Object) error {
	key, ok := index.(object.Hashable)
	if !ok {
		return fmt.Errorf("unusable as hash key: %s", index.Type())
	}

	pair, ok := hash.Pairs[key.HashKey()]
	if !ok {
		return vm.push(Null)
	}

	return vm.push(pair.Value)
}

func (vm *VM) executeNativeIndex(left, index object.Object) error {
	l := object.ToGo(left)
	if l == nil {
		return vm.push(Null)
	}

	rv := reflect.ValueOf(l)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return vm.push(Null)
		}
		rv = rv.Elem()
	}

	idx := object.ToGo(index)
	switch rv.Kind() {
	case reflect.Map:
		mapKeyType := rv.Type().Key().Kind()
		keyValue := reflect.ValueOf(idx)
		if mapKeyType != reflect.Interface && keyValue.Kind() != mapKeyType {
			return fmt.Errorf("cannot use %v (%s constant) as %s value in map index", idx, keyValue.Kind().String(), mapKeyType.String())
		}
		val := rv.MapIndex(keyValue)
		if !val.IsValid() {
			return vm.push(Null)
		}
		return vm.push(object.Wrap(val.Interface()))
	case reflect.Array, reflect.Slice:
		i, ok := idx.(int)
		if !ok {
			return fmt.Errorf("can't access Slice/Array with a non int Index (%v)", idx)
		}
		if i < 0 || rv.Len()-1 < i {
			return fmt.Errorf("array index out of bounds, got index %d, while array size is %d", i, rv.Len())
		}
		return vm.push(object.Wrap(rv.Index(i).Interface()))
	default:
		return fmt.Errorf("could not index %T with %T", l, idx)
	}
}

func (vm *VM) executeSetIndex(left, index, value object.Object) error {
	switch left := left.(type) {
	case *object.Array:
		i, ok := toInt(index)
		if !ok {
			return fmt.Errorf("can't access Slice/Array with a non int Index (%v)", object.ToGo(index))
		}
		if i < 0 || int(i) >= len(left.Elements) {
			return fmt.Errorf("array index out of bounds, got index %d, while array size is %d", i, len(left.Elements))
		}
		left.Elements[int(i)] = value
		return nil
	case *object.Hash:
		hashKey, ok := index.(object.Hashable)
		if !ok {
			return fmt.Errorf("unusable as hash key: %s", index.Type())
		}
		left.Pairs[hashKey.HashKey()] = object.HashPair{Key: index, Value: value}
		return nil
	default:
		return vm.executeNativeSetIndex(left, index, value)
	}
}

func (vm *VM) executeNativeSetIndex(left, index, value object.Object) error {
	l := object.ToGo(left)
	rv := reflect.ValueOf(l)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return fmt.Errorf("could not index %T with %T", l, object.ToGo(index))
		}
		rv = rv.Elem()
	}

	idx := object.ToGo(index)
	val := reflect.ValueOf(object.ToGo(value))

	switch rv.Kind() {
	case reflect.Map:
		rv.SetMapIndex(reflect.ValueOf(idx), val)
		return nil
	case reflect.Array, reflect.Slice:
		i, ok := idx.(int)
		if !ok {
			return fmt.Errorf("can't access Slice/Array with a non int Index (%v)", idx)
		}
		if i < 0 || rv.Len()-1 < i {
			return fmt.Errorf("array index out of bounds, got index %d, while array size is %d", i, rv.Len())
		}
		elemType := rv.Type().Elem()
		if elemType.Kind() != reflect.Interface && !val.Type().AssignableTo(elemType) {
			return fmt.Errorf("cannot use '%v' (untyped %s constant) as %s value in assignment", object.ToGo(value), val.Type(), elemType)
		}
		rv.Index(i).Set(val)
		return nil
	default:
		return fmt.Errorf("could not index %T with %T", l, idx)
	}
}

func (vm *VM) executeCall(name string, numArgs int, block *object.Closure, cacheSlot *object.InlineCacheSlot) error {
	if err := vm.spendFunctionCall(name); err != nil {
		return err
	}
	return vm.executeCallAfterSpend(name, numArgs, block, cacheSlot)
}

func (vm *VM) executeCallAfterSpend(name string, numArgs int, block *object.Closure, cacheSlot *object.InlineCacheSlot) error {
	callee := vm.stack[vm.sp-1-numArgs]
	switch callee := callee.(type) {
	case *object.Closure:
		return vm.callClosure(callee, numArgs, block, false, true)
	case *object.Builtin:
		return vm.callBuiltin(callee, numArgs)
	default:
		return vm.callNative(name, callee, numArgs, block, cacheSlot)
	}
}

func (vm *VM) executeWriteCall(name string, numArgs int, cacheSlot *object.InlineCacheSlot) error {
	if err := vm.spendFunctionCall(name); err != nil {
		return err
	}

	callee := vm.stack[vm.sp-1-numArgs]
	switch callee := callee.(type) {
	case *object.Closure:
		return vm.callClosure(callee, numArgs, nil, true, true)
	case *object.Builtin:
		return vm.writeBuiltinCall(callee, numArgs, true)
	default:
		handled, err := vm.tryWriteRegisteredFastHelper(name, numArgs, true)
		if handled || err != nil {
			return err
		}
		handled, err = vm.tryFastWriteNativeCall(name, callee, numArgs, cacheSlot, true)
		if handled || err != nil {
			return err
		}
		if err := vm.executeCallAfterSpend(name, numArgs, nil, cacheSlot); err != nil {
			return err
		}
		result := vm.pop()
		vm.writeFrameOutput(vm.currentFrame(), result)
		return nil
	}
}

func (vm *VM) executeWriteNameCall(nameIndex int, numArgs int, cacheSlot *object.InlineCacheSlot) error {
	name := vm.stringConstant(nameIndex)
	raw, ok := vm.contextValueByNameIndex(nameIndex)
	if !ok {
		return fmt.Errorf("%q: unknown identifier", name)
	}
	if err := vm.spendFunctionCall(name); err != nil {
		return err
	}
	if handled, err := vm.tryWriteLiteralPartialNameCall(name, raw, numArgs); handled || err != nil {
		return err
	}

	if callee, ok := raw.(object.Object); ok {
		switch callee := callee.(type) {
		case *object.Closure:
			return vm.callClosure(callee, numArgs, nil, true, false)
		case *object.Builtin:
			return vm.writeBuiltinCall(callee, numArgs, false)
		default:
			handled, err := vm.tryWriteRegisteredFastHelper(name, numArgs, false)
			if handled || err != nil {
				return err
			}
			return vm.writeNativeCall(name, callee, numArgs, cacheSlot, false)
		}
	}
	if handled, err := vm.tryWriteRegisteredFastHelper(name, numArgs, false); handled || err != nil {
		return err
	}
	return vm.writeNativeValueCall(name, raw, numArgs, cacheSlot, false)
}

func (vm *VM) callClosure(cl *object.Closure, numArgs int, block *object.Closure, writeReturn bool, calleeOnStack bool) error {
	if numArgs != cl.Fn.NumParameters {
		return fmt.Errorf("wrong number of arguments: want=%d, got=%d", cl.Fn.NumParameters, numArgs)
	}

	frame := newFrame(cl, vm.sp-numArgs, vm.pooled)
	frame.block = block
	frame.writeReturn = writeReturn
	frame.calleeOnStack = calleeOnStack
	vm.pushFrame(frame)
	vm.sp = frame.basePointer + cl.Fn.NumLocals
	vm.markStack()
	return nil
}

func (vm *VM) callBuiltin(builtin *object.Builtin, numArgs int) error {
	args := vm.stack[vm.sp-numArgs : vm.sp]

	result := builtin.Fn(args...)
	vm.sp = vm.sp - numArgs - 1
	if result != nil {
		return vm.push(result)
	}
	return vm.push(Null)
}

func (vm *VM) writeBuiltinCall(builtin *object.Builtin, numArgs int, calleeOnStack bool) error {
	args := vm.stack[vm.sp-numArgs : vm.sp]
	result := builtin.Fn(args...)
	vm.sp = vm.sp - numArgs
	if calleeOnStack {
		vm.sp--
	}
	if result == nil {
		result = Null
	}
	vm.writeFrameOutput(vm.currentFrame(), result)
	return nil
}

func (vm *VM) tryWriteRegisteredFastHelper(name string, numArgs int, calleeOnStack bool) (bool, error) {
	helper, ok := fastHelperForContext(vm.ctx, name)
	if !ok {
		return false, nil
	}
	frame := vm.currentFrame()
	if frame == nil {
		return false, nil
	}
	var args fastCallArgs
	for _, obj := range vm.stack[vm.sp-numArgs : vm.sp] {
		args.Append(obj)
	}
	handled, err := writeRegisteredFastHelperNamed(&frame.output, vm.ctx, name, helper, fastCallArgsOrNil(&args, numArgs))
	if err != nil || !handled {
		return handled, err
	}
	vm.sp -= numArgs
	if calleeOnStack {
		vm.sp--
	}
	frame.hasOutput = true
	return true, nil
}

func (vm *VM) callNative(name string, callee object.Object, numArgs int, block *object.Closure, cacheSlot *object.InlineCacheSlot) error {
	raw := object.ToGo(callee)
	return vm.callNativeValue(name, raw, numArgs, block, cacheSlot)
}

func (vm *VM) callNativeValue(name string, raw interface{}, numArgs int, block *object.Closure, cacheSlot *object.InlineCacheSlot) error {
	rv := reflect.ValueOf(raw)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return fmt.Errorf("%T is an invalid function", raw)
	}
	if rv.Kind() != reflect.Func {
		return fmt.Errorf("%+v (%T) is an invalid function", raw, raw)
	}

	rt := rv.Type()
	plan := cachedCallPlanForSlot(rt, cacheSlot)
	var scratch [1]reflect.Value
	args, err := vm.reflectArgs(name, plan, numArgs, block, scratch[:0])
	if err != nil {
		return err
	}

	res := rv.Call(args)
	vm.sp = vm.sp - numArgs - 1

	if len(res) == 0 {
		return vm.push(Null)
	}

	if err := lastReturnError(res); err != nil {
		return fmt.Errorf("could not call %s function: %w", name, err)
	}

	return vm.push(object.Wrap(res[0].Interface()))
}

func (vm *VM) writeNativeCall(name string, callee object.Object, numArgs int, cacheSlot *object.InlineCacheSlot, calleeOnStack bool) error {
	raw := object.ToGo(callee)
	return vm.writeNativeValueCall(name, raw, numArgs, cacheSlot, calleeOnStack)
}

func (vm *VM) writeNativeValueCall(name string, raw interface{}, numArgs int, cacheSlot *object.InlineCacheSlot, calleeOnStack bool) error {
	if handled, err := vm.tryFastWriteNativeValueCall(name, raw, numArgs, cacheSlot, calleeOnStack); handled || err != nil {
		return err
	}

	rv := reflect.ValueOf(raw)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return fmt.Errorf("%T is an invalid function", raw)
	}
	if rv.Kind() != reflect.Func {
		return fmt.Errorf("%+v (%T) is an invalid function", raw, raw)
	}

	rt := rv.Type()
	plan := cachedCallPlanForSlot(rt, cacheSlot)
	var scratch [1]reflect.Value
	args, err := vm.reflectArgs(name, plan, numArgs, nil, scratch[:0])
	if err != nil {
		return err
	}

	res := rv.Call(args)
	vm.sp = vm.sp - numArgs
	if calleeOnStack {
		vm.sp--
	}

	if len(res) == 0 {
		vm.writeFrameOutput(vm.currentFrame(), Null)
		return nil
	}

	if err := lastReturnError(res); err != nil {
		return fmt.Errorf("could not call %s function: %w", name, err)
	}

	frame := vm.currentFrame()
	if frame != nil {
		frame.hasOutput = true
		vm.writeNativeReturnValue(frame, res[0], plan)
	}
	return nil
}

func (vm *VM) tryFastWriteNativeCall(name string, callee object.Object, numArgs int, cacheSlot *object.InlineCacheSlot, calleeOnStack bool) (bool, error) {
	raw := object.ToGo(callee)
	return vm.tryFastWriteNativeValueCall(name, raw, numArgs, cacheSlot, calleeOnStack)
}

func (vm *VM) tryFastWriteNativeValueCall(name string, raw interface{}, numArgs int, cacheSlot *object.InlineCacheSlot, calleeOnStack bool) (bool, error) {
	rv := reflect.ValueOf(raw)
	if !rv.IsValid() {
		return false, fmt.Errorf("%T is an invalid function", raw)
	}
	if rv.Kind() == reflect.Ptr {
		return false, nil
	}
	if rv.Kind() != reflect.Func {
		return false, fmt.Errorf("%+v (%T) is an invalid function", raw, raw)
	}

	entry := cachedCallEntryForSlot(rv.Type(), raw, cacheSlot)
	if entry == nil || entry.invoker == nil {
		return false, nil
	}

	args := vm.stack[vm.sp-numArgs : vm.sp]
	oldSP := vm.sp
	vm.sp = vm.sp - numArgs
	if calleeOnStack {
		vm.sp--
	}
	if err := entry.invoker(vm, vm.currentFrame(), name, raw, args); err != nil {
		if errors.Is(err, errFastWriteUnsupported) {
			vm.sp = oldSP
			return false, nil
		}
		return true, err
	}
	return true, nil
}

func (vm *VM) writeNativeReturnValue(frame *Frame, value reflect.Value, plan *callPlan) {
	switch plan.returnKind {
	case callReturnNone:
		return
	case callReturnString:
		frame.output.WriteString(template.HTMLEscapeString(value.String()))
	case callReturnHTML:
		frame.output.WriteString(value.String())
	case callReturnBool:
		frame.output.WriteString(strconv.FormatBool(value.Bool()))
	case callReturnInt:
		frame.output.WriteString(strconv.FormatInt(value.Int(), 10))
	case callReturnUint:
		frame.output.WriteString(strconv.FormatUint(value.Uint(), 10))
	case callReturnFloat:
		frame.output.WriteString(strconv.FormatFloat(value.Float(), 'g', -1, int(value.Type().Bits())))
	case callReturnObject:
		if isNilReflectValue(value) {
			return
		}
		if obj, ok := value.Interface().(object.Object); ok {
			vm.writeObject(&frame.output, obj)
			return
		}
		vm.writeGoValue(&frame.output, value.Interface())
	default:
		vm.writeGoValue(&frame.output, value.Interface())
	}
}
