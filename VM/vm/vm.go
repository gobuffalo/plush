package vm

import (
	"fmt"
	"html/template"
	"reflect"
	"sync"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

const StackSize = 2048
const MaxFrames = 1024
const anonymousCallName = "<anonymous>"

var vmAlreadyInPartial = "__plush_vm_internal_already_in_partial_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"
var vmPartialBytecodeLinksKey = "__plush_vm_internal_partial_bytecode_links_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"
var vmFastHelpersKey = "__plush_vm_internal_fast_helpers_" + fmt.Sprintf("%d", time.Now().UnixNano()) + "__"

const vmPartialFeederName = "partialFeeder"

var fastStructLoopWriterPlanCache sync.Map
var fastFieldChainPlanCache sync.Map
var fastAccessChainPlanCache sync.Map

var True = object.TrueObject
var False = object.FalseObject
var Null = object.NullObject

type VM struct {
	constants []object.Object

	stack    []object.Object
	sp       int
	stackMax int

	globals     []object.Object
	globalNames map[int]string

	frames      []*Frame
	framesIndex int

	ctx hctx.Context

	holes              *[]plush.HoleMarker
	deferHolePositions bool
	nameIDCache        [8]nameIDEntry
	nameIDCacheLen     int
	nameIDOverflow     []nameIDEntry

	lastPopped object.Object
	lastIP     int
	halted     bool

	pooled     bool
	ownGlobals bool
	ownHoles   bool
}

type vmHole struct {
	input string
}

type nameIDEntry struct {
	name string
	id   int
}

type contextLookup interface {
	Lookup(string) (interface{}, bool)
}

type contextIDLookup interface {
	InternID(string) int
	LookupID(int) (interface{}, bool)
	SetID(int, interface{})
	UpdateID(int, interface{}) bool
}

type contextIDBinder interface {
	InternIDs([]string, []int)
}

type Template struct {
	bytecode *compiler.Bytecode
}

var stackPool = sync.Pool{
	New: func() interface{} {
		stack := make([]object.Object, StackSize)
		return &stack
	},
}

var framesPool = sync.Pool{
	New: func() interface{} {
		frames := make([]*Frame, MaxFrames)
		return &frames
	},
}

var framePool = sync.Pool{
	New: func() interface{} {
		frame := NewFrame(nil, 0)
		frame.pooled = true
		return frame
	},
}

var holesPool = sync.Pool{
	New: func() interface{} {
		holes := make([]plush.HoleMarker, 0, 8)
		return &holes
	},
}

var partialOverlayContextPool = sync.Pool{
	New: func() interface{} {
		return &partialOverlayContext{}
	},
}

var globalsPools sync.Map

var (
	errorType                  = reflect.TypeOf((*error)(nil)).Elem()
	stringType                 = reflect.TypeOf("")
	templateHTMLType           = reflect.TypeOf(template.HTML(""))
	boolType                   = reflect.TypeOf(false)
	emptyInterfaceType         = reflect.TypeOf((*interface{})(nil)).Elem()
	objectInterfaceType        = reflect.TypeOf((*object.Object)(nil)).Elem()
	plushHelperContextType     = reflect.TypeOf(plush.HelperContext{})
	hctxHelperContextInterface = reflect.TypeOf((*hctx.HelperContext)(nil)).Elem()
	emptyMapType               = reflect.TypeOf(map[string]interface{}{})
	callPlanCache              sync.Map
	propertyLookupCache        sync.Map
	regexCache                 sync.Map
)

type optionalArgKind uint8

const (
	optionalArgNone optionalArgKind = iota
	optionalArgHelperContext
	optionalArgMap
)

type callReturnKind uint8

const (
	callReturnGeneric callReturnKind = iota
	callReturnNone
	callReturnString
	callReturnHTML
	callReturnBool
	callReturnInt
	callReturnUint
	callReturnFloat
	callReturnObject
)

type callPlan struct {
	rt           reflect.Type
	numIn        int
	isVariadic   bool
	minArgs      int
	argTypes     []reflect.Type
	optionalArgs []optionalArgKind
	variadicElem reflect.Type
	returnKind   callReturnKind
}

// ErrFastUnsupported lets a custom fast helper decline a call and fall back to
// the normal VM helper path.
