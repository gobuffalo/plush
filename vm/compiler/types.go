package compiler

import (
	"sync/atomic"

	"github.com/gobuffalo/plush/v5/ast"
	"github.com/gobuffalo/plush/v5/vm/code"
	"github.com/gobuffalo/plush/v5/vm/object"
)

type Bytecode struct {
	Instructions              code.Instructions
	CallNames                 map[int]string
	LocalNames                map[int]string
	DynamicContextNameIndexes []int
	DynamicContextNamesReady  bool
	LineNumbers               map[int]int
	Properties                map[int]object.PropertyAccess
	PropertyCaches            []object.InlineCacheSlot
	CallCaches                []object.InlineCacheSlot
	RegexCaches               map[int]*object.InlineCacheSlot
	NumLocals                 int
	NumGlobals                int
	Constants                 []object.Object
	GlobalNames               map[int]string
	Static                    bool
	StaticOutput              string
	StaticSize                int
	FastRenderPlan            *FastRenderPlan
	FastRejectLine            int
	FastReject                string
	FastDiagnostics           atomic.Value
	OutputSizeStats           *OutputSizeStats
	LayoutSizeStats           *OutputSizeStats
	LayoutSizeProfile         *LayoutOutputSizeProfile
	PartialSizeStats          *OutputSizeStats
	HasHoles                  bool
	HasPartials               bool
	HasContextWrites          bool
}

func (b *Bytecode) ContextWrites() bool {
	return b != nil && b.HasContextWrites
}

type FastRenderReject struct {
	Line   int
	Reason string
}

type FastRenderSegmentKind uint8

// When adding a segment kind, classify its binding behavior in
// fastRenderSegmentBindingEffect. The binding-sync plan is trusted by the VM
// to restore lexical scopes without copying every local binding.
const (
	FastRenderSegmentStatic FastRenderSegmentKind = iota
	FastRenderSegmentName
	FastRenderSegmentProperty
	FastRenderSegmentLoop
	FastRenderSegmentValue
	FastRenderSegmentCall
	FastRenderSegmentBlockCall
	FastRenderSegmentConditional
	FastRenderSegmentPartial
	FastRenderSegmentLet
	FastRenderSegmentAssign
	FastRenderSegmentReturn
	FastRenderSegmentGeneric
	fastRenderSegmentKindCount
)

type FastRenderSegment struct {
	Kind          FastRenderSegmentKind
	Value         string
	NameIndex     int
	NullOnMissing bool
	Property      string
	Receiver      string
	Full          string
	Line          int
	Loop          *FastLoopPlan
	ValuePlan     FastValuePlan
	Call          *FastCallPlan
	BlockCall     *FastBlockCallPlan
	Conditional   *FastConditionalPlan
	Partial       *FastPartialPlan
	Generic       *FastGenericPlan
	AssignTarget  *FastAssignTarget
	PropertyCache object.InlineCacheSlot
	CallCache     object.InlineCacheSlot
	OutputCache   object.InlineCacheSlot
}

// FastBindingSyncPlan lists outer binding slots that a lexical scope may
// update and local slots that must be restored when the scope exits. Prepared
// distinguishes a completed plan from one that is older, manually assembled,
// unclassified, or deferred by the metadata budget and needs compatibility
// handling in the VM.
type FastBindingSyncPlan struct {
	NameIndexes       []int
	LocalNameIndexes  []int
	ParentNameIndexes []int
	Prepared          bool
}

type FastRenderPlan struct {
	Bindings   []string
	Segments   []FastRenderSegment
	StaticSize int
	NameCount  int
	// Prepared caches the VM-side mixed/static-name/simple plan built from this
	// compiler plan. BindingPrepared caches interned context IDs for stable
	// contexts. See vm/FAST_PATHS.md for the full fast path map.
	Prepared        atomic.Value
	BindingPrepared atomic.Value
}

type FastLoopPartKind uint8

// When adding a loop-part kind, classify its binding behavior in
// fastLoopPartBindingEffect. Unclassified kinds must leave binding-sync plans
// unprepared so the VM uses its compatibility path.
const (
	FastLoopPartStatic FastLoopPartKind = iota
	FastLoopPartKey
	FastLoopPartValue
	FastLoopPartValueProperty
	FastLoopPartValuePath
	FastLoopPartCall
	FastLoopPartConditional
	FastLoopPartLoop
	FastLoopPartBreak
	FastLoopPartContinue
	FastLoopPartBlockCall
	FastLoopPartPartial
	FastLoopPartLet
	FastLoopPartAssign
	FastLoopPartReturn
	fastLoopPartKindCount
)

