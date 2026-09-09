package compiler

import (
	"github.com/gobuffalo/plush/v5/vm/code"
	"github.com/gobuffalo/plush/v5/vm/object"
)

// regexCacheSlots returns one cache slot for each regex-match instruction.
// The map is immutable after compilation, while each slot is safe to update
// concurrently as the expression observes different dynamic patterns.
func regexCacheSlots(instructions code.Instructions) map[int]*object.InlineCacheSlot {
	var slots map[int]*object.InlineCacheSlot
	for pos := 0; pos < len(instructions); {
		def, err := code.Lookup(instructions[pos])
		if err != nil {
			pos++
			continue
		}
		if code.Opcode(instructions[pos]) == code.OpMatches {
			if slots == nil {
				slots = make(map[int]*object.InlineCacheSlot)
			}
			slots[pos] = &object.InlineCacheSlot{}
		}
		_, read := code.ReadOperands(def, instructions[pos+1:])
		pos += 1 + read
	}
	return slots
}

func fastRegexCacheSlot(operator string) *object.InlineCacheSlot {
	if operator != "~=" {
		return nil
	}
	return &object.InlineCacheSlot{}
}
