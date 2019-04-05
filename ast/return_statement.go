package ast

import (
	"bytes"

	"github.com/gobuffalo/plush/token"
)

type ReturnStatement struct {
	Type string
	TokenAble
	ReturnValue Expression
}

func (rs *ReturnStatement) Printable() bool {
	return true
}

func (rs *ReturnStatement) statementNode() {}

func (rs *ReturnStatement) String() string {
	var out bytes.Buffer

	if rs.Type == token.E_START {
		out.WriteString("<%= ")
	} else {
		out.WriteString("return ")
	}

	if rs.ReturnValue != nil {
		out.WriteString(rs.ReturnValue.String())
	}

	if rs.Type == token.E_START {
		out.WriteString("; %>")
	} else {
		out.WriteString(";")
	}

	return out.String()
}
