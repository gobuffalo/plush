package ast

import (
	"bytes"
	"monkey/token"
)

type ForExpression struct {
	Token       token.Token
	KeyName     string
	ValueName   string
	Consequence *BlockStatement
	Iterable    Expression
}

func (fe *ForExpression) expressionNode()      {}
func (fe *ForExpression) TokenLiteral() string { return fe.Token.Literal }
func (fe *ForExpression) String() string {
	var out bytes.Buffer
	out.WriteString("for (")
	out.WriteString(fe.KeyName)
	out.WriteString(", ")
	out.WriteString(fe.ValueName)
	out.WriteString(") in ")
	out.WriteString(fe.Iterable.String())
	out.WriteString(" { ")
	out.WriteString(fe.Consequence.String())
	out.WriteString(" }")
	return out.String()
}
