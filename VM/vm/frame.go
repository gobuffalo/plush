package vm

import (
	"strings"

	"github.com/gobuffalo/plush/v5/VM/code"
	"github.com/gobuffalo/plush/v5/VM/object"
)

type Frame struct {
	cl            *object.Closure
	ip            int
	basePointer   int
	output        strings.Builder
	hasOutput     bool
	block         *object.Closure
	pooled        bool
	writeReturn   bool
	calleeOnStack bool
}

func NewFrame(cl *object.Closure, basePointer int) *Frame {
	return &Frame{
		cl:          cl,
		ip:          -1,
		basePointer: basePointer,
	}
}

func (f *Frame) reset(cl *object.Closure, basePointer int) {
	f.cl = cl
	f.ip = -1
	f.basePointer = basePointer
	f.output.Reset()
	f.hasOutput = false
	f.block = nil
	f.writeReturn = false
	f.calleeOnStack = false
}

func (f *Frame) Instructions() code.Instructions {
	return f.cl.Fn.Instructions
}
