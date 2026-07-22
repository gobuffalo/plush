package vm

import (
	"fmt"
	"html/template"
	"reflect"
	"strconv"
	"strings"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

func (vm *VM) executeFor(iterable object.Object, block *object.Closure, keyName, valueName string) (object.Object, error) {
	writeLoopContext := loopNeedsContextWrites(block)
	oldCtx := vm.ctx
	loopCtx := vm.ctx
	if writeLoopContext && vm.ctx != nil {
		loopCtx = vm.ctx.New()
		vm.ctx = loopCtx
		defer func() { vm.ctx = oldCtx }()
	}

	ret := []object.Object{}
	iter := object.ToGo(iterable)
	if iter == nil {
		return &object.Array{Elements: ret}, nil
	}

	child := vm.childVM(block, nil, loopCtx)
	defer child.Release()
	child.deferHolePositions = true
	rawLoopValues := loopCanUseRawValues(block)

	run := func(key, value object.Object) (bool, error) {
		if err := vm.spendLoop(); err != nil {
			return false, err
		}
		if writeLoopContext && loopCtx != nil {
			loopCtx.Set(keyName, object.ToGo(key))
			loopCtx.Set(valueName, object.ToGo(value))
		}
		child.resetChild(block, []object.Object{key, value}, loopCtx)
		if err := child.Run(); err != nil {
			return false, child.wrapRuntimeError(err)
		}
		result := child.lastPopped
		if result == nil {
			result = child.frameOutputObject(child.frames[0])
		}
		if control, ok := result.(*object.Control); ok {
			ret = append(ret, control.Value...)
			return control.Kind == object.ControlBreak, nil
		}
		if !object.IsNull(result) {
			ret = append(ret, result)
		}
		return false, nil
	}

	switch iter := iter.(type) {
	case []string:
		ret = make([]object.Object, 0, len(iter))
		for i, value := range iter {
			var keyObj, valueObj object.Object
			if rawLoopValues {
				keyObj, valueObj = loopObjects(i, value, true)
			} else {
				keyObj, valueObj = &object.Integer{Value: int64(i)}, &object.String{Value: value}
			}
			stop, err := run(keyObj, valueObj)
			if err != nil || stop {
				return &object.Array{Elements: ret}, err
			}
		}
		return &object.Array{Elements: ret}, nil
	case []interface{}:
		ret = make([]object.Object, 0, len(iter))
		for i, value := range iter {
			var keyObj, valueObj object.Object
			if rawLoopValues {
				keyObj, valueObj = loopObjects(i, value, true)
			} else {
				keyObj, valueObj = &object.Integer{Value: int64(i)}, object.Wrap(value)
			}
			stop, err := run(keyObj, valueObj)
			if err != nil || stop {
				return &object.Array{Elements: ret}, err
			}
		}
		return &object.Array{Elements: ret}, nil
	case []object.Object:
		ret = make([]object.Object, 0, len(iter))
		for i, value := range iter {
			stop, err := run(&object.Integer{Value: int64(i)}, value)
			if err != nil || stop {
				return &object.Array{Elements: ret}, err
			}
		}
		return &object.Array{Elements: ret}, nil
	}

	rv := reflect.ValueOf(iter)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return &object.Array{Elements: ret}, nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Map, reflect.Array, reflect.Slice:
		ret = make([]object.Object, 0, rv.Len())
	}
	switch rv.Kind() {
	case reflect.Map:
		keys := rv.MapKeys()
		for _, key := range keys {
			rawKey := key.Interface()
			rawValue := rv.MapIndex(key).Interface()
			keyObj, valueObj := loopObjects(rawKey, rawValue, rawLoopValues)
			stop, err := run(keyObj, valueObj)
			if err != nil || stop {
				return &object.Array{Elements: ret}, err
			}
		}
	case reflect.Array, reflect.Slice:
		for i := 0; i < rv.Len(); i++ {
			rawValue := rv.Index(i).Interface()
			var keyObj, valueObj object.Object
			if rawLoopValues {
				keyObj, valueObj = loopObjects(i, rawValue, true)
			} else {
				keyObj, valueObj = &object.Integer{Value: int64(i)}, object.Wrap(rawValue)
			}
			stop, err := run(keyObj, valueObj)
			if err != nil || stop {
				return &object.Array{Elements: ret}, err
			}
		}
	default:
		if iterator, ok := iter.(interface{ Next() interface{} }); ok {
			i := 0
			for next := iterator.Next(); next != nil; next = iterator.Next() {
				var keyObj, valueObj object.Object
				if rawLoopValues {
					keyObj, valueObj = loopObjects(i, next, true)
				} else {
					keyObj, valueObj = &object.Integer{Value: int64(i)}, object.Wrap(next)
				}
				stop, err := run(keyObj, valueObj)
				if err != nil || stop {
					return &object.Array{Elements: ret}, err
				}
				i++
			}
		} else {
			return nil, fmt.Errorf("could not iterate over %T", iter)
		}
	}

	return &object.Array{Elements: ret}, nil
}

