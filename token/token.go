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

type delimiterLengthError struct {
	Delimiter string
	Length    int
}

func (e *delimiterLengthError) Error() string {
	return fmt.Sprintf("Incorrect delimiter \"%s\" length. %v chars allowed", e.Delimiter, e.Length)
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
	if err := checkDelimitersLength([]string{start, end}); err != nil {
		return err
	}
	dynamic[S_START] = Type(start)
	dynamic[C_START] = Type(fmt.Sprintf("%v#", start))
	dynamic[E_START] = Type(fmt.Sprintf("%v=", start))
	dynamic[E_END] = Type(end)
	return nil
}

// Resolve token.Type, return dynamic replacement if found or default Type
func Resolve(token Type) Type {
	if tok, ok := dynamic[token]; ok {
		return tok
	}
	return token
}

func checkDelimitersLength(arr []string) error {
	for _, d := range arr {
		if len(d) != templateDelimitersLen {
			return &delimiterLengthError{d, templateDelimitersLen}
		}
	}
	return nil
}

// BeginsWith returns true if first char of type matches input
func (t *Type) BeginsWith(ch byte) bool {
	return (*t)[0] == ch
}

func (t *Type) endsWith(ch byte) bool {
	return (*t)[1] == ch
}

// MatchAhead returns true if token matches firstChar and nextChar
func MatchAhead(token Type, firstChar, lastChar byte) bool {
	token = Resolve(token)
	return token.BeginsWith(firstChar) && token.endsWith(lastChar)
}
