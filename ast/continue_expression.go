package ast

type ContinueExpression struct {
	TokenAble
}

func (ce *ContinueExpression) expressionNode() {}

func (ce *ContinueExpression) String() string {
	return ce.Token.Literal
}
