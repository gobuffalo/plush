package plush

import "sync"

var sharedHelperScope = struct {
	sync.RWMutex
	version uint64
	scope   *SymbolTable
}{}

func cachedHelperScope() *SymbolTable {
	version := Helpers.Version()

	sharedHelperScope.RLock()
	if sharedHelperScope.scope != nil && sharedHelperScope.version == version {
		scope := sharedHelperScope.scope
		sharedHelperScope.RUnlock()
		return scope
	}
	sharedHelperScope.RUnlock()

	sharedHelperScope.Lock()
	defer sharedHelperScope.Unlock()
	if sharedHelperScope.scope != nil && sharedHelperScope.version == version {
		return sharedHelperScope.scope
	}

	helpers, version := Helpers.Snapshot()
	if sharedHelperScope.scope != nil && sharedHelperScope.version == version {
		return sharedHelperScope.scope
	}

	scope := newRootScopeFromMap(helpers)
	sharedHelperScope.scope = scope
	sharedHelperScope.version = version
	return scope
}
