package ast

import (
	"bytes"
)

type BlockStatement struct {
	TokenAble
	Statements []Statement
}

func (bs *BlockStatement) statementNode() {}

func (bs *BlockStatement) Value() string {
	var out bytes.Buffer
	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

func (bs *BlockStatement) String() string {
	var out bytes.Buffer
	for _, s := range bs.Statements {
		out.WriteString("\t" + s.String() + "\n")
	}
	return out.String()
}
