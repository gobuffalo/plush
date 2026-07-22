package compiler

type SymbolScope string

const (
	LocalScope    SymbolScope = "LOCAL"
	GlobalScope   SymbolScope = "GLOBAL"
	BuiltinScope  SymbolScope = "BUILTIN"
	FreeScope     SymbolScope = "FREE"
	FunctionScope SymbolScope = "FUNCTION"
)

type Symbol struct {
	Name  string
	Scope SymbolScope
	Index int
}

type SymbolTable struct {
	Outer *SymbolTable

	store          map[string]Symbol
	numDefinitions int

	FreeSymbols []Symbol

	inlineBlock bool
}

func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	s := NewSymbolTable()
	s.Outer = outer
	return s
}

func NewInlineBlockSymbolTable(outer *SymbolTable) *SymbolTable {
	s := NewEnclosedSymbolTable(outer)
	s.inlineBlock = true
	return s
}

func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		store:       map[string]Symbol{},
		FreeSymbols: []Symbol{},
	}
}

func (s *SymbolTable) Define(name string) Symbol {
	symbol := Symbol{Name: name}
	if s.Outer == nil && !s.inlineBlock {
		symbol.Index = s.numDefinitions
		symbol.Scope = GlobalScope
		s.numDefinitions++
		s.store[name] = symbol
		return symbol
	}

	if s.inlineBlock {
		owner := s.localDefinitionOwner()
		if owner != nil && owner.Outer != nil {
			symbol.Index = owner.numDefinitions
			owner.numDefinitions++
		} else {
			symbol.Index = s.numDefinitions
			s.numDefinitions++
		}
		symbol.Scope = LocalScope
		s.store[name] = symbol
		return symbol
	}

	symbol.Index = s.numDefinitions
	symbol.Scope = LocalScope
	s.store[name] = symbol
	s.numDefinitions++
	return symbol
}

func (s *SymbolTable) localDefinitionOwner() *SymbolTable {
	for owner := s.Outer; owner != nil; owner = owner.Outer {
		if !owner.inlineBlock {
			return owner
		}
	}
	return nil
}

func (s *SymbolTable) Resolve(name string) (Symbol, bool) {
	obj, ok := s.store[name]
	if !ok && s.Outer != nil {
		obj, ok = s.Outer.Resolve(name)
		if !ok {
			return obj, ok
		}

		if s.inlineBlock {
			return obj, ok
		}

		if obj.Scope == GlobalScope || obj.Scope == BuiltinScope {
			return obj, ok
		}

		free := s.defineFree(obj)
		return free, true
	}
	return obj, ok
}

func (s *SymbolTable) DefineBuiltin(index int, name string) Symbol {
	symbol := Symbol{Name: name, Index: index, Scope: BuiltinScope}
	s.store[name] = symbol
	return symbol
}

func (s *SymbolTable) DefineFunctionName(name string) Symbol {
	symbol := Symbol{Name: name, Index: 0, Scope: FunctionScope}
	s.store[name] = symbol
	return symbol
}

func (s *SymbolTable) defineFree(original Symbol) Symbol {
	s.FreeSymbols = append(s.FreeSymbols, original)

	symbol := Symbol{Name: original.Name, Index: len(s.FreeSymbols) - 1, Scope: FreeScope}
	s.store[original.Name] = symbol
	return symbol
}
