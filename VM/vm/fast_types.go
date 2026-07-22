package vm

import (
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/gobuffalo/plush/v5/VM/compiler"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
)

type callCacheEntry struct {
	rt      reflect.Type
	plan    *callPlan
	invoker writeFastInvoker
	noFast  bool
}

type fastBuilderCallCacheEntry struct {
	rt                     reflect.Type
	plan                   *callPlan
	invoker                writeFastBuilderInvoker
	valueInvoker           valueFastInvoker
	contextualValueInvoker contextualValueFastInvoker
}

type fastCallArgs struct {
	inline  [4]interface{}
	n       int
	extra   []interface{}
	objects []object.Object
}

type propertyLookupKind uint8

const (
	propertyLookupMissing propertyLookupKind = iota
	propertyLookupValueMethod
	propertyLookupPointerMethod
	propertyLookupField
)

type propertyLookupKey struct {
	typ  reflect.Type
	name string
}

type propertyLookup struct {
	kind       propertyLookupKind
	index      int
	fieldIndex []int
}

type propertyInlineCacheEntry struct {
	typ    reflect.Type
	lookup propertyLookup
	reader fastPropertyReader
	writer fastPropertyWriter
	next   *propertyInlineCacheEntry
}

type fastPropertyReader func(reflect.Value, object.PropertyAccess, string) (interface{}, error)
type fastPropertyWriter func(*strings.Builder, hctx.Context, reflect.Value, object.PropertyAccess, string) (bool, error)

const propertyInlineCacheDepth = 4

type fastStructLoopWriterPlanKey struct {
	loop *compiler.FastLoopPlan
	typ  reflect.Type
}

type fastStructLoopWriterOpKind uint8

const (
	fastStructLoopWriterStatic fastStructLoopWriterOpKind = iota
	fastStructLoopWriterKey
	fastStructLoopWriterField
	fastStructLoopWriterAccessChain
	fastStructLoopWriterMethodCall
	fastStructLoopWriterCall
	fastStructLoopWriterConditional
)

type fastStructLoopWriterPlan struct {
	ops []fastStructLoopWriterOp
}

type fastStructLoopWriterOp struct {
	kind        fastStructLoopWriterOpKind
	value       string
	name        string
	receiver    string
	full        string
	line        int
	fieldIndex  []int
	fieldType   reflect.Type
	accessPlan  *fastAccessChainPlan
	methodPlan  *fastLoopMethodCallPlan
	call        *fastStructLoopCallPlan
	conditional *fastStructLoopConditionalWriterPlan
}

type fastStructLoopConditionalWriterPlan struct {
	branches []fastStructLoopConditionalWriterBranch
	elseOps  []fastStructLoopWriterOp
}

type fastStructLoopConditionalWriterBranch struct {
	condition     compiler.FastValuePlan
	conditionPlan *fastStructLoopConditionPlan
	ops           []fastStructLoopWriterOp
	line          int
}

type fastStructLoopConditionKind uint8

const (
	fastStructLoopConditionTruthy fastStructLoopConditionKind = iota
	fastStructLoopConditionInfix
	fastStructLoopConditionLogical
)

type fastStructLoopConditionPlan struct {
	kind       fastStructLoopConditionKind
	operator   string
	value      fastStructLoopCallArgPlan
	leftValue  fastStructLoopCallArgPlan
	rightValue fastStructLoopCallArgPlan
	left       *fastStructLoopConditionPlan
	right      *fastStructLoopConditionPlan
	line       int
}

type fastConditionOperandValue struct {
	raw        interface{}
	reflect    reflect.Value
	hasReflect bool
}

type fastFieldChainPlanKey struct {
	plan *compiler.FastValuePlan
	typ  reflect.Type
}

type fastFieldChainPlan struct {
	steps []fastFieldChainStep
}

type fastFieldChainStep struct {
	name      string
	receiver  string
	full      string
	line      int
	fieldType reflect.Type
	lookup    propertyLookup
}

type fastAccessChainPlanKey struct {
	plan *compiler.FastValuePlan
	typ  reflect.Type
}

type fastAccessStepKind uint8

const (
	fastAccessStepField fastAccessStepKind = iota
	fastAccessStepIndex
)

type fastMapDirectKind uint8

const (
	fastMapDirectNone fastMapDirectKind = iota
	fastMapDirectStringString
	fastMapDirectStringInt
	fastMapDirectStringUint32
	fastMapDirectStringInterface
)

type fastAccessChainPlan struct {
	steps []fastAccessChainStep
}

type fastAccessChainStep struct {
	kind       fastAccessStepKind
	name       string
	receiver   string
	full       string
	line       int
	index      int
	fieldType  reflect.Type
	lookup     propertyLookup
	mapKey     reflect.Value
	mapString  string
	mapDirect  fastMapDirectKind
	resultType reflect.Type
}

type fastTopLevelAccessKind uint8

const (
	fastTopLevelAccessUnsupported fastTopLevelAccessKind = iota
	fastTopLevelAccessFieldChain
	fastTopLevelAccessChain
	fastTopLevelAccessMethodCall
)

