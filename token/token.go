package token

import "fmt"

// Type represents each type of token.
type Type string

// Token of a section of input source.
type Token struct {
	Type       Type
	Literal    string
	LineNumber int
}

var keywords = map[string]Type{
	"fn":     FUNCTION,
	"func":   FUNCTION,
	"let":    LET,
	"true":   TRUE,
	"false":  FALSE,
	"if":     IF,
	"else":   ELSE,
	"return": RETURN,
	"for":    FOR,
	"in":     IN,
}

var dynamic = map[Type]Type{}

// LookupIdent an ident and return a keyword type, or a plain ident
func LookupIdent(ident string) Type {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

func SetTemplatingDelimiters(start, end string) {
	replace(S_START, Type(start))
	replace(C_START, Type(fmt.Sprintf("%v#", start)))
	replace(E_START, Type(fmt.Sprintf("%v=", start)))
	replace(E_END, Type(end))
}

func replace(token Type, replacement Type) {
      dynamic[token] = replacement
}

func Resolve(token Type) Type {
	if tok, ok := dynamic[token]; ok {
		return tok
	}
	return token
}