func loopObjects(key, value interface{}, raw bool) (object.Object, object.Object) {
	if raw {
		return rawLoopObject(key), rawLoopObject(value)
	}
	return object.Wrap(key), object.Wrap(value)
}

func rawLoopObject(value interface{}) object.Object {
	if obj, ok := value.(object.Object); ok {
		return obj
	}
	if value == nil {
		return Null
	}
	return &object.Native{Value: value}
}

func loopCanUseRawValues(block *object.Closure) bool {
	if block == nil || block.Fn == nil {
		return false
	}
	hasLocalPropertyWrite := false
	ins := block.Fn.Instructions
	for i := 0; i < len(ins); {
		op := code.Opcode(ins[i])
		if op == code.OpWriteLocalProperty {
			hasLocalPropertyWrite = true
		}
		switch op {
		case code.OpWriteLocal, code.OpWriteLocalProperty,
			code.OpWriteConstant, code.OpWriteString, code.OpWriteHTML,
			code.OpNull, code.OpPop, code.OpReturn, code.OpReturnValue,
			code.OpJump, code.OpBreak, code.OpContinue:
		default:
			return false
		}

		def, _ := code.Lookup(byte(op))
		_, read := code.ReadOperands(def, ins[i+1:])
		i += 1 + read
	}
	return hasLocalPropertyWrite
}

func loopNeedsContextWrites(block *object.Closure) bool {
	if block == nil || block.Fn == nil {
		return true
	}
	ins := block.Fn.Instructions
	for i := 0; i < len(ins); {
		op := code.Opcode(ins[i])
		switch op {
		case code.OpGetName, code.OpGetNameOrNull, code.OpSetName, code.OpAssignName,
			code.OpWriteName, code.OpWriteNameOrNull, code.OpWriteNameProperty,
			code.OpWriteGlobalProperty,
			code.OpCall, code.OpWriteCall, code.OpWriteNameCall, code.OpCallBlock, code.OpRenderTemplate, code.OpHole:
			return true
		}

		def, err := code.Lookup(byte(op))
		if err != nil {
			i++
			continue
		}
		_, read := code.ReadOperands(def, ins[i+1:])
		i += 1 + read
	}
	return false
}

func (vm *VM) pushName(nameIndex int) error {
	name := vm.stringConstant(nameIndex)
	if name == "nil" {
		return vm.push(Null)
	}
	if value, ok := vm.contextValue(name); ok {
		return vm.push(object.Wrap(value))
	}
	return fmt.Errorf("%q: unknown identifier", name)
}

func (vm *VM) pushNameOrNull(nameIndex int) error {
	name := vm.stringConstant(nameIndex)
	if name == "nil" {
		return vm.push(Null)
	}
	if value, ok := vm.contextValue(name); ok {
		return vm.push(object.Wrap(value))
	}
	return vm.push(Null)
}

func (vm *VM) contextValue(name string) (interface{}, bool) {
	if vm.ctx == nil {
		return nil, false
	}
	if lookup, ok := vm.ctx.(contextLookup); ok {
		return lookup.Lookup(name)
	}
	if vm.ctx.Has(name) {
		return vm.ctx.Value(name), true
	}
	return nil, false
}

