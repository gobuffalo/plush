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

const templateDelimitersLen = 2

type delimitersLengthError struct {
	Delimiters []string
	Length     int
}

func (e *delimitersLengthError) Error() string {
	return fmt.Sprintf("Incorrect delimiters \"%s\" length. %v chars allowed", e.Delimiters, e.Length)
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

// SetTemplatingDelimiters to start and end, or return delimitersLengthError if delimiters length is incorrect
func SetTemplatingDelimiters(start, end string) error {
	if len(start) != templateDelimitersLen ||
		len(end) != templateDelimitersLen {
		return &delimitersLengthError{[]string{start, end}, templateDelimitersLen}
	}
	replace(S_START, Type(start))
	replace(C_START, Type(fmt.Sprintf("%v#", start)))
	replace(E_START, Type(fmt.Sprintf("%v=", start)))
	replace(E_END, Type(end))
	return nil
}

func replace(token, replacement Type) {
	dynamic[token] = replacement
}

// Resolve token.Type, return dynamic replacement if found or default Type
func Resolve(token Type) Type {
	if tok, ok := dynamic[token]; ok {
		return tok
	}
	return token
}
