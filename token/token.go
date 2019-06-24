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

const TEMPLATE_DELIMITERS_LEN = 2

type DelimitersLengthError struct {
	Delimiters []string
	Length     int
}

func (e *DelimitersLengthError) Error() string {
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

func SetTemplatingDelimiters(start, end string) error {
	if len(start) != TEMPLATE_DELIMITERS_LEN ||
		len(end) != TEMPLATE_DELIMITERS_LEN {
		return &DelimitersLengthError{[]string{start, end}, TEMPLATE_DELIMITERS_LEN}
	}
	replace(S_START, Type(start))
	replace(C_START, Type(fmt.Sprintf("%v#", start)))
	replace(E_START, Type(fmt.Sprintf("%v=", start)))
	replace(E_END, Type(end))
	return nil
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
