package ast

import (
	"bytes"
	"strings"
)

type HashLiteral struct {
	TokenAble
	Order []Expression
	Pairs map[Expression]Expression
}

var _ Expression = &HashLiteral{}

func (hl *HashLiteral) expressionNode() {}

func (hl *HashLiteral) String() string {
	if hl == nil {
		return ""
	}
	var out bytes.Buffer

	pairs := []string{}
	if len(hl.Pairs) == 0 {
		return ""
	}
	for _, key := range hl.Order {
		if key == nil {
			continue
		}
		p := hl.Pairs[key]
		if p != nil {
			pairs = append(pairs, key.String()+": "+p.String())
		} else {
			pairs = append(pairs, key.String()+": ")
		}
	}

	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")

	return out.String()
}
