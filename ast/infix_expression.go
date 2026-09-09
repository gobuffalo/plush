package ast

import (
	"bytes"
	"regexp"
	"sync/atomic"
)

type InfixExpression struct {
	TokenAble
	Left     Expression
	Operator string
	Right    Expression
	regex    *expressionRegexCache
}

type expressionRegexCache struct {
	entry atomic.Pointer[expressionRegexCacheEntry]
}

type expressionRegexCacheEntry struct {
	pattern string
	re      *regexp.Regexp
	err     error
}

// NewInfixExpression creates an infix AST node and prepares expression-local
// state for operators that cache reusable evaluation metadata.
func NewInfixExpression(token TokenAble, left Expression, operator string) *InfixExpression {
	expression := &InfixExpression{
		TokenAble: token,
		Left:      left,
		Operator:  operator,
	}
	if operator == "~=" {
		expression.regex = &expressionRegexCache{}
	}
	return expression
}

// CachedRegex returns the regex compiled for this expression's current
// pattern. A different dynamic pattern replaces the previous entry, keeping
// memory bounded to one regex per expression.
func (oe *InfixExpression) CachedRegex(pattern string) (*regexp.Regexp, error) {
	if oe != nil && oe.regex != nil {
		if cached := oe.regex.entry.Load(); cached != nil && cached.pattern == pattern {
			return cached.re, cached.err
		}
	}

	re, err := regexp.Compile(pattern)
	if oe != nil && oe.regex != nil {
		oe.regex.entry.Store(&expressionRegexCacheEntry{pattern: pattern, re: re, err: err})
	}
	return re, err
}

var _ Comparable = &InfixExpression{}
var _ Expression = &InfixExpression{}

func (oe *InfixExpression) validIfCondition() bool { return true }

func (oe *InfixExpression) expressionNode() {}

func (oe *InfixExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")

	if oe.Left != nil {
		out.WriteString(oe.Left.String())
	}

	out.WriteString(" " + oe.Operator + " ")

	if oe.Right != nil {
		out.WriteString(oe.Right.String())
	} else {
		out.WriteString(" !!MISSING '%>'!!")
	}

	out.WriteString(")")

	return out.String()
}