func (vm *VM) contextValueByNameIndex(nameIndex int) (interface{}, bool) {
	if vm.ctx == nil {
		return nil, false
	}
	if lookup, ok := vm.ctx.(contextIDLookup); ok {
		id := vm.contextNameID(lookup, nameIndex)
		return lookup.LookupID(id)
	}
	return vm.contextValue(vm.stringConstant(nameIndex))
}

func (vm *VM) contextNameID(lookup contextIDLookup, nameIndex int) int {
	name := vm.stringConstant(nameIndex)
	for i := 0; i < vm.nameIDCacheLen; i++ {
		if vm.nameIDCache[i].name == name {
			return vm.nameIDCache[i].id
		}
	}
	for _, entry := range vm.nameIDOverflow {
		if entry.name == name {
			return entry.id
		}
	}

	id := lookup.InternID(name)
	entry := nameIDEntry{name: name, id: id}
	if vm.nameIDCacheLen < len(vm.nameIDCache) {
		vm.nameIDCache[vm.nameIDCacheLen] = entry
		vm.nameIDCacheLen++
	} else {
		vm.nameIDOverflow = append(vm.nameIDOverflow, entry)
	}
	return id
}

func (vm *VM) clearNameIDCache() {
	for i := 0; i < vm.nameIDCacheLen; i++ {
		vm.nameIDCache[i] = nameIDEntry{}
	}
	vm.nameIDCacheLen = 0
	if len(vm.nameIDOverflow) > 0 {
		clear(vm.nameIDOverflow)
		vm.nameIDOverflow = vm.nameIDOverflow[:0]
	}
}

func (vm *VM) setName(nameIndex int, value object.Object) {
	if vm.ctx != nil {
		raw := object.ToGo(value)
		if lookup, ok := vm.ctx.(contextIDLookup); ok {
			lookup.SetID(vm.contextNameID(lookup, nameIndex), raw)
			return
		}
		vm.ctx.Set(vm.stringConstant(nameIndex), raw)
	}
}

func (vm *VM) writeConstant(frame *Frame, constIndex int) {
	if constIndex < 0 || constIndex >= len(vm.constants) {
		return
	}
	vm.writeFrameOutput(frame, vm.constants[constIndex])
}

func (vm *VM) writeStringConstant(frame *Frame, constIndex int) {
	if frame == nil {
		return
	}
	frame.hasOutput = true
	frame.output.WriteString(template.HTMLEscapeString(vm.stringConstant(constIndex)))
}

func (vm *VM) writeHTMLConstant(frame *Frame, constIndex int) {
	if frame == nil {
		return
	}
	frame.hasOutput = true
	frame.output.WriteString(vm.htmlConstantString(constIndex))
}

func (vm *VM) writeName(frame *Frame, nameIndex int, nullOnMissing bool) error {
	name := vm.stringConstant(nameIndex)
	if name == "nil" {
		return nil
	}
	value, ok := vm.contextValueByNameIndex(nameIndex)
	if !ok {
		if nullOnMissing {
			return nil
		}
		return fmt.Errorf("%q: unknown identifier", name)
	}
	if frame == nil {
		return nil
	}
	frame.hasOutput = true
	writeFastGoValue(&frame.output, vm.ctx, value)
	return nil
}

func (vm *VM) writeGlobal(frame *Frame, globalIndex int) {
	value := vm.globalValue(globalIndex)
	if value != nil {
		vm.writeFrameOutput(frame, value)
		return
	}
	name, ok := vm.globalNames[globalIndex]
	if !ok {
		return
	}
	if raw, ok := vm.contextValue(name); ok {
		if frame == nil {
			return
		}
		frame.hasOutput = true
		vm.writeGoValue(&frame.output, raw)
	}
}

func (vm *VM) writeLocalProperty(frame *Frame, localIndex, propertyNameIndex, ip int) error {
	if frame == nil {
		return nil
	}
	stackIndex := frame.basePointer + localIndex
	if stackIndex < 0 || stackIndex >= len(vm.stack) {
		return nil
	}
	return vm.writePropertyValue(frame, vm.stack[stackIndex], propertyNameIndex, ip)
}