type fastTopLevelAccessCacheEntry struct {
	typ        reflect.Type
	kind       fastTopLevelAccessKind
	fieldChain *fastFieldChainPlan
	chain      *fastAccessChainPlan
	method     *fastLoopMethodCallPlan
	next       *fastTopLevelAccessCacheEntry
}

type fastLoopMethodCallPlan struct {
	receiver *fastAccessChainPlan
	method   compiler.FastPathStep
	call     compiler.FastPathStep
	lookup   propertyLookup
}

type fastStructLoopCallArgKind uint8

const (
	fastStructLoopCallArgGeneric fastStructLoopCallArgKind = iota
	fastStructLoopCallArgKey
	fastStructLoopCallArgBinding
	fastStructLoopCallArgNil
	fastStructLoopCallArgString
	fastStructLoopCallArgInt
	fastStructLoopCallArgFloat
	fastStructLoopCallArgBool
	fastStructLoopCallArgAccessChain
)

type fastStructLoopCallPlan struct {
	call *compiler.FastCallPlan
	args []fastStructLoopCallArgPlan
}

type fastStructLoopCallArgPlan struct {
	kind       fastStructLoopCallArgKind
	value      compiler.FastValuePlan
	nameIndex  int
	stringVal  string
	intVal     int64
	floatVal   float64
	boolVal    bool
	accessPlan *fastAccessChainPlan
	line       int
}

type fastStructLoopRenderState struct {
	singleCall         *fastStructLoopCallPlan
	singleResolvedCall *fastStructLoopResolvedCall
	calls              map[*fastStructLoopCallPlan]*fastStructLoopResolvedCall
}

type fastStructLoopResolvedCall struct {
	raw                interface{}
	fn                 reflect.Value
	entry              *fastBuilderCallCacheEntry
	directWriter       fastStructLoopDirectCallWriter
	reflectArgs        []reflect.Value
	staticReflectArgs  []reflect.Value
	staticReflectArgOK []bool
	canReflect         bool
}

type fastMixedOpKind uint8

const (
	fastMixedOpStatic fastMixedOpKind = iota
	fastMixedOpName
	fastMixedOpProperty
	fastMixedOpValue
	fastMixedOpAccessChain
	fastMixedOpCall
	fastMixedOpBlockCall
	fastMixedOpConditional
	fastMixedOpPartial
	fastMixedOpLoop
	fastMixedOpLet
	fastMixedOpAssign
)

type fastMixedPlan struct {
	ops        []fastMixedOp
	staticName *fastStaticNamePlan
	simple     *fastSimplePlan
	staticSize int
	nameCount  int
}

type fastMixedOp struct {
	kind          fastMixedOpKind
	prefix        string
	value         string
	nameIndex     int
	nullOnMissing bool
	property      string
	receiver      string
	full          string
	line          int
	loop          *compiler.FastLoopPlan
	valuePlan     compiler.FastValuePlan
	call          *compiler.FastCallPlan
	blockCall     *compiler.FastBlockCallPlan
	conditional   *compiler.FastConditionalPlan
	partial       *compiler.FastPartialPlan
	partialData   *fastPartialDataBindingPlan
	simpleCond    *fastSimpleConditionalPlan
	accessCache   object.InlineCacheSlot
	propertyCache object.InlineCacheSlot
	outputCache   object.InlineCacheSlot
}

type fastStaticNamePlan struct {
	ops         []fastStaticNameOp
	nameIndexes []int
}

type fastStaticNameOp struct {
	prefix        string
	value         string
	nameIndex     int
	lookupIndex   int
	nullOnMissing bool
	line          int
	outputCache   *object.InlineCacheSlot
}

type fastSimplePlan struct {
	ops         []fastSimpleOp
	nameIndexes []int
}

type fastSimpleOp struct {
	op          *fastMixedOp
	lookupIndex int
	value       *fastSimpleValuePlan
}

type fastSimpleValuePlan struct {
	value       *compiler.FastValuePlan
	lookupIndex int
	left        *fastSimpleValuePlan
	right       *fastSimpleValuePlan
	args        []*fastSimpleValuePlan
}

type fastSimpleConditionalPlan struct {
	branches     []fastSimpleConditionalBranch
	elseSegments *fastSimplePlan
	nameIndexes  []int
}

type fastSimpleConditionalBranch struct {
	condition *fastSimpleValuePlan
	segments  *fastSimplePlan
	line      int
}

type fastPartialDataBindingPlan struct {
	pairs       []fastPartialDataBindingPair
	nameIndexes []int
	keys        []string
}

type fastPartialDataBindingPair struct {
	key   string
	value *fastSimpleValuePlan
	line  int
}

type fastSimpleNameBinder interface {
	bindNameIndex(int) int
}

type regexCacheEntry struct {
	re  *regexp.Regexp
	err error
}

type partialBytecodeLink struct {
	mu          sync.RWMutex
	sourceHash  uint64
	source      string
	bytecode    *compiler.Bytecode
	bindingPlan *fastRenderBindingPlan
}

type partialBytecodeLinkCache struct {
	mu          sync.RWMutex
	entries     map[string]*partialBytecodeLink
	feederID    int
	hasFeederID bool
	metaIDs     partialMetaIDs
	hasMetaIDs  bool
}
