package vm

import (
	"hash/maphash"
	"sync"

	"github.com/gobuffalo/plush/v5/VM/compiler"
)

const sourceBytecodeCacheLimit = 1024

var (
	vmSourceHashSeed  = maphash.MakeSeed()
	sourceBytecodeMap = newSourceBytecodeCache(sourceBytecodeCacheLimit)
)

type sourceBytecodeCacheEntry struct {
	source   string
	bytecode *compiler.Bytecode
}

type sourceBytecodeCache struct {
	mu      sync.RWMutex
	limit   int
	entries map[uint64]sourceBytecodeCacheEntry
	order   []uint64
}

func newSourceBytecodeCache(limit int) *sourceBytecodeCache {
	return &sourceBytecodeCache{
		limit:   limit,
		entries: map[uint64]sourceBytecodeCacheEntry{},
		order:   make([]uint64, 0, limit),
	}
}

func hashString(input string) uint64 {
	return maphash.String(vmSourceHashSeed, input)
}

func cachedSourceBytecode(source string) (*compiler.Bytecode, bool) {
	if source == "" {
		return nil, false
	}
	return sourceBytecodeMap.Get(source)
}

func cacheSourceBytecode(source string, bytecode *compiler.Bytecode) {
	if source == "" || bytecode == nil {
		return
	}
	sourceBytecodeMap.Set(source, bytecode)
}

func clearSourceBytecodeCacheForTest() {
	sourceBytecodeMap.Clear()
}

func (c *sourceBytecodeCache) Get(source string) (*compiler.Bytecode, bool) {
	if c == nil || source == "" {
		return nil, false
	}
	hash := hashString(source)
	c.mu.RLock()
	entry, ok := c.entries[hash]
	c.mu.RUnlock()
	if !ok || entry.source != source || entry.bytecode == nil {
		return nil, false
	}
	return entry.bytecode, true
}

func (c *sourceBytecodeCache) Set(source string, bytecode *compiler.Bytecode) {
	if c == nil || c.limit <= 0 || source == "" || bytecode == nil {
		return
	}
	hash := hashString(source)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[hash]; !ok {
		c.order = append(c.order, hash)
	}
	c.entries[hash] = sourceBytecodeCacheEntry{source: source, bytecode: bytecode}
	for len(c.entries) > c.limit && len(c.order) > 0 {
		oldest := c.order[0]
		copy(c.order, c.order[1:])
		c.order = c.order[:len(c.order)-1]
		delete(c.entries, oldest)
	}
}

func (c *sourceBytecodeCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.entries)
	clear(c.order)
	c.order = c.order[:0]
}