func (vm *VM) writeGlobalProperty(frame *Frame, globalIndex, propertyNameIndex, ip int) error {
	value := vm.globalValue(globalIndex)
	if value != nil {
		return vm.writePropertyValue(frame, value, propertyNameIndex, ip)
	}
	name, ok := vm.globalNames[globalIndex]
	if !ok {
		return nil
	}
	raw, ok := vm.contextValue(name)
	if !ok {
		return nil
	}
	return vm.writePropertyValue(frame, raw, propertyNameIndex, ip)
}

func (vm *VM) writeNameProperty(frame *Frame, baseNameIndex, propertyNameIndex, ip int) error {
	name := vm.stringConstant(baseNameIndex)
	if name == "nil" {
		return nil
	}
	value, ok := vm.contextValueByNameIndex(baseNameIndex)
	if !ok {
		return fmt.Errorf("%q: unknown identifier", name)
	}
	return vm.writePropertyValue(frame, value, propertyNameIndex, ip)
}

func (vm *VM) writePropertyValue(frame *Frame, base interface{}, propertyNameIndex, ip int) error {
	if frame == nil {
		return nil
	}
	if err := vm.spendTraversal(1); err != nil {
		return err
	}
	propertyName := vm.stringConstant(propertyNameIndex)
	value, err := vm.propertyValue(base, propertyName, vm.currentPropertyAccess(ip), vm.currentPropertyCacheSlot(ip))
	if err != nil {
		return err
	}
	frame.hasOutput = true
	vm.writeGoValue(&frame.output, value)
	return nil
}

func (vm *VM) assignName(nameIndex int, value object.Object) error {
	if err := vm.spendAssignment(); err != nil {
		return err
	}
	name := vm.stringConstant(nameIndex)
	if vm.ctx != nil {
		raw := object.ToGo(value)
		if lookup, ok := vm.ctx.(contextIDLookup); ok {
			if lookup.UpdateID(vm.contextNameID(lookup, nameIndex), raw) {
				return nil
			}
		} else if vm.ctx.Update(name, raw) {
			return nil
		}
	}
	return fmt.Errorf("%q: unknown identifier", name)
}

func (vm *VM) updateNamedGlobal(globalIndex int, value object.Object) {
	name, ok := vm.globalNames[globalIndex]
	if !ok || vm.ctx == nil {
		return
	}
	vm.ctx.Set(name, object.ToGo(value))
}

func (vm *VM) globalFromContext(globalIndex int) object.Object {
	name, ok := vm.globalNames[globalIndex]
	if !ok {
		return nil
	}
	value, ok := vm.contextValue(name)
	if !ok {
		return nil
	}
	return object.Wrap(value)
}

func (vm *VM) getProperty(obj object.Object, name string, access object.PropertyAccess, cacheSlot *object.InlineCacheSlot) error {
	if err := vm.spendTraversal(1); err != nil {
		return err
	}

	if hash, ok := obj.(*object.Hash); ok {
		key := (&object.String{Value: name}).HashKey()
		if pair, ok := hash.Pairs[key]; ok {
			return vm.push(pair.Value)
		}
		return vm.push(Null)
	}

	raw := object.ToGo(obj)
	if raw == nil {
		return vm.push(Null)
	}

	rv := reflect.ValueOf(raw)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return vm.push(Null)
		}
		rv = rv.Elem()
	}

	lookup := inlinePropertyLookup(cacheSlot, rv.Type(), name)
	switch lookup.kind {
	case propertyLookupValueMethod:
		method := rv.Method(lookup.index)
		if method.IsValid() {
			return vm.push(object.Wrap(method.Interface()))
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
			return vm.push(object.Wrap(method.Interface()))
		}
	case propertyLookupField:
		if access.Method {
			return propertyMissingError(access, raw, name)
		}
		field := rv.FieldByIndex(lookup.fieldIndex)
		return vm.pushField(field, access, name)
	}

	if access.Method || rv.Kind() != reflect.Struct {
		return propertyMissingError(access, raw, name)
	}

	return propertyMissingError(access, raw, name)
}