type FastLoopPart struct {
	Kind          FastLoopPartKind
	Value         string
	NameIndex     int
	Receiver      string
	Full          string
	Line          int
	ValuePlan     FastValuePlan
	Call          *FastCallPlan
	BlockCall     *FastBlockCallPlan
	Partial       *FastPartialPlan
	AssignTarget  *FastAssignTarget
	Conditional   *FastLoopConditionalPlan
	Loop          *FastLoopPlan
	PropertyCache object.InlineCacheSlot
}

type FastLoopPlan struct {
	IterableName      string
	IterableNameIndex int
	Iterable          FastValuePlan
	KeyName           string
	ValueName         string
	OuterNames        []string
	Parts             []FastLoopPart
	StaticSize        int
	Silent            bool
	HasLet            bool
	HasAssign         bool
	PartFlagsSet      bool
	BindingSync       FastBindingSyncPlan
	Line              int
	SizeStats         *LoopSizeStats
}

type FastValueKind uint8

const (
	FastValueInvalid FastValueKind = iota
	FastValueName
	FastValueString
	FastValueInteger
	FastValueFloat
	FastValueBool
	FastValuePath
	FastValueLoopKey
	FastValueInfix
	FastValueCall
	FastValuePrefix
	FastValueConcat
	FastValueArray
	FastValueHash
	FastValueIndex
)

type FastPathStepKind uint8

const (
	FastPathStepProperty FastPathStepKind = iota
	FastPathStepIndexInteger
	FastPathStepIndexString
	FastPathStepCall
)

type FastValuePlan struct {
	Kind          FastValueKind
	Value         string
	NameIndex     int
	NullOnMissing bool
	IntValue      int64
	FloatValue    float64
	BoolValue     bool
	Operator      string
	Left          *FastValuePlan
	Right         *FastValuePlan
	Call          *FastCallPlan
	Elements      []FastValuePlan
	Pairs         []FastValuePair
	Path          []FastPathStep
	RegexCache    *object.InlineCacheSlot
	Line          int
}

type FastValuePair struct {
	Key     string
	KeyPlan *FastValuePlan
	Value   FastValuePlan
	Line    int
}

type FastPathStep struct {
	Kind          FastPathStepKind
	Value         string
	Index         int
	Receiver      string
	Full          string
	Method        bool
	Line          int
	Args          []FastValuePlan
	PropertyCache object.InlineCacheSlot
	CallCache     object.InlineCacheSlot
}

type FastAssignTargetKind uint8

const (
	FastAssignTargetName FastAssignTargetKind = iota
	FastAssignTargetIndex
)

type FastAssignTarget struct {
	Kind      FastAssignTargetKind
	Name      string
	NameIndex int
	Container FastValuePlan
	Index     FastValuePlan
	Line      int
}

type FastCallPlan struct {
	Name      string
	NameIndex int
	Args      []FastValuePlan
	Silent    bool
	Line      int
	Cache     object.InlineCacheSlot
}

type FastBlockCallPlan struct {
	Name          string
	NameIndex     int
	Args          []FastValuePlan
	Block         *ast.BlockStatement
	BlockSource   string
	BlockBytecode *Bytecode
	Silent        bool
	Line          int
	Cache         object.InlineCacheSlot
}

type FastPartialPlan struct {
	Name string
	Data []FastPartialDataPair
	Line int
}

type FastPartialDataPair struct {
	Key   string
	Value FastValuePlan
	Line  int
}

type FastGenericPlan struct {
	WholeTemplate bool
	Reason        string
	Line          int
}

type FastConditionalBranch struct {
	Condition   FastValuePlan
	Segments    []FastRenderSegment
	BindingSync FastBindingSyncPlan
	Line        int
}

type FastConditionalPlan struct {
	Branches        []FastConditionalBranch
	ElseSegments    []FastRenderSegment
	ElseBindingSync FastBindingSyncPlan
	Line            int
	Silent          bool
}

type FastLoopConditionalBranch struct {
	Condition   FastValuePlan
	Parts       []FastLoopPart
	BindingSync FastBindingSyncPlan
	Line        int
}

type FastLoopConditionalPlan struct {
	Branches        []FastLoopConditionalBranch
	ElseParts       []FastLoopPart
	ElseBindingSync FastBindingSyncPlan
	Line            int
	Silent          bool
}

type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

type CompilationScope struct {
	instructions        code.Instructions
	callNames           map[int]string
	localNames          map[int]string
	lineNumbers         map[int]int
	properties          map[int]object.PropertyAccess
	numLocals           int
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
}
