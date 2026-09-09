package vm

import (
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/gobuffalo/plush/v5"
	"github.com/gobuffalo/plush/v5/helpers/hctx"
	"github.com/gobuffalo/plush/v5/vm/compiler"
	"github.com/gobuffalo/plush/v5/vm/object"
)

type callCacheEntry struct {
	rt      reflect.Type
	plan    *callPlan
	invoker writeFastInvoker
	noFast  bool
}

type fastBlockInvoker func(out *strings.Builder, ctx hctx.Context, name string, raw interface{}, args *fastCallArgs, helperCtx plush.HelperContext) error

type fastBuilderCallCacheEntry struct {
	rt                               reflect.Type
	plan                             *callPlan
	invoker                          writeFastBuilderInvoker
	blockInvoker                     fastBlockInvoker
	valueInvoker                     valueFastInvoker
	contextualValueInvoker           contextualValueFastInvoker
	contextualValueInvokerReflective bool
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
	regexCache *object.InlineCacheSlot
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
	fastMixedOpReturn
	fastMixedOpGeneric
)

type fastMixedPlan struct {
	ops          []fastMixedOp
	staticName   *fastStaticNamePlan
	simple       *fastSimplePlan
	runtimePlans *fastRuntimePlanCache
	staticSize   int
	nameCount    int
}

// fastRuntimePlanCache owns the reflection plans derived from one compiled
// FastRenderPlan. Keeping these maps under the prepared plan makes template
// cache invalidation release the derived plans with their bytecode instead of
// retaining compiled-plan pointers in process-wide caches.
type fastRuntimePlanCache struct {
	mu                    sync.RWMutex
	structLoopWriterPlans map[fastStructLoopWriterPlanKey]*fastStructLoopWriterPlan
	fieldChainPlans       map[fastFieldChainPlanKey]*fastFieldChainPlan
	accessChainPlans      map[fastAccessChainPlanKey]*fastAccessChainPlan
}

func optionalFastRuntimePlanCache(caches []*fastRuntimePlanCache) *fastRuntimePlanCache {
	if len(caches) == 0 {
		return nil
	}
	return caches[0]
}

func (c *fastRuntimePlanCache) structLoopWriterPlan(key fastStructLoopWriterPlanKey) (*fastStructLoopWriterPlan, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	plan, ok := c.structLoopWriterPlans[key]
	c.mu.RUnlock()
	return plan, ok
}

func (c *fastRuntimePlanCache) storeStructLoopWriterPlan(key fastStructLoopWriterPlanKey, plan *fastStructLoopWriterPlan) *fastStructLoopWriterPlan {
	if c == nil {
		return plan
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.structLoopWriterPlans[key]; ok {
		return cached
	}
	if c.structLoopWriterPlans == nil {
		c.structLoopWriterPlans = make(map[fastStructLoopWriterPlanKey]*fastStructLoopWriterPlan)
	}
	c.structLoopWriterPlans[key] = plan
	return plan
}

func (c *fastRuntimePlanCache) fieldChainPlan(key fastFieldChainPlanKey) (*fastFieldChainPlan, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	plan, ok := c.fieldChainPlans[key]
	c.mu.RUnlock()
	return plan, ok
}

func (c *fastRuntimePlanCache) storeFieldChainPlan(key fastFieldChainPlanKey, plan *fastFieldChainPlan) *fastFieldChainPlan {
	if c == nil {
		return plan
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.fieldChainPlans[key]; ok {
		return cached
	}
	if c.fieldChainPlans == nil {
		c.fieldChainPlans = make(map[fastFieldChainPlanKey]*fastFieldChainPlan)
	}
	c.fieldChainPlans[key] = plan
	return plan
}

func (c *fastRuntimePlanCache) accessChainPlan(key fastAccessChainPlanKey) (*fastAccessChainPlan, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	plan, ok := c.accessChainPlans[key]
	c.mu.RUnlock()
	return plan, ok
}

func (c *fastRuntimePlanCache) storeAccessChainPlan(key fastAccessChainPlanKey, plan *fastAccessChainPlan) *fastAccessChainPlan {
	if c == nil {
		return plan
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.accessChainPlans[key]; ok {
		return cached
	}
	if c.accessChainPlans == nil {
		c.accessChainPlans = make(map[fastAccessChainPlanKey]*fastAccessChainPlan)
	}
	c.accessChainPlans[key] = plan
	return plan
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
	generic       *compiler.FastGenericPlan
	assignTarget  *compiler.FastAssignTarget
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
	pattern string
	re      *regexp.Regexp
	err     error
}

type partialBytecodeLink struct {
	mu          sync.RWMutex
	sourceHash  uint64
	source      string
	bytecode    *compiler.Bytecode
	bindingPlan *fastRenderBindingPlan
}

type partialBytecodeLinkCache struct {
	mu                 sync.RWMutex
	entries            map[string]*partialBytecodeLink
	sourcePartialPlans map[sourcePartialPlanKey]*sourcePartialPlan
	feederID           int
	hasFeederID        bool
	metaIDs            partialMetaIDs
	hasMetaIDs         bool
}