func (vm *VM) propertyValue(base interface{}, name string, access object.PropertyAccess, cacheSlot *object.InlineCacheSlot) (interface{}, error) {
	if hash, ok := base.(*object.Hash); ok {
		key := (&object.String{Value: name}).HashKey()
		if pair, ok := hash.Pairs[key]; ok {
			return pair.Value, nil
		}
		return nil, nil
	}

	if obj, ok := base.(object.Object); ok {
		if object.IsNull(obj) {
			return nil, nil
		}
		base = object.ToGo(obj)
	}

	if base == nil {
		return nil, nil
	}

	rv := reflect.ValueOf(base)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
	}

	lookup := inlinePropertyLookup(cacheSlot, rv.Type(), name)
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
			return nil, propertyMissingError(access, base, name)
		}
		field := rv.FieldByIndex(lookup.fieldIndex)
		return vm.fieldValue(field, access, name)
	}

	if access.Method || rv.Kind() != reflect.Struct {
		return nil, propertyMissingError(access, base, name)
	}

	return nil, propertyMissingError(access, base, name)
}

func inlinePropertyLookup(slot *object.InlineCacheSlot, rt reflect.Type, name string) propertyLookup {
	return inlinePropertyEntry(slot, rt, name).lookup
}

func inlinePropertyEntry(slot *object.InlineCacheSlot, rt reflect.Type, name string) *propertyInlineCacheEntry {
	if slot == nil {
		lookup := cachedPropertyLookup(rt, name)
		return newPropertyInlineCacheEntry(rt, lookup)
	}

	if cached, ok := slot.Load().(*propertyInlineCacheEntry); ok {
		for entry := cached; entry != nil; entry = entry.next {
			if entry.typ == rt {
				return entry
			}
		}
	}

	lookup := cachedPropertyLookup(rt, name)
	current, _ := slot.Load().(*propertyInlineCacheEntry)
	entry := newPropertyInlineCacheEntry(rt, lookup)
	entry.next = clonePropertyInlineCache(current, propertyInlineCacheDepth-1)
	slot.Store(entry)
	return entry
}

func newPropertyInlineCacheEntry(rt reflect.Type, lookup propertyLookup) *propertyInlineCacheEntry {
	return &propertyInlineCacheEntry{
		typ:    rt,
		lookup: lookup,
		reader: buildFastPropertyReader(lookup),
		writer: buildFastPropertyWriter(rt, lookup),
	}
}

func clonePropertyInlineCache(head *propertyInlineCacheEntry, limit int) *propertyInlineCacheEntry {
	if head == nil || limit <= 0 {
		return nil
	}

	clone := &propertyInlineCacheEntry{
		typ:    head.typ,
		lookup: head.lookup,
		reader: head.reader,
		writer: head.writer,
	}
	tail := clone
	count := 1
	for entry := head.next; entry != nil && count < limit; entry = entry.next {
		tail.next = &propertyInlineCacheEntry{
			typ:    entry.typ,
			lookup: entry.lookup,
			reader: entry.reader,
			writer: entry.writer,
		}
		tail = tail.next
		count++
	}
	return clone
}

func buildFastPropertyReader(lookup propertyLookup) fastPropertyReader {
	switch lookup.kind {
	case propertyLookupValueMethod:
		return func(rv reflect.Value, access object.PropertyAccess, name string) (interface{}, error) {
			return rv.Method(lookup.index).Interface(), nil
		}
	case propertyLookupPointerMethod:
		return func(rv reflect.Value, access object.PropertyAccess, name string) (interface{}, error) {
			var method reflect.Value
			if rv.CanAddr() {
				method = rv.Addr().Method(lookup.index)
			} else {
				ptr := reflect.New(rv.Type())
				ptr.Elem().Set(rv)
				method = ptr.Method(lookup.index)
			}
			return method.Interface(), nil
		}
	case propertyLookupField:
		return func(rv reflect.Value, access object.PropertyAccess, name string) (interface{}, error) {
			if access.Method {
				return nil, propertyMissingError(access, rv.Interface(), name)
			}
			return fastFieldValue(rv.FieldByIndex(lookup.fieldIndex), access, name)
		}
	default:
		return nil
	}
}

