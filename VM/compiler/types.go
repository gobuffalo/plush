package compiler

import (
	"sync/atomic"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
	"github.com/gobuffalo/plush/v5/ast"
)

type Bytecode struct {
	Instructions    code.Instructions
	CallNames       map[int]string
	LocalNames      map[int]string
	LineNumbers     map[int]int
	Properties      map[int]object.PropertyAccess
	PropertyCaches  []object.InlineCacheSlot
	CallCaches      []object.InlineCacheSlot
	NumLocals       int
	NumGlobals      int
	Constants       []object.Object
	GlobalNames     map[int]string
	Static          bool
	StaticOutput    string
	FastRenderPlan  *FastRenderPlan
	FastRejectLine  int
	FastReject      string
	FastDiagnostics atomic.Value
	HasHoles        bool
	HasPartials     bool
}

type FastRenderReject struct {
	Line   int
	Reason string
}

type FastRenderSegmentKind uint8

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
	PropertyCache object.InlineCacheSlot
	CallCache     object.InlineCacheSlot
	OutputCache   object.InlineCacheSlot
}

type FastRenderPlan struct {
	Bindings   []string
	Segments   []FastRenderSegment
	StaticSize int
	NameCount  int
	// Prepared caches the VM-side mixed/static-name/simple plan built from this
	// compiler plan. BindingPrepared caches interned context IDs for stable
	// contexts. See VM/FAST_PATHS.md for the full fast path map.
	Prepared        atomic.Value
	BindingPrepared atomic.Value
}

type FastLoopPartKind uint8

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
	Line              int
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
	Line          int
}

type FastValuePair struct {
	Key   string
	Value FastValuePlan
	Line  int
}

type FastPathStep struct {
	Kind          FastPathStepKind
	Value         string
	Index         int
	Receiver      string
	Full          string
	Method        bool
	Line          int
	PropertyCache object.InlineCacheSlot
	CallCache     object.InlineCacheSlot
}

type FastCallPlan struct {
	Name      string
	NameIndex int
	Args      []FastValuePlan
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

type FastConditionalBranch struct {
	Condition FastValuePlan
	Segments  []FastRenderSegment
	Line      int
}

type FastConditionalPlan struct {
	Branches     []FastConditionalBranch
	ElseSegments []FastRenderSegment
	Line         int
	Silent       bool
}

type FastLoopConditionalBranch struct {
	Condition FastValuePlan
	Parts     []FastLoopPart
	Line      int
}

type FastLoopConditionalPlan struct {
	Branches  []FastLoopConditionalBranch
	ElseParts []FastLoopPart
	Line      int
	Silent    bool
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
