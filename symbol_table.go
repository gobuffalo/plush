package plush

// SymbolTable represents a scope
type SymbolTable struct {
	vars   map[int]interface{}
	parent *SymbolTable
	// Interning system
	localInterner  *InternTable
	globalInterner *InternTable
}

// NewScope creates a new scope with an optional parent
func NewScope(parent *SymbolTable) *SymbolTable {
	if parent == nil {
		global := NewInternTable()
		local := NewInternTable()
		return &SymbolTable{
			vars:           make(map[int]interface{}),
			parent:         nil,
			globalInterner: global,
			localInterner:  local,
		}
	}

	// Inherit interning from parent
	return &SymbolTable{
		vars:           make(map[int]interface{}),
		parent:         parent,
		globalInterner: parent.globalInterner,
		localInterner:  parent.localInterner,
	}
}

// Declare adds or updates a variable in the current scope
func (s *SymbolTable) Declare(name string, value interface{}) {
	if value == nil {
		return
	}
	id := s.localInterner.Intern(name)
	s.vars[id] = value
}

// Assign searches outer scopes and updates an existing variable
func (s *SymbolTable) Assign(name string, value interface{}) bool {
	if id, ok := s.localInterner.Lookup(name); ok {
		for curr := s; curr != nil; curr = curr.parent {
			if _, exists := curr.vars[id]; exists {
				curr.vars[id] = value
				return true
			}
		}
	}
	if id, ok := s.globalInterner.Lookup(name); ok {
		for curr := s; curr != nil; curr = curr.parent {
			if _, exists := curr.vars[id]; exists {
				curr.vars[id] = value
				return true
			}
		}
	}
	return false
}

// Resolve finds the value of a variable
func (s *SymbolTable) Resolve(name string) (interface{}, bool) {
	if id, ok := s.localInterner.Lookup(name); ok {
		for curr := s; curr != nil; curr = curr.parent {
			if val, exists := curr.vars[id]; exists {

				return val, true
			}
		}
	}

	if id, ok := s.globalInterner.Lookup(name); ok {
		for curr := s; curr != nil; curr = curr.parent {
			if val, exists := curr.vars[id]; exists {
				return val, true
			}
		}
	}

	return nil, false
}