func buildFastPropertyWriter(rt reflect.Type, lookup propertyLookup) fastPropertyWriter {
	if lookup.kind != propertyLookupField {
		return nil
	}
	field, ok := fieldByIndex(rt, lookup.fieldIndex)
	if !ok {
		return nil
	}
	return func(out *strings.Builder, ctx hctx.Context, rv reflect.Value, access object.PropertyAccess, name string) (bool, error) {
		if access.Method {
			return false, propertyMissingError(access, rv.Interface(), name)
		}
		return writeFastField(out, ctx, rv.FieldByIndex(lookup.fieldIndex), access, name, field.Type)
	}
}

func fieldByIndex(rt reflect.Type, index []int) (reflect.StructField, bool) {
	if rt.Kind() != reflect.Struct || len(index) == 0 {
		return reflect.StructField{}, false
	}
	field := reflect.StructField{}
	current := rt
	for _, i := range index {
		if current.Kind() == reflect.Ptr {
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct || i < 0 || i >= current.NumField() {
			return reflect.StructField{}, false
		}
		field = current.Field(i)
		current = field.Type
	}
	return field, true
}

func writeFastField(out *strings.Builder, ctx hctx.Context, field reflect.Value, access object.PropertyAccess, name string, fieldType reflect.Type) (bool, error) {
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return true, nil
		}
		field = field.Elem()
	}
	if !field.CanInterface() {
		if access.Receiver != "" && access.Full != "" {
			return false, fmt.Errorf("'%s'cannot return value obtained from unexported field or method '%s' (%s)", access.Receiver, name, access.Full)
		}
		return false, fmt.Errorf("cannot return value obtained from unexported field or method '%s'", name)
	}
	if field.Type() == templateHTMLType {
		out.WriteString(field.String())
		return true, nil
	}
	if field.Type().PkgPath() != "" {
		if stringer, ok := field.Interface().(fmt.Stringer); ok {
			out.WriteString(stringer.String())
			return true, nil
		}
	}
	_ = fieldType
	switch field.Kind() {
	case reflect.String:
		writeFastEscapedString(out, field.String())
		return true, nil
	case reflect.Bool:
		out.WriteString(strconv.FormatBool(field.Bool()))
		return true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		writeBuilderFastInt(out, field.Int())
		return true, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		writeBuilderFastUint(out, field.Uint())
		return true, nil
	case reflect.Float32, reflect.Float64:
		writeBuilderFastFloat(out, field.Float(), int(field.Type().Bits()))
		return true, nil
	default:
		return false, nil
	}
}

func cachedPropertyLookup(rt reflect.Type, name string) propertyLookup {
	key := propertyLookupKey{typ: rt, name: name}
	if lookup, ok := propertyLookupCache.Load(key); ok {
		return lookup.(propertyLookup)
	}

	lookup := buildPropertyLookup(rt, name)
	actual, _ := propertyLookupCache.LoadOrStore(key, lookup)
	return actual.(propertyLookup)
}

func buildPropertyLookup(rt reflect.Type, name string) propertyLookup {
	if method, ok := rt.MethodByName(name); ok {
		return propertyLookup{kind: propertyLookupValueMethod, index: method.Index}
	}
	if method, ok := reflect.PtrTo(rt).MethodByName(name); ok {
		return propertyLookup{kind: propertyLookupPointerMethod, index: method.Index}
	}
	if rt.Kind() == reflect.Struct {
		if field, ok := rt.FieldByName(name); ok {
			return propertyLookup{kind: propertyLookupField, fieldIndex: field.Index}
		}
	}
	return propertyLookup{kind: propertyLookupMissing}
}

func (vm *VM) pushField(field reflect.Value, access object.PropertyAccess, name string) error {
	value, err := vm.fieldValue(field, access, name)
	if err != nil {
		return err
	}
	return vm.push(object.Wrap(value))
}

func (vm *VM) fieldValue(field reflect.Value, access object.PropertyAccess, name string) (interface{}, error) {
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

func propertyMissingError(access object.PropertyAccess, raw interface{}, name string) error {
	if access.Receiver != "" && access.Full != "" {
		if access.Method {
			return fmt.Errorf("'%s' does not have a method named '%s' (%s)", access.Receiver, name, access.Full)
		}
		return fmt.Errorf("'%s' does not have a field or method named '%s' (%s)", access.Receiver, name, access.Full)
	}
	return fmt.Errorf("'%s' does not have a field or method named '%s'", fmt.Sprint(raw), name)
}
