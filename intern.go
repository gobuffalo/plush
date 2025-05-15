package plush

type InternTable struct {
	stringToID map[string]int
	idToString []string
}

func NewInternTable() *InternTable {
	return &InternTable{
		stringToID: make(map[string]int),
		idToString: []string{},
	}
}

func (it *InternTable) Intern(name string) int {
	if id, ok := it.stringToID[name]; ok {
		return id
	}
	id := len(it.idToString)
	it.stringToID[name] = id
	it.idToString = append(it.idToString, name)
	return id
}

func (it *InternTable) Lookup(name string) (int, bool) {
	id, ok := it.stringToID[name]
	return id, ok
}

func (it *InternTable) SymbolName(id int) string {
	if id < len(it.idToString) {
		return it.idToString[id]
	}
	return "<unknown>"
}
