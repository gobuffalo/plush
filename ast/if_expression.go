package ast

import (
	"bytes"
)

type IfExpression struct {
	TokenAble
	Condition Expression
	Block     *BlockStatement
	ElseIf    []*ElseIfExpression
	ElseBlock *BlockStatement
}

var _ Expression = &IfExpression{}

type ElseIfExpression struct {
	TokenAble
	Condition Expression
	Block     *BlockStatement
}

func (ie *IfExpression) expressionNode() {}

func (ie *IfExpression) String() string {
	var out bytes.Buffer

	out.WriteString("if (")
	if ie.Condition != nil {
		out.WriteString(ie.Condition.String())
	}
	out.WriteString(") { ")
	if ie.Block != nil {
		out.WriteString(ie.Block.String())
	}
	out.WriteString(" }")

	for _, elseIf := range ie.ElseIf {
		if elseIf == nil {
			continue
		}
		out.WriteString(" } else if (")
		if elseIf.Condition != nil {
			out.WriteString(elseIf.Condition.String())
		}
		out.WriteString(") { ")
		if elseIf.Block != nil {
			out.WriteString(elseIf.Block.String())
		}
		out.WriteString(" }")
	}

	if ie.ElseBlock != nil {
		out.WriteString(" } else { ")
		out.WriteString(ie.ElseBlock.String())
		out.WriteString(" }")
	}

	return out.String()
}
