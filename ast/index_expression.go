package ast

import (
	"bytes"
)

type IndexExpression struct {
	TokenAble
	Left  Expression
	Index Expression
	Value Expression
}

func (ie *IndexExpression) validIfCondition() bool { return true }

func (ie *IndexExpression) expressionNode() {}

func (ie *IndexExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString("[")
	out.WriteString(ie.Index.String())
	out.WriteString("])")
	if ie.Value != nil {
		out.WriteString("=")
		out.WriteString(ie.Value.String())
	}

	return out.String()
}
