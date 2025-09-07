package ast

import "strings"

type HoleStatement struct {
	TokenAble
	Statements string
}

func (h *HoleStatement) statementNode() {}
func (h *HoleStatement) String() string {
	return strings.TrimSpace(h.Statements)
}
