package ast

type BreakExpression struct {
	TokenAble
}

func (ce *BreakExpression) expressionNode() {}

func (ce *BreakExpression) String() string {
	return ce.Token.Literal
}
